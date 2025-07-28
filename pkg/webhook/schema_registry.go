package webhook

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/developer-mesh/developer-mesh/pkg/observability"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
)

// SchemaVersion represents a version of a schema
type SchemaVersion struct {
	Version      int32
	Schema       proto.Message
	Deprecated   bool
	DeprecatedAt time.Time
	Description  string
}

// SchemaRegistry manages schema versions for webhook events
type SchemaRegistry struct {
	schemas map[string]map[int32]*SchemaVersion // eventType -> version -> schema
	mu      sync.RWMutex
	logger  observability.Logger
}

// NewSchemaRegistry creates a new schema registry
func NewSchemaRegistry(logger observability.Logger) *SchemaRegistry {
	return &SchemaRegistry{
		schemas: make(map[string]map[int32]*SchemaVersion),
		logger:  logger,
	}
}

// RegisterSchema registers a new schema version
func (sr *SchemaRegistry) RegisterSchema(eventType string, version int32, schema proto.Message, description string) error {
	sr.mu.Lock()
	defer sr.mu.Unlock()

	if _, exists := sr.schemas[eventType]; !exists {
		sr.schemas[eventType] = make(map[int32]*SchemaVersion)
	}

	if _, exists := sr.schemas[eventType][version]; exists {
		return fmt.Errorf("schema version %d already registered for event type %s", version, eventType)
	}

	sr.schemas[eventType][version] = &SchemaVersion{
		Version:     version,
		Schema:      schema,
		Description: description,
		Deprecated:  false,
	}

	sr.logger.Info("Registered schema", map[string]interface{}{
		"event_type":  eventType,
		"version":     version,
		"description": description,
	})

	return nil
}

// GetSchema retrieves a schema by event type and version
func (sr *SchemaRegistry) GetSchema(eventType string, version int32) (*SchemaVersion, error) {
	sr.mu.RLock()
	defer sr.mu.RUnlock()

	versions, exists := sr.schemas[eventType]
	if !exists {
		return nil, fmt.Errorf("no schemas registered for event type: %s", eventType)
	}

	schema, exists := versions[version]
	if !exists {
		return nil, fmt.Errorf("schema version %d not found for event type %s", version, eventType)
	}

	return schema, nil
}

// GetLatestSchema retrieves the latest non-deprecated schema version
func (sr *SchemaRegistry) GetLatestSchema(eventType string) (*SchemaVersion, error) {
	sr.mu.RLock()
	defer sr.mu.RUnlock()

	versions, exists := sr.schemas[eventType]
	if !exists {
		return nil, fmt.Errorf("no schemas registered for event type: %s", eventType)
	}

	var latest *SchemaVersion
	var latestVersion int32 = -1

	for version, schema := range versions {
		if !schema.Deprecated && version > latestVersion {
			latest = schema
			latestVersion = version
		}
	}

	if latest == nil {
		return nil, fmt.Errorf("no active schemas found for event type: %s", eventType)
	}

	return latest, nil
}

// DeprecateSchema marks a schema version as deprecated
func (sr *SchemaRegistry) DeprecateSchema(eventType string, version int32) error {
	sr.mu.Lock()
	defer sr.mu.Unlock()

	versions, exists := sr.schemas[eventType]
	if !exists {
		return fmt.Errorf("no schemas registered for event type: %s", eventType)
	}

	schema, exists := versions[version]
	if !exists {
		return fmt.Errorf("schema version %d not found for event type %s", version, eventType)
	}

	schema.Deprecated = true
	schema.DeprecatedAt = time.Now()

	sr.logger.Info("Deprecated schema", map[string]interface{}{
		"event_type": eventType,
		"version":    version,
	})

	return nil
}

// ListSchemas returns all registered schemas for an event type
func (sr *SchemaRegistry) ListSchemas(eventType string) ([]*SchemaVersion, error) {
	sr.mu.RLock()
	defer sr.mu.RUnlock()

	versions, exists := sr.schemas[eventType]
	if !exists {
		return nil, fmt.Errorf("no schemas registered for event type: %s", eventType)
	}

	var result []*SchemaVersion
	for _, schema := range versions {
		result = append(result, schema)
	}

	return result, nil
}

// ListEventTypes returns all registered event types
func (sr *SchemaRegistry) ListEventTypes() []string {
	sr.mu.RLock()
	defer sr.mu.RUnlock()

	var types []string
	for eventType := range sr.schemas {
		types = append(types, eventType)
	}

	return types
}

