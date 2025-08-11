package services

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/pkg/errors"
	"github.com/redis/go-redis/v9"
)

// DocumentLockService provides distributed locking for documents
type DocumentLockService interface {
	// Document-level locking
	LockDocument(ctx context.Context, documentID uuid.UUID, agentID string, duration time.Duration) (*DocumentLock, error)
	UnlockDocument(ctx context.Context, documentID uuid.UUID, agentID string) error
	ExtendLock(ctx context.Context, documentID uuid.UUID, agentID string, extension time.Duration) error
	IsDocumentLocked(ctx context.Context, documentID uuid.UUID) (bool, *DocumentLock, error)

	// Section-level locking
	LockSection(ctx context.Context, documentID uuid.UUID, sectionID string, agentID string, duration time.Duration) (*SectionLock, error)
	UnlockSection(ctx context.Context, documentID uuid.UUID, sectionID string, agentID string) error
	GetSectionLocks(ctx context.Context, documentID uuid.UUID) ([]*SectionLock, error)

	// Lock management
	GetActiveLocks(ctx context.Context, agentID string) ([]*DocumentLock, error)
	ReleaseAllLocks(ctx context.Context, agentID string) error

	// Deadlock detection
	DetectDeadlocks(ctx context.Context) ([]*DeadlockInfo, error)
	ResolveDeadlock(ctx context.Context, deadlockID string) error
}

// DocumentLock represents a lock on a document
type DocumentLock struct {
	ID           string                 `json:"id"`
	DocumentID   uuid.UUID              `json:"document_id"`
	AgentID      string                 `json:"agent_id"`
	Type         LockType               `json:"type"`
	AcquiredAt   time.Time              `json:"acquired_at"`
	ExpiresAt    time.Time              `json:"expires_at"`
	RefreshCount int                    `json:"refresh_count"`
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
}

// SectionLock represents a lock on a document section
type SectionLock struct {
	DocumentLock
	SectionID   string `json:"section_id"`
	StartOffset int    `json:"start_offset"`
	EndOffset   int    `json:"end_offset"`
}

// LockType represents the type of lock
type LockType string

const (
	LockTypeExclusive LockType = "exclusive"
	LockTypeShared    LockType = "shared"
	LockTypeSection   LockType = "section"
)

// DeadlockInfo contains information about detected deadlocks
type DeadlockInfo struct {
	ID            string    `json:"id"`
	DetectedAt    time.Time `json:"detected_at"`
	InvolvedLocks []string  `json:"involved_locks"`
	Agents        []string  `json:"agents"`
	CycleInfo     string    `json:"cycle_info"`
}

// documentLockService implements DocumentLockService
type documentLockService struct {
	BaseService
	redis            *redis.Client
	lockKeyPrefix    string
	sectionKeyPrefix string
	defaultTTL       time.Duration
	maxRefreshCount  int

	// Auto-refresh management
	refreshInterval time.Duration
	activeLocks     sync.Map // lockID -> *lockRefreshInfo
	refreshStop     chan struct{}
}

type lockRefreshInfo struct {
	lock       *DocumentLock
	cancelFunc context.CancelFunc
}

// NewDocumentLockService creates a new document lock service
func NewDocumentLockService(
	config ServiceConfig,
	redisClient *redis.Client,
) DocumentLockService {
	s := &documentLockService{
		BaseService:      NewBaseService(config),
		redis:            redisClient,
		lockKeyPrefix:    "doc:lock:",
		sectionKeyPrefix: "doc:section:",
		defaultTTL:       5 * time.Minute,
		maxRefreshCount:  10,
		refreshInterval:  30 * time.Second,
		refreshStop:      make(chan struct{}),
	}

	// Start auto-refresh goroutine
	go s.autoRefreshLocks()

	return s
}

