package slack

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/shivani-1505/zapier-clone/internal/database"
)

// Function variable for getting slack token that will be set in main.go
var GetSlackToken func(userID string) (string, error)

// GetSlackUsers fetches all users from a Slack channel for a specific user's Slack workspace
func GetSlackUsers(userID, channelID, teamID string) ([]string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	log.Printf("🔍 GetSlackUsers called for userID=%s, channelID=%s, teamID=%s", userID, channelID, teamID)
	// Validate input parameters
	if userID == "" {
		log.Printf("❌ UserID is empty")
		return nil, fmt.Errorf("userID cannot be empty")
	}
	if channelID == "" {
		log.Printf("❌ ChannelID is empty")
		return nil, fmt.Errorf("channelID cannot be empty")
	}
	if teamID == "" {
		log.Printf("❌ TeamID is empty")
		return nil, fmt.Errorf("teamID cannot be empty")
	}
	// Get the Slack token for this team directly
	slackToken, err := database.GetSlackTokenForTeam(teamID)
	if err != nil {
		// Try to get token from credManager as fallback
		log.Printf("⚠️ Couldn't get token from team store for team %s, trying credential manager: %v", teamID, err)

		if GetSlackToken == nil {
			log.Printf("❌ GetSlackToken function not initialized")
			return nil, fmt.Errorf("slack token retrieval function not initialized")
		}

		slackToken, err = GetSlackToken(userID)
		if err != nil {
			log.Printf("❌ Failed to get Slack token from any source for userID=%s: %v", userID, err)
			return nil, fmt.Errorf("failed to retrieve Slack token: %v", err)
		}
	}
	// Validate Slack token
	err = ValidateSlackToken(slackToken)
	if err != nil {
		log.Printf("❌ Invalid Slack token: %v", err)
		// Try using fallback token
		fallbackToken, fallbackErr := database.GetSlackTokenForTeam("FALLBACK_TEAM")
		if fallbackErr != nil {
			log.Printf("❌ No fallback token available: %v", fallbackErr)
			return nil, fmt.Errorf("invalid Slack token and no fallback available: %v", err)
		}
		// Validate fallback token
		if validateErr := ValidateSlackToken(fallbackToken); validateErr != nil {
			log.Printf("❌ Invalid fallback token: %v", validateErr)
			return nil, fmt.Errorf("both primary and fallback tokens are invalid")
		}
		log.Printf("⚠️ Using fallback Slack token instead")
		slackToken = fallbackToken
	}
	// Construct API URL with additional query parameters
	url := fmt.Sprintf("https://slack.com/api/conversations.members?channel=%s&limit=1000", channelID)
	log.Printf("🌐 Fetching channel members from URL: %s", url)
	// Create request with context and comprehensive headers
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		log.Printf("❌ Failed to create HTTP request: %v", err)
		return nil, fmt.Errorf("failed to create request: %v", err)
	}
	// Set comprehensive headers
	req.Header.Set("Authorization", "Bearer "+slackToken)
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "AuditCue Integration Service")
	// Create HTTP client with timeout and transport settings
	client := &http.Client{
		Timeout: 15 * time.Second,
		Transport: &http.Transport{
			MaxIdleConns:        10,
			IdleConnTimeout:     30 * time.Second,
			DisableCompression:  false,
			TLSHandshakeTimeout: 10 * time.Second,
		},
	}
	// Execute request with comprehensive error handling
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("❌ HTTP request failed: %v", err)
		return nil, fmt.Errorf("HTTP request failed: %v", err)
	}
	defer resp.Body.Close()
	// Check HTTP status code
	if resp.StatusCode != http.StatusOK {
		log.Printf("❌ Unexpected HTTP status: %d", resp.StatusCode)
		return nil, fmt.Errorf("unexpected HTTP status: %d", resp.StatusCode)
	}
	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("❌ Failed to read response body: %v", err)
		return nil, fmt.Errorf("failed to read response body: %v", err)
	}
	// Don't log the full raw response
	log.Printf("✅ Received response from Slack API for channel members")
	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		log.Printf("❌ Failed to parse JSON response: %v", err)
		return nil, fmt.Errorf("failed to parse JSON: %v", err)
	}
	// Validate Slack API response
	if ok, exists := result["ok"].(bool); !exists || !ok {
		errorMsg := "unknown error"
		if errStr, exists := result["error"].(string); exists {
			errorMsg = errStr
		}
		log.Printf("❌ Slack API Error: %s", errorMsg)
		return nil, fmt.Errorf("slack API error: %s", errorMsg)
	}
	// Extract user IDs
	userIDs, ok := result["members"].([]interface{})
	if !ok {
		log.Printf("❌ Failed to extract members from response")
		return nil, fmt.Errorf("no members found in response")
	}
	log.Printf("✅ Found %d potential members", len(userIDs))
	// Limit concurrent email lookups
	maxConcurrent := 10
	semaphore := make(chan struct{}, maxConcurrent)
	var emails []string
	var emailsMutex sync.Mutex
	var wg sync.WaitGroup
	// Process user IDs concurrently
	for _, id := range userIDs {
		slackUserID, ok := id.(string)
		if !ok {
			log.Printf("⚠️ Skipping invalid user ID: %v", id)
			continue
		}
		wg.Add(1)
		go func(userID string, slackUserID string, teamID string) {
			defer wg.Done()
			semaphore <- struct{}{}        // Acquire semaphore slot
			defer func() { <-semaphore }() // Release semaphore slot
			// Create a per-user context with shorter timeout
			userCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
			defer cancel()
			email, err := GetUserEmail(userCtx, userID, slackUserID, teamID)
			if err != nil {
				log.Printf("⚠️ Failed to get email for user %s: %v", slackUserID, err)
				return
			}
			if email != "" {
				emailsMutex.Lock()
				emails = append(emails, email)
				emailsMutex.Unlock()
			}
		}(userID, slackUserID, teamID)
	}
	// Wait for all goroutines to complete
	wg.Wait()
	// Log final results
	log.Printf("📧 Retrieved %d unique emails", len(emails))
	if len(emails) == 0 {
		log.Printf("⚠️ WARNING: No emails found for channel members")
		return nil, fmt.Errorf("no emails found for channel members")
	}
	return emails, nil
}

