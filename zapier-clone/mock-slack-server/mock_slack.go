package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gorilla/mux"
)

// SlackMessage represents a message sent to Slack
type SlackMessage struct {
	ChannelID string        `json:"channel_id"`
	Text      string        `json:"text"`
	ThreadTS  string        `json:"thread_ts,omitempty"`
	Blocks    []interface{} `json:"blocks,omitempty"`
	Timestamp string        `json:"timestamp"`
}

// SlackCommand represents a slash command from Slack
type SlackCommand struct {
	Command     string `json:"command"`
	Text        string `json:"text"`
	UserID      string `json:"user_id"`
	UserName    string `json:"user_name"`
	ChannelID   string `json:"channel_id"`
	ChannelName string `json:"channel_name"`
	ResponseURL string `json:"response_url"`
	TriggerID   string `json:"trigger_id"`
}

// SlackInteraction represents an interaction with Slack buttons/menus
type SlackInteraction struct {
	Type        string            `json:"type"`
	ActionID    string            `json:"action_id"`
	BlockID     string            `json:"block_id"`
	User        map[string]string `json:"user"`
	Channel     map[string]string `json:"channel"`
	ActionValue string            `json:"value"`
	ActionTS    string            `json:"action_ts"`
	MessageTS   string            `json:"message_ts"`
	CallbackID  string            `json:"callback_id"`
	ResponseURL string            `json:"response_url"`
	TriggerID   string            `json:"trigger_id"`
}

// MockDatabase holds our mock Slack data
var MockDatabase = struct {
	Messages map[string]SlackMessage
	Threads  map[string][]SlackMessage
	Channels map[string]string
	Users    map[string]string
}{
	Messages: make(map[string]SlackMessage),
	Threads:  make(map[string][]SlackMessage),
	Channels: map[string]string{
		"C12345": "general",
		"C67890": "grc-alerts",
		"C54321": "audit",
	},
	Users: map[string]string{
		"U12345": "john.doe",
		"U67890": "jane.smith",
		"U54321": "audit.bot",
	},
}

// ServiceNowSlackMapping maps ServiceNow IDs to Slack threads
var ServiceNowSlackMapping = map[string]string{}

func main() {
	r := mux.NewRouter()

	// Slack API endpoints
	r.HandleFunc("/api/chat.postMessage", handlePostMessage).Methods("POST")
	r.HandleFunc("/api/chat.update", handleUpdateMessage).Methods("POST")
	r.HandleFunc("/api/chat.postEphemeral", handlePostEphemeral).Methods("POST")
	r.HandleFunc("/api/reactions.add", handleAddReaction).Methods("POST")

	// Channel endpoints
	r.HandleFunc("/api/conversations.list", handleListChannels).Methods("GET", "POST")
	r.HandleFunc("/api/conversations.history", handleChannelHistory).Methods("GET", "POST")

	// User endpoints
	r.HandleFunc("/api/users.list", handleListUsers).Methods("GET", "POST")
	r.HandleFunc("/api/users.info", handleUserInfo).Methods("GET", "POST")

	// Webhook endpoints (these would be in your application)
	r.HandleFunc("/api/slack/commands", handleReceiveCommand).Methods("POST")
	r.HandleFunc("/api/slack/interactions", handleReceiveInteraction).Methods("POST")

	// Internal response URL endpoint (used by the mock)
	r.HandleFunc("/mock_response", handleMockResponseURL).Methods("POST")

	// Webhook triggers (special endpoints to simulate sending webhooks to your app)
	r.HandleFunc("/trigger_command", triggerCommand).Methods("POST")
	r.HandleFunc("/trigger_interaction", triggerInteraction).Methods("POST")

	// Health check and UI
	r.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	}).Methods("GET")

	r.HandleFunc("/", handleUI).Methods("GET")

	// Start server
	port := "3002" // Different port from ServiceNow and Jira mocks
	fmt.Printf("Starting mock Slack server on port %s...\n", port)
	log.Fatal(http.ListenAndServe(":"+port, r))
}

// Slack API handler implementations

