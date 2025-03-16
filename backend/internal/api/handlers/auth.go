package handlers

import (
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/auditcue/integration-framework/internal/config"
	"github.com/auditcue/integration-framework/internal/db"
	"github.com/auditcue/integration-framework/pkg/logger"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v4"
	"golang.org/x/crypto/bcrypt"
)

// AuthHandler handles authentication-related API endpoints
type AuthHandler struct {
	db     *db.Database
	config *config.Config
	logger *logger.Logger
}

// NewAuthHandler creates a new auth handler
func NewAuthHandler(cfg *config.Config, db *db.Database, logger *logger.Logger) *AuthHandler {
	return &AuthHandler{
		db:     db,
		config: cfg,
		logger: logger,
	}
}

// LoginRequest represents a request to login
type LoginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required,min=6"`
}

// RegisterRequest represents a request to register a new user
type RegisterRequest struct {
	Username string `json:"username" binding:"required,min=3,max=50"`
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required,min=6"`
	FullName string `json:"full_name"`
}

// User represents a user in the system
type User struct {
	ID        int64     `json:"id"`
	Username  string    `json:"username"`
	Email     string    `json:"email"`
	FullName  string    `json:"full_name"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Claims represents the JWT claims
type Claims struct {
	UserID int64 `json:"user_id"`
	jwt.RegisteredClaims
}

// Login handles user login
func (h *AuthHandler) Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Get the user from the database
	var userID int64
	var passwordHash string
	err := h.db.DB().QueryRow(`
		SELECT id, password_hash FROM users WHERE email = ?
	`, req.Email).Scan(&userID, &passwordHash)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid credentials"})
			return
		}
		h.logger.Error("Failed to get user", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to authenticate user"})
		return
	}

	// Verify the password
	if err := bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(req.Password)); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid credentials"})
		return
	}

	// Generate a JWT token
	token, err := h.generateToken(userID)
	if err != nil {
		h.logger.Error("Failed to generate token", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate token"})
		return
	}

	// Generate a refresh token
	refreshToken, err := h.generateRefreshToken(userID)
	if err != nil {
		h.logger.Error("Failed to generate refresh token", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate refresh token"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"token":         token,
		"refresh_token": refreshToken,
		"expires_in":    h.config.Auth.JWTExpirationMinutes * 60,
	})
}

// Register handles user registration
func (h *AuthHandler) Register(c *gin.Context) {
	var req RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Check if email already exists
	var count int
	err := h.db.DB().QueryRow(`
		SELECT COUNT(*) FROM users WHERE email = ?
	`, req.Email).Scan(&count)
	if err != nil {
		h.logger.Error("Failed to check email", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to register user"})
		return
	}
	if count > 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Email already registered"})
		return
	}

	// Check if username already exists
	err = h.db.DB().QueryRow(`
		SELECT COUNT(*) FROM users WHERE username = ?
	`, req.Username).Scan(&count)
	if err != nil {
		h.logger.Error("Failed to check username", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to register user"})
		return
	}
	if count > 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Username already taken"})
		return
	}

	// Hash the password
	passwordHash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		h.logger.Error("Failed to hash password", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to register user"})
		return
	}

	// Create the user
	var userID int64
	now := time.Now()
	err = h.db.DB().QueryRow(`
		INSERT INTO users (username, email, password_hash, full_name, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)
		RETURNING id
	`, req.Username, req.Email, passwordHash, req.FullName, now, now).Scan(&userID)
	if err != nil {
		h.logger.Error("Failed to create user", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to register user"})
		return
	}

	// Generate a JWT token
	token, err := h.generateToken(userID)
	if err != nil {
		h.logger.Error("Failed to generate token", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate token"})
		return
	}

	// Generate a refresh token
	refreshToken, err := h.generateRefreshToken(userID)
	if err != nil {
		h.logger.Error("Failed to generate refresh token", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate refresh token"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"id":            userID,
		"token":         token,
		"refresh_token": refreshToken,
		"expires_in":    h.config.Auth.JWTExpirationMinutes * 60,
	})
}

// RefreshToken handles token refresh
func (h *AuthHandler) RefreshToken(c *gin.Context) {
	var req struct {
		Token string `json:"token" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Verify the refresh token
	var userID int64
	var expiresAt time.Time
	err := h.db.DB().QueryRow(`
		SELECT user_id, expires_at FROM auth_tokens WHERE token = ?
	`, req.Token).Scan(&userID, &expiresAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid refresh token"})
			return
		}
		h.logger.Error("Failed to verify refresh token", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to refresh token"})
		return
	}

	// Check if token has expired
	if time.Now().After(expiresAt) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Refresh token expired"})
		return
	}

	// Generate a new JWT token
	token, err := h.generateToken(userID)
	if err != nil {
		h.logger.Error("Failed to generate token", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate token"})
		return
	}

	// Generate a new refresh token
	refreshToken, err := h.generateRefreshToken(userID)
	if err != nil {
		h.logger.Error("Failed to generate refresh token", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate refresh token"})
		return
	}

	// Delete the old refresh token
	_, err = h.db.DB().Exec(`DELETE FROM auth_tokens WHERE token = ?`, req.Token)
	if err != nil {
		h.logger.Error("Failed to delete old refresh token", "error", err)
	}

	c.JSON(http.StatusOK, gin.H{
		"token":         token,
		"refresh_token": refreshToken,
		"expires_in":    h.config.Auth.JWTExpirationMinutes * 60,
	})
}

// OAuthCallback handles the OAuth callback
func (h *AuthHandler) OAuthCallback(c *gin.Context) {
	service := c.Param("service")
	code := c.Query("code")
	state := c.Query("state")

	if code == "" || state == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Missing code or state parameter"})
		return
	}

	// The ConnectionHandler will handle the actual OAuth flow
	// This is just a placeholder to handle redirects
	c.HTML(http.StatusOK, "oauth_callback.html", gin.H{
		"service": service,
		"code":    code,
		"state":   state,
	})
}