// GetUserEmail retrieves a user's email with context timeout support
func GetUserEmail(ctx context.Context, ownerID, slackUserID, teamID string) (string, error) {
	reqID := fmt.Sprintf("email-req-%d", time.Now().UnixNano())
	log.Printf("[%s] 🔍 Looking up email for Slack user %s", reqID, slackUserID)
	// Get the Slack token for this team
	slackToken, err := database.GetSlackTokenForTeam(teamID)
	if err != nil {
		// Try to get token from credManager as fallback
		log.Printf("[%s] ⚠️ Couldn't get token from team store, trying credential manager: %v", reqID, err)

		if GetSlackToken == nil {
			log.Printf("[%s] ❌ GetSlackToken function not initialized", reqID)
			return "", fmt.Errorf("slack token retrieval function not initialized")
		}

		slackToken, err = GetSlackToken(ownerID)
		if err != nil {
			log.Printf("[%s] ❌ Failed to get Slack token from any source: %v", reqID, err)
			return "", fmt.Errorf("failed to get Slack token: %v", err)
		}
	}
	// Validate token
	if err := ValidateSlackToken(slackToken); err != nil {
		log.Printf("[%s] ❌ Invalid Slack token: %v", reqID, err)
		// Try using fallback token
		fallbackToken, fallbackErr := database.GetSlackTokenForTeam("FALLBACK_TEAM")
		if fallbackErr != nil {
			log.Printf("[%s] ❌ No fallback token available: %v", reqID, fallbackErr)
			return "", fmt.Errorf("invalid Slack token and no fallback available: %v", err)
		}
		log.Printf("[%s] ⚠️ Using fallback Slack token", reqID)
		slackToken = fallbackToken
	}
	url := "https://slack.com/api/users.info?user=" + slackUserID
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		log.Printf("[%s] ❌ Failed to create request: %v", reqID, err)
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+slackToken)
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("[%s] ❌ HTTP request failed: %v", reqID, err)
		return "", err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("[%s] ❌ Failed to read response body: %v", reqID, err)
		return "", err
	}
	// Don't log the full response
	log.Printf("[%s] ✅ Received response from Slack API", reqID)
	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		log.Printf("[%s] ❌ Failed to parse JSON: %v", reqID, err)
		return "", err
	}
	// Check for Slack API errors
	if ok, exists := result["ok"].(bool); !exists || !ok {
		errorMsg := "unknown error"
		if err, exists := result["error"].(string); exists {
			errorMsg = err
		}
		log.Printf("[%s] ❌ Slack API error: %v", reqID, errorMsg)
		return "", fmt.Errorf("failed to fetch user info: %v", errorMsg)
	}
	user, ok := result["user"].(map[string]interface{})
	if !ok {
		log.Printf("[%s] ❌ User object not found", reqID)
		return "", fmt.Errorf("user object not found")
	}
	profile, ok := user["profile"].(map[string]interface{})
	if !ok {
		log.Printf("[%s] ❌ Profile object not found", reqID)
		return "", fmt.Errorf("profile object not found")
	}
	// Only log relevant information, not the entire profile
	if email, exists := profile["email"].(string); exists && email != "" {
		log.Printf("[%s] ✅ Found email for user %s: %s", reqID, slackUserID, email)
		return email, nil
	}
	log.Printf("[%s] ⚠️ No email found for user %s", reqID, slackUserID)
	return "", nil
}

// ValidateSlackToken performs a simple validation check on a Slack token
func ValidateSlackToken(token string) error {
	if token == "" {
		return fmt.Errorf("slack token is empty")
	}
	// Basic format check - Slack tokens typically start with xoxb- or xoxp-
	if !strings.HasPrefix(token, "xoxb-") && !strings.HasPrefix(token, "xoxp-") {
		return fmt.Errorf("slack token has invalid format")
	}
	// Additional length check
	if len(token) < 20 {
		return fmt.Errorf("slack token appears too short (%d characters)", len(token))
	}
	return nil
}
