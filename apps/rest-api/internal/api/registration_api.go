package api

import (
	"net/http"

	"github.com/developer-mesh/developer-mesh/apps/rest-api/internal/services"
	"github.com/developer-mesh/developer-mesh/pkg/auth"
	"github.com/developer-mesh/developer-mesh/pkg/observability"
	pkgservices "github.com/developer-mesh/developer-mesh/pkg/services"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// RegistrationAPI handles organization and user registration endpoints
type RegistrationAPI struct {
	orgService        *services.OrganizationService
	userService       *services.UserAuthService
	authService       *auth.Service
	credentialService *pkgservices.CredentialService
	logger            observability.Logger
}

// NewRegistrationAPI creates a new registration API handler
func NewRegistrationAPI(
	orgService *services.OrganizationService,
	userService *services.UserAuthService,
	authService *auth.Service,
	credentialService *pkgservices.CredentialService,
	logger observability.Logger,
) *RegistrationAPI {
	return &RegistrationAPI{
		orgService:        orgService,
		userService:       userService,
		authService:       authService,
		credentialService: credentialService,
		logger:            logger,
	}
}

// RegisterPublicRoutes registers public auth routes (no authentication required)
func (api *RegistrationAPI) RegisterPublicRoutes(router *gin.RouterGroup) {
	// Public routes (no authentication required)
	public := router.Group("/auth")
	{
		// Organization registration
		public.POST("/register/organization", api.RegisterOrganization)

		// User authentication
		public.POST("/login", api.Login)
		public.POST("/refresh", api.RefreshToken)
		public.POST("/logout", api.Logout)

		// Edge MCP authentication
		public.POST("/edge-mcp", api.AuthenticateEdgeMCP)

		// Password reset
		public.POST("/password/reset", api.RequestPasswordReset)
		public.POST("/password/reset/confirm", api.ConfirmPasswordReset)

		// Email verification
		public.POST("/email/verify", api.VerifyEmail)
		public.POST("/email/resend", api.ResendVerificationEmail)

		// Invitation acceptance
		public.GET("/invitation/:token", api.GetInvitationDetails)
		public.POST("/invitation/accept", api.AcceptInvitation)
	}
}

// RegisterProtectedRoutes registers protected routes (authentication required)
func (api *RegistrationAPI) RegisterProtectedRoutes(router *gin.RouterGroup) {
	// User management (admin only)
	router.POST("/users/invite", api.InviteUser)
	router.GET("/users", api.ListOrganizationUsers)
	router.PUT("/users/:id/role", api.UpdateUserRole)
	router.DELETE("/users/:id", api.RemoveUser)

	// Organization management
	router.GET("/organization", api.GetOrganization)
	router.PUT("/organization", api.UpdateOrganization)
	router.GET("/organization/usage", api.GetOrganizationUsage)

	// User profile
	router.GET("/profile", api.GetProfile)
	router.PUT("/profile", api.UpdateProfile)
	router.POST("/profile/password", api.ChangePassword)
}

// RegisterOrganization handles new organization registration
func (api *RegistrationAPI) RegisterOrganization(c *gin.Context) {
	var req services.OrganizationRegistration
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid request",
			"details": err.Error(),
		})
		return
	}

	org, user, apiKey, err := api.orgService.RegisterOrganization(c.Request.Context(), &req)
	if err != nil {
		api.logger.Error("Failed to register organization", map[string]interface{}{
			"error": err.Error(),
			"email": req.AdminEmail,
		})

		// Return appropriate error message
		statusCode := http.StatusInternalServerError
		message := "Failed to register organization"

		if err.Error() == "organization slug already exists" {
			statusCode = http.StatusConflict
			message = "Organization slug already taken"
		} else if err.Error() == "email already registered" {
			statusCode = http.StatusConflict
			message = "Email already registered"
		} else if err.Error() == "invalid organization slug format" {
			statusCode = http.StatusBadRequest
			message = "Invalid organization slug format (use lowercase letters, numbers, and hyphens)"
		}

		c.JSON(statusCode, gin.H{
			"error":   message,
			"details": err.Error(), // Temporarily add actual error for debugging
		})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"organization": org,
		"user": gin.H{
			"id":             user.ID,
			"email":          user.Email,
			"name":           user.Name,
			"role":           user.Role,
			"email_verified": user.EmailVerified,
		},
		"api_key": apiKey,
		"message": "Organization registered successfully. Please check your email to verify your account.",
	})
}