func handlePostMessage(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// Parse the request
	var channelID, text, threadTS string
	var blocks []interface{}

	contentType := r.Header.Get("Content-Type")
	if strings.Contains(contentType, "application/json") {
		var messageData map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&messageData); err != nil {
			http.Error(w, "Invalid JSON body", http.StatusBadRequest)
			return
		}

		if channel, ok := messageData["channel"].(string); ok {
			channelID = channel
		}
		if txt, ok := messageData["text"].(string); ok {
			text = txt
		}
		if thread, ok := messageData["thread_ts"].(string); ok {
			threadTS = thread
		}
		if blks, ok := messageData["blocks"].([]interface{}); ok {
			blocks = blks
		}
	} else {
		if err := r.ParseForm(); err != nil {
			http.Error(w, "Invalid form data", http.StatusBadRequest)
			return
		}

		channelID = r.FormValue("channel")
		text = r.FormValue("text")
		threadTS = r.FormValue("thread_ts")
		blocksJSON := r.FormValue("blocks")
		if blocksJSON != "" {
			if err := json.Unmarshal([]byte(blocksJSON), &blocks); err != nil {
				// Ignore blocks if invalid
				blocks = nil
			}
		}
	}

	// Validate required fields
	if channelID == "" {
		http.Error(w, "Missing required field: channel", http.StatusBadRequest)
		return
	}

	// Generate timestamp
	messageTS := fmt.Sprintf("%d.%d", time.Now().Unix(), time.Now().Nanosecond()/1000000)

	// Create the message
	message := SlackMessage{
		ChannelID: channelID,
		Text:      text,
		ThreadTS:  threadTS,
		Blocks:    blocks,
		Timestamp: messageTS,
	}

	// Store the message
	MockDatabase.Messages[messageTS] = message

	// If this is a reply, add to threads
	if threadTS != "" {
		threadMessages, exists := MockDatabase.Threads[threadTS]
		if !exists {
			threadMessages = []SlackMessage{}
		}
		threadMessages = append(threadMessages, message)
		MockDatabase.Threads[threadTS] = threadMessages
	}

	// Check for ServiceNow ID in the text to create mappings
	if strings.Contains(text, "AUDIT-") || strings.Contains(text, "RISK-") {
		// This is simplified - in reality you'd need more sophisticated parsing
		for _, word := range strings.Fields(text) {
			if strings.HasPrefix(word, "AUDIT-") || strings.HasPrefix(word, "RISK-") {
				ServiceNowSlackMapping[word] = messageTS
				break
			}
		}
	}

	// Log the message
	log.Printf("[MOCK SLACK] Message posted to %s: %s (ts: %s)\n", channelID, text, messageTS)

	// Return success
	json.NewEncoder(w).Encode(map[string]interface{}{
		"ok":      true,
		"channel": channelID,
		"ts":      messageTS,
		"message": map[string]interface{}{
			"text":   text,
			"user":   "U54321", // Audit bot user ID
			"bot_id": "B12345",
			"ts":     messageTS,
		},
	})
}

func handleUpdateMessage(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// Parse the request
	var channelID, messageTS, text string
	var blocks []interface{}

	contentType := r.Header.Get("Content-Type")
	if strings.Contains(contentType, "application/json") {
		var messageData map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&messageData); err != nil {
			http.Error(w, "Invalid JSON body", http.StatusBadRequest)
			return
		}

		if channel, ok := messageData["channel"].(string); ok {
			channelID = channel
		}
		if ts, ok := messageData["ts"].(string); ok {
			messageTS = ts
		}
		if txt, ok := messageData["text"].(string); ok {
			text = txt
		}
		if blks, ok := messageData["blocks"].([]interface{}); ok {
			blocks = blks
		}
	} else {
		if err := r.ParseForm(); err != nil {
			http.Error(w, "Invalid form data", http.StatusBadRequest)
			return
		}

		channelID = r.FormValue("channel")
		messageTS = r.FormValue("ts")
		text = r.FormValue("text")
		blocksJSON := r.FormValue("blocks")
		if blocksJSON != "" {
			if err := json.Unmarshal([]byte(blocksJSON), &blocks); err != nil {
				// Ignore blocks if invalid
				blocks = nil
			}
		}
	}

	// Validate required fields
	if channelID == "" || messageTS == "" {
		http.Error(w, "Missing required fields: channel and ts", http.StatusBadRequest)
		return
	}

	// Check if message exists
	message, exists := MockDatabase.Messages[messageTS]
	if !exists {
		http.Error(w, "Message not found", http.StatusNotFound)
		return
	}

	// Update the message
	if text != "" {
		message.Text = text
	}
	if blocks != nil {
		message.Blocks = blocks
	}

	// Save the updated message
	MockDatabase.Messages[messageTS] = message

	// Log the update
	log.Printf("[MOCK SLACK] Message updated in %s (ts: %s)\n", channelID, messageTS)

	// Return success
	json.NewEncoder(w).Encode(map[string]interface{}{
		"ok":      true,
		"channel": channelID,
		"ts":      messageTS,
		"text":    message.Text,
	})
}

