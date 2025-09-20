package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/developer-mesh/developer-mesh/apps/mockserver/internal/handlers"
)

func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func main() {
	// Command line flags
	var (
		port        = flag.String("port", getEnvOrDefault("PORT", "8082"), "Port to run the mock server on")
		host        = flag.String("host", "", "Host to bind the server to")
		healthCheck = flag.Bool("health-check", false, "Run health check and exit")
	)
	flag.Parse()

	// Handle health check mode
	if *healthCheck {
		// Validate port before using it
		portNum, err := strconv.Atoi(*port)
		if err != nil || portNum < 1 || portNum > 65535 {
			log.Fatalf("Invalid port number: %s", *port)
		}

		resp, err := http.Get(fmt.Sprintf("http://localhost:%d/health", portNum))
		if err != nil {
			os.Exit(1)
		}
		defer func() {
			if err := resp.Body.Close(); err != nil {
				log.Printf("Failed to close response body: %v", err)
			}
		}()
		if resp.StatusCode == http.StatusOK {
			os.Exit(0)
		}
		os.Exit(1)
	}

	// Setup logging
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	addr := fmt.Sprintf("%s:%s", *host, *port)
	log.Printf("Starting mock server on %s", addr)

	// Create a new ServeMux for better control
	mux := http.NewServeMux()

	// Setup all the handlers
	handlers.SetupHandlers(mux)

	// Add a health check endpoint
	mux.HandleFunc("/health", healthHandler)

	// Create server with timeouts for production use
	server := &http.Server{
		Addr:         addr,
		Handler:      mux,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Run server in a goroutine so it doesn't block
	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Failed to start server: %v", err)
		}
	}()

	// Wait for interrupt signal to gracefully shutdown the server
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down server...")

	// Graceful shutdown with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Printf("Server forced to shutdown: %v", err)
	}

	log.Println("Server stopped")
}

// healthHandler returns the health status of the mock server
func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	// Use fmt.Fprintf for better error handling
	if _, err := fmt.Fprintf(w, `{"status":"healthy","timestamp":"%s"}`, time.Now().Format(time.RFC3339)); err != nil {
		log.Printf("Failed to write health response: %v", err)
	}
}
