package auth

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"gopkg.in/yaml.v3"
)

// APIKeyConfig represents API key configuration
type APIKeyConfig struct {
	// Development keys (only loaded in dev/test environments)
	DevelopmentKeys map[string]APIKeySettings `yaml:"development_keys"`

	// Production key sources
	ProductionKeySource string `yaml:"production_key_source"` // "env", "vault", "aws-secrets"
}

// APIKeySettings represents settings for an API key
type APIKeySettings struct {
	Role      string   `yaml:"role"`
	Scopes    []string `yaml:"scopes"`
	TenantID  string   `yaml:"tenant_id"`
	ExpiresIn string   `yaml:"expires_in"` // Duration string like "30d"
}

// KeyConfig is used for backward compatibility with existing code
type KeyConfig struct {
	Key      string
	Role     string
	Scopes   []string
	TenantID string
	UserID   string
}

// LoadAPIKeys loads API keys based on environment
func (s *Service) LoadAPIKeys(config *APIKeyConfig) error {
	env := os.Getenv("ENVIRONMENT")

	switch env {
	case "development", "test", "docker", "":
		return s.loadDevelopmentKeys(config.DevelopmentKeys)
	case "production":
		return s.loadProductionKeys(config.ProductionKeySource)
	default:
		return fmt.Errorf("unknown environment: %s", env)
	}
}

