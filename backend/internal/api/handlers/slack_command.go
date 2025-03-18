// backend/internal/api/handlers/slack_command.go
package handlers

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/shivani-1505/zapier-clone/backend/internal/integrations/servicenow"
	"github.com/shivani-1505/zapier-clone/backend/internal/integrations/slack"
)

// SlackCommandHandler handles incoming slash commands from Slack
type SlackCommandHandler struct {
	ServiceNowClient        *servicenow.Client
	SlackClient             *slack.Client
	RiskHandler             *servicenow.RiskHandler
	ComplianceHandler       *servicenow.ComplianceTaskHandler
	IncidentHandler         *servicenow.IncidentHandler
	ControlTestHandler      *servicenow.PolicyControlHandler
	AuditHandler            *servicenow.AuditHandler
	VendorRiskHandler       *servicenow.VendorRiskHandler
	RegulatoryChangeHandler *servicenow.RegulatoryChangeHandler
	ReportingHandler        *servicenow.ReportingHandler
}

// NewSlackCommandHandler creates a new Slack command handler
func NewSlackCommandHandler(
	serviceNowClient *servicenow.Client,
	slackClient *slack.Client,
) *SlackCommandHandler {
	return &SlackCommandHandler{
		ServiceNowClient:        serviceNowClient,
		SlackClient:             slackClient,
		RiskHandler:             servicenow.NewRiskHandler(serviceNowClient, slackClient),
		ComplianceHandler:       servicenow.NewComplianceTaskHandler(serviceNowClient, slackClient),
		IncidentHandler:         servicenow.NewIncidentHandler(serviceNowClient, slackClient),
		ControlTestHandler:      servicenow.NewPolicyControlHandler(serviceNowClient, slackClient),
		AuditHandler:            servicenow.NewAuditHandler(serviceNowClient, slackClient),
		VendorRiskHandler:       servicenow.NewVendorRiskHandler(serviceNowClient, slackClient),
		RegulatoryChangeHandler: servicenow.NewRegulatoryChangeHandler(serviceNowClient, slackClient),
		ReportingHandler:        servicenow.NewReportingHandler(serviceNowClient, slackClient),
	}
}

// HandleCommand processes incoming slash commands from Slack
func (h *SlackCommandHandler) HandleCommand(w http.ResponseWriter, r *http.Request) {
	// Parse the form to get the command data
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid form data", http.StatusBadRequest)
		return
	}

	// Create a Command object from the form data
	command := &slack.Command{
		Token:       r.FormValue("token"),
		TeamID:      r.FormValue("team_id"),
		TeamDomain:  r.FormValue("team_domain"),
		ChannelID:   r.FormValue("channel_id"),
		ChannelName: r.FormValue("channel_name"),
		UserID:      r.FormValue("user_id"),
		UserName:    r.FormValue("user_name"),
		Command:     r.FormValue("command"),
		Text:        r.FormValue("text"),
		ResponseURL: r.FormValue("response_url"),
		TriggerID:   r.FormValue("trigger_id"),
	}

	// Process the command
	response, err := h.processCommand(command)
	if err != nil {
		log.Printf("Error processing command: %v", err)
		http.Error(w, "Error processing command", http.StatusInternalServerError)
		return
	}

	// Create the response
	responseJSON, err := json.Marshal(map[string]string{"text": response})
	if err != nil {
		http.Error(w, "Error creating response", http.StatusInternalServerError)
		return
	}

	// Send the response
	w.Header().Set("Content-Type", "application/json")
	w.Write(responseJSON)
}

// processCommand processes the Slack command
func (h *SlackCommandHandler) processCommand(command *slack.Command) (string, error) {
	// Process the command based on the command type
	switch command.Command {
	case "/upload-evidence":
		return h.ComplianceHandler.ProcessComplianceTaskCommand(command)

	case "/incident-update", "/resolve-incident":
		return h.IncidentHandler.ProcessIncidentCommand(command)

	case "/submit-test":
		return h.ControlTestHandler.ProcessControlCommand(command)

	case "/resolve-finding":
		return h.AuditHandler.ProcessAuditCommand(command)

	case "/update-vendor":
		return h.VendorRiskHandler.ProcessVendorCommand(command)

	case "/assess-impact", "/plan-implementation":
		return h.RegulatoryChangeHandler.ProcessRegulatoryCommand(command)

	case "/grc-status":
		return h.ReportingHandler.ProcessReportingCommand(command)

	case "/assign-owner":
		// This general command would handle assignment for any GRC object
		// In a real implementation, you'd parse the command and route to the appropriate handler
		return "Owner assignment functionality is under development. Please use the buttons in the message.", nil

	default:
		log.Printf("Unknown command: %s", command.Command)
		return "Unknown command. Available commands: /upload-evidence, /incident-update, /resolve-incident, /submit-test, /resolve-finding, /update-vendor, /assess-impact, /plan-implementation, /grc-status, /assign-owner", nil
	}
}
