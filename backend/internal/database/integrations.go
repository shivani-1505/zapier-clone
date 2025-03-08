package database

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"sync"

	_ "github.com/lib/pq"
)

// IntegrationStore handles the storage and retrieval of team to user mappings and tokens
type IntegrationStore struct {
	db *sql.DB
	mu sync.RWMutex
}

// Global integration store instance
var integrationStore *IntegrationStore

// NewIntegrationStore creates a new integration store
func NewIntegrationStore(db *sql.DB) *IntegrationStore {
	return &IntegrationStore{
		db: db,
	}
}

// InitIntegrationTable ensures the integrations table exists with slack_token column
func (store *IntegrationStore) InitIntegrationTable() error {
	// First, create the basic table if it doesn't exist
	_, err := store.db.Exec(`
        CREATE TABLE IF NOT EXISTS team_to_user (
            team_id VARCHAR(50) PRIMARY KEY,
            user_id VARCHAR(50) NOT NULL,
            slack_token VARCHAR(100),
            created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
        )
    `)
	if err != nil {
		log.Printf("Error creating team_to_user table: %v", err)
		return err
	}
	// Check if the slack_token column exists, add it if it doesn't
	var columnExists bool
	err = store.db.QueryRow(`
        SELECT EXISTS (
            SELECT 1 
            FROM information_schema.columns 
            WHERE table_name = 'team_to_user' AND column_name = 'slack_token'
        )
    `).Scan(&columnExists)
	if err != nil {
		log.Printf("Error checking if slack_token column exists: %v", err)
		return err
	}
	if !columnExists {
		_, err = store.db.Exec(`
            ALTER TABLE team_to_user 
            ADD COLUMN slack_token VARCHAR(100)
        `)
		if err != nil {
			log.Printf("Error adding slack_token column: %v", err)
			return err
		}
		log.Printf("Added slack_token column to team_to_user table")
	}
	return nil
}

// RegisterIntegration adds or updates a team to user mapping with slack token
func RegisterIntegration(teamID, userID string, slackToken string) error {
	if integrationStore == nil || integrationStore.db == nil {
		return fmt.Errorf("integration store not initialized")
	}
	integrationStore.mu.Lock()
	defer integrationStore.mu.Unlock()
	_, err := integrationStore.db.Exec(`
        INSERT INTO team_to_user (team_id, user_id, slack_token)
        VALUES ($1, $2, $3)
        ON CONFLICT (team_id) DO UPDATE
        SET user_id = EXCLUDED.user_id,
            slack_token = EXCLUDED.slack_token
    `, teamID, userID, slackToken)
	if err != nil {
		return fmt.Errorf("failed to register integration: %v", err)
	}
	log.Printf("Successfully registered/updated integration for team %s with user %s", teamID, userID)
	return nil
}

// GetUserIDForTeam retrieves the user ID associated with a team
func GetUserIDForTeam(teamID string) (string, bool) {
	if integrationStore == nil || integrationStore.db == nil {
		log.Printf("Integration store not initialized")
		return "", false
	}
	integrationStore.mu.RLock()
	defer integrationStore.mu.RUnlock()
	var userID string
	err := integrationStore.db.QueryRow(`
        SELECT user_id FROM team_to_user WHERE team_id = $1
    `, teamID).Scan(&userID)
	if err != nil {
		if err != sql.ErrNoRows {
			log.Printf("Error retrieving user ID for team %s: %v", teamID, err)
		}
		return "", false
	}
	return userID, true
}

// GetSlackTokenForTeam retrieves the Slack token associated with a team
func GetSlackTokenForTeam(teamID string) (string, error) {
	if integrationStore == nil || integrationStore.db == nil {
		return "", fmt.Errorf("integration store not initialized")
	}
	integrationStore.mu.RLock()
	defer integrationStore.mu.RUnlock()
	var slackToken string
	err := integrationStore.db.QueryRow(`
        SELECT slack_token FROM team_to_user WHERE team_id = $1
    `, teamID).Scan(&slackToken)
	if err != nil {
		if err == sql.ErrNoRows {
			return "", fmt.Errorf("no integration found for team ID: %s", teamID)
		}
		return "", fmt.Errorf("error retrieving Slack token for team %s: %v", teamID, err)
	}
	if slackToken == "" {
		return "", fmt.Errorf("slack token is empty for team ID: %s", teamID)
	}
	return slackToken, nil
}

