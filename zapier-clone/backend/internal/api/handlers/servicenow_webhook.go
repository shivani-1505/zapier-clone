// backend/internal/api/handlers/servicenow_webhook.go
package handlers

import (
	"encoding/json"
	"log"
	"net/http"

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
		_, err := h.ControlTestHandler.HandleNewControlTest(test)
		if err != nil {
			log.Printf("Error handling new control test: %v", err)
		}
	case "updated":
		// Control test updated
		log.Printf("Control test updated: %s", test.ID)
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
		_, err := h.RegulatoryChangeHandler.HandleNewRegulatoryChange(change)
		if err != nil {
			log.Printf("Error handling new regulatory change: %v", err)
		}
	case "updated":
		// Regulatory change updated
		log.Printf("Regulatory change updated: %s", change.ID)
	case "deleted":
		// Regulatory change deleted
		log.Printf("Regulatory change deleted: %s", change.ID)
	}
}
