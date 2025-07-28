package main

import (
	"context"
	"crypto/tls"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/developer-mesh/developer-mesh/pkg/observability"
	"github.com/developer-mesh/developer-mesh/pkg/queue"
	pkgworker "github.com/developer-mesh/developer-mesh/pkg/worker"
	redis "github.com/go-redis/redis/v8"
)

// Version information (set via ldflags during build)
var (
	version   = "dev"
	buildTime = "unknown"
	gitCommit = "unknown"
)

// Command-line flags
var (
	showVersion = flag.Bool("version", false, "Show version information and exit")
	healthCheck = flag.Bool("health-check", false, "Perform health check and exit")
)

// redisIdempotencyAdapter adapts Redis client to the worker interface
type redisIdempotencyAdapter struct {
	client *redis.Client
}

func (r *redisIdempotencyAdapter) Exists(ctx context.Context, key string) (int64, error) {
	return r.client.Exists(ctx, key).Result()
}

func (r *redisIdempotencyAdapter) Set(ctx context.Context, key string, value string, ttl time.Duration) error {
	return r.client.Set(ctx, key, value, ttl).Err()
}

func main() {
	flag.Parse()

	// Show version information if requested
	if *showVersion {
		fmt.Printf("Worker\nVersion: %s\nBuild Time: %s\nGit Commit: %s\n", version, buildTime, gitCommit)
		os.Exit(0)
	}

	// Perform health check if requested
	if *healthCheck {
		if err := performHealthCheck(); err != nil {
			fmt.Fprintf(os.Stderr, "Health check failed: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("Health check passed")
		os.Exit(0)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Setup signal handling
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Start worker in a goroutine
	errChan := make(chan error, 1)
	go func() {
		if err := runWorker(ctx); err != nil {
			errChan <- err
		}
	}()

	// Wait for signal or error
	select {
	case sig := <-sigChan:
		log.Printf("Received signal: %v", sig)
		cancel()
		// Give worker time to shut down gracefully
		time.Sleep(5 * time.Second)
	case err := <-errChan:
		log.Fatalf("Worker error: %v", err)
	}

	log.Println("Worker stopped")
}

func runWorker(ctx context.Context) error {
	// Initialize logger
	logger := observability.NewNoopLogger()

	// Initialize Redis queue client
	queueClient, err := queue.NewClient(ctx, &queue.Config{
		Logger: logger,
	})
	if err != nil {
		return fmt.Errorf("failed to initialize queue client: %w", err)
	}
	defer queueClient.Close()

	// Initialize Redis client for idempotency
	redisAddr := os.Getenv("REDIS_ADDR")
	if redisAddr == "" {
		redisAddr = "localhost:6379"
	}
	log.Printf("Connecting to Redis at %s", redisAddr)

	// Configure Redis options with TLS support
	redisOptions := &redis.Options{
		Addr: redisAddr,
	}

	// Check if TLS is enabled
	if os.Getenv("REDIS_TLS_ENABLED") == "true" {
		log.Printf("Redis TLS enabled")
		redisOptions.TLSConfig = &tls.Config{
			MinVersion:         tls.VersionTLS12,
			InsecureSkipVerify: os.Getenv("REDIS_TLS_SKIP_VERIFY") == "true", // #nosec G402 - Configurable for dev
		}
	}

	redisClient := redis.NewClient(redisOptions)

	// Test Redis connection
	pingCtx, pingCancel := context.WithTimeout(ctx, 5*time.Second)
	defer pingCancel()
	if err := redisClient.Ping(pingCtx).Err(); err != nil {
		return fmt.Errorf("failed to connect to Redis: %w", err)
	}

	redisAdapter := &redisIdempotencyAdapter{client: redisClient}

	// Create Redis worker
	worker, err := pkgworker.NewRedisWorker(&pkgworker.Config{
		QueueClient:    queueClient,
		RedisClient:    redisAdapter,
		Processor:      pkgworker.ProcessEvent,
		Logger:         logger,
		ConsumerName:   fmt.Sprintf("worker-%s", os.Getenv("HOSTNAME")),
		IdempotencyTTL: 24 * time.Hour,
	})
	if err != nil {
		return fmt.Errorf("failed to create worker: %w", err)
	}

	log.Println("Starting Redis worker...")
	return worker.Run(ctx)
}

// performHealthCheck performs a basic health check
func performHealthCheck() error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Check Redis connectivity
	redisAddr := os.Getenv("REDIS_ADDR")
	if redisAddr == "" {
		redisAddr = "localhost:6379"
	}

	// Configure Redis options with TLS support
	redisOptions := &redis.Options{
		Addr: redisAddr,
	}

	// Check if TLS is enabled
	if os.Getenv("REDIS_TLS_ENABLED") == "true" {
		redisOptions.TLSConfig = &tls.Config{
			MinVersion:         tls.VersionTLS12,
			InsecureSkipVerify: os.Getenv("REDIS_TLS_SKIP_VERIFY") == "true", // #nosec G402
		}
	}

	redisClient := redis.NewClient(redisOptions)
	defer redisClient.Close()

	if err := redisClient.Ping(ctx).Err(); err != nil {
		return fmt.Errorf("redis health check failed: %w", err)
	}

	// Check queue connectivity
	queueClient, err := queue.NewClient(ctx, &queue.Config{})
	if err != nil {
		return fmt.Errorf("queue client health check failed: %w", err)
	}
	defer queueClient.Close()

	if err := queueClient.Health(ctx); err != nil {
		return fmt.Errorf("queue health check failed: %w", err)
	}

	return nil
}