// UpdateSlackToken updates the Slack token for a team
func UpdateSlackToken(teamID, slackToken string) error {
	if integrationStore == nil || integrationStore.db == nil {
		return fmt.Errorf("integration store not initialized")
	}
	integrationStore.mu.Lock()
	defer integrationStore.mu.Unlock()
	result, err := integrationStore.db.Exec(`
        UPDATE team_to_user 
        SET slack_token = $2
        WHERE team_id = $1
    `, teamID, slackToken)
	if err != nil {
		return fmt.Errorf("failed to update Slack token: %v", err)
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("error checking rows affected: %v", err)
	}
	if rowsAffected == 0 {
		return fmt.Errorf("no team found with ID: %s", teamID)
	}
	log.Printf("Successfully updated Slack token for team %s", teamID)
	return nil
}

// GetAllIntegrations retrieves all team to user mappings with their tokens
func GetAllIntegrations() map[string]map[string]string {
	if integrationStore == nil || integrationStore.db == nil {
		log.Printf("Integration store not initialized")
		return make(map[string]map[string]string)
	}
	integrationStore.mu.RLock()
	defer integrationStore.mu.RUnlock()
	rows, err := integrationStore.db.Query(`
        SELECT team_id, user_id, slack_token FROM team_to_user
    `)
	if err != nil {
		log.Printf("Error retrieving integrations: %v", err)
		return make(map[string]map[string]string)
	}
	defer rows.Close()
	integrations := make(map[string]map[string]string)
	for rows.Next() {
		var teamID, userID, slackToken string
		if err := rows.Scan(&teamID, &userID, &slackToken); err != nil {
			log.Printf("Error scanning integration row: %v", err)
			continue
		}
		integrations[teamID] = map[string]string{
			"user_id":     userID,
			"slack_token": slackToken,
		}
	}
	return integrations
}

// InitIntegrationStore initializes the integration store with a database connection
func InitIntegrationStore() error {
	connStr := os.Getenv("DATABASE_URL")
	if connStr == "" {
		connStr = "postgres://postgres:postgres@localhost:5432/auditcue?sslmode=disable"
	}
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		return fmt.Errorf("failed to connect to database: %v", err)
	}
	if err := db.Ping(); err != nil {
		db.Close()
		return fmt.Errorf("failed to ping database: %v", err)
	}
	integrationStore = NewIntegrationStore(db)
	if err := integrationStore.InitIntegrationTable(); err != nil {
		return err
	}
	// Set up a fallback token if provided via environment variable
	fallbackToken := os.Getenv("FALLBACK_SLACK_TOKEN")
	if fallbackToken != "" {
		fallbackTeamID := "FALLBACK_TEAM"
		fallbackUserID := "fallback-system-user"
		err := RegisterIntegration(fallbackTeamID, fallbackUserID, fallbackToken)
		if err != nil {
			log.Printf("Warning: Failed to register fallback integration: %v", err)
		} else {
			log.Printf("Registered fallback Slack token with team ID: %s", fallbackTeamID)
		}
	}
	return nil
}

// CloseDB closes the database connection
func CloseDB() {
	if integrationStore != nil && integrationStore.db != nil {
		log.Println("Closing database connection...")
		integrationStore.db.Close()
	}
}

// CheckDatabaseConnection verifies the database connection is active
func CheckDatabaseConnection() error {
	if integrationStore == nil || integrationStore.db == nil {
		return fmt.Errorf("database connection not initialized")
	}
	return integrationStore.db.Ping()
}
