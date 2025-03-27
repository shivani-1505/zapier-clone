// backend/internal/integrations/servicenow/reporting.go
package servicenow

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/shivani-1505/zapier-clone/backend/internal/integrations/slack"
)

// GRCSummary represents a summary of GRC data from ServiceNow
type GRCSummary struct {
	OpenRisks              int `json:"open_risks"`
	OpenComplianceTasks    int `json:"open_compliance_tasks"`
	OpenIncidents          int `json:"open_incidents"`
	ControlTestsInProgress int `json:"control_tests_in_progress"`
	OpenAuditFindings      int `json:"open_audit_findings"`
	OpenVendorRisks        int `json:"open_vendor_risks"`
	PendingRegChanges      int `json:"pending_regulatory_changes"`
	OverdueItems           int `json:"overdue_items"`
	ComplianceScore        int `json:"compliance_score"`
}

// RiskByCategory represents risk counts by category
type RiskByCategory struct {
	Category string `json:"category"`
	Count    int    `json:"count"`
}

// ReportingHandler handles GRC reporting and dashboards
type ReportingHandler struct {
	ServiceNowClient *Client
	SlackClient      *slack.Client
}

// NewReportingHandler creates a new reporting handler
func NewReportingHandler(serviceNowClient *Client, slackClient *slack.Client) *ReportingHandler {
	return &ReportingHandler{
		ServiceNowClient: serviceNowClient,
		SlackClient:      slackClient,
	}
}

