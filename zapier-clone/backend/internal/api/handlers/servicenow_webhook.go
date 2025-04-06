// backend/internal/api/handlers/servicenow_webhook.go
package handlers

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/shivani-1505/zapier-clone/backend/internal/integrations/jira"
	"github.com/shivani-1505/zapier-clone/backend/internal/integrations/servicenow"
	"github.com/shivani-1505/zapier-clone/backend/internal/integrations/slack"
)

// ServiceNowWebhookHandler handles incoming webhooks from ServiceNow
type ServiceNowWebhookHandler struct {
	ServiceNowClient        *servicenow.Client
	SlackClient             *slack.Client
	JiraClient              *jira.Client
	RiskHandler             *servicenow.RiskHandler
	ComplianceHandler       *servicenow.ComplianceTaskHandler
	IncidentHandler         *servicenow.IncidentHandler
	ControlTestHandler      *servicenow.PolicyControlHandler
	AuditHandler            *servicenow.AuditHandler
	VendorRiskHandler       *servicenow.VendorRiskHandler
	RegulatoryChangeHandler *servicenow.RegulatoryChangeHandler
	ReportingHandler        *servicenow.ReportingHandler
}

// NewServiceNowWebhookHandler creates a new ServiceNow webhook handler
func NewServiceNowWebhookHandler(
	serviceNowClient *servicenow.Client,
	slackClient *slack.Client,
	jiraClient *jira.Client,
) *ServiceNowWebhookHandler {
	// Initialize risk-jira mapping
	riskJiraMapping, err := jira.NewRiskJiraMapping("./data")
	if err != nil {
		log.Printf("Warning: Failed to initialize risk-jira mapping: %v", err)
		// Create an empty mapping as fallback
		riskJiraMapping = &jira.RiskJiraMapping{
			RiskIDToJiraKey: make(map[string]string),
			JiraKeyToRiskID: make(map[string]string),
		}
	}

	return &ServiceNowWebhookHandler{
		ServiceNowClient:        serviceNowClient,
		SlackClient:             slackClient,
		JiraClient:              jiraClient,
		RiskHandler:             servicenow.NewRiskHandler(serviceNowClient, slackClient, jiraClient, riskJiraMapping),
		ComplianceHandler:       servicenow.NewComplianceTaskHandler(serviceNowClient, slackClient),
		IncidentHandler:         servicenow.NewIncidentHandler(serviceNowClient, slackClient, jiraClient),
		ControlTestHandler:      servicenow.NewPolicyControlHandler(serviceNowClient, slackClient),
		AuditHandler:            servicenow.NewAuditHandler(serviceNowClient, slackClient, jiraClient),
		VendorRiskHandler:       servicenow.NewVendorRiskHandler(serviceNowClient, slackClient),
		RegulatoryChangeHandler: servicenow.NewRegulatoryChangeHandler(serviceNowClient, slackClient),
		ReportingHandler:        servicenow.NewReportingHandler(serviceNowClient, slackClient),
	}
}

