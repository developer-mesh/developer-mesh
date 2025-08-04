package auth_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"

	"github.com/developer-mesh/developer-mesh/pkg/auth"
	"github.com/developer-mesh/developer-mesh/pkg/observability"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func ExampleService_GinMiddleware() {
	// Create auth service
	config := auth.DefaultConfig()
	config.JWTSecret = "example-secret"
	logger := observability.NewLogger("example")
	authService := auth.NewService(config, nil, nil, logger)

	// Initialize some API keys
	authService.InitializeDefaultAPIKeys(map[string]string{
		"test-key": "read",
	})

	// Create Gin router with auth
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()

	// Apply auth middleware
	router.Use(authService.GinMiddleware(auth.TypeAPIKey))

	// Add a protected endpoint
	router.GET("/api/data", func(c *gin.Context) {
		user, _ := auth.GetUserFromContext(c)
		c.JSON(200, gin.H{
			"message": fmt.Sprintf("Hello user %s", user.ID),
		})
	})

	// Test the endpoint with valid API key
	req := httptest.NewRequest("GET", "/api/data", nil)
	req.Header.Set("Authorization", "Bearer test-key")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	fmt.Println("Response:", w.Code)
	// Output: Response: 200
}

func ExampleService_CreateAPIKey() {
	// Create auth service
	config := auth.DefaultConfig()
	logger := observability.NewLogger("example")
	authService := auth.NewService(config, nil, nil, logger)

	// Create an API key
	ctx := context.Background()
	apiKey, err := authService.CreateAPIKey(
		ctx,
		uuid.MustParse("00000000-0000-0000-0000-000000000001"),
		uuid.MustParse("00000000-0000-0000-0000-000000000002"),
		"My API Key",
		[]string{"read", "write"},
		nil, // No expiration
	)

	if err == nil {
		fmt.Printf("Created API key: %s\n", apiKey.Name)
	}
	// Output: Created API key: My API Key
}

func ExampleService_GenerateJWT() {
	// Create auth service
	config := auth.DefaultConfig()
	config.JWTSecret = "example-secret"
	logger := observability.NewLogger("example")
	authService := auth.NewService(config, nil, nil, logger)

	// Generate a JWT token
	ctx := context.Background()
	user := &auth.User{
		ID:       uuid.MustParse("00000000-0000-0000-0000-000000000003"),
		TenantID: uuid.MustParse("00000000-0000-0000-0000-000000000004"),
		Email:    "user@example.com",
		Scopes:   []string{"read"},
	}

	_, err := authService.GenerateJWT(ctx, user)
	if err == nil {
		fmt.Println("Generated JWT token")
	}
	// Output: Generated JWT token
}

func ExampleService_StandardMiddleware() {
	// Create auth service
	config := auth.DefaultConfig()
	logger := observability.NewLogger("example")
	authService := auth.NewService(config, nil, nil, logger)

	// Initialize some API keys
	authService.InitializeDefaultAPIKeys(map[string]string{
		"test-key": "read",
	})

	// Create standard HTTP handler
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, ok := auth.GetUserFromRequest(r)
		if ok {
			if _, err := fmt.Fprintf(w, "Hello user %s", user.ID); err != nil {
				// Log error if needed - in example code, we'll just ignore
				_ = err
			}
		}
	})

	// Wrap with auth middleware
	protectedHandler := authService.StandardMiddleware(auth.TypeAPIKey)(handler)

	// Test the handler
	req := httptest.NewRequest("GET", "/api/data", nil)
	req.Header.Set("Authorization", "test-key")
	w := httptest.NewRecorder()
	protectedHandler.ServeHTTP(w, req)

	fmt.Println("Response:", w.Code)
	// Output: Response: 200
}