func handlePostEphemeral(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// Parse the request
	var channelID, userID, text string
	var blocks []interface{}

	contentType := r.Header.Get("Content-Type")
	if strings.Contains(contentType, "application/json") {
		var messageData map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&messageData); err != nil {
			http.Error(w, "Invalid JSON body", http.StatusBadRequest)
			return
		}

		if channel, ok := messageData["channel"].(string); ok {
			channelID = channel
		}
		if user, ok := messageData["user"].(string); ok {
			userID = user
		}
		if txt, ok := messageData["text"].(string); ok {
			text = txt
		}
		if blks, ok := messageData["blocks"].([]interface{}); ok {
			blocks = blks
		}
	} else {
		if err := r.ParseForm(); err != nil {
			http.Error(w, "Invalid form data", http.StatusBadRequest)
			return
		}

		channelID = r.FormValue("channel")
		userID = r.FormValue("user")
		text = r.FormValue("text")
		blocksJSON := r.FormValue("blocks")
		if blocksJSON != "" {
			if err := json.Unmarshal([]byte(blocksJSON), &blocks); err != nil {
				// Ignore blocks if invalid
				blocks = nil
			}
		}
	}

	// Validate required fields
	if channelID == "" || userID == "" {
		http.Error(w, "Missing required fields: channel and user", http.StatusBadRequest)
		return
	}

	// Generate timestamp (but don't store the message - it's ephemeral)
	messageTS := fmt.Sprintf("%d.%d", time.Now().Unix(), time.Now().Nanosecond()/1000000)

	// Log the ephemeral message
	log.Printf("[MOCK SLACK] Ephemeral message posted to %s for user %s: %s\n", channelID, userID, text)

	// Return success
	json.NewEncoder(w).Encode(map[string]interface{}{
		"ok":         true,
		"message_ts": messageTS,
	})
}

func handleAddReaction(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// Parse the request
	var name, channelID, timestamp string

	contentType := r.Header.Get("Content-Type")
	if strings.Contains(contentType, "application/json") {
		var reactionData map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&reactionData); err != nil {
			http.Error(w, "Invalid JSON body", http.StatusBadRequest)
			return
		}

		if n, ok := reactionData["name"].(string); ok {
			name = n
		}
		if channel, ok := reactionData["channel"].(string); ok {
			channelID = channel
		}
		if ts, ok := reactionData["timestamp"].(string); ok {
			timestamp = ts
		}
	} else {
		if err := r.ParseForm(); err != nil {
			http.Error(w, "Invalid form data", http.StatusBadRequest)
			return
		}

		name = r.FormValue("name")
		channelID = r.FormValue("channel")
		timestamp = r.FormValue("timestamp")
	}

	// Validate required fields
	if name == "" || channelID == "" || timestamp == "" {
		http.Error(w, "Missing required fields: name, channel, and timestamp", http.StatusBadRequest)
		return
	}

	// Check if message exists
	_, exists := MockDatabase.Messages[timestamp]
	if !exists {
		http.Error(w, "Message not found", http.StatusNotFound)
		return
	}

	// Log the reaction (we don't actually store reactions in this mock)
	log.Printf("[MOCK SLACK] Reaction :%s: added to message in %s (ts: %s)\n", name, channelID, timestamp)

	// Return success
	json.NewEncoder(w).Encode(map[string]interface{}{
		"ok": true,
	})
}

func handleListChannels(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// Convert channels to Slack format
	var channels []map[string]interface{}
	for id, name := range MockDatabase.Channels {
		channels = append(channels, map[string]interface{}{
			"id":          id,
			"name":        name,
			"is_channel":  true,
			"is_group":    false,
			"is_im":       false,
			"created":     time.Now().AddDate(0, 0, -30).Unix(),
			"creator":     "U12345",
			"is_archived": false,
			"is_general":  id == "C12345",
			"members":     []string{"U12345", "U67890", "U54321"},
			"topic": map[string]interface{}{
				"value":    "Channel topic",
				"creator":  "U12345",
				"last_set": time.Now().AddDate(0, 0, -15).Unix(),
			},
			"purpose": map[string]interface{}{
				"value":    "Channel purpose",
				"creator":  "U12345",
				"last_set": time.Now().AddDate(0, 0, -15).Unix(),
			},
		})
	}

	// Return the channel list
	json.NewEncoder(w).Encode(map[string]interface{}{
		"ok":       true,
		"channels": channels,
	})
}

