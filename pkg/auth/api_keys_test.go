package auth

import (
	"context"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/developer-mesh/developer-mesh/pkg/observability"
	"github.com/developer-mesh/developer-mesh/pkg/testutil"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreateAPIKeyWithType(t *testing.T) {
	tests := []struct {
		name     string
		request  CreateAPIKeyRequest
		setupDB  func(sqlmock.Sqlmock)
		wantErr  bool
		validate func(*testing.T, *APIKey)
	}{
		{
			name: "create admin key with defaults",
			request: CreateAPIKeyRequest{
				Name:     "Admin Key",
				TenantID: testutil.TestTenantIDString(),
				KeyType:  KeyTypeAdmin,
			},
			setupDB: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`INSERT INTO mcp.api_keys`).
					WithArgs(
						sqlmock.AnyArg(), // key_hash
						sqlmock.AnyArg(), // key_prefix
						sqlmock.AnyArg(), // tenant_id
						nil,              // user_id
						"Admin Key",      // name
						KeyTypeAdmin,     // key_type
						sqlmock.AnyArg(), // scopes
						true,             // is_active
						nil,              // expires_at
						10000,            // rate_limit_requests
						60,               // rate_limit_window_seconds
						nil,              // parent_key_id
						sqlmock.AnyArg(), // allowed_services
						sqlmock.AnyArg(), // created_at/updated_at
					).
					WillReturnRows(sqlmock.NewRows([]string{"id", "created_at"}).
						AddRow("key-123", time.Now()))
			},
			validate: func(t *testing.T, key *APIKey) {
				assert.Equal(t, testutil.TestTenantID, key.TenantID)
				assert.Equal(t, KeyTypeAdmin, key.KeyType)
				assert.Equal(t, "Admin Key", key.Name)
				assert.True(t, key.Active)
				assert.Contains(t, key.Key, "adm_")
				assert.Equal(t, []string{"read", "write", "admin"}, key.Scopes)
				assert.Equal(t, 10000, key.RateLimitRequests)
			},
		},
		{
			name: "create gateway key with allowed services",
			request: CreateAPIKeyRequest{
				Name:            "Gateway Key",
				TenantID:        testutil.TestTenantIDString(),
				KeyType:         KeyTypeGateway,
				AllowedServices: []string{"github", "gitlab"},
			},
			setupDB: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`INSERT INTO mcp.api_keys`).
					WithArgs(
						sqlmock.AnyArg(), // key_hash
						sqlmock.AnyArg(), // key_prefix
						sqlmock.AnyArg(), // tenant_id
						nil,              // user_id
						"Gateway Key",    // name
						KeyTypeGateway,   // key_type
						sqlmock.AnyArg(), // scopes
						true,             // is_active
						nil,              // expires_at
						5000,             // rate_limit_requests
						60,               // rate_limit_window_seconds
						nil,              // parent_key_id
						sqlmock.AnyArg(), // allowed_services
						sqlmock.AnyArg(), // created_at/updated_at
					).
					WillReturnRows(sqlmock.NewRows([]string{"id", "created_at"}).
						AddRow("key-456", time.Now()))
			},
			validate: func(t *testing.T, key *APIKey) {
				assert.Equal(t, KeyTypeGateway, key.KeyType)
				assert.Contains(t, key.Key, "gw_")
				assert.Equal(t, []string{"github", "gitlab"}, key.AllowedServices)
				assert.Equal(t, []string{"read", "write", "gateway"}, key.Scopes)
				assert.Equal(t, 5000, key.RateLimitRequests)
			},
		},
		{
			name: "create agent key with custom rate limit",
			request: CreateAPIKeyRequest{
				Name:      "Agent Key",
				TenantID:  testutil.TestTenantIDString(),
				KeyType:   KeyTypeAgent,
				UserID:    testutil.TestUserIDString(),
				RateLimit: intPtr(2000),
			},
			setupDB: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`INSERT INTO mcp.api_keys`).
					WithArgs(
						sqlmock.AnyArg(), // key_hash
						sqlmock.AnyArg(), // key_prefix
						sqlmock.AnyArg(), // tenant_id
						sqlmock.AnyArg(), // user_id
						"Agent Key",      // name
						KeyTypeAgent,     // key_type
						sqlmock.AnyArg(), // scopes
						true,             // is_active
						nil,              // expires_at
						2000,             // rate_limit_requests (custom)
						60,               // rate_limit_window_seconds
						nil,              // parent_key_id
						sqlmock.AnyArg(), // allowed_services
						sqlmock.AnyArg(), // created_at/updated_at
					).
					WillReturnRows(sqlmock.NewRows([]string{"id", "created_at"}).
						AddRow("key-789", time.Now()))
			},
			validate: func(t *testing.T, key *APIKey) {
				assert.Equal(t, KeyTypeAgent, key.KeyType)
				assert.Contains(t, key.Key, "agt_")
				assert.Equal(t, testutil.TestUserID, key.UserID)
				assert.Equal(t, 2000, key.RateLimitRequests)
			},
		},
		{
			name: "create user key with expiration",
			request: CreateAPIKeyRequest{
				Name:      "User Key",
				TenantID:  testutil.TestTenantIDString(),
				KeyType:   KeyTypeUser,
				ExpiresAt: timePtr(time.Now().Add(24 * time.Hour)),
				Scopes:    []string{"read", "custom:scope"},
			},
			setupDB: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`INSERT INTO mcp.api_keys`).
					WithArgs(
						sqlmock.AnyArg(), // key_hash
						sqlmock.AnyArg(), // key_prefix
						sqlmock.AnyArg(), // tenant_id
						nil,              // user_id
						"User Key",       // name
						KeyTypeUser,      // key_type
						sqlmock.AnyArg(), // scopes
						true,             // is_active
						sqlmock.AnyArg(), // expires_at
						100,              // rate_limit_requests
						60,               // rate_limit_window_seconds
						nil,              // parent_key_id
						sqlmock.AnyArg(), // allowed_services
						sqlmock.AnyArg(), // created_at/updated_at
					).
					WillReturnRows(sqlmock.NewRows([]string{"id", "created_at"}).
						AddRow("key-999", time.Now()))
			},
			validate: func(t *testing.T, key *APIKey) {
				assert.Equal(t, KeyTypeUser, key.KeyType)
				assert.Contains(t, key.Key, "usr_")
				assert.Equal(t, []string{"read", "custom:scope"}, key.Scopes)
				assert.NotNil(t, key.ExpiresAt)
			},
		},
		{
			name: "invalid key type",
			request: CreateAPIKeyRequest{
				Name:     "Invalid Key",
				TenantID: testutil.TestTenantIDString(),
				KeyType:  "invalid",
			},
			wantErr: true,
		},
		{
			name: "memory storage when no database",
			request: CreateAPIKeyRequest{
				Name:     "Memory Key",
				TenantID: testutil.TestTenantIDString(),
				KeyType:  KeyTypeUser,
			},
			setupDB: nil, // No database setup
			validate: func(t *testing.T, key *APIKey) {
				assert.Equal(t, "Memory Key", key.Name)
				assert.Equal(t, KeyTypeUser, key.KeyType)
				assert.True(t, key.Active)
				assert.NotEmpty(t, key.Key)
				assert.NotEmpty(t, key.KeyHash)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup
			logger := observability.NewNoopLogger()
			var service *Service

			if tt.setupDB != nil {
				// Setup with database
				mockDB, mock, err := sqlmock.New()
				require.NoError(t, err)
				defer func() { _ = mockDB.Close() }()

				sqlxDB := sqlx.NewDb(mockDB, "sqlmock")
				tt.setupDB(mock)

				service = NewService(DefaultConfig(), sqlxDB, nil, logger)
			} else {
				// Setup without database (memory only)
				service = NewService(DefaultConfig(), nil, nil, logger)
			}

			// Execute
			result, err := service.CreateAPIKeyWithType(context.Background(), tt.request)

			// Verify
			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, result)
			} else {
				require.NoError(t, err)
				require.NotNil(t, result)

				// Validate the result
				if tt.validate != nil {
					tt.validate(t, result)
				}

				// For memory storage, verify key is in the map
				if tt.setupDB == nil {
					service.mu.RLock()
					storedKey, exists := service.apiKeys[result.Key]
					service.mu.RUnlock()
					assert.True(t, exists)
					assert.Equal(t, result, storedKey)
				}
			}
		})
	}
}