// HandleWebhook processes incoming webhooks from ServiceNow
func (h *ServiceNowWebhookHandler) HandleWebhook(w http.ResponseWriter, r *http.Request) {
	// Parse the incoming webhook payload
	var payload servicenow.WebhookPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "Invalid payload", http.StatusBadRequest)
		return
	}

	// Process the webhook based on the table name and action type
	go h.processWebhook(payload)

	// Respond immediately to ServiceNow
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"received"}`))
}

// processWebhook processes the webhook payload asynchronously
func (h *ServiceNowWebhookHandler) processWebhook(payload servicenow.WebhookPayload) {
	switch payload.TableName {
	case "sn_risk_risk":
		h.processRiskWebhook(payload)
	case "sn_compliance_task":
		h.processComplianceTaskWebhook(payload)
	case "sn_si_incident":
		h.processIncidentWebhook(payload)
	case "sn_policy_control_test":
		h.processControlTestWebhook(payload)
	case "sn_audit_finding":
		h.processAuditFindingWebhook(payload)
	case "sn_vendor_risk":
		h.processVendorRiskWebhook(payload)
	case "sn_regulatory_change":
		h.processRegulatoryChangeWebhook(payload)
	default:
		log.Printf("Unsupported table: %s", payload.TableName)
	}
}

// processRiskWebhook processes risk-related webhooks
func (h *ServiceNowWebhookHandler) processRiskWebhook(payload servicenow.WebhookPayload) {
	// Convert the payload data to a Risk object
	riskData, err := json.Marshal(payload.Data)
	if err != nil {
		log.Printf("Error marshaling risk data: %v", err)
		return
	}

	var risk servicenow.Risk
	if err := json.Unmarshal(riskData, &risk); err != nil {
		log.Printf("Error unmarshaling risk data: %v", err)
		return
	}

	// Process the risk based on the action type
	switch payload.ActionType {
	case "inserted":
		// New risk created
		_, err := h.RiskHandler.HandleNewRisk(risk)
		if err != nil {
			log.Printf("Error handling new risk: %v", err)
		}
	case "updated":
		// Risk updated
		// In a real implementation, you'd look up the thread info from a database
		// For simplicity, we're just logging it
		log.Printf("Risk updated: %s", risk.ID)
	case "deleted":
		// Risk deleted
		log.Printf("Risk deleted: %s", risk.ID)
	}
}

// processComplianceTaskWebhook processes compliance task webhooks
func (h *ServiceNowWebhookHandler) processComplianceTaskWebhook(payload servicenow.WebhookPayload) {
	// Convert the payload data to a ComplianceTask object
	taskData, err := json.Marshal(payload.Data)
	if err != nil {
		log.Printf("Error marshaling compliance task data: %v", err)
		return
	}

	var task servicenow.ComplianceTask
	if err := json.Unmarshal(taskData, &task); err != nil {
		log.Printf("Error unmarshaling compliance task data: %v", err)
		return
	}

	// Process the compliance task based on the action type
	switch payload.ActionType {
	case "inserted":
		// New compliance task created
		_, err := h.ComplianceHandler.HandleNewComplianceTask(task)
		if err != nil {
			log.Printf("Error handling new compliance task: %v", err)
		}
	case "updated":
		// Compliance task updated
		// In a real implementation, you'd look up the thread info from a database
		log.Printf("Compliance task updated: %s", task.ID)
	case "deleted":
		// Compliance task deleted
		log.Printf("Compliance task deleted: %s", task.ID)
	}
}

// processIncidentWebhook processes security incident webhooks
func (h *ServiceNowWebhookHandler) processIncidentWebhook(payload servicenow.WebhookPayload) {
	// Convert the payload data to an Incident object
	incidentData, err := json.Marshal(payload.Data)
	if err != nil {
		log.Printf("Error marshaling incident data: %v", err)
		return
	}

	var incident servicenow.Incident
	if err := json.Unmarshal(incidentData, &incident); err != nil {
		log.Printf("Error unmarshaling incident data: %v", err)
		return
	}

	// Process the incident based on the action type
	switch payload.ActionType {
	case "inserted":
		// New incident created
		_, err := h.IncidentHandler.HandleNewIncident(incident)
		if err != nil {
			log.Printf("Error handling new incident: %v", err)
		}
	case "updated":
		// Incident updated
		// In a real implementation, you'd look up the thread info from a database
		log.Printf("Incident updated: %s", incident.ID)
	case "deleted":
		// Incident deleted
		log.Printf("Incident deleted: %s", incident.ID)
	}
}

// processControlTestWebhook processes control test webhooks
func (h *ServiceNowWebhookHandler) processControlTestWebhook(payload servicenow.WebhookPayload) {
	// Convert the payload data to a ControlTest object
	testData, err := json.Marshal(payload.Data)
	if err != nil {
		log.Printf("Error marshaling control test data: %v", err)
		return
	}

	var test servicenow.ControlTest
	if err := json.Unmarshal(testData, &test); err != nil {
		log.Printf("Error unmarshaling control test data: %v", err)
		return
	}

	// Process the control test based on the action type
	switch payload.ActionType {
	case "inserted":
		// New control test created
		messageTS, err := h.ControlTestHandler.HandleNewControlTest(test)
		if err != nil {
			log.Printf("Error handling new control test: %v", err)
			return
		}

		// Enhanced functionality: Create Jira issue for control test
		jiraTicket := &jira.Ticket{
			Project:     h.JiraClient.ProjectKey,
			IssueType:   "Task",
			Summary:     fmt.Sprintf("Test Control: %s", test.Control),
			Description: test.Description,
			DueDate:     test.DueDate,
			Fields: map[string]interface{}{
				"customfield_servicenow_id": test.ID,
			},
		}

		// If the test has an assignee, add it to the ticket
		if test.AssignedTo != "" {
			jiraTicket.Fields["assignee"] = map[string]string{"name": test.AssignedTo}
		}

		createdTicket, err := h.JiraClient.CreateIssue(jiraTicket)
		if err != nil {
			log.Printf("Error creating Jira issue for control test %s: %v", test.ID, err)
			return
		}

		log.Printf("Created Jira issue %s for control test: %s", createdTicket.Key, test.ID)

		// Add Jira ticket info to the Slack thread if we have a message timestamp
		if messageTS != "" {
			jiraLink := fmt.Sprintf("%s/browse/%s", h.JiraClient.BaseURL, createdTicket.Key)
			message := slack.Message{
				Text: fmt.Sprintf("📋 Jira ticket created: <%s|%s>", jiraLink, createdTicket.Key),
			}

			channelID := slack.ChannelMapping["control-testing"]
			if _, err := h.SlackClient.PostReply(channelID, messageTS, message); err != nil {
				log.Printf("Error posting Jira ticket info to Slack: %v", err)
			}
		}
	case "updated":
		// Control test updated
		log.Printf("Control test updated: %s", test.ID)

		// Find corresponding Jira ticket and update it
		jql := fmt.Sprintf("project=%s AND customfield_servicenow_id=\"%s\"", h.JiraClient.ProjectKey, test.ID)
		searchResult, err := h.JiraClient.SearchIssues(jql)
		if err != nil {
			log.Printf("Error searching for Jira ticket for control test %s: %v", test.ID, err)
			return
		}

		if len(searchResult.Issues) > 0 {
			issueKey := searchResult.Issues[0].Key

			// Create update object
			update := &jira.TicketUpdate{
				Description: test.Description,
			}

			// If test has a status, map it to Jira status
			if test.Status != "" {
				update.Status = mapControlTestStatusToJiraStatus(test.Status)
			}

			// If test has results, add them to the description
			if test.Results != "" {
				update.Description = fmt.Sprintf("%s\n\nTest Results:\n%s", test.Description, test.Results)
			}

			// Update the Jira ticket
			if err := h.JiraClient.UpdateIssue(issueKey, update); err != nil {
				log.Printf("Error updating Jira ticket %s for control test %s: %v", issueKey, test.ID, err)
			} else {
				log.Printf("Updated Jira ticket %s for control test %s", issueKey, test.ID)
			}
		}
	case "deleted":
		// Control test deleted
		log.Printf("Control test deleted: %s", test.ID)
	}
}

// processAuditFindingWebhook processes audit finding webhooks
func (h *ServiceNowWebhookHandler) processAuditFindingWebhook(payload servicenow.WebhookPayload) {
	// Convert the payload data to an AuditFinding object
	findingData, err := json.Marshal(payload.Data)
	if err != nil {
		log.Printf("Error marshaling audit finding data: %v", err)
		return
	}

	var finding servicenow.AuditFinding
	if err := json.Unmarshal(findingData, &finding); err != nil {
		log.Printf("Error unmarshaling audit finding data: %v", err)
		return
	}

	// Process the audit finding based on the action type
	switch payload.ActionType {
	case "inserted":
		// New audit finding created
		_, err := h.AuditHandler.HandleNewAuditFinding(finding)
		if err != nil {
			log.Printf("Error handling new audit finding: %v", err)
		}
	case "updated":
		// Audit finding updated
		log.Printf("Audit finding updated: %s", finding.ID)
	case "deleted":
		// Audit finding deleted
		log.Printf("Audit finding deleted: %s", finding.ID)
	}
}

// processVendorRiskWebhook processes vendor risk webhooks
func (h *ServiceNowWebhookHandler) processVendorRiskWebhook(payload servicenow.WebhookPayload) {
	// Convert the payload data to a VendorRisk object
	riskData, err := json.Marshal(payload.Data)
	if err != nil {
		log.Printf("Error marshaling vendor risk data: %v", err)
		return
	}

	var risk servicenow.VendorRisk
	if err := json.Unmarshal(riskData, &risk); err != nil {
		log.Printf("Error unmarshaling vendor risk data: %v", err)
		return
	}

	// Process the vendor risk based on the action type
	switch payload.ActionType {
	case "inserted":
		// New vendor risk created
		_, err := h.VendorRiskHandler.HandleNewVendorRisk(risk)
		if err != nil {
			log.Printf("Error handling new vendor risk: %v", err)
		}

		// Enhanced functionality for vendor risk to better integrate with Jira
		if err == nil {
			// Create Jira issue for vendor risk
			jiraTicket := &jira.Ticket{
				Project:     h.JiraClient.ProjectKey,
				IssueType:   "Task",
				Summary:     fmt.Sprintf("Vendor Risk: %s", risk.VendorName),
				Description: fmt.Sprintf("Request updated report for vendor risk:\n\n%s", risk.Description),
				Priority:    mapSeverityToPriority(risk.Severity),
				Fields: map[string]interface{}{
					"customfield_servicenow_id": risk.ID,
				},
			}

			createdTicket, err := h.JiraClient.CreateIssue(jiraTicket)
			if err != nil {
				log.Printf("Error creating Jira issue for vendor risk %s: %v", risk.ID, err)
			} else {
				log.Printf("Created Jira issue %s for vendor risk: %s", createdTicket.Key, risk.ID)
			}
		}
	case "updated":
		// Vendor risk updated
		log.Printf("Vendor risk updated: %s", risk.ID)
	case "deleted":
		// Vendor risk deleted
		log.Printf("Vendor risk deleted: %s", risk.ID)
	}
}

// processRegulatoryChangeWebhook processes regulatory change webhooks
func (h *ServiceNowWebhookHandler) processRegulatoryChangeWebhook(payload servicenow.WebhookPayload) {
	// Convert the payload data to a RegulatoryChange object
	changeData, err := json.Marshal(payload.Data)
	if err != nil {
		log.Printf("Error marshaling regulatory change data: %v", err)
		return
	}

	var change servicenow.RegulatoryChange
	if err := json.Unmarshal(changeData, &change); err != nil {
		log.Printf("Error unmarshaling regulatory change data: %v", err)
		return
	}

	// Process the regulatory change based on the action type
	switch payload.ActionType {
	case "inserted":
		// New regulatory change created
		messageTS, err := h.RegulatoryChangeHandler.HandleNewRegulatoryChange(change)
		if err != nil {
			log.Printf("Error handling new regulatory change: %v", err)
			return
		}

		// Enhanced functionality: Create Jira Epic with subtasks for regulatory change
		epicTicket := &jira.Ticket{
			Project:     h.JiraClient.ProjectKey,
			IssueType:   "Epic",
			Summary:     fmt.Sprintf("Regulatory Change: %s", change.ShortDesc),
			Description: change.Description,
			Epic: &jira.EpicDetails{
				Name: fmt.Sprintf("Regulatory Change: %s", change.ShortDesc),
			},
			DueDate: change.EffectiveDate,
			Fields: map[string]interface{}{
				"customfield_servicenow_id": change.ID,
			},
		}

		createdEpic, err := h.JiraClient.CreateIssue(epicTicket)
		if err != nil {
			log.Printf("Error creating Jira epic for regulatory change %s: %v", change.ID, err)
			return
		}

		// Create standard subtasks for regulatory implementation
		tasks := []struct {
			summary     string
			description string
		}{
			{
				summary:     "Update privacy policy",
				description: fmt.Sprintf("Update privacy policy due to %s", change.ShortDesc),
			},
			{
				summary:     "Train staff",
				description: fmt.Sprintf("Train staff on %s requirements", change.ShortDesc),
			},
		}

		for _, task := range tasks {
			subtask := &jira.Ticket{
				Project:     h.JiraClient.ProjectKey,
				IssueType:   "Task",
				Summary:     task.summary,
				Description: task.description,
				Parent:      createdEpic.Key,
				DueDate:     change.EffectiveDate,
				Fields: map[string]interface{}{
					"customfield_servicenow_id": change.ID,
				},
			}

			if _, err := h.JiraClient.CreateIssue(subtask); err != nil {
				log.Printf("Error creating subtask '%s' for regulatory change %s: %v", task.summary, change.ID, err)
			}
		}

		// Add Jira epic info to the Slack thread
		if messageTS != "" {
			jiraLink := fmt.Sprintf("%s/browse/%s", h.JiraClient.BaseURL, createdEpic.Key)
			message := slack.Message{
				Text: fmt.Sprintf("📎 Jira epic created: <%s|%s> - Implementation tasks have been created.", jiraLink, createdEpic.Key),
			}

			channelID := slack.ChannelMapping["regulatory"]
			if _, err := h.SlackClient.PostReply(channelID, messageTS, message); err != nil {
				log.Printf("Error posting Jira epic info to Slack: %v", err)
			}
		}
	case "updated":
		// Regulatory change updated
		log.Printf("Regulatory change updated: %s", change.ID)
	case "deleted":
		// Regulatory change deleted
		log.Printf("Regulatory change deleted: %s", change.ID)
	}
}

// Helper function to map severity to priority
func mapSeverityToPriority(severity string) string {
	switch strings.ToLower(severity) {
	case "critical":
		return "Highest"
	case "high":
		return "High"
	case "medium":
		return "Medium"
	case "low":
		return "Low"
	default:
		return "Medium"
	}
}

// mapControlTestStatusToJiraStatus maps ServiceNow control test status to Jira status
func mapControlTestStatusToJiraStatus(status string) string {
	switch strings.ToLower(status) {
	case "pass", "passed":
		return "Done"
	case "fail", "failed":
		return "Done" // Even failed tests are considered "Done" from a workflow perspective
	case "in progress":
		return "In Progress"
	case "open":
		return "To Do"
	default:
		return status // Use the status as-is if no mapping found
	}
}