func handleChannelHistory(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// Parse the request
	var channelID, oldest, latest, threadTS string
	var limit int

	if r.Method == "GET" {
		channelID = r.URL.Query().Get("channel")
		oldest = r.URL.Query().Get("oldest")
		latest = r.URL.Query().Get("latest")
		threadTS = r.URL.Query().Get("thread_ts")
		if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
			fmt.Sscanf(limitStr, "%d", &limit)
		}
	} else {
		if err := r.ParseForm(); err != nil {
			http.Error(w, "Invalid form data", http.StatusBadRequest)
			return
		}

		channelID = r.FormValue("channel")
		oldest = r.FormValue("oldest")
		latest = r.FormValue("latest")
		threadTS = r.FormValue("thread_ts")
		if limitStr := r.FormValue("limit"); limitStr != "" {
			fmt.Sscanf(limitStr, "%d", &limit)
		}
	}

	// Validate required fields
	if channelID == "" {
		http.Error(w, "Missing required field: channel", http.StatusBadRequest)
		return
	}

	// Default limit
	if limit <= 0 {
		limit = 100
	}

	// If threadTS is provided, return thread messages
	if threadTS != "" {
		threadMessages, exists := MockDatabase.Threads[threadTS]
		if !exists {
			// Return empty result
			json.NewEncoder(w).Encode(map[string]interface{}{
				"ok":       true,
				"messages": []interface{}{},
				"has_more": false,
			})
			return
		}

		// Convert to Slack format and apply limit
		var messages []map[string]interface{}
		for i, msg := range threadMessages {
			if i >= limit {
				break
			}

			messages = append(messages, map[string]interface{}{
				"type":      "message",
				"user":      "U54321", // Mock user
				"text":      msg.Text,
				"ts":        msg.Timestamp,
				"thread_ts": threadTS,
			})
		}

		// Return the thread messages
		json.NewEncoder(w).Encode(map[string]interface{}{
			"ok":       true,
			"messages": messages,
			"has_more": len(threadMessages) > limit,
		})
		return
	}

	// Otherwise return channel messages (filtering by channel)
	var messages []map[string]interface{}
	for ts, msg := range MockDatabase.Messages {
		if msg.ChannelID != channelID {
			continue
		}

		// Apply time filters if provided
		if oldest != "" {
			var oldestTime float64
			fmt.Sscanf(oldest, "%f", &oldestTime)

			var msgTime float64
			fmt.Sscanf(ts, "%f", &msgTime)

			if msgTime < oldestTime {
				continue
			}
		}

		if latest != "" {
			var latestTime float64
			fmt.Sscanf(latest, "%f", &latestTime)

			var msgTime float64
			fmt.Sscanf(ts, "%f", &msgTime)

			if msgTime > latestTime {
				continue
			}
		}

		// Only include top-level messages (no replies)
		if msg.ThreadTS != "" {
			continue
		}

		// Convert to Slack format
		message := map[string]interface{}{
			"type": "message",
			"user": "U54321", // Mock user
			"text": msg.Text,
			"ts":   msg.Timestamp,
		}

		// Add blocks if present
		if len(msg.Blocks) > 0 {
			message["blocks"] = msg.Blocks
		}

		// Add thread info if this message has replies
		if _, hasReplies := MockDatabase.Threads[ts]; hasReplies {
			message["reply_count"] = len(MockDatabase.Threads[ts])
			message["latest_reply"] = MockDatabase.Threads[ts][len(MockDatabase.Threads[ts])-1].Timestamp
		}

		messages = append(messages, message)

		// Apply limit
		if len(messages) >= limit {
			break
		}
	}

	// Return the channel history
	json.NewEncoder(w).Encode(map[string]interface{}{
		"ok":       true,
		"messages": messages,
		"has_more": len(messages) == limit, // This is a simplification
	})
}

func handleListUsers(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// Convert users to Slack format
	var users []map[string]interface{}
	for id, name := range MockDatabase.Users {
		users = append(users, map[string]interface{}{
			"id":        id,
			"name":      name,
			"real_name": strings.Replace(strings.Title(strings.Replace(name, ".", " ", -1)), "Bot", "Bot User", -1),
			"is_admin":  id == "U12345",
			"is_bot":    id == "U54321",
			"profile": map[string]interface{}{
				"real_name": strings.Replace(strings.Title(strings.Replace(name, ".", " ", -1)), "Bot", "Bot User", -1),
				"email":     name + "@example.com",
				"image_48":  "https://via.placeholder.com/48",
				"image_72":  "https://via.placeholder.com/72",
			},
		})
	}

	// Return the user list
	json.NewEncoder(w).Encode(map[string]interface{}{
		"ok":      true,
		"members": users,
	})
}

func handleUserInfo(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// Parse the request
	var userID string

	if r.Method == "GET" {
		userID = r.URL.Query().Get("user")
	} else {
		if err := r.ParseForm(); err != nil {
			http.Error(w, "Invalid form data", http.StatusBadRequest)
			return
		}

		userID = r.FormValue("user")
	}

	// Validate required fields
	if userID == "" {
		http.Error(w, "Missing required field: user", http.StatusBadRequest)
		return
	}

	// Check if user exists
	name, exists := MockDatabase.Users[userID]
	if !exists {
		http.Error(w, "User not found", http.StatusNotFound)
		return
	}

	// Return the user info
	json.NewEncoder(w).Encode(map[string]interface{}{
		"ok": true,
		"user": map[string]interface{}{
			"id":        userID,
			"name":      name,
			"real_name": strings.Replace(strings.Title(strings.Replace(name, ".", " ", -1)), "Bot", "Bot User", -1),
			"is_admin":  userID == "U12345",
			"is_bot":    userID == "U54321",
			"profile": map[string]interface{}{
				"real_name": strings.Replace(strings.Title(strings.Replace(name, ".", " ", -1)), "Bot", "Bot User", -1),
				"email":     name + "@example.com",
				"image_48":  "https://via.placeholder.com/48",
				"image_72":  "https://via.placeholder.com/72",
			},
		},
	})
}