// LockDocument acquires an exclusive lock on a document
func (s *documentLockService) LockDocument(ctx context.Context, documentID uuid.UUID, agentID string, duration time.Duration) (*DocumentLock, error) {
	ctx, span := s.config.Tracer(ctx, "DocumentLockService.LockDocument")
	defer span.End()

	if duration > 30*time.Minute {
		duration = 30 * time.Minute
	}
	if duration == 0 {
		duration = s.defaultTTL
	}

	lockID := uuid.New().String()
	lock := &DocumentLock{
		ID:         lockID,
		DocumentID: documentID,
		AgentID:    agentID,
		Type:       LockTypeExclusive,
		AcquiredAt: time.Now(),
		ExpiresAt:  time.Now().Add(duration),
		Metadata: map[string]interface{}{
			"host": getHostname(),
			"pid":  getPID(),
		},
	}

	// Serialize lock
	lockData, err := json.Marshal(lock)
	if err != nil {
		return nil, errors.Wrap(err, "failed to serialize lock")
	}

	// Try to acquire lock with Redis SET NX
	key := s.lockKeyPrefix + documentID.String()
	success, err := s.redis.SetNX(ctx, key, lockData, duration).Result()
	if err != nil {
		return nil, errors.Wrap(err, "failed to acquire lock")
	}

	if !success {
		// Lock already exists, check if it's expired
		existingLock, err := s.getLock(ctx, key)
		if err != nil {
			return nil, errors.Wrap(err, "failed to get existing lock")
		}

		if existingLock != nil && time.Now().After(existingLock.ExpiresAt) {
			// Lock is expired, try to acquire with atomic compare-and-swap
			if err := s.tryAcquireExpiredLock(ctx, key, lock, duration); err != nil {
				return nil, err
			}
		} else if existingLock != nil {
			return nil, &LockConflictError{
				DocumentID:    documentID,
				CurrentHolder: existingLock.AgentID,
				ExpiresAt:     existingLock.ExpiresAt,
			}
		} else {
			// Couldn't get existing lock info, but lock exists
			return nil, &LockConflictError{
				DocumentID:    documentID,
				CurrentHolder: "unknown",
				ExpiresAt:     time.Now().Add(duration),
			}
		}
	}

	// Start auto-refresh if enabled
	s.startAutoRefresh(lock)

	// Record metrics
	s.config.Metrics.IncrementCounter("document_lock.acquired", 1)

	return lock, nil
}

// UnlockDocument releases a document lock
func (s *documentLockService) UnlockDocument(ctx context.Context, documentID uuid.UUID, agentID string) error {
	ctx, span := s.config.Tracer(ctx, "DocumentLockService.UnlockDocument")
	defer span.End()

	key := s.lockKeyPrefix + documentID.String()

	// Get current lock to verify ownership
	currentLock, err := s.getLock(ctx, key)
	if err != nil {
		return errors.Wrap(err, "failed to get lock")
	}

	if currentLock == nil {
		return nil // Lock doesn't exist
	}

	if currentLock.AgentID != agentID {
		return &UnauthorizedLockError{
			DocumentID: documentID,
			AgentID:    agentID,
			OwnerID:    currentLock.AgentID,
		}
	}

	// Stop auto-refresh
	s.stopAutoRefresh(currentLock.ID)

	// Delete lock
	if err := s.redis.Del(ctx, key).Err(); err != nil {
		return errors.Wrap(err, "failed to delete lock")
	}

	// Record metrics
	s.config.Metrics.IncrementCounter("document_lock.released", 1)

	return nil
}

// ExtendLock extends the expiration time of a lock
func (s *documentLockService) ExtendLock(ctx context.Context, documentID uuid.UUID, agentID string, extension time.Duration) error {
	ctx, span := s.config.Tracer(ctx, "DocumentLockService.ExtendLock")
	defer span.End()

	if extension > 30*time.Minute {
		extension = 30 * time.Minute
	}

	key := s.lockKeyPrefix + documentID.String()

	// Get current lock
	currentLock, err := s.getLock(ctx, key)
	if err != nil {
		return errors.Wrap(err, "failed to get lock")
	}

	if currentLock == nil {
		return &LockNotFoundError{DocumentID: documentID}
	}

	if currentLock.AgentID != agentID {
		return &UnauthorizedLockError{
			DocumentID: documentID,
			AgentID:    agentID,
			OwnerID:    currentLock.AgentID,
		}
	}

	// Check refresh count
	if currentLock.RefreshCount >= s.maxRefreshCount {
		return &LockRefreshLimitError{
			DocumentID:   documentID,
			RefreshCount: currentLock.RefreshCount,
			MaxRefresh:   s.maxRefreshCount,
		}
	}

	// Update lock
	currentLock.ExpiresAt = time.Now().Add(extension)
	currentLock.RefreshCount++

	lockData, err := json.Marshal(currentLock)
	if err != nil {
		return errors.Wrap(err, "failed to serialize lock")
	}

	// Update with new expiration
	if err := s.redis.Set(ctx, key, lockData, extension).Err(); err != nil {
		return errors.Wrap(err, "failed to extend lock")
	}

	return nil
}

