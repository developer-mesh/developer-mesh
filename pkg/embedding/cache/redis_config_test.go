package cache

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestDefaultRedisPoolConfig(t *testing.T) {
	config := DefaultRedisPoolConfig()

	assert.Equal(t, 100, config.PoolSize)
	assert.Equal(t, 10, config.MinIdleConns)
	assert.Equal(t, 3, config.MaxRetries)
	assert.Equal(t, 5*time.Second, config.DialTimeout)
	assert.Equal(t, 3*time.Second, config.ReadTimeout)
	assert.Equal(t, 3*time.Second, config.WriteTimeout)
	assert.Equal(t, 4*time.Second, config.PoolTimeout)
	assert.Equal(t, 5*time.Minute, config.IdleTimeout)
	assert.Equal(t, 1*time.Minute, config.IdleCheckFreq)
	assert.Equal(t, 30*time.Minute, config.MaxConnAge)
}

func TestHighLoadRedisPoolConfig(t *testing.T) {
	config := HighLoadRedisPoolConfig()

	assert.Equal(t, 500, config.PoolSize)
	assert.Equal(t, 50, config.MinIdleConns)
	assert.Equal(t, 5, config.MaxRetries)
	assert.Equal(t, 3*time.Second, config.DialTimeout)
}

func TestLowLatencyRedisPoolConfig(t *testing.T) {
	config := LowLatencyRedisPoolConfig()

	assert.Equal(t, 200, config.PoolSize)
	assert.Equal(t, 20, config.MinIdleConns)
	assert.Equal(t, 2, config.MaxRetries)
	assert.Equal(t, 500*time.Millisecond, config.ReadTimeout)
	assert.Equal(t, 500*time.Millisecond, config.WriteTimeout)
}

func TestRedisPoolConfig_ToRedisOptions(t *testing.T) {
	config := DefaultRedisPoolConfig()
	addr := "localhost:6379"
	db := 0

	options := config.ToRedisOptions(addr, db)

	assert.NotNil(t, options)
	assert.Equal(t, addr, options.Addr)
	assert.Equal(t, db, options.DB)
	assert.Equal(t, config.PoolSize, options.PoolSize)
	assert.Equal(t, config.MinIdleConns, options.MinIdleConns)
	assert.Equal(t, config.MaxRetries, options.MaxRetries)
	assert.Equal(t, config.DialTimeout, options.DialTimeout)
	assert.Equal(t, config.ReadTimeout, options.ReadTimeout)
	assert.Equal(t, config.WriteTimeout, options.WriteTimeout)
	assert.Equal(t, config.PoolTimeout, options.PoolTimeout)
	assert.Equal(t, config.IdleTimeout, options.ConnMaxIdleTime)
}

func TestNewRedisClientWithPool(t *testing.T) {
	// Test with default config
	client := NewRedisClientWithPool("localhost:6379", 0, nil)
	assert.NotNil(t, client)

	// Test with custom config
	customConfig := &RedisPoolConfig{
		PoolSize:     50,
		MinIdleConns: 5,
	}
	clientWithCustom := NewRedisClientWithPool("localhost:6379", 0, customConfig)
	assert.NotNil(t, clientWithCustom)
}
