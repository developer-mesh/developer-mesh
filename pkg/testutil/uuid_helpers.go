package testutil

import "github.com/google/uuid"

// Common test UUIDs for consistency across tests
var (
	// TestTenantID returns a consistent UUID for tenant testing
	TestTenantID = uuid.MustParse("11111111-1111-1111-1111-111111111111")

	// TestUserID returns a consistent UUID for user testing
	TestUserID = uuid.MustParse("22222222-2222-2222-2222-222222222222")

	// TestAgentID returns a consistent UUID for agent testing
	TestAgentID = uuid.MustParse("33333333-3333-3333-3333-333333333333")

	// TestModelID returns a consistent UUID for model testing
	TestModelID = uuid.MustParse("44444444-4444-4444-4444-444444444444")

	// TestAPIKeyID returns a consistent UUID for API key testing
	TestAPIKeyID = uuid.MustParse("55555555-5555-5555-5555-555555555555")
)

// NewTestTenantID generates a new random tenant ID for testing
func NewTestTenantID() uuid.UUID {
	return uuid.New()
}

// NewTestUserID generates a new random user ID for testing
func NewTestUserID() uuid.UUID {
	return uuid.New()
}

// TestTenantIDString returns the test tenant ID as a string
func TestTenantIDString() string {
	return TestTenantID.String()
}

// TestUserIDString returns the test user ID as a string
func TestUserIDString() string {
	return TestUserID.String()
}

// NewTestTenantIDString generates a new random tenant ID string for testing
func NewTestTenantIDString() string {
	return uuid.New().String()
}

// NewTestUserIDString generates a new random user ID string for testing
func NewTestUserIDString() string {
	return uuid.New().String()
}