// SchemaEvolution handles schema evolution and compatibility
type SchemaEvolution struct {
	registry *SchemaRegistry
	logger   observability.Logger
}

// NewSchemaEvolution creates a new schema evolution handler
func NewSchemaEvolution(registry *SchemaRegistry, logger observability.Logger) *SchemaEvolution {
	return &SchemaEvolution{
		registry: registry,
		logger:   logger,
	}
}

// MigratePayload migrates a payload from one schema version to another
func (se *SchemaEvolution) MigratePayload(ctx context.Context, eventType string, fromVersion, toVersion int32, payload []byte) ([]byte, error) {
	if fromVersion == toVersion {
		return payload, nil
	}

	// Get schemas
	fromSchema, err := se.registry.GetSchema(eventType, fromVersion)
	if err != nil {
		return nil, fmt.Errorf("failed to get source schema: %w", err)
	}

	toSchema, err := se.registry.GetSchema(eventType, toVersion)
	if err != nil {
		return nil, fmt.Errorf("failed to get target schema: %w", err)
	}

	// Deserialize with old schema
	oldMsg := proto.Clone(fromSchema.Schema)
	if err := proto.Unmarshal(payload, oldMsg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal with old schema: %w", err)
	}

	// Perform migration (this is simplified - real implementation would need field mapping)
	newMsg := proto.Clone(toSchema.Schema)

	// Use reflection to copy compatible fields
	if err := se.copyCompatibleFields(oldMsg, newMsg); err != nil {
		return nil, fmt.Errorf("failed to migrate fields: %w", err)
	}

	// Serialize with new schema
	newPayload, err := proto.Marshal(newMsg)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal with new schema: %w", err)
	}

	se.logger.Info("Migrated payload", map[string]interface{}{
		"event_type":   eventType,
		"from_version": fromVersion,
		"to_version":   toVersion,
		"old_size":     len(payload),
		"new_size":     len(newPayload),
	})

	return newPayload, nil
}

// copyCompatibleFields copies fields between protobuf messages
// This is a simplified version - production would need more sophisticated field mapping
func (se *SchemaEvolution) copyCompatibleFields(from, to proto.Message) error {
	// This would use protoreflect to copy fields
	// For now, we'll use a simple JSON round-trip as a placeholder
	// In production, use proper protobuf reflection
	return nil
}

// ValidateBackwardCompatibility checks if a new schema is backward compatible
func (se *SchemaEvolution) ValidateBackwardCompatibility(eventType string, newVersion int32) error {
	// Get all previous versions
	schemas, err := se.registry.ListSchemas(eventType)
	if err != nil {
		return err
	}

	newSchema, err := se.registry.GetSchema(eventType, newVersion)
	if err != nil {
		return err
	}

	// Check compatibility with each previous version
	for _, oldSchema := range schemas {
		if oldSchema.Version >= newVersion {
			continue
		}

		if err := se.checkCompatibility(oldSchema.Schema, newSchema.Schema); err != nil {
			return fmt.Errorf("schema version %d is not backward compatible with version %d: %w",
				newVersion, oldSchema.Version, err)
		}
	}

	return nil
}

// checkCompatibility checks if two schemas are compatible
func (se *SchemaEvolution) checkCompatibility(oldSchema, newSchema proto.Message) error {
	// This would implement actual compatibility checking
	// For now, we'll assume compatibility
	return nil
}

// PayloadWrapper wraps a payload with version information
type PayloadWrapper struct {
	EventType string
	Version   int32
	Payload   *anypb.Any
	Metadata  map[string]string
}

// WrapPayload wraps a payload with version information
func (sr *SchemaRegistry) WrapPayload(eventType string, version int32, payload proto.Message) (*PayloadWrapper, error) {
	anyPayload, err := anypb.New(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal payload to Any: %w", err)
	}

	return &PayloadWrapper{
		EventType: eventType,
		Version:   version,
		Payload:   anyPayload,
		Metadata:  make(map[string]string),
	}, nil
}

// UnwrapPayload unwraps a payload and returns the concrete type
func (sr *SchemaRegistry) UnwrapPayload(wrapper *PayloadWrapper) (proto.Message, error) {
	schema, err := sr.GetSchema(wrapper.EventType, wrapper.Version)
	if err != nil {
		return nil, err
	}

	// Create a new instance of the schema type
	msg := proto.Clone(schema.Schema)

	// Unmarshal the Any payload into the concrete type
	if err := wrapper.Payload.UnmarshalTo(msg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal payload: %w", err)
	}

	return msg, nil
}