// loadDevelopmentKeys loads keys for development/test
func (s *Service) loadDevelopmentKeys(keys map[string]APIKeySettings) error {
	// For docker environment, also load from environment variables
	env := os.Getenv("ENVIRONMENT")
	if env == "docker" {
		s.logger.Info("Docker environment detected, loading keys from environment", nil)
		return s.loadKeysFromEnv()
	}

	if keys == nil {
		return nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	for key, settings := range keys {
		// Parse tenant ID
		var tenantUUID uuid.UUID
		if settings.TenantID == "" {
			tenantUUID = DefaultTenantID
		} else {
			var err error
			tenantUUID, err = uuid.Parse(settings.TenantID)
			if err != nil {
				s.logger.Warn("Invalid tenant ID in config, using default", map[string]interface{}{
					"tenant_id": settings.TenantID,
					"error":     err.Error(),
				})
				tenantUUID = DefaultTenantID
			}
		}

		// Generate deterministic user ID for development
		userUUID := uuid.NewSHA1(uuid.NameSpaceURL, []byte(fmt.Sprintf("dev-user-%s", key)))

		// Generate deterministic key for development
		apiKey := &APIKey{
			Key:       fmt.Sprintf("dev_%s", key),
			TenantID:  tenantUUID,
			UserID:    userUUID,
			Name:      fmt.Sprintf("Development %s key", settings.Role),
			KeyType:   getKeyTypeFromRole(settings.Role),
			Scopes:    settings.Scopes,
			CreatedAt: time.Now(),
			Active:    true,
		}

		s.apiKeys[apiKey.Key] = apiKey

		s.logger.Debug("Loaded development API key", map[string]interface{}{
			"key_name": key,
			"role":     settings.Role,
		})
	}

	return nil
}

// loadProductionKeys loads keys from secure sources
func (s *Service) loadProductionKeys(source string) error {
	switch source {
	case "database":
		// In production, keys are loaded on-demand from database
		// This enables dynamic key management without restarts
		s.logger.Info("Production mode: API keys will be loaded from database on demand", nil)
		return nil
	case "env":
		return s.loadKeysFromEnv()
	case "vault":
		return s.loadKeysFromVault()
	case "aws-secrets":
		return s.loadKeysFromAWSSecrets()
	default:
		return fmt.Errorf("unsupported key source: %s", source)
	}
}

// loadKeysFromEnv loads API keys from environment variables
func (s *Service) loadKeysFromEnv() error {
	s.logger.Info("Loading API keys from environment variables", nil)

	// Look for API_KEY_* environment variables
	foundKeys := 0

	// First check for standard named API keys
	standardKeys := []struct {
		envVar string
		name   string
		role   string
	}{
		{"ADMIN_API_KEY", "admin", "admin"},
		{"READER_API_KEY", "reader", "read"},
		{"MCP_API_KEY", "mcp", "admin"},
		{"E2E_API_KEY", "e2e", "admin"}, // Support E2E testing
	}

	for _, sk := range standardKeys {
		keyValue := os.Getenv(sk.envVar)
		s.logger.Debug("Checking environment variable", map[string]interface{}{
			"env_var": sk.envVar,
			"found":   keyValue != "",
		})
		if keyValue != "" {
			// Use a fixed UUID for default tenant in docker environment
			defaultTenantIDStr := getEnvOrDefault("DEFAULT_TENANT_ID", "00000000-0000-0000-0000-000000000001")
			defaultTenantUUID, err := uuid.Parse(defaultTenantIDStr)
			if err != nil {
				defaultTenantUUID = DefaultTenantID
			}

			apiKey := &APIKey{
				Key:       keyValue,
				TenantID:  defaultTenantUUID,
				UserID:    SystemUserID,
				Name:      sk.name,
				KeyType:   getKeyTypeFromRole(sk.role),
				Scopes:    getRoleScopes(sk.role),
				CreatedAt: time.Now(),
				Active:    true,
			}

			s.mu.Lock()
			s.apiKeys[keyValue] = apiKey
			s.mu.Unlock()

			// If database is available, also store in database
			if s.db != nil {
				if err := s.storeAPIKeyInDB(keyValue, apiKey); err != nil {
					s.logger.Warn("Failed to store API key in database", map[string]interface{}{
						"name":  sk.name,
						"error": err.Error(),
					})
				}
			}

			s.logger.Info("Loaded standard API key from environment", map[string]interface{}{
				"env_var":    sk.envVar,
				"name":       sk.name,
				"role":       sk.role,
				"key_suffix": truncateKey(keyValue, 8),
				"full_key":   keyValue, // TEMPORARY: Remove in production
			})
			foundKeys++
		}
	}

	// Then look for custom API_KEY_* environment variables
	for _, env := range os.Environ() {
		if strings.HasPrefix(env, "API_KEY_") {
			parts := strings.SplitN(env, "=", 2)
			if len(parts) != 2 {
				continue
			}

			keyName := strings.TrimPrefix(parts[0], "API_KEY_")
			keyValue := parts[1]

			// Parse key metadata from env var name
			// Format: API_KEY_<NAME>_<ROLE>
			keyParts := strings.Split(keyName, "_")
			if len(keyParts) < 2 {
				continue
			}

			role := strings.ToLower(keyParts[len(keyParts)-1])
			name := strings.Join(keyParts[:len(keyParts)-1], "_")

			// Use a fixed UUID for default tenant in docker environment
			defaultTenantIDStr := getEnvOrDefault("DEFAULT_TENANT_ID", "00000000-0000-0000-0000-000000000001")
			defaultTenantUUID, err := uuid.Parse(defaultTenantIDStr)
			if err != nil {
				defaultTenantUUID = DefaultTenantID
			}

			apiKey := &APIKey{
				Key:       keyValue,
				TenantID:  defaultTenantUUID,
				UserID:    SystemUserID,
				Name:      name,
				KeyType:   getKeyTypeFromRole(role),
				Scopes:    getRoleScopes(role),
				CreatedAt: time.Now(),
				Active:    true,
			}

			s.mu.Lock()
			s.apiKeys[keyValue] = apiKey
			s.mu.Unlock()

			s.logger.Info("Loaded API key from environment", map[string]interface{}{
				"key_name": name,
				"role":     role,
			})
			foundKeys++
		}
	}

	if foundKeys == 0 {
		s.logger.Warn("No API keys found in environment variables", map[string]interface{}{})
	} else {
		s.logger.Info("Successfully loaded API keys from environment", map[string]interface{}{
			"count":             foundKeys,
			"total_keys_loaded": len(s.apiKeys),
		})
	}

	return nil
}

// loadKeysFromVault placeholder - implement based on your Vault setup
func (s *Service) loadKeysFromVault() error {
	return fmt.Errorf("vault integration not implemented")
}

// loadKeysFromAWSSecrets placeholder - implement based on your AWS setup
func (s *Service) loadKeysFromAWSSecrets() error {
	return fmt.Errorf("AWS Secrets Manager integration not implemented")
}

// getRoleScopes returns scopes for a role
func getRoleScopes(role string) []string {
	switch role {
	case "admin":
		return []string{"read", "write", "admin"}
	case "write":
		return []string{"read", "write"}
	case "read":
		return []string{"read"}
	default:
		return []string{"read"}
	}
}

// getKeyTypeFromRole returns the key type based on role
func getKeyTypeFromRole(role string) KeyType {
	switch role {
	case "admin":
		return KeyTypeAdmin
	case "agent":
		return KeyTypeAgent
	case "gateway":
		return KeyTypeGateway
	default:
		return KeyTypeUser
	}
}

// LoadAuthConfigFromFile loads auth configuration from a YAML file
func LoadAuthConfigFromFile(filename string) (*APIKeyConfig, error) {
	// Clean and validate the file path to prevent path traversal
	cleanPath := filepath.Clean(filename)

	// Ensure the path is not trying to escape to parent directories
	if strings.Contains(cleanPath, "..") {
		return nil, fmt.Errorf("invalid file path: %s", filename)
	}

	file, err := os.Open(cleanPath)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := file.Close(); err != nil {
			// Log error but don't fail - file was already read
			_ = err
		}
	}()

	var config struct {
		Auth APIKeyConfig `yaml:"auth"`
	}

	decoder := yaml.NewDecoder(file)
	if err := decoder.Decode(&config); err != nil {
		return nil, err
	}

	return &config.Auth, nil
}

// LoadAuthConfigBasedOnEnvironment loads the appropriate auth config file
func (s *Service) LoadAuthConfigBasedOnEnvironment() error {
	env := os.Getenv("ENVIRONMENT")
	if env == "" {
		env = "development"
	}

	s.logger.Info("Loading auth config based on environment", map[string]interface{}{
		"environment": env,
	})

	var configFile string
	switch env {
	case "production":
		configFile = "configs/auth.production.yaml"
	case "development", "test", "docker", "":
		configFile = "configs/auth.development.yaml"
	default:
		configFile = "configs/auth.development.yaml"
	}

	config, err := LoadAuthConfigFromFile(configFile)
	if err != nil {
		// Don't fail if file doesn't exist, just log
		if os.IsNotExist(err) {
			s.logger.Info("Auth config file not found, loading from environment", map[string]interface{}{
				"file":        configFile,
				"environment": env,
			})
			// In production, still try to load from env vars even if config file missing
			if env == "production" {
				return s.loadKeysFromEnv()
			}
			return nil
		}
		return fmt.Errorf("failed to load auth config: %w", err)
	}

	return s.LoadAPIKeys(config)
}
