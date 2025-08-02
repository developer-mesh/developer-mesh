package config

import (
	"time"
)

// DatabaseConfig holds database configuration
type DatabaseConfig struct {
	Driver          string               `yaml:"driver" mapstructure:"driver"`
	DSN             string               `yaml:"dsn" mapstructure:"dsn"`
	Host            string               `yaml:"host" mapstructure:"host"`
	Port            int                  `yaml:"port" mapstructure:"port"`
	Username        string               `yaml:"username" mapstructure:"username"`
	Password        string               `yaml:"password" mapstructure:"password"`
	Database        string               `yaml:"database" mapstructure:"database"`
	SearchPath      string               `yaml:"search_path" mapstructure:"search_path"`
	SSLMode         string               `yaml:"ssl_mode" mapstructure:"ssl_mode"`
	MaxOpenConns    int                  `yaml:"max_open_conns" mapstructure:"max_open_conns"`
	MaxIdleConns    int                  `yaml:"max_idle_conns" mapstructure:"max_idle_conns"`
	ConnMaxLifetime time.Duration        `yaml:"conn_max_lifetime" mapstructure:"conn_max_lifetime"`
	ConnMaxIdleTime time.Duration        `yaml:"conn_max_idle_time" mapstructure:"conn_max_idle_time"`
	UseIAMAuth      bool                 `yaml:"use_iam_auth" mapstructure:"use_iam_auth"`
	TokenExpiration int                  `yaml:"token_expiration" mapstructure:"token_expiration"`
	Vector          DatabaseVectorConfig `yaml:"vector" mapstructure:"vector"`
	AuthConfig      struct {
		Region     string `yaml:"region" mapstructure:"region"`
		Endpoint   string `yaml:"endpoint" mapstructure:"endpoint"`
		AssumeRole string `yaml:"assume_role" mapstructure:"assume_role"`
	} `yaml:"auth" mapstructure:"auth"`
}

// DatabaseVectorConfig holds configuration for vector database operations
type DatabaseVectorConfig struct {
	Enabled          bool                     `yaml:"enabled"`
	IndexType        string                   `yaml:"index_type"`
	Lists            int                      `yaml:"lists"`
	Probes           int                      `yaml:"probes"`
	Dimensions       int                      `yaml:"dimensions"`
	SimilarityMetric string                   `yaml:"similarity_metric"`
	Pool             DatabaseVectorPoolConfig `yaml:"pool"`
}

// DatabaseVectorPoolConfig holds configuration for the vector database connection pool
type DatabaseVectorPoolConfig struct {
	Enabled         bool          `yaml:"enabled"`
	MaxOpenConns    int           `yaml:"max_open_conns"`
	MaxIdleConns    int           `yaml:"max_idle_conns"`
	ConnMaxLifetime time.Duration `yaml:"conn_max_lifetime"`
}

// GetDefaultDatabaseConfig returns default database configuration
func GetDefaultDatabaseConfig() DatabaseConfig {
	return DatabaseConfig{
		Driver:          "postgres",
		Host:            "localhost",
		Port:            5432,
		Database:        "mcp",
		Username:        "postgres",
		Password:        "",
		SSLMode:         "disable",
		MaxOpenConns:    25,
		MaxIdleConns:    5,
		ConnMaxLifetime: 5 * time.Minute,
		ConnMaxIdleTime: 2 * time.Minute,
		UseIAMAuth:      false,
		TokenExpiration: 900,
		AuthConfig: struct {
			Region     string `yaml:"region" mapstructure:"region"`
			Endpoint   string `yaml:"endpoint" mapstructure:"endpoint"`
			AssumeRole string `yaml:"assume_role" mapstructure:"assume_role"`
		}{
			Region:   "us-west-2",
			Endpoint: "",
		},
		Vector: DatabaseVectorConfig{
			Enabled:   true,
			IndexType: "ivfflat",
			Lists:     100,
			Probes:    10,
			Pool: DatabaseVectorPoolConfig{
				Enabled:         false,
				MaxOpenConns:    25,
				MaxIdleConns:    5,
				ConnMaxLifetime: 10 * time.Minute,
			},
		},
	}
}
