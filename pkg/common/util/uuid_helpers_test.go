package util

import (
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseUUID(t *testing.T) {
	validUUID := uuid.New()
	validUUIDString := validUUID.String()

	tests := []struct {
		name      string
		input     interface{}
		want      uuid.UUID
		wantError bool
		errorMsg  string
	}{
		{
			name:      "valid UUID type",
			input:     validUUID,
			want:      validUUID,
			wantError: false,
		},
		{
			name:      "valid UUID string",
			input:     validUUIDString,
			want:      validUUID,
			wantError: false,
		},
		{
			name:      "valid UUID bytes",
			input:     validUUID[:],
			want:      validUUID,
			wantError: false,
		},
		{
			name:      "empty string",
			input:     "",
			want:      uuid.Nil,
			wantError: true,
			errorMsg:  "empty UUID string",
		},
		{
			name:      "invalid UUID string",
			input:     "not-a-uuid",
			want:      uuid.Nil,
			wantError: true,
		},
		{
			name:      "nil input",
			input:     nil,
			want:      uuid.Nil,
			wantError: true,
			errorMsg:  "nil UUID value",
		},
		{
			name:      "unsupported type",
			input:     123,
			want:      uuid.Nil,
			wantError: true,
			errorMsg:  "cannot parse UUID from type int",
		},
		{
			name:      "nil UUID string",
			input:     "00000000-0000-0000-0000-000000000000",
			want:      uuid.Nil,
			wantError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseUUID(tt.input)

			if tt.wantError {
				require.Error(t, err)
				if tt.errorMsg != "" {
					assert.Contains(t, err.Error(), tt.errorMsg)
				}
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

func TestMustParseUUID(t *testing.T) {
	validUUID := uuid.New()

	t.Run("valid UUID", func(t *testing.T) {
		assert.NotPanics(t, func() {
			result := MustParseUUID(validUUID.String())
			assert.Equal(t, validUUID, result)
		})
	})

	t.Run("invalid UUID panics", func(t *testing.T) {
		assert.Panics(t, func() {
			MustParseUUID("not-a-uuid")
		})
	})
}

func TestGetUUIDFromGinContext(t *testing.T) {
	validUUID := uuid.New()

	tests := []struct {
		name      string
		setupCtx  func(*gin.Context)
		key       string
		want      uuid.UUID
		wantError bool
		errorMsg  string
	}{
		{
			name: "UUID in context",
			setupCtx: func(c *gin.Context) {
				c.Set("test_id", validUUID)
			},
			key:       "test_id",
			want:      validUUID,
			wantError: false,
		},
		{
			name: "string UUID in context",
			setupCtx: func(c *gin.Context) {
				c.Set("test_id", validUUID.String())
			},
			key:       "test_id",
			want:      validUUID,
			wantError: false,
		},
		{
			name: "key not found",
			setupCtx: func(c *gin.Context) {
				// Don't set anything
			},
			key:       "missing_id",
			want:      uuid.Nil,
			wantError: true,
			errorMsg:  "missing_id not found in context",
		},
		{
			name: "invalid type in context",
			setupCtx: func(c *gin.Context) {
				c.Set("test_id", 12345)
			},
			key:       "test_id",
			want:      uuid.Nil,
			wantError: true,
			errorMsg:  "cannot parse UUID from type int",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gin.SetMode(gin.TestMode)
			c, _ := gin.CreateTestContext(nil)
			tt.setupCtx(c)

			got, err := GetUUIDFromGinContext(c, tt.key)

			if tt.wantError {
				require.Error(t, err)
				if tt.errorMsg != "" {
					assert.Contains(t, err.Error(), tt.errorMsg)
				}
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

func TestGetTenantIDFromGinContext(t *testing.T) {
	tenantID := uuid.New()

	t.Run("valid tenant ID", func(t *testing.T) {
		gin.SetMode(gin.TestMode)
		c, _ := gin.CreateTestContext(nil)
		c.Set("tenant_id", tenantID)

		got, err := GetTenantIDFromGinContext(c)
		require.NoError(t, err)
		assert.Equal(t, tenantID, got)
	})

	t.Run("string tenant ID", func(t *testing.T) {
		gin.SetMode(gin.TestMode)
		c, _ := gin.CreateTestContext(nil)
		c.Set("tenant_id", tenantID.String())

		got, err := GetTenantIDFromGinContext(c)
		require.NoError(t, err)
		assert.Equal(t, tenantID, got)
	})

	t.Run("missing tenant ID", func(t *testing.T) {
		gin.SetMode(gin.TestMode)
		c, _ := gin.CreateTestContext(nil)

		_, err := GetTenantIDFromGinContext(c)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "tenant_id not found")
	})
}

func TestSetUUIDsInGinContext(t *testing.T) {
	userID := uuid.New()
	tenantID := uuid.New()

	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(nil)

	SetUUIDsInGinContext(c, userID, tenantID)

	// Verify values were set correctly
	gotUserID, exists := c.Get("user_id")
	assert.True(t, exists)
	assert.Equal(t, userID, gotUserID)

	gotTenantID, exists := c.Get("tenant_id")
	assert.True(t, exists)
	assert.Equal(t, tenantID, gotTenantID)
}

func TestNullableUUID(t *testing.T) {
	t.Run("scan valid UUID", func(t *testing.T) {
		validUUID := uuid.New()
		n := &NullableUUID{}

		err := n.Scan(validUUID.String())
		require.NoError(t, err)
		assert.True(t, n.Valid)
		assert.Equal(t, validUUID, n.UUID)
	})

	t.Run("scan nil", func(t *testing.T) {
		n := &NullableUUID{}

		err := n.Scan(nil)
		require.NoError(t, err)
		assert.False(t, n.Valid)
		assert.Equal(t, uuid.Nil, n.UUID)
	})

	t.Run("value with valid UUID", func(t *testing.T) {
		validUUID := uuid.New()
		n := NullableUUID{UUID: validUUID, Valid: true}

		val, err := n.Value()
		require.NoError(t, err)
		assert.Equal(t, validUUID.String(), val)
	})

	t.Run("value with nil UUID", func(t *testing.T) {
		n := NullableUUID{UUID: uuid.Nil, Valid: false}

		val, err := n.Value()
		require.NoError(t, err)
		assert.Nil(t, val)
	})
}

func TestValidateUUID(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantError bool
		errorMsg  string
	}{
		{
			name:      "valid UUID",
			input:     uuid.New().String(),
			wantError: false,
		},
		{
			name:      "empty string",
			input:     "",
			wantError: true,
			errorMsg:  "UUID cannot be empty",
		},
		{
			name:      "invalid format",
			input:     "not-a-uuid",
			wantError: true,
			errorMsg:  "invalid UUID format",
		},
		{
			name:      "nil UUID",
			input:     "00000000-0000-0000-0000-000000000000",
			wantError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateUUID(tt.input)

			if tt.wantError {
				require.Error(t, err)
				if tt.errorMsg != "" {
					assert.Contains(t, err.Error(), tt.errorMsg)
				}
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestUUIDSliceConversions(t *testing.T) {
	uuid1 := uuid.New()
	uuid2 := uuid.New()
	uuid3 := uuid.New()

	t.Run("UUIDSliceFromStrings valid", func(t *testing.T) {
		strings := []string{uuid1.String(), uuid2.String(), uuid3.String()}

		uuids, err := UUIDSliceFromStrings(strings)
		require.NoError(t, err)
		assert.Len(t, uuids, 3)
		assert.Equal(t, uuid1, uuids[0])
		assert.Equal(t, uuid2, uuids[1])
		assert.Equal(t, uuid3, uuids[2])
	})

	t.Run("UUIDSliceFromStrings with invalid", func(t *testing.T) {
		strings := []string{uuid1.String(), "invalid-uuid", uuid3.String()}

		_, err := UUIDSliceFromStrings(strings)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid UUID at position 1")
	})

	t.Run("StringSliceFromUUIDs", func(t *testing.T) {
		uuids := []uuid.UUID{uuid1, uuid2, uuid3}

		strings := StringSliceFromUUIDs(uuids)
		assert.Len(t, strings, 3)
		assert.Equal(t, uuid1.String(), strings[0])
		assert.Equal(t, uuid2.String(), strings[1])
		assert.Equal(t, uuid3.String(), strings[2])
	})
}