// GetProfile gets the user profile
func (h *AuthHandler) GetProfile(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	var user User
	err := h.db.DB().QueryRow(`
		SELECT id, username, email, full_name, created_at, updated_at
		FROM users WHERE id = ?
	`, userID).Scan(
		&user.ID,
		&user.Username,
		&user.Email,
		&user.FullName,
		&user.CreatedAt,
		&user.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
			return
		}
		h.logger.Error("Failed to get user", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get user profile"})
		return
	}

	c.JSON(http.StatusOK, user)
}

// UpdateProfile updates the user profile
func (h *AuthHandler) UpdateProfile(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	var req struct {
		Username string `json:"username"`
		Email    string `json:"email"`
		FullName string `json:"full_name"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Start building the update query
	query := "UPDATE users SET updated_at = ?"
	args := []interface{}{time.Now()}

	if req.Username != "" {
		// Check if username is already taken
		var count int
		err := h.db.DB().QueryRow(`
			SELECT COUNT(*) FROM users WHERE username = ? AND id != ?
		`, req.Username, userID).Scan(&count)
		if err != nil {
			h.logger.Error("Failed to check username", "error", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update profile"})
			return
		}
		if count > 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Username already taken"})
			return
		}

		query += ", username = ?"
		args = append(args, req.Username)
	}

	if req.Email != "" {
		// Check if email is already taken
		var count int
		err := h.db.DB().QueryRow(`
			SELECT COUNT(*) FROM users WHERE email = ? AND id != ?
		`, req.Email, userID).Scan(&count)
		if err != nil {
			h.logger.Error("Failed to check email", "error", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update profile"})
			return
		}
		if count > 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Email already registered"})
			return
		}

		query += ", email = ?"
		args = append(args, req.Email)
	}

	if req.FullName != "" {
		query += ", full_name = ?"
		args = append(args, req.FullName)
	}

	query += " WHERE id = ?"
	args = append(args, userID)

	// Execute the update
	_, err := h.db.DB().Exec(query, args...)
	if err != nil {
		h.logger.Error("Failed to update user", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update profile"})
		return
	}

	// Get the updated user
	var user User
	err = h.db.DB().QueryRow(`
		SELECT id, username, email, full_name, created_at, updated_at
		FROM users WHERE id = ?
	`, userID).Scan(
		&user.ID,
		&user.Username,
		&user.Email,
		&user.FullName,
		&user.CreatedAt,
		&user.UpdatedAt,
	)
	if err != nil {
		h.logger.Error("Failed to get updated user", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get updated profile"})
		return
	}

	c.JSON(http.StatusOK, user)
}

// ChangePassword changes the user password
func (h *AuthHandler) ChangePassword(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	var req struct {
		CurrentPassword string `json:"current_password" binding:"required"`
		NewPassword     string `json:"new_password" binding:"required,min=6"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Get the current password hash
	var passwordHash string
	err := h.db.DB().QueryRow(`
		SELECT password_hash FROM users WHERE id = ?
	`, userID).Scan(&passwordHash)
	if err != nil {
		h.logger.Error("Failed to get user", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to change password"})
		return
	}

	// Verify the current password
	if err := bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(req.CurrentPassword)); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Current password is incorrect"})
		return
	}

	// Hash the new password
	newPasswordHash, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		h.logger.Error("Failed to hash password", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to change password"})
		return
	}

	// Update the password
	_, err = h.db.DB().Exec(`
		UPDATE users SET password_hash = ?, updated_at = ? WHERE id = ?
	`, newPasswordHash, time.Now(), userID)
	if err != nil {
		h.logger.Error("Failed to update password", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to change password"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}

// generateToken generates a JWT token for a user
func (h *AuthHandler) generateToken(userID int64) (string, error) {
	// Set the expiration time
	expirationTime := time.Now().Add(time.Duration(h.config.Auth.JWTExpirationMinutes) * time.Minute)

	// Create the JWT claims
	claims := &Claims{
		UserID: userID,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expirationTime),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now()),
			Issuer:    "auditcue-integration-framework",
		},
	}

	// Create the token
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	// Sign the token
	tokenString, err := token.SignedString([]byte(h.config.Auth.JWTSecret))
	if err != nil {
		return "", err
	}

	return tokenString, nil
}

// generateRefreshToken generates a refresh token for a user
func (h *AuthHandler) generateRefreshToken(userID int64) (string, error) {
	// Generate a random token
	refreshToken := generateRandomString(32)

	// Set the expiration time (30 days)
	expiresAt := time.Now().Add(30 * 24 * time.Hour)

	// Store the token in the database
	_, err := h.db.DB().Exec(`
		INSERT INTO auth_tokens (user_id, token, expires_at, created_at)
		VALUES (?, ?, ?, ?)
	`, userID, refreshToken, expiresAt, time.Now())
	if err != nil {
		return "", fmt.Errorf("failed to store refresh token: %w", err)
	}

	return refreshToken, nil
}