// GetGRCSummary fetches a summary of GRC data from ServiceNow
func (h *ReportingHandler) GetGRCSummary() (*GRCSummary, error) {
	resp, err := h.ServiceNowClient.makeRequest("GET", "api/now/table/sn_grc_summary", nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var response struct {
		Result GRCSummary `json:"result"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("error decoding GRC summary response: %w", err)
	}

	return &response.Result, nil
}

// GetRisksByCategory fetches risk counts by category from ServiceNow
func (h *ReportingHandler) GetRisksByCategory() ([]RiskByCategory, error) {
	resp, err := h.ServiceNowClient.makeRequest("GET", "api/now/table/sn_risk_by_category", nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var response struct {
		Result []RiskByCategory `json:"result"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("error decoding risks by category response: %w", err)
	}

	return response.Result, nil
}

// SendWeeklySummary sends a weekly GRC summary to Slack
func (h *ReportingHandler) SendWeeklySummary() error {
	// Get the GRC summary data
	summary, err := h.GetGRCSummary()
	if err != nil {
		return fmt.Errorf("error getting GRC summary: %w", err)
	}

	// Create a Slack message for the weekly summary
	message := slack.Message{
		Blocks: []slack.Block{
			{
				Type: "header",
				Text: slack.NewTextObject("plain_text", "📊 Weekly GRC Summary", true),
			},
			{
				Type: "section",
				Fields: []*slack.TextObject{
					slack.NewTextObject("mrkdwn", fmt.Sprintf("*Open Risks:*\n%d", summary.OpenRisks), false),
					slack.NewTextObject("mrkdwn", fmt.Sprintf("*Open Compliance Tasks:*\n%d", summary.OpenComplianceTasks), false),
					slack.NewTextObject("mrkdwn", fmt.Sprintf("*Open Incidents:*\n%d", summary.OpenIncidents), false),
					slack.NewTextObject("mrkdwn", fmt.Sprintf("*Control Tests In Progress:*\n%d", summary.ControlTestsInProgress), false),
					slack.NewTextObject("mrkdwn", fmt.Sprintf("*Open Audit Findings:*\n%d", summary.OpenAuditFindings), false),
					slack.NewTextObject("mrkdwn", fmt.Sprintf("*Open Vendor Risks:*\n%d", summary.OpenVendorRisks), false),
				},
			},
			{
				Type: "section",
				Fields: []*slack.TextObject{
					slack.NewTextObject("mrkdwn", fmt.Sprintf("*Pending Regulatory Changes:*\n%d", summary.PendingRegChanges), false),
					slack.NewTextObject("mrkdwn", fmt.Sprintf("*Overdue Items:*\n%d", summary.OverdueItems), false),
					slack.NewTextObject("mrkdwn", fmt.Sprintf("*Compliance Score:*\n%d%%", summary.ComplianceScore), false),
				},
			},
			{
				Type: "actions",
				Elements: []interface{}{
					map[string]interface{}{
						"type": "button",
						"text": map[string]interface{}{
							"type":  "plain_text",
							"text":  "View Detailed Report",
							"emoji": true,
						},
						"url":       fmt.Sprintf("%s/nav_to.do?uri=sn_grc_dashboard.do", h.ServiceNowClient.BaseURL),
						"action_id": "view_detailed_report",
					},
				},
			},
			{
				Type: "context",
				Elements: []interface{}{
					map[string]interface{}{
						"type": "mrkdwn",
						"text": fmt.Sprintf("Report generated on %s", time.Now().Format("Jan 2, 2006 15:04 MST")),
					},
				},
			},
		},
	}

	// Post the message to the grc-reports channel
	_, err = h.SlackClient.PostMessage(slack.ChannelMapping["reports"], message)
	if err != nil {
		return fmt.Errorf("error posting weekly summary to Slack: %w", err)
	}

	return nil
}

// SendRiskCategorySummary sends a summary of risks by category to Slack
func (h *ReportingHandler) SendRiskCategorySummary() error {
	// Get the risks by category data
	risksByCategory, err := h.GetRisksByCategory()
	if err != nil {
		return fmt.Errorf("error getting risks by category: %w", err)
	}

	// Format the risks by category as a text block
	var riskText string
	for _, riskCategory := range risksByCategory {
		riskText += fmt.Sprintf("• *%s*: %d\n", riskCategory.Category, riskCategory.Count)
	}

	// Create a Slack message for the risk category summary
	message := slack.Message{
		Blocks: []slack.Block{
			{
				Type: "header",
				Text: slack.NewTextObject("plain_text", "🔍 Risk Distribution by Category", true),
			},
			{
				Type: "section",
				Text: slack.NewTextObject("mrkdwn", riskText, false), // Fixed: Added the missing boolean parameter
			},
			{
				Type: "context",
				Elements: []interface{}{
					map[string]interface{}{
						"type": "mrkdwn",
						"text": fmt.Sprintf("Report generated on %s", time.Now().Format("Jan 2, 2006 15:04 MST")),
					},
				},
			},
		},
	}

	// Post the message to the grc-reports channel
	_, err = h.SlackClient.PostMessage(slack.ChannelMapping["reports"], message)
	if err != nil {
		return fmt.Errorf("error posting risk category summary to Slack: %w", err)
	}

	return nil
}

// HandleStatusRequest processes a GRC status request
func (h *ReportingHandler) HandleStatusRequest(channelID, userID string) error {
	// Get the GRC summary data
	summary, err := h.GetGRCSummary()
	if err != nil {
		return fmt.Errorf("error getting GRC status: %w", err)
	}

	// Create a Slack message for the status report
	message := slack.Message{
		Blocks: []slack.Block{
			{
				Type: "header",
				Text: slack.NewTextObject("plain_text", "📊 Current GRC Status", true),
			},
			{
				Type: "section",
				Fields: []*slack.TextObject{
					slack.NewTextObject("mrkdwn", fmt.Sprintf("*Open Risks:*\n%d", summary.OpenRisks), false),
					slack.NewTextObject("mrkdwn", fmt.Sprintf("*Open Compliance Tasks:*\n%d", summary.OpenComplianceTasks), false),
					slack.NewTextObject("mrkdwn", fmt.Sprintf("*Open Incidents:*\n%d", summary.OpenIncidents), false),
					slack.NewTextObject("mrkdwn", fmt.Sprintf("*Overdue Items:*\n%d", summary.OverdueItems), false),
				},
			},
			{
				Type: "context",
				Elements: []interface{}{
					map[string]interface{}{
						"type": "mrkdwn",
						"text": fmt.Sprintf("Status requested by <@%s> on %s", userID, time.Now().Format("Jan 2, 2006 15:04 MST")),
					},
				},
			},
		},
	}

	// Post the message to the requested channel
	_, err = h.SlackClient.PostMessage(channelID, message)
	if err != nil {
		return fmt.Errorf("error posting GRC status to Slack: %w", err)
	}

	return nil
}

// ProcessReportingCommand handles slash commands for GRC reporting
func (h *ReportingHandler) ProcessReportingCommand(command *slack.Command) (string, error) {
	// Handle different reporting commands
	switch {
	case command.Command == "/grc-status":
		// Handle status request in the channel where the command was issued
		err := h.HandleStatusRequest(command.ChannelID, command.UserID)
		if err != nil {
			return fmt.Sprintf("Error getting GRC status: %s", err), nil
		}

		return "", nil // Empty response as we're sending a message to the channel directly

	default:
		return "Unknown command", nil
	}
}
