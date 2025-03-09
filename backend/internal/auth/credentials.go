package auth

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"sync"
	
)

// CredentialsManager handles storing and retrieving user credentials
type CredentialsManager struct {
	credentialsFile string
	credentials     map[string]UserCredentials
	fallbackCreds   UserCredentials
	mu              sync.RWMutex
}

// Global credentials manager instance
var CredManager *CredentialsManager

// NewCredentialsManager creates a new credentials manager
func NewCredentialsManager(filename string, fallbackEmail, fallbackPassword string) (*CredentialsManager, error) {
	cm := &CredentialsManager{
		credentialsFile: filename,
		credentials:     make(map[string]UserCredentials),
		// Store fallback credentials
		fallbackCreds: UserCredentials{
			GmailAccount:     fallbackEmail,
			GmailAppPassword: fallbackPassword,
		},
	}
	// Try to load existing credentials
	if _, err := os.Stat(filename); err == nil {
		data, err := os.ReadFile(filename)
		if err != nil {
			return nil, fmt.Errorf("error reading credentials file: %v", err)
		}
		if err := json.Unmarshal(data, &cm.credentials); err != nil {
			return nil, fmt.Errorf("error parsing credentials file: %v", err)
		}
	}
	return cm, nil
}

// SaveCredentials adds or updates user credentials
func (cm *CredentialsManager) SaveCredentials(creds UserCredentials) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	// Ensure values are not overwritten if empty
	existingCreds, exists := cm.credentials[creds.UserID]
	if exists {
		if creds.GmailAccount == "" {
			creds.GmailAccount = existingCreds.GmailAccount
		}
		if creds.GmailAppPassword == "" {
			creds.GmailAppPassword = existingCreds.GmailAppPassword
		}
		if creds.SlackBotToken == "" {
			creds.SlackBotToken = existingCreds.SlackBotToken
		}
		if creds.SlackTeamID == "" {
			creds.SlackTeamID = existingCreds.SlackTeamID
		}
	}
	cm.credentials[creds.UserID] = creds
	// Save to file
	data, err := json.MarshalIndent(cm.credentials, "", "  ")
	if err != nil {
		return fmt.Errorf("error serializing credentials: %v", err)
	}
	if err := os.WriteFile(cm.credentialsFile, data, 0600); err != nil {
		return fmt.Errorf("error writing credentials file: %v", err)
	}
	return nil
}

// GetCredentials retrieves credentials for a specific user
func (cm *CredentialsManager) GetCredentials(userID string) (UserCredentials, error) {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	creds, ok := cm.credentials[userID]
	if !ok || creds.GmailAccount == "" || creds.GmailAppPassword == "" {
		// Log that fallback credentials are being used
		log.Printf("[%s] ⚠️ Using fallback SMTP credentials", userID)
		return cm.fallbackCreds, nil
	}
	return creds, nil
}

// GetSlackToken returns the Slack bot token for a user
func (cm *CredentialsManager) GetSlackToken(userID string) (string, error) {
	creds, err := cm.GetCredentials(userID)
	if err != nil {
		return "", err
	}
	return creds.SlackBotToken, nil
}

// InitCredentialsManager initializes the global credentials manager
func InitCredentialsManager() error {
	var err error
	CredManager, err = NewCredentialsManager(
		"user_credentials.json",
		// Add these default fallback credentials
		"connectify.workflow@gmail.com",
		"dvhv tmod qdzu jyrj",
	)
	if err != nil {
		log.Printf("Error initializing credentials manager: %v", err)
		return err
	}
	return nil
}
