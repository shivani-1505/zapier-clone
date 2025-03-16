package jira

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/auditcue/integration-framework/internal/config"
	"github.com/auditcue/integration-framework/internal/integrations/common"
)

// AuthHandler implements the AuthHandler interface for Jira
type AuthHandler struct {
	config config.JiraConfig
	client *http.Client
}

// NewAuthHandler creates a new auth handler for Jira
func NewAuthHandler(config config.JiraConfig) *AuthHandler {
	return &AuthHandler{
		config: config,
		client: &http.Client{Timeout: 30 * time.Second},
	}
}

// GetAuthURL returns the URL to redirect the user to for OAuth
func (h *AuthHandler) GetAuthURL(state string) string {
	// Build the authorization URL
	authURL, err := url.Parse(h.config.AuthURL)
	if err != nil {
		return ""
	}

	q := authURL.Query()
	q.Set("audience", "api.atlassian.com")
	q.Set("client_id", h.config.ClientID)
	q.Set("scope", "read:jira-work write:jira-work read:jira-user offline_access")
	q.Set("redirect_uri", h.config.RedirectURL)
	q.Set("state", state)
	q.Set("response_type", "code")
	q.Set("prompt", "consent")
	authURL.RawQuery = q.Encode()

	return authURL.String()
}

// HandleCallback processes the OAuth callback
func (h *AuthHandler) HandleCallback(ctx context.Context, code string) (*common.AuthData, error) {
	// Prepare the token request
	data := url.Values{}
	data.Set("grant_type", "authorization_code")
	data.Set("client_id", h.config.ClientID)
	data.Set("client_secret", h.config.ClientSecret)
	data.Set("code", code)
	data.Set("redirect_uri", h.config.RedirectURL)

	// Send the token request
	req, err := http.NewRequestWithContext(ctx, "POST", h.config.TokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, fmt.Errorf("failed to create token request: %w", err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := h.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send token request: %w", err)
	}
	defer resp.Body.Close()

	// Read and parse the response
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read token response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("token request failed with status %d: %s", resp.StatusCode, string(body))
	}

	var tokenResp struct {
		AccessToken  string `json:"access_token"`
		TokenType    string `json:"token_type"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int    `json:"expires_in"`
		Scope        string `json:"scope"`
	}

	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return nil, fmt.Errorf("failed to parse token response: %w", err)
	}

	// Calculate expires_at
	expiresAt := time.Now().Add(time.Duration(tokenResp.ExpiresIn) * time.Second)

	// Return auth data
	return &common.AuthData{
		AccessToken:  tokenResp.AccessToken,
		TokenType:    tokenResp.TokenType,
		RefreshToken: tokenResp.RefreshToken,
		ExpiresAt:    expiresAt,
	}, nil
}

// RefreshToken refreshes an expired OAuth token
func (h *AuthHandler) RefreshToken(ctx context.Context, authData *common.AuthData) (*common.AuthData, error) {
	// Check if we have a refresh token
	if authData.RefreshToken == "" {
		return nil, fmt.Errorf("no refresh token available")
	}

	// Prepare the refresh request
	data := url.Values{}
	data.Set("grant_type", "refresh_token")
	data.Set("client_id", h.config.ClientID)
	data.Set("client_secret", h.config.ClientSecret)
	data.Set("refresh_token", authData.RefreshToken)

	// Send the refresh request
	req, err := http.NewRequestWithContext(ctx, "POST", h.config.TokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, fmt.Errorf("failed to create refresh request: %w", err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := h.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send refresh request: %w", err)
	}
	defer resp.Body.Close()

	// Read and parse the response
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read refresh response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("refresh request failed with status %d: %s", resp.StatusCode, string(body))
	}

	var refreshResp struct {
		AccessToken  string `json:"access_token"`
		TokenType    string `json:"token_type"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int    `json:"expires_in"`
		Scope        string `json:"scope"`
	}

	if err := json.Unmarshal(body, &refreshResp); err != nil {
		return nil, fmt.Errorf("failed to parse refresh response: %w", err)
	}

	// Calculate expires_at
	expiresAt := time.Now().Add(time.Duration(refreshResp.ExpiresIn) * time.Second)

	// Return updated auth data
	return &common.AuthData{
		AccessToken:  refreshResp.AccessToken,
		TokenType:    refreshResp.TokenType,
		RefreshToken: refreshResp.RefreshToken,
		ExpiresAt:    expiresAt,
	}, nil
}

// ValidateAuth validates authentication credentials
func (h *AuthHandler) ValidateAuth(ctx context.Context, authData *common.AuthData) error {
	// Check if token is expired
	if time.Now().After(authData.ExpiresAt) {
		// Try to refresh the token
		refreshedAuth, err := h.RefreshToken(ctx, authData)
		if err != nil {
			return fmt.Errorf("token expired and refresh failed: %w", err)
		}

		// Update the auth data with refreshed token
		*authData = *refreshedAuth
	}

	// Make a test request to the API
	client := &JiraClient{
		config:     h.config,
		httpClient: h.client,
	}

	// Try to get accessible resources as a simple validation
	_, err := client.getAccessibleResources(ctx, authData.AccessToken)
	if err != nil {
		return fmt.Errorf("failed to validate token: %w", err)
	}

	return nil
}
