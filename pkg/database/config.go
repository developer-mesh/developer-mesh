// Package database provides database access functionality for the MCP system.
package database

import (
	"fmt"
	"time"

	securitytls "github.com/developer-mesh/developer-mesh/pkg/security/tls"
)

// Config defines what the database package needs - no external imports!
type Config struct {
	// Core database settings
	Driver          string
	DSN             string
	Host            string
	Port            int
	Database        string
	Username        string
	Password        string
	SSLMode         string
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration

	// TLS Configuration
	TLS *securitytls.Config

	// Timeout configurations (best practice)
	QueryTimeout   time.Duration // Default: 30s
	ConnectTimeout time.Duration // Default: 10s

	// AWS RDS specific settings (optional)
	UseAWS     bool
	UseIAM     bool
	AWSRegion  string
	AWSRoleARN string

	// RDS-specific configuration
	RDSHost              string
	RDSPort              int
	RDSDatabase          string
	RDSUsername          string
	RDSTokenExpiration   int // seconds
	RDSEnablePooling     bool
	RDSMinPoolSize       int
	RDSMaxPoolSize       int
	RDSConnectionTimeout int // seconds

	// Migration settings
	AutoMigrate          bool
	MigrationsPath       string
	FailOnMigrationError bool

	// Schema settings
	SearchPath string
}

// NewConfig creates config with sensible defaults
func NewConfig() *Config {
	return &Config{
		Driver:          "postgres",
		MaxOpenConns:    25,
		MaxIdleConns:    5,
		ConnMaxLifetime: 5 * time.Minute,
		QueryTimeout:    30 * time.Second,
		ConnectTimeout:  10 * time.Second,
		MigrationsPath:  "migrations",
		SSLMode:         "disable",
		Port:            5432,

		// RDS defaults
		RDSPort:              5432,
		RDSTokenExpiration:   900, // 15 minutes
		RDSEnablePooling:     true,
		RDSMinPoolSize:       2,
		RDSMaxPoolSize:       10,
		RDSConnectionTimeout: 30,
	}
}

// GetDSN returns the connection string for the database
func (c *Config) GetDSN() string {
	if c.DSN != "" {
		return c.DSN
	}

	// Build DSN from components if not explicitly set
	if c.UseAWS && c.UseIAM {
		// For AWS RDS with IAM, DSN will be built by the RDS client
		return ""
	}

	// Build standard PostgreSQL DSN
	return buildPostgresDSN(c)
}

// buildPostgresDSN constructs a PostgreSQL connection string
func buildPostgresDSN(c *Config) string {
	// Simple DSN builder - in production, use a proper DSN builder
	if c.Host == "" {
		c.Host = "localhost"
	}

	// Debug logging to see what SSL mode is being used
	fmt.Printf("DEBUG: Building DSN with SSLMode=%s\n", c.SSLMode)

	dsn := "postgres://"
	if c.Username != "" {
		dsn += c.Username
		if c.Password != "" {
			dsn += ":" + c.Password
		}
		dsn += "@"
	}
	dsn += fmt.Sprintf("%s:%d/%s", c.Host, c.Port, c.Database)
	dsn += "?sslmode=" + c.SSLMode

	// Add TLS parameters if configured
	if c.TLS != nil && c.TLS.Enabled && c.SSLMode != "disable" {
		if c.TLS.CertFile != "" {
			dsn += "&sslcert=" + c.TLS.CertFile
		}
		if c.TLS.KeyFile != "" {
			dsn += "&sslkey=" + c.TLS.KeyFile
		}
		if c.TLS.CAFile != "" {
			dsn += "&sslrootcert=" + c.TLS.CAFile
		}
		// Note: PostgreSQL driver doesn't support min TLS version in DSN
		// This would need to be handled at the driver level
	}

	return dsn
}

// Validate checks if the configuration is valid
func (c *Config) Validate() error {
	if c.Driver == "" {
		c.Driver = "postgres"
	}

	if c.UseAWS && c.UseIAM {
		// Validate AWS-specific settings
		if c.AWSRegion == "" {
			return ErrMissingAWSRegion
		}
		if c.RDSHost == "" {
			return ErrMissingRDSHost
		}
	} else {
		// Validate standard database settings
		if c.GetDSN() == "" && (c.Host == "" || c.Database == "") {
			return ErrInvalidDatabaseConfig
		}
	}

	return nil
}