func TestHashAPIKey(t *testing.T) {
	service := &Service{}

	tests := []struct {
		name     string
		apiKey   string
		wantHash string
	}{
		{
			name:     "simple key",
			apiKey:   "test_key_123",
			wantHash: "6e0e9e6e4c8e5e6e5c8e5e6e5c8e5e6e5c8e5e6e5c8e5e6e5c8e5e6e5c8e5e6e", // Not the actual hash
		},
		{
			name:     "empty key",
			apiKey:   "",
			wantHash: "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hash1 := service.hashAPIKey(tt.apiKey)
			hash2 := service.hashAPIKey(tt.apiKey)

			// Should be consistent
			assert.Equal(t, hash1, hash2)

			// Should be 64 characters (SHA256 hex)
			assert.Len(t, hash1, 64)

			// Should be different for different inputs
			if tt.apiKey != "" {
				differentHash := service.hashAPIKey(tt.apiKey + "_different")
				assert.NotEqual(t, hash1, differentHash)
			}
		})
	}
}

func TestGeneratePrefix(t *testing.T) {
	tests := []struct {
		keyType KeyType
		want    string
	}{
		{KeyTypeAdmin, "adm"},
		{KeyTypeGateway, "gw"},
		{KeyTypeAgent, "agt"},
		{KeyTypeUser, "usr"},
		{"unknown", "usr"}, // Default case
	}

	for _, tt := range tests {
		t.Run(string(tt.keyType), func(t *testing.T) {
			got := generatePrefix(tt.keyType)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestGetKeyPrefix(t *testing.T) {
	tests := []struct {
		apiKey string
		want   string
	}{
		{"adm_1234567890abcdef", "adm_1234"},
		{"short", "short"},
		{"exactly8", "exactly8"},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.apiKey, func(t *testing.T) {
			got := getKeyPrefix(tt.apiKey)
			assert.Equal(t, tt.want, got)
		})
	}
}

// Helper functions
func intPtr(i int) *int {
	return &i
}

func timePtr(t time.Time) *time.Time {
	return &t
}
