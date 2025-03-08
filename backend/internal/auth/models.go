package auth

// UserCredentials represents the credentials for a specific user
type UserCredentials struct {
	UserID           string `json:"user_id"`
	SlackBotToken    string `json:"slack_bot_token"`
	SlackTeamID      string `json:"slack_team_id"`
	GmailAccount     string `json:"gmail_account"`
	GmailAppPassword string `json:"gmail_app_password"`
}

// SaveUserCredentialsRequest represents the request to save user credentials
type SaveUserCredentialsRequest struct {
	UserID           string `json:"user_id"`
	SlackBotToken    string `json:"slack_bot_token"`
	SlackTeamID      string `json:"slack_team_id"`
	GmailAccount     string `json:"gmail_account"`
	GmailAppPassword string `json:"gmail_app_password"`
}