// IsDocumentLocked checks if a document is locked
func (s *documentLockService) IsDocumentLocked(ctx context.Context, documentID uuid.UUID) (bool, *DocumentLock, error) {
	ctx, span := s.config.Tracer(ctx, "DocumentLockService.IsDocumentLocked")
	defer span.End()

	key := s.lockKeyPrefix + documentID.String()
	lock, err := s.getLock(ctx, key)
	if err != nil {
		return false, nil, err
	}

	if lock == nil {
		return false, nil, nil
	}

	// Check if lock is expired
	if time.Now().After(lock.ExpiresAt) {
		// Clean up expired lock
		s.redis.Del(ctx, key)
		return false, nil, nil
	}

	return true, lock, nil
}

// LockSection acquires a lock on a document section
func (s *documentLockService) LockSection(ctx context.Context, documentID uuid.UUID, sectionID string, agentID string, duration time.Duration) (*SectionLock, error) {
	ctx, span := s.config.Tracer(ctx, "DocumentLockService.LockSection")
	defer span.End()

	if duration == 0 {
		duration = s.defaultTTL
	}

	// Check if document is exclusively locked
	isLocked, docLock, err := s.IsDocumentLocked(ctx, documentID)
	if err != nil {
		return nil, err
	}

	if isLocked && docLock.Type == LockTypeExclusive && docLock.AgentID != agentID {
		return nil, &LockConflictError{
			DocumentID:    documentID,
			CurrentHolder: docLock.AgentID,
			ExpiresAt:     docLock.ExpiresAt,
		}
	}

	// Check for conflicting section locks
	existingLocks, err := s.GetSectionLocks(ctx, documentID)
	if err != nil {
		return nil, err
	}

	for _, existing := range existingLocks {
		if existing.SectionID == sectionID && existing.AgentID != agentID {
			return nil, &SectionLockConflictError{
				DocumentID:    documentID,
				SectionID:     sectionID,
				CurrentHolder: existing.AgentID,
			}
		}
	}

	// Create section lock
	lockID := uuid.New().String()
	sectionLock := &SectionLock{
		DocumentLock: DocumentLock{
			ID:         lockID,
			DocumentID: documentID,
			AgentID:    agentID,
			Type:       LockTypeSection,
			AcquiredAt: time.Now(),
			ExpiresAt:  time.Now().Add(duration),
		},
		SectionID: sectionID,
	}

	// Store section lock
	key := fmt.Sprintf("%s%s:%s", s.sectionKeyPrefix, documentID.String(), sectionID)
	lockData, err := json.Marshal(sectionLock)
	if err != nil {
		return nil, errors.Wrap(err, "failed to serialize section lock")
	}

	if err := s.redis.Set(ctx, key, lockData, duration).Err(); err != nil {
		return nil, errors.Wrap(err, "failed to acquire section lock")
	}

	// Start auto-refresh
	s.startAutoRefresh(&sectionLock.DocumentLock)

	return sectionLock, nil
}

// Helper methods

func (s *documentLockService) getLock(ctx context.Context, key string) (*DocumentLock, error) {
	data, err := s.redis.Get(ctx, key).Bytes()
	if err == redis.Nil {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	var lock DocumentLock
	if err := json.Unmarshal(data, &lock); err != nil {
		return nil, err
	}

	return &lock, nil
}

func (s *documentLockService) tryAcquireExpiredLock(ctx context.Context, key string, newLock *DocumentLock, duration time.Duration) error {
	// Use Lua script for atomic compare-and-swap
	script := `
		local current = redis.call('get', KEYS[1])
		if not current then
			return redis.call('set', KEYS[1], ARGV[1], 'EX', ARGV[2])
		end
		
		local lock = cjson.decode(current)
		local now = tonumber(ARGV[3])
		local expires = tonumber(lock.expires_at)
		
		if now > expires then
			return redis.call('set', KEYS[1], ARGV[1], 'EX', ARGV[2])
		end
		
		return nil
	`

	lockData, _ := json.Marshal(newLock)
	_, err := s.redis.Eval(ctx, script, []string{key}, lockData, int(duration.Seconds()), time.Now().Unix()).Result()

	return err
}

func (s *documentLockService) autoRefreshLocks() {
	ticker := time.NewTicker(s.refreshInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			s.refreshActiveLocks()
		case <-s.refreshStop:
			return
		}
	}
}