// Login handles user login
func (api *RegistrationAPI) Login(c *gin.Context) {
	var req services.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid request",
			"details": err.Error(),
		})
		return
	}

	response, err := api.userService.Login(c.Request.Context(), &req)
	if err != nil {
		api.logger.Warn("Login failed", map[string]interface{}{
			"email": req.Email,
			"error": err.Error(),
		})

		// Generic error message for security
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "Invalid email or password",
		})
		return
	}

	c.JSON(http.StatusOK, response)
}

// InviteUser invites a new user to the organization
func (api *RegistrationAPI) InviteUser(c *gin.Context) {
	// Get current user from context (set by auth middleware)
	currentUser, exists := c.Get("user")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	user := currentUser.(*auth.User)

	// Get organization ID from user's metadata
	orgID, ok := user.Metadata["organization_id"].(uuid.UUID)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Organization not found"})
		return
	}

	var req services.InviteUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid request",
			"details": err.Error(),
		})
		return
	}

	err := api.userService.InviteUser(c.Request.Context(), user.ID, orgID, &req)
	if err != nil {
		api.logger.Error("Failed to invite user", map[string]interface{}{
			"error":           err.Error(),
			"inviter_id":      user.ID,
			"organization_id": orgID,
			"invited_email":   req.Email,
		})

		statusCode := http.StatusInternalServerError
		message := "Failed to invite user"

		if err.Error() == "insufficient permissions to invite users" {
			statusCode = http.StatusForbidden
			message = "You don't have permission to invite users"
		} else if err.Error() == "user with this email already exists" {
			statusCode = http.StatusConflict
			message = "User with this email already exists"
		} else if err.Error() == "invitation already sent to this email" {
			statusCode = http.StatusConflict
			message = "Invitation already sent to this email"
		}

		c.JSON(statusCode, gin.H{"error": message})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Invitation sent successfully",
		"email":   req.Email,
	})
}

// AcceptInvitation accepts a user invitation
func (api *RegistrationAPI) AcceptInvitation(c *gin.Context) {
	var req services.AcceptInvitationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid request",
			"details": err.Error(),
		})
		return
	}

	user, err := api.userService.AcceptInvitation(c.Request.Context(), &req)
	if err != nil {
		api.logger.Error("Failed to accept invitation", map[string]interface{}{
			"error": err.Error(),
		})

		statusCode := http.StatusBadRequest
		message := err.Error()

		c.JSON(statusCode, gin.H{"error": message})
		return
	}

	// Auto-login the user
	loginReq := &services.LoginRequest{
		Email:    user.Email,
		Password: req.Password,
	}

	response, err := api.userService.Login(c.Request.Context(), loginReq)
	if err != nil {
		// User created but login failed
		c.JSON(http.StatusCreated, gin.H{
			"message": "Account created successfully. Please login.",
			"user": gin.H{
				"id":    user.ID,
				"email": user.Email,
				"name":  user.Name,
			},
		})
		return
	}

	c.JSON(http.StatusCreated, response)
}

// EdgeMCPAuthRequest represents the authentication request from Edge MCP
type EdgeMCPAuthRequest struct {
	EdgeMCPID string `json:"edge_mcp_id"`
	APIKey    string `json:"api_key"`
}

// EdgeMCPAuthResponse represents the authentication response to Edge MCP
type EdgeMCPAuthResponse struct {
	Success     bool              `json:"success"`
	Token       string            `json:"token,omitempty"`
	Message     string            `json:"message,omitempty"`
	TenantID    string            `json:"tenant_id,omitempty"`
	UserID      string            `json:"user_id,omitempty"`
	Credentials map[string]string `json:"credentials,omitempty"` // provider → token
}

