package main

import (
	"bytes"
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
		"C67890": "risk-management",
		"C54321": "audit",
		"C11111": "compliance-team",
		"C22222": "incident-response",
		"C33333": "vendor-risk",
		"C44444": "regulatory-updates",
		"C55555": "grc-reports",
		"C66666": "control-testing",
	},
	Users: map[string]string{
		"U12345": "john.doe",
		"U67890": "jane.smith",
		"U54321": "audit.bot",
	},
}

// ServiceNowSlackMapping maps ServiceNow IDs to Slack threads
var ServiceNowSlackMapping = map[string]string{}

func init() {
	// Initialize empty maps
	MockDatabase.Messages = make(map[string]SlackMessage)
	MockDatabase.Threads = make(map[string][]SlackMessage)

	// Keep the pre-defined channels and users
	log.Println("[MOCK SLACK] Initializing clean message database")
}

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

	r.HandleFunc("/test_servicenow_integration", testServiceNowIntegration).Methods("POST")
	// Add this to your main() function
	r.HandleFunc("/test_connectivity", func(w http.ResponseWriter, r *http.Request) {
		log.Println("[MOCK SLACK] Received connectivity test")
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"ok","message":"Slack mock server is running"}`))
	}).Methods("GET")

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

	requestBody, err := io.ReadAll(r.Body)
	if err != nil {
		log.Printf("[MOCK SLACK] ERROR reading request body: %v", err)
		http.Error(w, "Error reading request body", http.StatusBadRequest)
		return
	}
	// Log the raw request body for debugging
	log.Printf("[MOCK SLACK] Received POST message request body: %s", string(requestBody))

	// Replace the consumed body with a new reader to use in the handler
	r.Body = io.NopCloser(bytes.NewBuffer(requestBody))

	// Parse the request
	var channelID, text, threadTS string
	var blocks []interface{}

	contentType := r.Header.Get("Content-Type")
	if strings.Contains(contentType, "application/json") {
		var messageData map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&messageData); err != nil {
			log.Printf("[MOCK SLACK] ERROR decoding JSON body: %v", err)
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
			log.Printf("[MOCK SLACK] ERROR parsing form data: %v", err)
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
		log.Printf("[MOCK SLACK] ERROR: Missing required field: channel")
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
	if strings.Contains(text, "AUDIT-") || strings.Contains(text, "RISK-") ||
		strings.Contains(text, "INC") || strings.Contains(text, "VR") ||
		strings.Contains(text, "REG") || strings.Contains(text, "TEST") {
		// This is simplified - in reality you'd need more sophisticated parsing
		for _, word := range strings.Fields(text) {
			if strings.HasPrefix(word, "AUDIT-") || strings.HasPrefix(word, "RISK") ||
				strings.HasPrefix(word, "INC") || strings.HasPrefix(word, "VR") ||
				strings.HasPrefix(word, "REG") || strings.HasPrefix(word, "TEST") {
				ServiceNowSlackMapping[word] = messageTS
				break
			}
		}
	}

	// Log the message
	log.Printf("[MOCK SLACK] Message posted to %s: %s (ts: %s)\n", channelID, text, messageTS)

	// Return success
	responseData := map[string]interface{}{
		"ok":      true,
		"channel": channelID,
		"ts":      messageTS,
		"message": map[string]interface{}{
			"text":   text,
			"user":   "U54321", // Audit bot user ID
			"bot_id": "B12345",
			"ts":     messageTS,
		},
	}

	responseJSON, err := json.Marshal(responseData)
	if err != nil {
		log.Printf("[MOCK SLACK] ERROR marshaling response: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	log.Printf("[MOCK SLACK] Returning response: %s", string(responseJSON))
	w.Write(responseJSON)
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
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid form data", http.StatusBadRequest)
		return
	}

	// Log the command details more verbosely for debugging
	command := r.FormValue("command")
	text := r.FormValue("text")
	channelID := r.FormValue("channel_id")
	userID := r.FormValue("user_id")

	log.Printf("[MOCK SLACK] Received command: %s with text: %s from user %s in channel %s",
		command, text, userID, channelID)

	// Create a complete copy of the form values to forward
	formValues := url.Values{}
	for key, values := range r.Form {
		formValues[key] = values
	}

	// Forward to ServiceNow with correctly formatted form data
	client := &http.Client{Timeout: 10 * time.Second} // Increased timeout
	resp, err := client.PostForm("http://localhost:3000/api/slack/commands", formValues)
	if err != nil {
		log.Printf("[MOCK SLACK] ERROR forwarding command to ServiceNow: %v", err)
		http.Error(w, "Error forwarding command: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	// Read the response body
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("[MOCK SLACK] ERROR reading response from ServiceNow: %v", err)
		http.Error(w, "Error reading response", http.StatusInternalServerError)
		return
	}

	log.Printf("[MOCK SLACK] ServiceNow response: %s", string(respBody))

	// Return ServiceNow's response
	w.Header().Set("Content-Type", "application/json")
	w.Write(respBody)

	// Check if this is an in_channel response
	var response map[string]interface{}
	if json.Unmarshal(respBody, &response) == nil {
		if responseType, ok := response["response_type"].(string); ok && responseType == "in_channel" {
			if respText, ok := response["text"].(string); ok {
				// Post to channel
				postMessageToChannel(channelID, respText)
			}
		}
	}
}

// Replace handleReceiveInteraction with this enhanced version
func handleReceiveInteraction(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	payloadStr := r.FormValue("payload")

	// Forward to ServiceNow
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.PostForm("http://localhost:3000/api/slack/interactions", url.Values{
		"payload": {payloadStr},
	})
	if err != nil {
		http.Error(w, "Error forwarding interaction", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	// Return ServiceNow's response
	respBody, _ := io.ReadAll(resp.Body)
	w.Header().Set("Content-Type", "application/json")
	w.Write(respBody)
}

// Add this helper function
func postMessageToChannel(channelID, text string) {
	messageData := map[string]interface{}{
		"channel": channelID,
		"text":    text,
	}

	jsonData, _ := json.Marshal(messageData)
	http.Post("http://localhost:3002/api/chat.postMessage", "application/json", bytes.NewBuffer(jsonData))
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
	if name, exists := MockDatabase.Users[requestData.UserID]; exists {
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
		webhookURL = "http://localhost:8081/api/slack/commands"
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

func testServiceNowIntegration(w http.ResponseWriter, r *http.Request) {
	// Forward the request to ServiceNow
	client := &http.Client{Timeout: 10 * time.Second}
	jsonData, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Error reading request body", http.StatusInternalServerError)
		return
	}

	// Forward the exact request body to ServiceNow
	resp, err := client.Post("http://localhost:3000/servicenow/create_risk",
		"application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		http.Error(w, fmt.Sprintf("Error forwarding to ServiceNow: %v", err), http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	// Read the response from ServiceNow
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		http.Error(w, "Error reading ServiceNow response", http.StatusInternalServerError)
		return
	}

	// Return ServiceNow's response to the client
	w.Header().Set("Content-Type", "application/json")
	w.Write(respBody)
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
		webhookURL = "http://localhost:8081/api/slack/interactions"
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
					body { font-family: Arial, sans-serif; margin: 0; padding: 0; }
					.app-container { display: flex; height: 100vh; }
					.sidebar { width: 220px; background: #4a154b; color: white; padding: 10px 0; overflow-y: auto; }
					.sidebar h3 { padding: 0 15px; margin-bottom: 10px; font-size: 16px; }
					.channel-list { list-style: none; padding: 0; margin: 0; }
					.channel-list li { padding: 5px 15px; cursor: pointer; }
					.channel-list li:hover { background: #611f69; }
					.channel-list li.active { background: #1164a3; font-weight: bold; }
					.content { flex: 1; display: flex; flex-direction: column; }
					.channel-header { padding: 10px 20px; border-bottom: 1px solid #ddd; display: flex; justify-content: space-between; align-items: center; }
					.message-area { flex: 1; padding: 20px; overflow-y: auto; background: #f8f8f8; }
					.message-composer { padding: 15px; border-top: 1px solid #ddd; display: flex; }
					.message-composer input { flex: 1; padding: 10px; border: 1px solid #ddd; border-radius: 4px; margin-right: 10px; }
					.message-composer button { background: #007a5a; color: white; border: none; padding: 10px 15px; border-radius: 4px; cursor: pointer; }
					.message { margin-bottom: 15px; display: flex; }
					.message .avatar { width: 36px; height: 36px; background: #ddd; border-radius: 3px; margin-right: 10px; }
					.message .message-content { flex: 1; }
					.message .header { margin-bottom: 5px; }
					.message .sender { font-weight: bold; margin-right: 8px; }
					.message .time { color: #616061; font-size: 12px; }
					.message .body { line-height: 1.46; }
					.message .buttons { margin-top: 8px; display: flex; gap: 5px; }
					.message .button { padding: 5px 10px; background: #fff; border: 1px solid #ddd; border-radius: 4px; cursor: pointer; font-size: 13px; }
					.message .button:hover { background: #f8f8f8; }
					.message .button.primary { background: #1264a3; color: white; border-color: #1264a3; }
					.message .button.primary:hover { background: #0b5394; }
					.message .button.danger { background: #e01e5a; color: white; border-color: #e01e5a; }
					.message .button.danger:hover { background: #c41e56; }
					.interactions { margin-top: 30px; padding: 20px; border-top: 1px solid #ddd; }
					.command-input { display: flex; margin-bottom: 20px; }
					.command-input input { flex: 1; padding: 8px; border: 1px solid #ddd; border-radius: 4px 0 0 4px; }
					.command-input button { background: #007a5a; color: white; border: none; padding: 8px 15px; border-radius: 0 4px 4px 0; cursor: pointer; }
					.response { background: #f8f8f8; padding: 10px; border-radius: 4px; margin-top: 10px; display: none; }
					.notification { position: fixed; bottom: 20px; right: 20px; background: #007a5a; color: white; padding: 15px; border-radius: 4px; display: none; }
					.risk-high { border-left: 4px solid #e01e5a; padding-left: 10px; }
					.risk-medium { border-left: 4px solid #ecb22e; padding-left: 10px; }
					.risk-low { border-left: 4px solid #2eb67d; padding-left: 10px; }
					.tabs { display: flex; border-bottom: 1px solid #ddd; }
					.tab { padding: 10px 15px; cursor: pointer; }
					.tab.active { border-bottom: 2px solid #1264a3; font-weight: bold; }
					.tab-content { display: none; padding: 15px; }
					.tab-content.active { display: block; }
					#slack-commands { list-style: none; padding: 0; }
					#slack-commands li { margin-bottom: 5px; }
					.reload-button { background: #007a5a; color: white; border: none; padding: 5px 10px; border-radius: 4px; cursor: pointer; }
					.timestamp { color: #616061; font-size: 12px; margin-bottom: 10px; }
				</style>
			</head>
			<body>
				<div class="app-container">
					<div class="sidebar">
						<h3>Channels</h3>
						<ul class="channel-list" id="channel-list">
							<li data-channel="C12345" class="active"># general</li>
							<li data-channel="C67890"># risk-management</li>
							<li data-channel="C54321"># audit</li>
							<li data-channel="C11111"># compliance-team</li>
							<li data-channel="C22222"># incident-response</li>
							<li data-channel="C33333"># vendor-risk</li>
							<li data-channel="C44444"># regulatory-updates</li>
							<li data-channel="C55555"># grc-reports</li>
							<li data-channel="C66666"># control-testing</li>
						</ul>
					</div>
					<div class="content">
						<div class="channel-header">
							<h2 id="current-channel"># general</h2>
							<button class="reload-button" id="reload-messages">Reload Messages</button>
						</div>
						<div class="message-area" id="message-area">
							<div class="timestamp">Today</div>
							<!-- Messages will be displayed here -->
						</div>
						
						<div class="interactions">
							<div class="tabs">
								<div class="tab active" data-tab="command-tab">Slash Commands</div>
								<div class="tab" data-tab="interaction-tab">Button Actions</div>
								<div class="tab" data-tab="integration-tab">ServiceNow Integration</div>
							</div>
							
							<div class="tab-content active" id="command-tab">
								<h3>Send Slash Command</h3>
								<div class="command-input">
									<input type="text" id="slash-command" placeholder="/grc-status" value="/grc-status">
									<button id="send-command">Send Command</button>
								</div>
								<div class="response" id="command-response"></div>
								
								<h4>Available Commands</h4>
								<ul id="slack-commands">
									<li><strong>/grc-status</strong> - Get current GRC status overview</li>
									<li><strong>/upload-evidence</strong> TASK_ID URL - Upload evidence for compliance task</li>
									<li><strong>/incident-update</strong> INC_ID details - Update an incident</li>
									<li><strong>/resolve-incident</strong> INC_ID resolution - Resolve an incident</li>
									<li><strong>/submit-test</strong> TEST_ID PASS|FAIL details - Submit test results</li>
									<li><strong>/resolve-finding</strong> AUDIT_ID resolution - Resolve an audit finding</li>
									<li><strong>/update-vendor</strong> VR_ID STATUS details - Update vendor status</li>
									<li><strong>/assess-impact</strong> REG_ID details - Add regulatory impact assessment</li>
									<li><strong>/plan-implementation</strong> REG_ID plan - Create implementation plan</li>
								</ul>
							</div>
							
							<div class="tab-content" id="interaction-tab">
								<h3>Send Button Click</h3>
								<form id="button-form">
									<div style="margin-bottom: 10px;">
										<label>Action ID:</label>
										<select id="action-id" style="width: 100%; padding: 5px;">
											<option value="acknowledge_incident">acknowledge_incident</option>
											<option value="resolve_incident">resolve_incident</option>
											<option value="update_incident">update_incident</option>
											<option value="assign_finding">assign_finding</option>
											<option value="resolve_finding">resolve_finding</option>
											<option value="discuss_risk">discuss_risk</option>
											<option value="assign_risk">assign_risk</option>
											<option value="request_compliance_report">request_compliance_report</option>
											<option value="update_vendor_status">update_vendor_status</option>
											<option value="add_impact_assessment">add_impact_assessment</option>
											<option value="create_implementation_plan">create_implementation_plan</option>
											<option value="submit_test_results">submit_test_results</option>
										</select>
									</div>
									<div style="margin-bottom: 10px;">
										<label>Value (item ID):</label>
										<input type="text" id="action-value" style="width: 100%; padding: 5px;" placeholder="e.g., ack_incident_INC0001">
									</div>
									<div style="margin-bottom: 10px;">
										<label>User:</label>
										<select id="action-user" style="width: 100%; padding: 5px;">
											<option value="U12345">john.doe</option>
											<option value="U67890">jane.smith</option>
										</select>
									</div>
									<button type="submit" style="background: #007a5a; color: white; border: none; padding: 8px 15px; border-radius: 4px; cursor: pointer;">
										Send Interaction
									</button>
								</form>
								<div class="response" id="interaction-response"></div>
							</div>
							
							<div class="tab-content" id="integration-tab">
								<h3>Test ServiceNow Integration</h3>
								<form id="create-risk-form">
									<div style="margin-bottom: 10px;">
										<label>Risk Title:</label>
										<input type="text" id="risk-title" style="width: 100%; padding: 5px;" placeholder="e.g., Database vulnerability">
									</div>
									<div style="margin-bottom: 10px;">
										<label>Description:</label>
										<textarea id="risk-description" style="width: 100%; padding: 5px;" placeholder="Describe the risk..."></textarea>
									</div>
									<div style="margin-bottom: 10px;">
										<label>Severity:</label>
										<select id="risk-severity" style="width: 100%; padding: 5px;">
											<option value="Critical">Critical</option>
											<option value="High">High</option>
											<option value="Medium">Medium</option>
											<option value="Low">Low</option>
										</select>
									</div>
									<div style="margin-bottom: 10px;">
										<label>Category:</label>
										<select id="risk-category" style="width: 100%; padding: 5px;">
											<option value="Cybersecurity">Cybersecurity</option>
											<option value="Financial">Financial</option>
											<option value="Operational">Operational</option>
											<option value="Compliance">Compliance</option>
											<option value="Strategic">Strategic</option>
										</select>
									</div>
									<button type="submit" style="background: #007a5a; color: white; border: none; padding: 8px 15px; border-radius: 4px; cursor: pointer;">
										Create Risk in ServiceNow
									</button>
								</form>
								<div class="response" id="integration-response"></div>
							</div>
						</div>
					</div>
				</div>
				
				<div class="notification" id="notification">New message received!</div>
				
				<script>
					// Current channel state
					let currentChannel = 'C12345';
					
					// Function to load messages for a channel
					function loadMessages(channelId) {
						fetch('/api/conversations.history?channel=' + channelId)
							.then(response => response.json())
							.then(data => {
								const messageArea = document.getElementById('message-area');
								messageArea.innerHTML = '<div class="timestamp">Today</div>';
								
								if (data.messages && data.messages.length > 0) {
									// Sort messages by timestamp (newest first)
									data.messages.sort((a, b) => parseFloat(b.ts) - parseFloat(a.ts));
									
									// Add messages to the DOM
									data.messages.forEach(msg => {
										const messageElement = createMessageElement(msg);
										messageArea.appendChild(messageElement);
									});
									
									// Check if this is an initial load or a refresh
									if (window.lastMessageTimestamp === undefined) {
										// Initial load
										if (data.messages.length > 0) {
											window.lastMessageTimestamp = parseFloat(data.messages[0].ts);
										}
									} else {
										// Check for new messages
										if (data.messages.length > 0 && parseFloat(data.messages[0].ts) > window.lastMessageTimestamp) {
											showNotification('New message received!');
											window.lastMessageTimestamp = parseFloat(data.messages[0].ts);
										}
									}
								} else {
									messageArea.innerHTML += '<p>No messages in this channel yet.</p>';
								}
							})
							.catch(error => {
								console.error('Error loading messages:', error);
								document.getElementById('message-area').innerHTML = 
									'<div class="timestamp">Today</div><p>Error loading messages. Please try again.</p>';
							});
					}
					
					// Function to create a message element
					function createMessageElement(msg) {
						const message = document.createElement('div');
						message.className = 'message';
						
						// Check for risk severity in message text and add appropriate class
						if (msg.text.includes('High-Severity Risk') || msg.text.includes('Critical')) {
							message.classList.add('risk-high');
						} else if (msg.text.includes('Medium-Severity Risk')) {
							message.classList.add('risk-medium');
						} else if (msg.text.includes('Low-Severity Risk')) {
							message.classList.add('risk-low');
						}
						
						// Create avatar
						const avatar = document.createElement('div');
						avatar.className = 'avatar';
						message.appendChild(avatar);
						
						// Create message content container
						const content = document.createElement('div');
						content.className = 'message-content';
						
						// Create header with sender and time
						const header = document.createElement('div');
						header.className = 'header';
						
						const sender = document.createElement('span');
						sender.className = 'sender';
						sender.textContent = msg.user ? 'User' : 'GRC Bot';
						header.appendChild(sender);
						
						const time = document.createElement('span');
						time.className = 'time';
						const date = new Date(parseFloat(msg.ts) * 1000);
						time.textContent = date.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' });
						header.appendChild(time);
						
						content.appendChild(header);
						
						// Create message body
						const body = document.createElement('div');
						body.className = 'body';
						body.textContent = msg.text;
						content.appendChild(body);
						
						// Create buttons if this is a risk or incident message
						if (msg.blocks && msg.blocks.some(block => block.type === 'actions')) {
							const buttonsContainer = document.createElement('div');
							buttonsContainer.className = 'buttons';
							
							// Check message content to determine appropriate buttons
							if (msg.text.includes('Risk')) {
								buttonsContainer.innerHTML = '' +
									'<button class="button" data-action="discuss_risk">Discuss Mitigation</button>' +
									'<button class="button" data-action="assign_risk">Assign Owner</button>' +
									'<button class="button" data-action="view_in_servicenow">View in ServiceNow</button>';
							} else if (msg.text.includes('Incident')) {
								buttonsContainer.innerHTML = '' +
									'<button class="button primary" data-action="acknowledge_incident">🚨 Acknowledge</button>' +
									'<button class="button" data-action="update_incident">📝 Add Update</button>' +
									'<button class="button danger" data-action="resolve_incident">✅ Resolve</button>';
							} else if (msg.text.includes('Finding')) {
								buttonsContainer.innerHTML = '' +
									'<button class="button" data-action="assign_finding">Assign Owner</button>' +
									'<button class="button" data-action="resolve_finding">Resolve Finding</button>' +
									'<button class="button" data-action="view_in_servicenow">View in ServiceNow</button>';
							} else if (msg.text.includes('Vendor')) {
								buttonsContainer.innerHTML = '' +
									'<button class="button" data-action="request_compliance_report">Request Report</button>' +
									'<button class="button" data-action="update_vendor_status">Update Status</button>' +
									'<button class="button" data-action="view_in_servicenow">View in ServiceNow</button>';
							} else if (msg.text.includes('Regulatory')) {
								buttonsContainer.innerHTML = '' +
									'<button class="button" data-action="add_impact_assessment">Add Impact Assessment</button>' +
									'<button class="button" data-action="create_implementation_plan">Create Implementation Plan</button>' +
									'<button class="button" data-action="view_in_servicenow">View in ServiceNow</button>';
							} else if (msg.text.includes('Test')) {
								buttonsContainer.innerHTML = '' +
									'<button class="button" data-action="submit_test_results">Submit Results</button>' +
									'<button class="button" data-action="view_in_servicenow">View in ServiceNow</button>';
							}
						
							// Add button click handlers
							const buttons = buttonsContainer.querySelectorAll('.button');
							buttons.forEach(button => {
								button.addEventListener('click', function() {
									const actionId = this.getAttribute('data-action');
									if (actionId === 'view_in_servicenow') {
										alert('This would open ServiceNow in a real implementation');
										return;
									}
									
									// Extract item ID from the message text
									let itemId = '';
									const messageText = msg.text;
									
									// Try to find a standard ID pattern
									const idMatches = messageText.match(/(RISK\d+|AUDIT-\d+|INC\d+|TEST\d+|VR\d+|REG\d+)/);
									if (idMatches && idMatches[1]) {
										itemId = idMatches[1];
									}
									
									// Format the value based on the action
									let actionValue = actionId + '_' + itemId;
									
									// Populate the interaction form
									document.getElementById('action-id').value = actionId;
									document.getElementById('action-value').value = actionValue;
									
									// Switch to interaction tab
									const tabs = document.querySelectorAll('.tab');
									tabs.forEach(tab => tab.classList.remove('active'));
									document.querySelector('.tab[data-tab="interaction-tab"]').classList.add('active');
									
									const tabContents = document.querySelectorAll('.tab-content');
									tabContents.forEach(content => content.classList.remove('active'));
									document.getElementById('interaction-tab').classList.add('active');
									
									// Scroll to the form
									document.getElementById('button-form').scrollIntoView({ behavior: 'smooth' });
								});
							});
							
							content.appendChild(buttonsContainer);
						}
						
						message.appendChild(content);
						return message;
					}
					
					// Function to send a slash command
					function sendCommand(command) {
						// First, create a loading indicator
						const responseArea = document.getElementById('command-response');
						responseArea.style.display = 'block';
						responseArea.innerHTML = 'Sending command...';
						
						// Split the command into parts
						let parts = command.split(' ');
						let commandName = parts[0];
						let commandArgs = parts.slice(1).join(' ');
						
						// Send the request to our trigger endpoint
						fetch('/trigger_command', {
							method: 'POST',
							headers: {
								'Content-Type': 'application/json'
							},
							body: JSON.stringify({
								command: commandName,
								text: commandArgs,
								user_id: 'U12345',  // Default user
								channel_id: currentChannel
							})
						})
						.then(response => response.json())
						.then(data => {
							// Display the response
							responseArea.innerHTML = '<strong>Command sent:</strong> ' + command + 
												'<br><strong>Response:</strong> ' + data.response;
							
							// Show notification
							showNotification('Command sent: ' + command);
							
							// Reload messages to show any updates
							setTimeout(() => loadMessages(currentChannel), 1000);
						})
						.catch(error => {
							console.error('Error sending command:', error);
							responseArea.innerHTML = 'Error sending command: ' + error.message;
						});
					}
					
					// Function to send a button interaction
					function sendInteraction(actionId, actionValue, userId) {
						// First, create a loading indicator
						const responseArea = document.getElementById('interaction-response');
						responseArea.style.display = 'block';
						responseArea.innerHTML = 'Sending interaction...';
						
						// Send the request to our trigger endpoint
						fetch('/trigger_interaction', {
							method: 'POST',
							headers: {
								'Content-Type': 'application/json'
							},
							body: JSON.stringify({
								type: 'block_actions',
								action_id: actionId,
								value: actionValue,
								user_id: userId,
								channel_id: currentChannel
							})
						})
						.then(response => response.json())
						.then(data => {
							// Display the response
							responseArea.innerHTML = '<strong>Interaction sent:</strong> ' + actionId + ' (' + actionValue + ')' + 
												'<br><strong>Response:</strong> ' + data.response;
							
							// Show notification
							showNotification('Interaction sent: ' + actionId);
							
							// Reload messages to show any updates
							setTimeout(() => loadMessages(currentChannel), 1000);
						})
						.catch(error => {
							console.error('Error sending interaction:', error);
							responseArea.innerHTML = 'Error sending interaction: ' + error.message;
						});
					}
					
					// Function to show a notification
					function showNotification(message) {
						const notification = document.getElementById('notification');
						notification.textContent = message;
						notification.style.display = 'block';
						
						setTimeout(() => {
							notification.style.display = 'none';
						}, 3000);
					}
					
					// Set up event listeners
					document.addEventListener('DOMContentLoaded', function() {
						// Load messages for the default channel
						loadMessages(currentChannel);
						
						// Channel switching
						const channelItems = document.querySelectorAll('.channel-list li');
						channelItems.forEach(item => {
							item.addEventListener('click', function() {
								// Update active state
								channelItems.forEach(i => i.classList.remove('active'));
								this.classList.add('active');
								
								// Update current channel and title
								currentChannel = this.getAttribute('data-channel');
								document.getElementById('current-channel').textContent = '# ' + this.textContent.trim().substring(2);
								
								// Load messages for the selected channel
								loadMessages(currentChannel);
							});
						});
						
						// Reload button
						document.getElementById('reload-messages').addEventListener('click', function() {
							loadMessages(currentChannel);
						});
						
						// Tab switching
						const tabs = document.querySelectorAll('.tab');
						tabs.forEach(tab => {
							tab.addEventListener('click', function() {
								// Update active tab
								tabs.forEach(t => t.classList.remove('active'));
								this.classList.add('active');
								
								// Update active content
								const tabId = this.getAttribute('data-tab');
								const tabContents = document.querySelectorAll('.tab-content');
								tabContents.forEach(content => content.classList.remove('active'));
								document.getElementById(tabId).classList.add('active');
							});
						});
						
						// Command form
						document.getElementById('send-command').addEventListener('click', function() {
							const command = document.getElementById('slash-command').value;
							if (command) {
								sendCommand(command);
							}
						});
						
						// Button interaction form
						document.getElementById('button-form').addEventListener('submit', function(e) {
							e.preventDefault();
							
							const actionId = document.getElementById('action-id').value;
							const actionValue = document.getElementById('action-value').value;
							const userId = document.getElementById('action-user').value;
							
							if (actionId && actionValue) {
								sendInteraction(actionId, actionValue, userId);
							}
						});
						
						// ServiceNow integration form
						document.getElementById('create-risk-form').addEventListener('submit', function(e) {
							e.preventDefault();
							
							const title = document.getElementById('risk-title').value;
							const description = document.getElementById('risk-description').value;
							const severity = document.getElementById('risk-severity').value;
							const category = document.getElementById('risk-category').value;
							
							if (!title || !description) {
								alert('Please provide a title and description');
								return;
							}
							
							const responseArea = document.getElementById('integration-response');
							responseArea.style.display = 'block';
							responseArea.innerHTML = 'Creating risk in ServiceNow...';
							
							// Send the request to create a risk in ServiceNow
							fetch('/test_servicenow_integration', {
								method: 'POST',
								headers: {
									'Content-Type': 'application/json'
								},
								body: JSON.stringify({
									title: title,
									description: description,
									severity: severity,
									category: category,
									owner: 'jane.smith'
								})
							})
							.then(response => response.json())
							.then(data => {
								// Display the response
								responseArea.innerHTML = '<strong>Risk created in ServiceNow:</strong><br>ID: ' + 
													(data.result.number || data.result.sys_id) + '<br>Title: ' + title;
								
								// Show notification
								showNotification('Risk created in ServiceNow');
								
								// Auto-switch to the risk-management channel
								const riskChannelEl = document.querySelector('.channel-list li[data-channel="C67890"]');
								if (riskChannelEl) {
									riskChannelEl.click();
								}
								
								// Wait a moment for the webhook to process, then reload messages
								setTimeout(() => loadMessages('C67890'), 2000);
							})
							.catch(error => {
								console.error('Error creating risk:', error);
								responseArea.innerHTML = 'Error creating risk: ' + error.message;
							});
						});
						
						// Auto-refresh messages every 3 seconds
						setInterval(() => {
							loadMessages(currentChannel);
						}, 3000);
					});
				</script>
			</body>
			</html>
		`))
}