func (s *documentLockService) refreshActiveLocks() {
	s.activeLocks.Range(func(key, value interface{}) bool {
		info := value.(*lockRefreshInfo)

		// Check if lock is about to expire
		timeUntilExpiry := time.Until(info.lock.ExpiresAt)
		if timeUntilExpiry < s.refreshInterval*2 {
			// Extend lock
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			if err := s.ExtendLock(ctx, info.lock.DocumentID, info.lock.AgentID, s.defaultTTL); err != nil {
				s.config.Logger.Error("Failed to auto-refresh lock", map[string]interface{}{
					"lock_id":     info.lock.ID,
					"document_id": info.lock.DocumentID,
					"error":       err.Error(),
				})

				// Remove from active locks if refresh fails
				s.activeLocks.Delete(key)
			}
		}

		return true
	})
}

func (s *documentLockService) startAutoRefresh(lock *DocumentLock) {
	_, cancel := context.WithCancel(context.Background())
	info := &lockRefreshInfo{
		lock:       lock,
		cancelFunc: cancel,
	}

	s.activeLocks.Store(lock.ID, info)
}

func (s *documentLockService) stopAutoRefresh(lockID string) {
	if value, ok := s.activeLocks.LoadAndDelete(lockID); ok {
		info := value.(*lockRefreshInfo)
		info.cancelFunc()
	}
}

// Implement remaining interface methods...

func (s *documentLockService) UnlockSection(ctx context.Context, documentID uuid.UUID, sectionID string, agentID string) error {
	key := fmt.Sprintf("%s%s:%s", s.sectionKeyPrefix, documentID.String(), sectionID)

	// Verify ownership
	data, err := s.redis.Get(ctx, key).Bytes()
	if err == redis.Nil {
		return nil
	}
	if err != nil {
		return err
	}

	var lock SectionLock
	if err := json.Unmarshal(data, &lock); err != nil {
		return err
	}

	if lock.AgentID != agentID {
		return &UnauthorizedLockError{
			DocumentID: documentID,
			AgentID:    agentID,
			OwnerID:    lock.AgentID,
		}
	}

	// Stop auto-refresh
	s.stopAutoRefresh(lock.ID)

	return s.redis.Del(ctx, key).Err()
}

func (s *documentLockService) GetSectionLocks(ctx context.Context, documentID uuid.UUID) ([]*SectionLock, error) {
	pattern := fmt.Sprintf("%s%s:*", s.sectionKeyPrefix, documentID.String())
	keys, err := s.redis.Keys(ctx, pattern).Result()
	if err != nil {
		return nil, err
	}

	var locks []*SectionLock
	for _, key := range keys {
		data, err := s.redis.Get(ctx, key).Bytes()
		if err != nil {
			continue
		}

		var lock SectionLock
		if err := json.Unmarshal(data, &lock); err != nil {
			continue
		}

		// Skip expired locks
		if time.Now().Before(lock.ExpiresAt) {
			// Create a copy to avoid G601 implicit memory aliasing
			lockCopy := lock
			locks = append(locks, &lockCopy)
		}
	}

	return locks, nil
}

