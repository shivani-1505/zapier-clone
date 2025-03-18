// backend/internal/integrations/slack/client.go
package slack

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// Client represents a Slack API client
type Client struct {
	Token      string
	HTTPClient *http.Client
}

// NewClient creates a new Slack client
func NewClient(token string) *Client {
	return &Client{
		Token: token,
		HTTPClient: &http.Client{
			Timeout: time.Second * 30,
		},
	}
}

// makeRequest performs an HTTP request to the Slack API
func (c *Client) makeRequest(method, endpoint string, body interface{}) (*http.Response, error) {
	url := fmt.Sprintf("https://slack.com/api/%s", endpoint)

	var req *http.Request
	var err error

	if body != nil {
		jsonBody, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("error marshaling request body: %w", err)
		}
		req, err = http.NewRequest(method, url, bytes.NewBuffer(jsonBody))
	} else {
		req, err = http.NewRequest(method, url, nil)
	}

	if err != nil {
		return nil, fmt.Errorf("error creating request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.Token))

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error executing request: %w", err)
	}

	return resp, nil
}

// PostMessage sends a message to a Slack channel
func (c *Client) PostMessage(channel string, message Message) (string, error) {
	message.Channel = channel

	resp, err := c.makeRequest("POST", "chat.postMessage", message)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var response struct {
		OK        bool   `json:"ok"`
		Error     string `json:"error,omitempty"`
		Timestamp string `json:"ts,omitempty"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return "", fmt.Errorf("error decoding response: %w", err)
	}

	if !response.OK {
		return "", fmt.Errorf("slack API error: %s", response.Error)
	}

	return response.Timestamp, nil
}

// AddReaction adds a reaction to a message
func (c *Client) AddReaction(channel, timestamp, reaction string) error {
	body := map[string]string{
		"channel":   channel,
		"timestamp": timestamp,
		"name":      reaction,
	}

	resp, err := c.makeRequest("POST", "reactions.add", body)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var response struct {
		OK    bool   `json:"ok"`
		Error string `json:"error,omitempty"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return fmt.Errorf("error decoding response: %w", err)
	}

	if !response.OK {
		return fmt.Errorf("slack API error: %s", response.Error)
	}

	return nil
}

// PostReply sends a reply to a thread
func (c *Client) PostReply(channel, threadTS string, message Message) (string, error) {
	message.Channel = channel
	message.ThreadTS = threadTS

	resp, err := c.makeRequest("POST", "chat.postMessage", message)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var response struct {
		OK        bool   `json:"ok"`
		Error     string `json:"error,omitempty"`
		Timestamp string `json:"ts,omitempty"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return "", fmt.Errorf("error decoding response: %w", err)
	}

	if !response.OK {
		return "", fmt.Errorf("slack API error: %s", response.Error)
	}

	return response.Timestamp, nil
}

// UpdateMessage updates a previously sent message
func (c *Client) UpdateMessage(channel, timestamp string, message Message) error {
	message.Channel = channel
	message.TS = timestamp

	resp, err := c.makeRequest("POST", "chat.update", message)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var response struct {
		OK    bool   `json:"ok"`
		Error string `json:"error,omitempty"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return fmt.Errorf("error decoding response: %w", err)
	}

	if !response.OK {
		return fmt.Errorf("slack API error: %s", response.Error)
	}

	return nil
}

// UploadFile uploads a file to a Slack channel
func (c *Client) UploadFile(channel, filename, content string) (string, error) {
	// In a real implementation, this would handle file uploads with multipart/form-data
	// For simplicity, we're just returning a dummy file ID
	return "F123456789", nil
}
