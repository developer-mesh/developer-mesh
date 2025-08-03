package database

import (
	"context"
	"log"
)

// InitializeTables initializes all required database tables
func (db *Database) InitializeTables(ctx context.Context) error {
	log.Println("Initializing database tables...")

	// Skip table creation - tables are created by migrations
	// The previous implementation had hardcoded table definitions that didn't match our schema

	log.Println("Database tables initialized successfully (using migrations)")
	return nil
}