func handleReceiveCommand(w http.ResponseWriter, r *http.Request) {
	// This simulates your application's endpoint for receiving Slack slash commands
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid form data", http.StatusBadRequest)
		return
	}

	command := r.FormValue("command")
	text := r.FormValue("text")
	userID := r.FormValue("user_id")
	channelID := r.FormValue("channel_id")

	// Log the command
	log.Printf("[MOCK SLACK] Received slash command: %s %s from user %s in channel %s\n",
		command, text, userID, channelID)

	// Return an immediate response
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"response_type": "ephemeral",
		"text":          "Your command is being processed...",
	})
}

func handleReceiveInteraction(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid form data", http.StatusBadRequest)
		return
	}

	payloadStr := r.FormValue("payload")
	if payloadStr == "" {
		http.Error(w, "Missing payload", http.StatusBadRequest)
		return
	}

	var payload map[string]interface{}
	if err := json.Unmarshal([]byte(payloadStr), &payload); err != nil {
		http.Error(w, "Invalid payload JSON", http.StatusBadRequest)
		return
	}

	// Log the interaction
	log.Printf("[MOCK SLACK] Received interaction: %s\n", payloadStr)

	// Extract basic information
	interactionType, _ := payload["type"].(string)

	// Handle different interaction types
	switch interactionType {
	case "block_actions":
		// Extract action information
		actions, ok := payload["actions"].([]interface{})
		if !ok || len(actions) == 0 {
			http.Error(w, "Invalid actions format", http.StatusBadRequest)
			return
		}

		action, ok := actions[0].(map[string]interface{})
		if !ok {
			http.Error(w, "Invalid action format", http.StatusBadRequest)
			return
		}

		actionID, _ := action["action_id"].(string)
		actionValue, _ := action["value"].(string)

		// Process based on action ID
		switch actionID {
		case "assign_finding":
			// Generic response for assign finding action
			json.NewEncoder(w).Encode(map[string]interface{}{
				"response_type":    "in_channel",
				"text":             fmt.Sprintf("Finding has been assigned to user (value: %s)", actionValue),
				"replace_original": true,
			})

		case "resolve_finding":
			// Generic response for resolve finding action
			json.NewEncoder(w).Encode(map[string]interface{}{
				"response_type":    "in_channel",
				"text":             fmt.Sprintf("Finding has been resolved with reason: %s", actionValue),
				"replace_original": true,
			})

		default:
			// Generic response
			json.NewEncoder(w).Encode(map[string]interface{}{
				"response_type": "ephemeral",
				"text":          fmt.Sprintf("Action '%s' with value '%s' received and being processed...", actionID, actionValue),
			})
		}

	case "view_submission":
		// Handle form submissions
		json.NewEncoder(w).Encode(map[string]interface{}{
			"response_action": "clear",
		})

	case "view_closed":
		// Handle modal closing
		// No response needed
		w.WriteHeader(http.StatusOK)

	default:
		// Generic response for other interaction types
		json.NewEncoder(w).Encode(map[string]interface{}{
			"response_type": "ephemeral",
			"text":          "Interaction received and being processed...",
		})
	}
}

