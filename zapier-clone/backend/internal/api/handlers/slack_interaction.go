// backend/internal/api/handlers/slack_interaction.go
package handlers

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"

	"github.com/shivani-1505/zapier-clone/backend/internal/integrations/jira"
	"github.com/shivani-1505/zapier-clone/backend/internal/integrations/servicenow"
	"github.com/shivani-1505/zapier-clone/backend/internal/integrations/slack"
)

// SlackInteractionHandler handles incoming interactions from Slack
type SlackInteractionHandler struct {
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

// NewSlackInteractionHandler creates a new Slack interaction handler
func NewSlackInteractionHandler(
	serviceNowClient *servicenow.Client,
	slackClient *slack.Client,
	jiraClient *jira.Client,
	incidentHandler *servicenow.IncidentHandler,
) *SlackInteractionHandler {
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

	return &SlackInteractionHandler{
		ServiceNowClient:        serviceNowClient,
		SlackClient:             slackClient,
		RiskHandler:             servicenow.NewRiskHandler(serviceNowClient, slackClient, jiraClient, riskJiraMapping),
		ComplianceHandler:       servicenow.NewComplianceTaskHandler(serviceNowClient, slackClient),
		IncidentHandler:         incidentHandler,
		ControlTestHandler:      servicenow.NewPolicyControlHandler(serviceNowClient, slackClient),
		AuditHandler:            servicenow.NewAuditHandler(serviceNowClient, slackClient, jiraClient),
		VendorRiskHandler:       servicenow.NewVendorRiskHandler(serviceNowClient, slackClient),
		RegulatoryChangeHandler: servicenow.NewRegulatoryChangeHandler(serviceNowClient, slackClient),
		ReportingHandler:        servicenow.NewReportingHandler(serviceNowClient, slackClient),
	}
}

// HandleInteraction processes incoming interactions from Slack
func (h *SlackInteractionHandler) HandleInteraction(w http.ResponseWriter, r *http.Request) {
	// Parse the form to get the payload
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid form data", http.StatusBadRequest)
		return
	}

	// Get the payload from the form
	payloadStr := r.FormValue("payload")
	if payloadStr == "" {
		http.Error(w, "Missing payload", http.StatusBadRequest)
		return
	}

	// Parse the payload
	var payload slack.InteractionPayload
	if err := json.Unmarshal([]byte(payloadStr), &payload); err != nil {
		http.Error(w, "Invalid payload format", http.StatusBadRequest)
		return
	}

	// Process the interaction asynchronously
	go h.processInteraction(payload)

	// Respond immediately to Slack
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"text":"Processing your request..."}`))
}

// processInteraction processes the Slack interaction payload asynchronously
func (h *SlackInteractionHandler) processInteraction(payload slack.InteractionPayload) {
	// Skip if no actions
	if len(payload.Actions) == 0 {
		return
	}

	// Get the first action
	action := payload.Actions[0]
	actionID := action["action_id"]
	actionValue := action["value"]

	// Process based on the action ID
	switch actionID {
	// Risk Management interactions
	case "discuss_risk", "assign_risk":
		// Extract the risk ID from the value
		parts := strings.Split(actionValue, "_")
		if len(parts) < 3 {
			log.Printf("Invalid risk action value: %s", actionValue)
			return
		}
		riskID := parts[2]

		if actionID == "discuss_risk" {
			log.Printf("Risk discussion initiated for: %s", riskID)
		} else if actionID == "assign_risk" {
			err := h.RiskHandler.HandleRiskAssignment(riskID, payload.ChannelID, payload.MessageTS, payload.UserID)
			if err != nil {
				log.Printf("Error assigning risk: %v", err)
			}
		}

	// Compliance Task interactions
	case "upload_evidence", "assign_task":
		// Extract the task ID from the value
		parts := strings.Split(actionValue, "_")
		if len(parts) < 3 {
			log.Printf("Invalid task action value: %s", actionValue)
			return
		}
		taskID := parts[2]

		if actionID == "upload_evidence" {
			log.Printf("Evidence upload initiated for task: %s", taskID)
		} else if actionID == "assign_task" {
			err := h.ComplianceHandler.HandleComplianceTaskAssignment(taskID, payload.ChannelID, payload.MessageTS, payload.UserID)
			if err != nil {
				log.Printf("Error assigning compliance task: %v", err)
			}
		}

	// Replace the existing incident handling code (around lines 143-156) with this enhanced version:

	// Incident Response interactions
	case "acknowledge_incident", "update_incident", "resolve_incident":
		// Extract the incident ID from the value
		parts := strings.Split(actionValue, "_")
		if len(parts) < 3 {
			log.Printf("Invalid incident action value: %s", actionValue)
			return
		}
		incidentID := parts[2]

		if actionID == "acknowledge_incident" {
			err := h.IncidentHandler.HandleIncidentAcknowledgment(incidentID, payload.ChannelID, payload.MessageTS, payload.UserID)
			if err != nil {
				log.Printf("Error acknowledging incident: %v", err)
			}
		} else if actionID == "update_incident" {
			// Open a modal for detailed update
			modalRequest := slack.ModalRequest{
				TriggerID: payload.TriggerID,
				View: slack.Modal{
					Type: "modal",
					Title: slack.TextObject{
						Type: "plain_text",
						Text: "Update Incident",
					},
					CallbackID:      "incident_update_modal",
					PrivateMetadata: incidentID + ":" + payload.ChannelID + ":" + payload.MessageTS,
					Submit: slack.TextObject{
						Type: "plain_text",
						Text: "Submit",
					},
					Close: slack.TextObject{
						Type: "plain_text",
						Text: "Cancel",
					},
					Blocks: []slack.Block{
						{
							Type:    "input",
							BlockID: "update_text",
							Element: map[string]interface{}{
								"type":      "plain_text_input",
								"action_id": "update_text_input",
								"multiline": true,
								"placeholder": map[string]interface{}{
									"type": "plain_text",
									"text": "Enter your update about this incident...",
								},
							},
							Label: slack.TextObject{
								Type: "plain_text",
								Text: "Update details",
							},
							Optional: false,
						},
					},
				},
			}

			err := h.SlackClient.OpenModal(modalRequest)
			if err != nil {
				log.Printf("Error opening incident update modal: %v", err)
			}
		} else if actionID == "resolve_incident" {
			// Open a modal for resolution details
			modalRequest := slack.ModalRequest{
				TriggerID: payload.TriggerID,
				View: slack.Modal{
					Type: "modal",
					Title: slack.TextObject{
						Type: "plain_text",
						Text: "Resolve Incident",
					},
					CallbackID:      "incident_resolve_modal",
					PrivateMetadata: incidentID + ":" + payload.ChannelID + ":" + payload.MessageTS,
					Submit: slack.TextObject{
						Type: "plain_text",
						Text: "Resolve",
					},
					Close: slack.TextObject{
						Type: "plain_text",
						Text: "Cancel",
					},
					Blocks: []slack.Block{
						{
							Type:    "input",
							BlockID: "resolution_notes",
							Element: map[string]interface{}{
								"type":      "plain_text_input",
								"action_id": "resolution_notes_input",
								"multiline": true,
								"placeholder": map[string]string{
									"type": "plain_text",
									"text": "Describe how the incident was resolved...",
								},
							},
							Label: slack.TextObject{
								Type: "plain_text",
								Text: "Resolution details",
							},
							Optional: false,
						},
					},
				},
			}

			err := h.SlackClient.OpenModal(modalRequest)
			if err != nil {
				log.Printf("Error opening incident resolution modal: %v", err)
			}
		}

	// Control Testing interactions
	case "submit_test_results":
		// Extract the test ID from the value
		parts := strings.Split(actionValue, "_")
		if len(parts) < 3 {
			log.Printf("Invalid test results value: %s", actionValue)
			return
		}
		testID := parts[2]

		// In a real implementation, you'd open a modal for test result input
		log.Printf("Test result submission initiated for: %s", testID)

	// Audit Management interactions
	case "assign_finding", "resolve_finding":
		// Extract the finding ID from the value
		parts := strings.Split(actionValue, "_")
		if len(parts) < 3 {
			log.Printf("Invalid finding action value: %s", actionValue)
			return
		}
		findingID := parts[2]

		if actionID == "assign_finding" {
			err := h.AuditHandler.HandleAuditFindingAssignment(findingID, payload.ChannelID, payload.MessageTS, payload.UserID)
			if err != nil {
				log.Printf("Error assigning audit finding: %v", err)
			}
		} else if actionID == "resolve_finding" {
			// In a real implementation, you'd open a modal for resolution notes
			log.Printf("Audit finding resolution initiated for: %s", findingID)
		}

	// Vendor Risk Management interactions
	case "request_compliance_report", "update_vendor_status":
		// Extract the vendor risk ID from the value
		parts := strings.Split(actionValue, "_")
		if len(parts) < 3 {
			log.Printf("Invalid vendor action value: %s", actionValue)
			return
		}
		riskID := parts[2]

		if actionID == "request_compliance_report" {
			err := h.VendorRiskHandler.HandleComplianceReportRequest(riskID, payload.ChannelID, payload.MessageTS, payload.UserID)
			if err != nil {
				log.Printf("Error requesting compliance report: %v", err)
			}
		} else if actionID == "update_vendor_status" {
			// In a real implementation, you'd open a modal for status update input
			log.Printf("Vendor status update initiated for: %s", riskID)
		}

	// Regulatory Change Management interactions
	case "add_impact_assessment", "create_implementation_plan":
		// Extract the change ID from the value
		parts := strings.Split(actionValue, "_")
		if len(parts) < 3 {
			log.Printf("Invalid regulatory change action value: %s", actionValue)
			return
		}
		changeID := parts[2]

		if actionID == "add_impact_assessment" {
			// In a real implementation, you'd open a modal for impact assessment input
			log.Printf("Impact assessment initiated for: %s", changeID)
		} else if actionID == "create_implementation_plan" {
			// In a real implementation, you'd open a modal for implementation plan input
			log.Printf("Implementation plan creation initiated for: %s", changeID)
		}

		// Handle modal submissions if this is a view submission
		if payload.Type == "view_submission" {
			switch payload.View.CallbackID {
			case "incident_update_modal":
				// Handle incident update modal submission
				values := payload.View.State.Values
				updateText := values["update_text"]["update_text_input"].Value

				// Parse metadata to get incident ID and thread info
				metaParts := strings.Split(payload.View.PrivateMetadata, ":")
				if len(metaParts) >= 3 {
					incidentID := metaParts[0]
					channelID := metaParts[1]
					threadTS := metaParts[2]

					err := h.IncidentHandler.HandleIncidentUpdate(
						incidentID,
						channelID,
						threadTS,
						payload.User.ID,
						updateText,
					)
					if err != nil {
						log.Printf("Error handling incident update: %v", err)
					}
				}

			case "incident_resolve_modal":
				// Handle incident resolution modal submission
				values := payload.View.State.Values
				resolutionNotes := values["resolution_notes"]["resolution_notes_input"].Value

				// Parse metadata to get incident ID and thread info
				metaParts := strings.Split(payload.View.PrivateMetadata, ":")
				if len(metaParts) >= 3 {
					incidentID := metaParts[0]
					channelID := metaParts[1]
					threadTS := metaParts[2]

					err := h.IncidentHandler.HandleIncidentResolution(
						incidentID,
						channelID,
						threadTS,
						payload.User.ID,
						resolutionNotes,
					)
					if err != nil {
						log.Printf("Error handling incident resolution: %v", err)
					}
				}
			}
		}

	default:
		log.Printf("Unhandled action ID: %s", actionID)
	}
}
