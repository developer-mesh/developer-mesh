package util

import (
	"database/sql/driver"
	"fmt"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// ParseUUID safely parses a UUID from various input types
func ParseUUID(input interface{}) (uuid.UUID, error) {
	switch v := input.(type) {
	case uuid.UUID:
		return v, nil
	case string:
		if v == "" {
			return uuid.Nil, fmt.Errorf("empty UUID string")
		}
		return uuid.Parse(v)
	case []byte:
		if len(v) == 16 {
			return uuid.FromBytes(v)
		}
		return uuid.Parse(string(v))
	case nil:
		return uuid.Nil, fmt.Errorf("nil UUID value")
	default:
		return uuid.Nil, fmt.Errorf("cannot parse UUID from type %T", input)
	}
}

// MustParseUUID parses a UUID and panics on error (only for tests/known values)
func MustParseUUID(input interface{}) uuid.UUID {
	id, err := ParseUUID(input)
	if err != nil {
		panic(fmt.Sprintf("failed to parse UUID: %v", err))
	}
	return id
}

// GetUUIDFromGinContext safely retrieves a UUID from Gin context
func GetUUIDFromGinContext(c *gin.Context, key string) (uuid.UUID, error) {
	val, exists := c.Get(key)
	if !exists {
		return uuid.Nil, fmt.Errorf("%s not found in context", key)
	}

	return ParseUUID(val)
}

// GetTenantIDFromGinContext is a specialized helper for tenant IDs
func GetTenantIDFromGinContext(c *gin.Context) (uuid.UUID, error) {
	return GetUUIDFromGinContext(c, "tenant_id")
}

// GetUserIDFromGinContext is a specialized helper for user IDs
func GetUserIDFromGinContext(c *gin.Context) (uuid.UUID, error) {
	return GetUUIDFromGinContext(c, "user_id")
}

// SetUUIDsInGinContext safely sets UUID values in context
func SetUUIDsInGinContext(c *gin.Context, userID, tenantID uuid.UUID) {
	c.Set("user_id", userID)
	c.Set("tenant_id", tenantID)
}

// NullableUUID handles nullable UUID fields for database operations
type NullableUUID struct {
	UUID  uuid.UUID
	Valid bool
}

// Scan implements sql.Scanner interface
func (n *NullableUUID) Scan(value interface{}) error {
	if value == nil {
		n.UUID = uuid.Nil
		n.Valid = false
		return nil
	}

	id, err := ParseUUID(value)
	if err != nil {
		return err
	}

	n.UUID = id
	n.Valid = (id != uuid.Nil)
	return nil
}

// Value implements driver.Valuer interface
func (n NullableUUID) Value() (driver.Value, error) {
	if !n.Valid || n.UUID == uuid.Nil {
		return nil, nil
	}
	return n.UUID.String(), nil
}

// ValidateUUID checks if a string is a valid UUID
func ValidateUUID(s string) error {
	if s == "" {
		return fmt.Errorf("UUID cannot be empty")
	}

	_, err := uuid.Parse(s)
	if err != nil {
		return fmt.Errorf("invalid UUID format: %w", err)
	}

	return nil
}

// UUIDSliceFromStrings converts string slice to UUID slice
func UUIDSliceFromStrings(strings []string) ([]uuid.UUID, error) {
	uuids := make([]uuid.UUID, 0, len(strings))
	for _, s := range strings {
		id, err := uuid.Parse(s)
		if err != nil {
			return nil, fmt.Errorf("invalid UUID at position %d: %w", len(uuids), err)
		}
		uuids = append(uuids, id)
	}
	return uuids, nil
}

// StringSliceFromUUIDs converts UUID slice to string slice
func StringSliceFromUUIDs(uuids []uuid.UUID) []string {
	strings := make([]string, len(uuids))
	for i, id := range uuids {
		strings[i] = id.String()
	}
	return strings
}
