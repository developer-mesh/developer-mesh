package webhook

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/developer-mesh/developer-mesh/pkg/observability"
)

// Common storage errors
var (
	ErrNotFound = errors.New("storage: key not found")
)

// S3Config contains configuration for S3 storage
type S3Config struct {
	Bucket       string
	Region       string
	Endpoint     string // For S3-compatible services
	UsePathStyle bool   // For MinIO and other S3-compatible services
}

// S3StorageBackend implements cold storage using S3
type S3StorageBackend struct {
	config *S3Config
	client *s3.S3
	logger observability.Logger
}

// NewS3StorageBackend creates a new S3 storage backend
func NewS3StorageBackend(config *S3Config, logger observability.Logger) (*S3StorageBackend, error) {
	awsConfig := &aws.Config{
		Region: aws.String(config.Region),
	}

	if config.Endpoint != "" {
		awsConfig.Endpoint = aws.String(config.Endpoint)
		awsConfig.S3ForcePathStyle = aws.Bool(config.UsePathStyle)
	}

	sess, err := session.NewSession(awsConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create AWS session: %w", err)
	}

	return &S3StorageBackend{
		config: config,
		client: s3.New(sess),
		logger: logger,
	}, nil
}

// Store stores data in S3
func (s *S3StorageBackend) Store(ctx context.Context, key string, data []byte) error {
	input := &s3.PutObjectInput{
		Bucket: aws.String(s.config.Bucket),
		Key:    aws.String(key),
		Body:   bytes.NewReader(data),

		// Add metadata
		Metadata: map[string]*string{
			"stored-by": aws.String("webhook-pipeline"),
			"timestamp": aws.String(fmt.Sprintf("%d", time.Now().Unix())),
		},

		// Use intelligent tiering for cost optimization
		StorageClass: aws.String("INTELLIGENT_TIERING"),
	}

	_, err := s.client.PutObjectWithContext(ctx, input)
	if err != nil {
		return fmt.Errorf("failed to store object in S3: %w", err)
	}

	s.logger.Debug("Stored object in S3", map[string]interface{}{
		"bucket": s.config.Bucket,
		"key":    key,
		"size":   len(data),
	})

	return nil
}

// Retrieve retrieves data from S3
func (s *S3StorageBackend) Retrieve(ctx context.Context, key string) ([]byte, error) {
	input := &s3.GetObjectInput{
		Bucket: aws.String(s.config.Bucket),
		Key:    aws.String(key),
	}

	result, err := s.client.GetObjectWithContext(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve object from S3: %w", err)
	}
	defer func() { _ = result.Body.Close() }()

	data, err := io.ReadAll(result.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read object data: %w", err)
	}

	s.logger.Debug("Retrieved object from S3", map[string]interface{}{
		"bucket": s.config.Bucket,
		"key":    key,
		"size":   len(data),
	})

	return data, nil
}

// Delete deletes data from S3
func (s *S3StorageBackend) Delete(ctx context.Context, key string) error {
	input := &s3.DeleteObjectInput{
		Bucket: aws.String(s.config.Bucket),
		Key:    aws.String(key),
	}

	_, err := s.client.DeleteObjectWithContext(ctx, input)
	if err != nil {
		return fmt.Errorf("failed to delete object from S3: %w", err)
	}

	s.logger.Debug("Deleted object from S3", map[string]interface{}{
		"bucket": s.config.Bucket,
		"key":    key,
	})

	return nil
}

// List lists objects with a given prefix
func (s *S3StorageBackend) List(ctx context.Context, prefix string) ([]string, error) {
	input := &s3.ListObjectsV2Input{
		Bucket: aws.String(s.config.Bucket),
		Prefix: aws.String(prefix),
	}

	var keys []string
	err := s.client.ListObjectsV2PagesWithContext(ctx, input,
		func(page *s3.ListObjectsV2Output, lastPage bool) bool {
			for _, obj := range page.Contents {
				keys = append(keys, *obj.Key)
			}
			return !lastPage
		})

	if err != nil {
		return nil, fmt.Errorf("failed to list objects in S3: %w", err)
	}

	return keys, nil
}

// LocalStorageBackend implements a local file system storage backend for testing
type LocalStorageBackend struct {
	basePath string
	logger   observability.Logger
}

// NewLocalStorageBackend creates a new local storage backend
func NewLocalStorageBackend(basePath string, logger observability.Logger) *LocalStorageBackend {
	return &LocalStorageBackend{
		basePath: basePath,
		logger:   logger,
	}
}

// Store stores data locally
func (l *LocalStorageBackend) Store(ctx context.Context, key string, data []byte) error {
	path := filepath.Join(l.basePath, key)

	// Create directory if needed
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Write file
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	l.logger.Debug("Stored file locally", map[string]interface{}{
		"path": path,
		"size": len(data),
	})

	return nil
}

// Retrieve retrieves data from local storage
func (l *LocalStorageBackend) Retrieve(ctx context.Context, key string) ([]byte, error) {
	path := filepath.Join(l.basePath, key)

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	return data, nil
}

// Delete deletes data from local storage
func (l *LocalStorageBackend) Delete(ctx context.Context, key string) error {
	path := filepath.Join(l.basePath, key)

	if err := os.Remove(path); err != nil {
		return fmt.Errorf("failed to delete file: %w", err)
	}

	return nil
}

// List lists files with a given prefix
func (l *LocalStorageBackend) List(ctx context.Context, prefix string) ([]string, error) {
	searchPath := filepath.Join(l.basePath, prefix)

	var keys []string
	err := filepath.Walk(searchPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if !info.IsDir() {
			// Get relative path from base
			relPath, err := filepath.Rel(l.basePath, path)
			if err == nil {
				keys = append(keys, relPath)
			}
		}

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to list files: %w", err)
	}

	return keys, nil
}