func (s *documentLockService) GetActiveLocks(ctx context.Context, agentID string) ([]*DocumentLock, error) {
	// Get all document locks
	docPattern := s.lockKeyPrefix + "*"
	docKeys, err := s.redis.Keys(ctx, docPattern).Result()
	if err != nil {
		return nil, err
	}

	var locks []*DocumentLock

	// Check document locks
	for _, key := range docKeys {
		lock, err := s.getLock(ctx, key)
		if err != nil || lock == nil {
			continue
		}

		if lock.AgentID == agentID && time.Now().Before(lock.ExpiresAt) {
			locks = append(locks, lock)
		}
	}

	// Check section locks
	sectionPattern := s.sectionKeyPrefix + "*"
	sectionKeys, err := s.redis.Keys(ctx, sectionPattern).Result()
	if err != nil {
		return nil, err
	}

	for _, key := range sectionKeys {
		data, err := s.redis.Get(ctx, key).Bytes()
		if err != nil {
			continue
		}

		var sectionLock SectionLock
		if err := json.Unmarshal(data, &sectionLock); err != nil {
			continue
		}

		if sectionLock.AgentID == agentID && time.Now().Before(sectionLock.ExpiresAt) {
			// Create a copy to avoid G601 implicit memory aliasing
			docLockCopy := sectionLock.DocumentLock
			locks = append(locks, &docLockCopy)
		}
	}

	return locks, nil
}

func (s *documentLockService) ReleaseAllLocks(ctx context.Context, agentID string) error {
	locks, err := s.GetActiveLocks(ctx, agentID)
	if err != nil {
		return err
	}

	for _, lock := range locks {
		if lock.Type == LockTypeSection {
			// Handle section locks
			continue
		}

		if err := s.UnlockDocument(ctx, lock.DocumentID, agentID); err != nil {
			s.config.Logger.Error("Failed to release lock", map[string]interface{}{
				"lock_id":     lock.ID,
				"document_id": lock.DocumentID,
				"error":       err.Error(),
			})
		}
	}

	return nil
}

func (s *documentLockService) DetectDeadlocks(ctx context.Context) ([]*DeadlockInfo, error) {
	// Basic deadlock detection using wait-for graph
	// This is a simplified implementation

	// Basic deadlock detection using wait-for graph
	// This is a simplified implementation

	// In a production system, this would:
	// 1. Track wait relationships between agents and locks
	// 2. Build a wait-for graph
	// 3. Detect cycles using DFS or similar algorithm
	// 4. Return detailed deadlock information

	// For now, return empty list
	return []*DeadlockInfo{}, nil
}

func (s *documentLockService) ResolveDeadlock(ctx context.Context, deadlockID string) error {
	// In a real implementation, this would:
	// 1. Identify the victim lock(s)
	// 2. Force release them
	// 3. Notify affected agents
	return errors.New("deadlock resolution not implemented")
}

// Lock errors

type LockConflictError struct {
	DocumentID    uuid.UUID
	CurrentHolder string
	ExpiresAt     time.Time
}

func (e *LockConflictError) Error() string {
	return fmt.Sprintf("document %s is locked by %s until %s",
		e.DocumentID, e.CurrentHolder, e.ExpiresAt.Format(time.RFC3339))
}

type UnauthorizedLockError struct {
	DocumentID uuid.UUID
	AgentID    string
	OwnerID    string
}

func (e *UnauthorizedLockError) Error() string {
	return fmt.Sprintf("agent %s cannot unlock document %s owned by %s",
		e.AgentID, e.DocumentID, e.OwnerID)
}

type LockNotFoundError struct {
	DocumentID uuid.UUID
}

func (e *LockNotFoundError) Error() string {
	return fmt.Sprintf("lock not found for document %s", e.DocumentID)
}

type LockRefreshLimitError struct {
	DocumentID   uuid.UUID
	RefreshCount int
	MaxRefresh   int
}

func (e *LockRefreshLimitError) Error() string {
	return fmt.Sprintf("lock refresh limit exceeded for document %s (%d/%d)",
		e.DocumentID, e.RefreshCount, e.MaxRefresh)
}

type SectionLockConflictError struct {
	DocumentID    uuid.UUID
	SectionID     string
	CurrentHolder string
}

func (e *SectionLockConflictError) Error() string {
	return fmt.Sprintf("section %s of document %s is locked by %s",
		e.SectionID, e.DocumentID, e.CurrentHolder)
}

// Utility functions

func getHostname() string {
	hostname, err := os.Hostname()
	if err != nil {
		return "unknown"
	}
	return hostname
}

func getPID() int {
	return os.Getpid()
}