// AuthenticateEdgeMCP handles Edge MCP authentication requests
func (api *RegistrationAPI) AuthenticateEdgeMCP(c *gin.Context) {
	var req EdgeMCPAuthRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		api.logger.Warn("Failed to decode Edge MCP auth request", map[string]interface{}{
			"error": err.Error(),
		})
		c.JSON(http.StatusBadRequest, EdgeMCPAuthResponse{
			Success: false,
			Message: "Invalid request",
		})
		return
	}

	// Validate the API key and get the user/tenant information
	user, err := api.authService.ValidateAPIKey(c.Request.Context(), req.APIKey)
	if err != nil {
		api.logger.Warn("Edge MCP authentication failed", map[string]interface{}{
			"edge_mcp_id": req.EdgeMCPID,
			"error":       err.Error(),
		})
		c.JSON(http.StatusUnauthorized, EdgeMCPAuthResponse{
			Success: false,
			Message: "Authentication failed",
		})
		return
	}

	// Generate a session token for the Edge MCP instance
	token, err := api.authService.GenerateJWT(c.Request.Context(), user)
	if err != nil {
		api.logger.Error("Failed to generate JWT for Edge MCP", map[string]interface{}{
			"edge_mcp_id": req.EdgeMCPID,
			"tenant_id":   user.TenantID.String(),
			"error":       err.Error(),
		})
		c.JSON(http.StatusInternalServerError, EdgeMCPAuthResponse{
			Success: false,
			Message: "Internal server error",
		})
		return
	}

	// Fetch user credentials (all active third-party service tokens)
	userCredsMap, err := api.credentialService.GetAllUserCredentials(
		c.Request.Context(),
		user.TenantID.String(),
		user.ID.String(),
	)

	// Convert to simple string map for response
	credentials := make(map[string]string)
	if err != nil {
		// Log warning but don't fail auth - user might not have credentials yet
		api.logger.Warn("Failed to fetch user credentials", map[string]interface{}{
			"error":     err.Error(),
			"tenant_id": user.TenantID.String(),
			"user_id":   user.ID.String(),
		})
	} else {
		// Extract primary token from each service's credentials
		for serviceType, decryptedCreds := range userCredsMap {
			// Get the "token" field from credentials map, fallback to first value
			if token, ok := decryptedCreds.Credentials["token"]; ok {
				credentials[string(serviceType)] = token
			} else if token, ok := decryptedCreds.Credentials["access_token"]; ok {
				credentials[string(serviceType)] = token
			} else {
				// Use first available credential value
				for _, value := range decryptedCreds.Credentials {
					credentials[string(serviceType)] = value
					break
				}
			}
		}
	}

	// Log successful authentication
	api.logger.Info("Edge MCP authenticated successfully", map[string]interface{}{
		"edge_mcp_id":       req.EdgeMCPID,
		"tenant_id":         user.TenantID.String(),
		"credentials_count": len(credentials),
	})

	// Send success response with tenant_id, user_id, and credentials
	c.JSON(http.StatusOK, EdgeMCPAuthResponse{
		Success:     true,
		Token:       token,
		TenantID:    user.TenantID.String(),
		UserID:      user.ID.String(),
		Credentials: credentials,
	})
}

// Placeholder implementations for other endpoints

func (api *RegistrationAPI) RefreshToken(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, gin.H{"error": "Not implemented yet"})
}

func (api *RegistrationAPI) Logout(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, gin.H{"error": "Not implemented yet"})
}

func (api *RegistrationAPI) RequestPasswordReset(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, gin.H{"error": "Not implemented yet"})
}

func (api *RegistrationAPI) ConfirmPasswordReset(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, gin.H{"error": "Not implemented yet"})
}

func (api *RegistrationAPI) VerifyEmail(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, gin.H{"error": "Not implemented yet"})
}

func (api *RegistrationAPI) ResendVerificationEmail(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, gin.H{"error": "Not implemented yet"})
}

func (api *RegistrationAPI) GetInvitationDetails(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, gin.H{"error": "Not implemented yet"})
}

func (api *RegistrationAPI) ListOrganizationUsers(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, gin.H{"error": "Not implemented yet"})
}

func (api *RegistrationAPI) UpdateUserRole(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, gin.H{"error": "Not implemented yet"})
}

func (api *RegistrationAPI) RemoveUser(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, gin.H{"error": "Not implemented yet"})
}

func (api *RegistrationAPI) GetOrganization(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, gin.H{"error": "Not implemented yet"})
}

func (api *RegistrationAPI) UpdateOrganization(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, gin.H{"error": "Not implemented yet"})
}

func (api *RegistrationAPI) GetOrganizationUsage(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, gin.H{"error": "Not implemented yet"})
}

func (api *RegistrationAPI) GetProfile(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, gin.H{"error": "Not implemented yet"})
}

func (api *RegistrationAPI) UpdateProfile(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, gin.H{"error": "Not implemented yet"})
}

func (api *RegistrationAPI) ChangePassword(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, gin.H{"error": "Not implemented yet"})
}