func handleMockResponseURL(w http.ResponseWriter, r *http.Request) {
	// This simulates the response_url endpoint in Slack
	var response map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&response); err != nil {
		http.Error(w, "Invalid JSON body", http.StatusBadRequest)
		return
	}

	// Log the response
	log.Printf("[MOCK SLACK] Response URL received payload: %v\n", response)

	// Always return success
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"ok":true}`))
}

func triggerCommand(w http.ResponseWriter, r *http.Request) {
	var requestData struct {
		Command    string `json:"command"`
		Text       string `json:"text"`
		UserID     string `json:"user_id"`
		UserName   string `json:"user_name"`
		ChannelID  string `json:"channel_id"`
		WebhookURL string `json:"webhook_url"`
	}

	if err := json.NewDecoder(r.Body).Decode(&requestData); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Set defaults if not provided
	if requestData.Command == "" {
		requestData.Command = "/grc"
	}
	if requestData.UserID == "" {
		requestData.UserID = "U12345"
	}
	if requestData.UserName == "" {
		requestData.UserName = "john.doe"
	}
	if requestData.ChannelID == "" {
		requestData.ChannelID = "C12345"
	}

	userName := "unknown-user"
	if name, exists := MockDatabase.Users[requestData.UserName]; exists {
		userName = name
	}

	channelName := "unknown-channel"
	if name, exists := MockDatabase.Channels[requestData.ChannelID]; exists {
		channelName = name
	}
	// Build the form data
	formData := url.Values{
		"command":      []string{requestData.Command},
		"text":         []string{requestData.Text},
		"user_id":      []string{requestData.UserID},
		"user_name":    []string{userName},
		"channel_id":   []string{requestData.ChannelID},
		"channel_name": []string{channelName},
		"team_id":      []string{"T12345"},
		"team_domain":  []string{"mockteam"},
		"response_url": []string{"http://localhost:3002/mock_response"},
		"trigger_id":   []string{fmt.Sprintf("trigger.%d", time.Now().Unix())},
	}

	// Get the webhook URL from the request or use default
	webhookURL := requestData.WebhookURL
	if webhookURL == "" {
		webhookURL = "http://localhost:8080/api/slack/commands"
	}

	// Send the webhook as form data (as Slack does)
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.PostForm(webhookURL, formData)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error sending webhook: %v", err), http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	// Read the response
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error reading response: %v", err), http.StatusInternalServerError)
		return
	}

	// Return status and the response from your application
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":     "success",
		"message":    fmt.Sprintf("Slack command sent to %s", webhookURL),
		"command":    requestData.Command,
		"text":       requestData.Text,
		"response":   string(respBody),
		"webhook_id": fmt.Sprintf("mock-slack-command-%d", time.Now().UnixNano()),
	})
}

func triggerInteraction(w http.ResponseWriter, r *http.Request) {
	var requestData struct {
		Type       string                 `json:"type"`
		ActionID   string                 `json:"action_id"`
		Value      string                 `json:"value"`
		UserID     string                 `json:"user_id"`
		ChannelID  string                 `json:"channel_id"`
		MessageTS  string                 `json:"message_ts"`
		WebhookURL string                 `json:"webhook_url"`
		Blocks     []interface{}          `json:"blocks"`
		CustomData map[string]interface{} `json:"custom_data"`
	}

	if err := json.NewDecoder(r.Body).Decode(&requestData); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Set defaults if not provided
	if requestData.Type == "" {
		requestData.Type = "block_actions"
	}
	if requestData.ActionID == "" {
		requestData.ActionID = "mock_action"
	}
	if requestData.UserID == "" {
		requestData.UserID = "U12345"
	}
	if requestData.ChannelID == "" {
		requestData.ChannelID = "C12345"
	}
	if requestData.MessageTS == "" {
		requestData.MessageTS = fmt.Sprintf("%d.%d", time.Now().Unix()-100, time.Now().Nanosecond()/1000000)
	}

	// Build the payload object based on interaction type
	payload := map[string]interface{}{
		"type":         requestData.Type,
		"user":         map[string]string{"id": requestData.UserID, "name": MockDatabase.Users[requestData.UserID]},
		"channel":      map[string]string{"id": requestData.ChannelID, "name": MockDatabase.Channels[requestData.ChannelID]},
		"team":         map[string]string{"id": "T12345", "domain": "mockteam"},
		"api_app_id":   "A12345",
		"token":        "mock_token",
		"trigger_id":   fmt.Sprintf("trigger.%d", time.Now().Unix()),
		"response_url": "http://localhost:3002/mock_response",
	}

	// Add type-specific data
	switch requestData.Type {
	case "block_actions":
		action := map[string]interface{}{
			"action_id": requestData.ActionID,
			"block_id":  "mock_block",
			"value":     requestData.Value,
			"type":      "button",
			"action_ts": fmt.Sprintf("%d.%d", time.Now().Unix(), time.Now().Nanosecond()/1000000),
		}

		// Use provided blocks or create default
		blocks := requestData.Blocks
		if blocks == nil || len(blocks) == 0 {
			blocks = []interface{}{
				map[string]interface{}{
					"type": "section",
					"text": map[string]interface{}{
						"type": "mrkdwn",
						"text": "Mock message text",
					},
				},
				map[string]interface{}{
					"type": "actions",
					"elements": []interface{}{
						map[string]interface{}{
							"type": "button",
							"text": map[string]interface{}{
								"type": "plain_text",
								"text": "Mock Button",
							},
							"action_id": requestData.ActionID,
							"value":     requestData.Value,
						},
					},
				},
			}
		}

		payload["message"] = map[string]interface{}{
			"ts":     requestData.MessageTS,
			"text":   "Mock message text",
			"blocks": blocks,
		}

		payload["actions"] = []interface{}{action}

	case "view_submission":
		payload["view"] = map[string]interface{}{
			"id":    "V12345",
			"type":  "modal",
			"title": map[string]string{"text": "Mock Modal", "type": "plain_text"},
			"state": map[string]interface{}{
				"values": requestData.CustomData,
			},
		}

	case "view_closed":
		payload["view"] = map[string]interface{}{
			"id":    "V12345",
			"type":  "modal",
			"title": map[string]string{"text": "Mock Modal", "type": "plain_text"},
		}
	}

	// Add any custom data
	if requestData.CustomData != nil {
		for k, v := range requestData.CustomData {
			payload[k] = v
		}
	}

	// Encode the payload
	payloadStr, err := json.Marshal(payload)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error encoding payload: %v", err), http.StatusInternalServerError)
		return
	}

	// Create form data with the payload
	formData := url.Values{
		"payload": []string{string(payloadStr)},
	}

	// Get the webhook URL from the request or use default
	webhookURL := requestData.WebhookURL
	if webhookURL == "" {
		webhookURL = "http://localhost:8080/api/slack/interactions"
	}

	// Send the webhook
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.PostForm(webhookURL, formData)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error sending webhook: %v", err), http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	// Read the response
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error reading response: %v", err), http.StatusInternalServerError)
		return
	}

	// Return status and the response from your application
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":     "success",
		"message":    fmt.Sprintf("Slack interaction sent to %s", webhookURL),
		"type":       requestData.Type,
		"action_id":  requestData.ActionID,
		"response":   string(respBody),
		"webhook_id": fmt.Sprintf("mock-slack-interaction-%d", time.Now().UnixNano()),
	})
}

func handleUI(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(`
        <!DOCTYPE html>
        <html>
        <head>
            <title>Mock Slack Server</title>
            <style>
                body { font-family: Arial, sans-serif; margin: 40px; line-height: 1.6; }
                h1, h2, h3 { color: #4a154b; }
                .section { margin: 20px 0; padding: 15px; background: #f5f5f5; border-radius: 5px; }
                .endpoint { margin: 15px 0; padding: 15px; background: #fff; border: 1px solid #ddd; border-radius: 4px; }
                .btn { display: inline-block; padding: 8px 16px; background: #4a154b; color: white; 
                       text-decoration: none; border-radius: 4px; margin-right: 10px; border: none; cursor: pointer; }
                .btn:hover { background: #611f69; }
                input, select, textarea { padding: 8px; margin: 5px 0; width: 100%; box-sizing: border-box; border: 1px solid #ddd; border-radius: 4px; }
                textarea { height: 100px; font-family: monospace; }
                label { font-weight: bold; }
                .form-group { margin-bottom: 10px; }
                .response-area { background: #f9f9f9; border: 1px solid #ddd; padding: 10px; border-radius: 4px; margin-top: 15px; display: none; }
                .header { display: flex; justify-content: space-between; align-items: center; }
                .status { padding: 5px 10px; border-radius: 10px; font-size: 12px; font-weight: bold; }
                .status.online { background: #2eb886; color: white; }
            </style>
        </head>
        <body>
            <div class="header">
                <h1>Mock Slack Server</h1>
                <span class="status online">Online - Port 3002</span>
            </div>
            
            <div class="section">
                <h2>Trigger Slack Commands</h2>
                <div class="endpoint">
                    <h3>Send Slash Command</h3>
                    <form id="commandForm">
                        <div class="form-group">
                            <label>Command:</label>
                            <input type="text" name="command" value="/grc">
                        </div>
                        <div class="form-group">
                            <label>Text:</label>
                            <input type="text" name="text" value="list findings">
                        </div>
                        <div class="form-group">
                            <label>Channel:</label>
                            <select name="channelID">
                                <option value="C12345">general</option>
                                <option value="C67890">grc-alerts</option>
                                <option value="C54321">audit</option>
                            </select>
                        </div>
                        <div class="form-group">
                            <label>User:</label>
                            <select name="userID">
                                <option value="U12345">john.doe</option>
                                <option value="U67890">jane.smith</option>
                                <option value="U54321">audit.bot</option>
                            </select>
                        </div>
                        <div class="form-group">
                            <label>Webhook URL:</label>
                            <input type="text" name="webhookURL" value="http://localhost:8080/api/slack/commands">
                        </div>
                        <button type="submit" class="btn">Send Command</button>
                        <div class="response-area" id="commandResponse">
                            <h4>Response:</h4>
                            <pre id="commandResponseText"></pre>
                        </div>
                    </form>
                </div>
                
                <div class="endpoint">
                    <h3>Send Button Click Interaction</h3>
                    <form id="buttonForm">
                        <div class="form-group">
                            <label>Action ID:</label>
                            <input type="text" name="actionID" value="resolve_finding">
                        </div>
                        <div class="form-group">
                            <label>Value:</label>
                            <input type="text" name="value" value="AUDIT-123">
                        </div>
                        <div class="form-group">
                            <label>Channel:</label>
                            <select name="channelID">
                                <option value="C12345">general</option>
                                <option value="C67890">grc-alerts</option>
                                <option value="C54321">audit</option>
                            </select>
                        </div>
                        <div class="form-group">
                            <label>User:</label>
                            <select name="userID">
                                <option value="U12345">john.doe</option>
                                <option value="U67890">jane.smith</option>
                            </select>
                        </div>
                        <div class="form-group">
                            <label>Webhook URL:</label>
                            <input type="text" name="webhookURL" value="http://localhost:8080/api/slack/interactions">
                        </div>
                        <button type="submit" class="btn">Send Interaction</button>
                        <div class="response-area" id="buttonResponse">
                            <h4>Response:</h4>
                            <pre id="buttonResponseText"></pre>
                        </div>
                    </form>
                </div>
                
                <div class="endpoint">
                    <h3>Send Modal Submit Interaction</h3>
                    <form id="modalForm">
                        <div class="form-group">
                            <label>Custom JSON Data (optional):</label>
                            <textarea name="customData">{
  "block1": {
    "finding_input": {
      "value": "This is a finding description"
    }
  }
}</textarea>
                        </div>
                        <div class="form-group">
                            <label>User:</label>
                            <select name="userID">
                                <option value="U12345">john.doe</option>
                                <option value="U67890">jane.smith</option>
                            </select>
                        </div>
                        <div class="form-group">
                            <label>Webhook URL:</label>
                            <input type="text" name="webhookURL" value="http://localhost:8080/api/slack/interactions">
                        </div>
                        <button type="submit" class="btn">Submit Modal</button>
                        <div class="response-area" id="modalResponse">
                            <h4>Response:</h4>
                            <pre id="modalResponseText"></pre>
                        </div>
                    </form>
                </div>
            </div>
            
            <div class="section">
                <h2>API Endpoints</h2>
                <div class="endpoint">
                    <p>Use these endpoints to test your Slack integration:</p>
                    <ul>
                        <li><strong>Post Message:</strong> http://localhost:3002/api/chat.postMessage</li>
                        <li><strong>Update Message:</strong> http://localhost:3002/api/chat.update</li>
                        <li><strong>Post Ephemeral:</strong> http://localhost:3002/api/chat.postEphemeral</li>
                        <li><strong>Add Reaction:</strong> http://localhost:3002/api/reactions.add</li>
                        <li><strong>List Channels:</strong> http://localhost:3002/api/conversations.list</li>
                        <li><strong>Channel History:</strong> http://localhost:3002/api/conversations.history</li>
                        <li><strong>List Users:</strong> http://localhost:3002/api/users.list</li>
                        <li><strong>User Info:</strong> http://localhost:3002/api/users.info</li>
                    </ul>
                    <p>Webhook endpoints to implement in your application:</p>
                    <ul>
                        <li><strong>Slash Commands:</strong> http://localhost:8080/api/slack/commands</li>
                        <li><strong>Interactions:</strong> http://localhost:8080/api/slack/interactions</li>
                    </ul>
                    <p>Mock triggers (to simulate Slack sending webhooks to your app):</p>
                    <ul>
                        <li><strong>Trigger Command:</strong> http://localhost:3002/trigger_command</li>
                        <li><strong>Trigger Interaction:</strong> http://localhost:3002/trigger_interaction</li>
                    </ul>
                </div>
            </div>
            
            <script>
                // Helper function for form submission
                function handleFormSubmit(formId, endpoint, responseAreaId, responseTextId) {
                    document.getElementById(formId).addEventListener('submit', function(e) {
                        e.preventDefault();
                        const formData = new FormData(this);
                        
                        // Build the request data
                        const requestData = {};
                        for (const [key, value] of formData.entries()) {
                            if (key === 'customData') {
                                try {
                                    requestData[key] = JSON.parse(value);
                                } catch (err) {
                                    alert('Invalid JSON in custom data field');
                                    return;
                                }
                            } else {
                                requestData[key] = value;
                            }
                        }
                        
                        // Send the request
                        fetch(endpoint, {
                            method: 'POST',
                            headers: {
                                'Content-Type': 'application/json',
                            },
                            body: JSON.stringify(requestData)
                        })
                        .then(response => response.json())
                        .then(data => {
                            // Show the response
                            document.getElementById(responseAreaId).style.display = 'block';
                            document.getElementById(responseTextId).textContent = JSON.stringify(data, null, 2);
                        })
                        .catch(error => {
                            document.getElementById(responseAreaId).style.display = 'block';
                            document.getElementById(responseTextId).textContent = 'Error: ' + error.message;
                        });
                    });
                }
                
                // Set up form handlers
                handleFormSubmit('commandForm', '/trigger_command', 'commandResponse', 'commandResponseText');
                handleFormSubmit('buttonForm', '/trigger_interaction', 'buttonResponse', 'buttonResponseText');
                
                // Special handling for modal form
                document.getElementById('modalForm').addEventListener('submit', function(e) {
                    e.preventDefault();
                    const formData = new FormData(this);
                    
                    const requestData = {
                        type: 'view_submission',
                        userID: formData.get('userID'),
                        webhookURL: formData.get('webhookURL')
                    };
                    
                    // Parse custom data if provided
                    try {
                        const customDataStr = formData.get('customData');
                        if (customDataStr && customDataStr.trim() !== '') {
                            requestData.customData = JSON.parse(customDataStr);
                        }
                    } catch (err) {
                        alert('Invalid JSON in custom data field');
                        return;
                    }
                    
                    // Send the request
                    fetch('/trigger_interaction', {
                        method: 'POST',
                        headers: {
                            'Content-Type': 'application/json',
                        },
                        body: JSON.stringify(requestData)
                    })
                    .then(response => response.json())
                    .then(data => {
                        // Show the response
                        document.getElementById('modalResponse').style.display = 'block';
                        document.getElementById('modalResponseText').textContent = JSON.stringify(data, null, 2);
                    })
                    .catch(error => {
                        document.getElementById('modalResponse').style.display = 'block';
                        document.getElementById('modalResponseText').textContent = 'Error: ' + error.message;
                    });
                });
            </script>
        </body>
        </html>
    `))
}
