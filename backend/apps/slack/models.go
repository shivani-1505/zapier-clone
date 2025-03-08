package slack

// SlackChallenge specifically for URL verification
type SlackChallenge struct {
	Type      string `json:"type"`
	Challenge string `json:"challenge"`
	Token     string `json:"token"`
}

// SlackEvent struct for parsing incoming Slack event JSON
type SlackEvent struct {
	Type   string `json:"type"`
	TeamID string `json:"team_id"`
	Event  struct {
		Type      string `json:"type"`
		Text      string `json:"text"`
		Channel   string `json:"channel"`
		User      string `json:"user"`
		Subtype   string `json:"subtype,omitempty"`
		BotID     string `json:"bot_id,omitempty"`
		Timestamp string `json:"ts"`
	} `json:"event"`
}
