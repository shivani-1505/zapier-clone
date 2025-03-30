package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/mux"
)

// ResponseResult represents a ServiceNow API response
type ResponseResult struct {
	Result interface{} `json:"result"`
}

// GRCSummary represents mock GRC summary data
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

// RiskByCategory represents mock risk by category data
type RiskByCategory struct {
	Category string `json:"category"`
	Count    int    `json:"count"`
}

// MockDatabase holds our mock data
var MockDatabase = map[string]map[string]interface{}{
	"risks":              {},
	"compliance_tasks":   {},
	"incidents":          {},
	"control_tests":      {},
	"audit_findings":     {},
	"vendor_risks":       {},
	"regulatory_changes": {},
}

// Special handler for risk creation that also sends a webhook to Jira
func handleCreateRisk(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// Parse the risk data
	var riskData map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&riskData); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Generate a risk ID and number if not provided
	sysID, ok := riskData["sys_id"].(string)
	if !ok || sysID == "" {
		sysID = fmt.Sprintf("risk_%d", time.Now().UnixNano())
		riskData["sys_id"] = sysID
	}
	riskNumber, ok := riskData["number"].(string)
	if !ok || riskNumber == "" {
		riskNumber = fmt.Sprintf("RISK%d", len(MockDatabase["risks"])+1001)
		riskData["number"] = riskNumber
	}

	// Add system fields
	riskData["created_on"] = time.Now().Format(time.RFC3339)
	riskData["updated_on"] = time.Now().Format(time.RFC3339)

	// Make sure risks map exists
	if MockDatabase["risks"] == nil {
		MockDatabase["risks"] = make(map[string]interface{})
	}

	MockDatabase["risks"][sysID] = riskData

	// Send a webhook to sync this with Jira
	go triggerWebhook("sn_risk_risk", sysID, "insert", riskData)

	// Also send a notification directly to Slack
	go sendSlackNotification(riskData)

	// Return the created risk
	json.NewEncoder(w).Encode(ResponseResult{Result: riskData})
}

// Generic function to send Slack notifications for any GRC item
func sendGenericSlackNotification(itemType string, itemData map[string]interface{}) {
	// Log the beginning of the function
	log.Printf("[SERVICENOW] Beginning to send Slack notification for %s: %v", itemType, itemData["number"])

	// Extract common fields, with fallbacks for different field names
	title := ""
	if shortDesc, ok := itemData["short_description"].(string); ok && shortDesc != "" {
		title = shortDesc
	} else if itemTitle, ok := itemData["title"].(string); ok && itemTitle != "" {
		title = itemTitle
	} else {
		title = "New " + itemType
	}

	description := ""
	if desc, ok := itemData["description"].(string); ok {
		description = desc
	}

	severity := "Unknown"
	if sev, ok := itemData["severity"].(string); ok {
		severity = sev
	}

	itemNumber := ""
	if num, ok := itemData["number"].(string); ok && num != "" {
		itemNumber = num
	} else if sysID, ok := itemData["sys_id"].(string); ok {
		itemNumber = sysID
	}

	// Determine the appropriate channel based on item type
	channelID := "C12345" // default to general
	switch itemType {
	case "risk":
		channelID = "C67890" // risk-management
	case "incident":
		channelID = "C22222" // incident-response
	case "compliance_task":
		channelID = "C11111" // compliance-team
	case "audit_finding":
		channelID = "C54321" // audit
	case "control_test":
		channelID = "C66666" // control-testing
	case "vendor_risk":
		channelID = "C33333" // vendor-risk
	case "regulatory_change":
		channelID = "C44444" // regulatory-updates
	}

	// Log the extracted details
	log.Printf("[SERVICENOW] Item details - Title: %s, Severity: %s, Number: %s",
		title, severity, itemNumber)

	// Create appropriate message format
	message := fmt.Sprintf("*New %s: %s*\n*ID:* %s", itemType, title, itemNumber)

	if severity != "Unknown" {
		message += fmt.Sprintf("\n*Severity:* %s", severity)
	}

	if description != "" {
		message += fmt.Sprintf("\n*Description:* %s", description)
	}

	// Add any item-specific fields
	switch itemType {
	case "compliance_task":
		if framework, ok := itemData["compliance_framework"].(string); ok {
			message += fmt.Sprintf("\n*Framework:* %s", framework)
		}
		if dueDate, ok := itemData["due_date"].(string); ok {
			message += fmt.Sprintf("\n*Due Date:* %s", dueDate)
		}
	case "control_test":
		if controlName, ok := itemData["control_name"].(string); ok {
			message += fmt.Sprintf("\n*Control:* %s", controlName)
		}
		if framework, ok := itemData["framework"].(string); ok {
			message += fmt.Sprintf("\n*Framework:* %s", framework)
		}
	case "audit_finding":
		if auditName, ok := itemData["audit_name"].(string); ok {
			message += fmt.Sprintf("\n*Audit:* %s", auditName)
		}
	case "vendor_risk":
		if vendorName, ok := itemData["vendor_name"].(string); ok {
			message += fmt.Sprintf("\n*Vendor:* %s", vendorName)
		}
	case "regulatory_change":
		if regName, ok := itemData["regulation_name"].(string); ok {
			message += fmt.Sprintf("\n*Regulation:* %s", regName)
		}
		if jurisdiction, ok := itemData["jurisdiction"].(string); ok {
			message += fmt.Sprintf("\n*Jurisdiction:* %s", jurisdiction)
		}
	}

	// Create appropriate action buttons based on item type
	var actionElements []map[string]interface{}

	switch itemType {
	case "risk":
		actionElements = []map[string]interface{}{
			{
				"type": "button",
				"text": map[string]interface{}{
					"type": "plain_text",
					"text": "Discuss Mitigation",
				},
				"action_id": "discuss_risk",
				"value":     "discuss_" + itemNumber,
			},
			{
				"type": "button",
				"text": map[string]interface{}{
					"type": "plain_text",
					"text": "Assign Owner",
				},
				"action_id": "assign_risk",
				"value":     "assign_" + itemNumber,
			},
		}
	case "incident":
		actionElements = []map[string]interface{}{
			{
				"type": "button",
				"text": map[string]interface{}{
					"type": "plain_text",
					"text": "🚨 Acknowledge",
				},
				"action_id": "acknowledge_incident",
				"value":     "ack_incident_" + itemNumber,
			},
			{
				"type": "button",
				"text": map[string]interface{}{
					"type": "plain_text",
					"text": "📝 Add Update",
				},
				"action_id": "update_incident",
				"value":     "update_" + itemNumber,
			},
			{
				"type": "button",
				"text": map[string]interface{}{
					"type": "plain_text",
					"text": "✅ Resolve",
				},
				"action_id": "resolve_incident",
				"value":     "resolve_" + itemNumber,
			},
		}
	case "audit_finding":
		actionElements = []map[string]interface{}{
			{
				"type": "button",
				"text": map[string]interface{}{
					"type": "plain_text",
					"text": "Assign Owner",
				},
				"action_id": "assign_finding",
				"value":     "assign_finding_" + itemNumber,
			},
			{
				"type": "button",
				"text": map[string]interface{}{
					"type": "plain_text",
					"text": "Resolve Finding",
				},
				"action_id": "resolve_finding",
				"value":     "resolve_finding_" + itemNumber,
			},
		}
	case "control_test":
		actionElements = []map[string]interface{}{
			{
				"type": "button",
				"text": map[string]interface{}{
					"type": "plain_text",
					"text": "Submit Results",
				},
				"action_id": "submit_test_results",
				"value":     "test_results_" + itemNumber,
			},
		}
	case "vendor_risk":
		actionElements = []map[string]interface{}{
			{
				"type": "button",
				"text": map[string]interface{}{
					"type": "plain_text",
					"text": "Request Report",
				},
				"action_id": "request_compliance_report",
				"value":     "request_report_" + itemNumber,
			},
			{
				"type": "button",
				"text": map[string]interface{}{
					"type": "plain_text",
					"text": "Update Status",
				},
				"action_id": "update_vendor_status",
				"value":     "update_vendor_" + itemNumber,
			},
		}
	case "regulatory_change":
		actionElements = []map[string]interface{}{
			{
				"type": "button",
				"text": map[string]interface{}{
					"type": "plain_text",
					"text": "Add Impact Assessment",
				},
				"action_id": "add_impact_assessment",
				"value":     "assess_impact_" + itemNumber,
			},
			{
				"type": "button",
				"text": map[string]interface{}{
					"type": "plain_text",
					"text": "Create Implementation Plan",
				},
				"action_id": "create_implementation_plan",
				"value":     "plan_" + itemNumber,
			},
		}
	}

	// Prepare the Slack message payload
	data := map[string]interface{}{
		"channel": channelID,
		"text":    message,
		"blocks": []map[string]interface{}{
			{
				"type": "section",
				"text": map[string]interface{}{
					"type": "mrkdwn",
					"text": message,
				},
			},
		},
	}

	// Add action buttons if available
	if len(actionElements) > 0 {
		data["blocks"] = append(data["blocks"].([]map[string]interface{}), map[string]interface{}{
			"type":     "actions",
			"elements": actionElements,
		})
	}

	jsonData, err := json.Marshal(data)
	if err != nil {
		log.Printf("[SERVICENOW] ERROR: Failed to marshal Slack notification JSON: %v", err)
		return
	}

	log.Printf("[SERVICENOW] Sending Slack notification payload: %s", string(jsonData))

	resp, err := http.Post("http://localhost:3002/api/chat.postMessage", "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		log.Printf("[SERVICENOW] ERROR: Failed to send Slack notification: %v", err)
		return
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	log.Printf("[SERVICENOW] Slack notification response: %s", string(respBody))

	// Check if the response was successful
	if resp.StatusCode != http.StatusOK {
		log.Printf("[SERVICENOW] ERROR: Slack responded with non-OK status code: %d", resp.StatusCode)
	} else {
		log.Printf("[SERVICENOW] Successfully sent notification to Slack")
	}
}

// Inside your servicenow_mock.go, add this function:
// Original sendSlackNotification can just call the generic function for risks
func sendSlackNotification(riskData map[string]interface{}) {
	sendGenericSlackNotification("risk", riskData)
}
func main() {
	r := mux.NewRouter()

	for _, tableName := range []string{
		"risks", "compliance_tasks", "incidents", "control_tests",
		"audit_findings", "vendor_risks", "regulatory_changes",
	} {
		if MockDatabase[tableName] == nil {
			MockDatabase[tableName] = make(map[string]interface{})
		}
	}
	// Add routes for different ServiceNow tables
	// Keep the existing specific GET handlers while using generic handlers for POST
	// Risk
	r.HandleFunc("/api/now/table/sn_risk_risk", handleRisks).Methods("GET")
	r.HandleFunc("/api/now/table/sn_risk_risk", handleGenericTable).Methods("POST", "PATCH")
	r.HandleFunc("/api/now/table/sn_risk_risk/{id}", handleRiskByID).Methods("GET", "PATCH", "DELETE")

	// Compliance Tasks
	r.HandleFunc("/api/now/table/sn_compliance_task", handleComplianceTasks).Methods("GET")
	r.HandleFunc("/api/now/table/sn_compliance_task", handleGenericTable).Methods("POST", "PATCH")
	r.HandleFunc("/api/now/table/sn_compliance_task/{id}", handleComplianceTaskByID).Methods("GET", "PATCH", "DELETE")

	// Incidents
	r.HandleFunc("/api/now/table/sn_si_incident", handleIncidents).Methods("GET")
	r.HandleFunc("/api/now/table/sn_si_incident", handleGenericTable).Methods("POST", "PATCH")
	r.HandleFunc("/api/now/table/sn_si_incident/{id}", handleIncidentByID).Methods("GET", "PATCH", "DELETE")

	// Control Tests
	r.HandleFunc("/api/now/table/sn_policy_control_test", handleControlTests).Methods("GET")
	r.HandleFunc("/api/now/table/sn_policy_control_test", handleGenericTable).Methods("POST", "PATCH")
	r.HandleFunc("/api/now/table/sn_policy_control_test/{id}", handleControlTestByID).Methods("GET", "PATCH", "DELETE")

	// Audit Findings
	r.HandleFunc("/api/now/table/sn_audit_finding", handleAuditFindings).Methods("GET")
	r.HandleFunc("/api/now/table/sn_audit_finding", handleGenericTable).Methods("POST", "PATCH")
	r.HandleFunc("/api/now/table/sn_audit_finding/{id}", handleAuditFindingByID).Methods("GET", "PATCH", "DELETE")

	// Vendor Risks
	r.HandleFunc("/api/now/table/sn_vendor_risk", handleVendorRisks).Methods("GET")
	r.HandleFunc("/api/now/table/sn_vendor_risk", handleGenericTable).Methods("POST", "PATCH")
	r.HandleFunc("/api/now/table/sn_vendor_risk/{id}", handleVendorRiskByID).Methods("GET", "PATCH", "DELETE")

	// Regulatory Changes
	r.HandleFunc("/api/now/table/sn_regulatory_change", handleRegulatoryChanges).Methods("GET")
	r.HandleFunc("/api/now/table/sn_regulatory_change", handleGenericTable).Methods("POST", "PATCH")
	r.HandleFunc("/api/now/table/sn_regulatory_change/{id}", handleRegulatoryChangeByID).Methods("GET", "PATCH", "DELETE")

	// Special risk creation handler
	r.HandleFunc("/servicenow/create_risk", handleCreateRisk).Methods("POST")

	// Other endpoints
	r.HandleFunc("/api/now/table/sn_grc_summary", handleGRCSummary).Methods("GET")
	r.HandleFunc("/api/now/table/sn_risk_by_category", handleRisksByCategory).Methods("GET")
	r.HandleFunc("/reset", resetHandler).Methods("POST")
	r.HandleFunc("/api/slack/commands", handleSlackCommands).Methods("POST")
	r.HandleFunc("/api/slack/interactions", handleSlackInteractions).Methods("POST")

	// Create initial test data
	createInitialTestData()

	// Health check
	r.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	}).Methods("GET")

	// Test webhook trigger endpoint
	r.HandleFunc("/trigger_webhook/{table_name}/{action_type}", triggerWebhookHandler).Methods("POST")

	// Add webhook receiver for Jira updates (optional, if bypassing port 8080)
	r.HandleFunc("/api/webhooks/jira", handleJiraWebhook).Methods("POST")

	// Add a simple UI handler for the root path
	r.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		html := `<!DOCTYPE html><html><head><title>Mock ServiceNow GRC</title>` +
			`<style>body { font-family: Arial, sans-serif; margin: 0; padding: 20px; }` +
			`.header { background: #0057a6; color: white; padding: 15px; }` +
			`.container { max-width: 1200px; margin: 0 auto; }` +
			`.card { border: 1px solid #ddd; border-radius: 4px; padding: 15px; margin-bottom: 15px; background: white; }` +
			`h2 { color: #0057a6; } table { width: 100%; border-collapse: collapse; }` +
			`table, th, td { border: 1px solid #ddd; } th, td { padding: 10px; text-align: left; }` +
			`th { background-color: #f2f2f2; } .risk-high { background-color: #ffebee; }` +
			`.risk-medium { background-color: #fff8e1; } .risk-low { background-color: #e8f5e9; }` +
			`.button { display: inline-block; padding: 8px 16px; background: #0057a6; color: white; text-decoration: none; border-radius: 4px; cursor: pointer; border: none; }` +
			`.tab-container { border-bottom: 1px solid #ddd; margin-bottom: 20px; }` +
			`.tab { display: inline-block; padding: 10px 15px; cursor: pointer; }` +
			`.tab.active { background: #0057a6; color: white; }` +
			`.tab-content { display: none; }` +
			`.tab-content.active { display: block; }` +
			`form div { margin-bottom: 10px; }` +
			`input, select, textarea { width: 100%; padding: 8px; box-sizing: border-box; }` +
			`.grid { display: grid; grid-template-columns: 1fr 1fr; gap: 10px; }` +
			`@media (max-width: 768px) { .grid { grid-template-columns: 1fr; } }` +
			`</style></head><body><div class="header"><div class="container">` +
			`<h1>ServiceNow GRC Platform</h1></div></div>` +
			`<div class="container">` +
			`<div class="card"><h2>GRC Dashboard</h2><div id="grc-summary"></div></div>` +

			// Tab navigation
			`<div class="tab-container">` +
			`<div class="tab active" data-tab="risks">Risks</div>` +
			`<div class="tab" data-tab="compliance">Compliance Tasks</div>` +
			`<div class="tab" data-tab="incidents">Incidents</div>` +
			`<div class="tab" data-tab="audit">Audit Findings</div>` +
			`<div class="tab" data-tab="control">Control Tests</div>` +
			`<div class="tab" data-tab="vendor">Vendor Risks</div>` +
			`<div class="tab" data-tab="regulatory">Regulatory Changes</div>` +
			`</div>` +

			// Risk tab
			`<div class="tab-content active" id="risks-tab">` +
			`<div class="card"><h2>Risk Register</h2><table id="risk-table"><thead><tr>` +
			`<th>ID</th><th>Title</th><th>Severity</th><th>Category</th><th>Owner</th><th>Status</th><th>Created</th>` +
			`</tr></thead><tbody id="risk-data"></tbody></table></div>` +
			`<div class="card"><h2>Add New Risk</h2><form id="risk-form">` +
			`<div class="grid">` +
			`<div><label>Title:</label><input type="text" name="title" required></div>` +
			`<div><label>Severity:</label><select name="severity" required>` +
			`<option value="Critical">Critical</option><option value="High">High</option><option value="Medium">Medium</option><option value="Low">Low</option>` +
			`</select></div></div><div><label>Description:</label>` +
			`<textarea name="description" rows="3" required></textarea></div>` +
			`<div class="grid">` +
			`<div><label>Category:</label><select name="category" required>` +
			`<option value="Cybersecurity">Cybersecurity</option><option value="Financial">Financial</option>` +
			`<option value="Operational">Operational</option><option value="Compliance">Compliance</option>` +
			`<option value="Strategic">Strategic</option></select></div>` +
			`<div><label>Owner:</label><input type="text" name="owner" required></div></div>` +
			`<div><button type="submit" class="button">Create Risk</button></div>` +
			`</form></div>` +
			`</div>` +

			// Compliance Tasks tab
			`<div class="tab-content" id="compliance-tab">` +
			`<div class="card"><h2>Compliance Tasks</h2><table id="compliance-table"><thead><tr>` +
			`<th>ID</th><th>Title</th><th>Framework</th><th>Assigned To</th><th>Due Date</th><th>Status</th>` +
			`</tr></thead><tbody id="compliance-data"></tbody></table></div>` +
			`<div class="card"><h2>Add New Compliance Task</h2><form id="compliance-form">` +
			`<div class="grid">` +
			`<div><label>Title:</label><input type="text" name="short_description" required></div>` +
			`<div><label>Framework:</label><select name="compliance_framework" required>` +
			`<option value="GDPR">GDPR</option><option value="HIPAA">HIPAA</option><option value="PCI-DSS">PCI-DSS</option>` +
			`<option value="SOX">SOX</option><option value="NIST">NIST</option></select></div></div>` +
			`<div><label>Description:</label><textarea name="description" rows="3" required></textarea></div>` +
			`<div class="grid">` +
			`<div><label>Assigned To:</label><input type="text" name="assigned_to" required></div>` +
			`<div><label>Due Date:</label><input type="date" name="due_date" required></div></div>` +
			`<div><button type="submit" class="button">Create Compliance Task</button></div>` +
			`</form></div>` +
			`</div>` +

			// Incidents tab
			`<div class="tab-content" id="incidents-tab">` +
			`<div class="card"><h2>Incidents</h2><table id="incident-table"><thead><tr>` +
			`<th>ID</th><th>Title</th><th>Severity</th><th>Category</th><th>Status</th><th>Created</th>` +
			`</tr></thead><tbody id="incident-data"></tbody></table></div>` +
			`<div class="card"><h2>Add New Incident</h2><form id="incident-form">` +
			`<div class="grid">` +
			`<div><label>Title:</label><input type="text" name="short_description" required></div>` +
			`<div><label>Severity:</label><select name="severity" required>` +
			`<option value="Critical">Critical</option><option value="High">High</option><option value="Medium">Medium</option><option value="Low">Low</option>` +
			`</select></div></div>` +
			`<div><label>Description:</label><textarea name="description" rows="3" required></textarea></div>` +
			`<div class="grid">` +
			`<div><label>Category:</label><select name="category" required>` +
			`<option value="Security">Security</option><option value="Hardware">Hardware</option>` +
			`<option value="Software">Software</option><option value="Network">Network</option>` +
			`<option value="Database">Database</option></select></div>` +
			`<div><label>Impact:</label><select name="impact" required>` +
			`<option value="High">High</option><option value="Medium">Medium</option><option value="Low">Low</option>` +
			`</select></div></div>` +
			`<div><button type="submit" class="button">Create Incident</button></div>` +
			`</form></div>` +
			`</div>` +

			// Audit Findings tab
			`<div class="tab-content" id="audit-tab">` +
			`<div class="card"><h2>Audit Findings</h2><table id="audit-table"><thead><tr>` +
			`<th>ID</th><th>Title</th><th>Audit Name</th><th>Severity</th><th>Due Date</th><th>Status</th>` +
			`</tr></thead><tbody id="audit-data"></tbody></table></div>` +
			`<div class="card"><h2>Add New Audit Finding</h2><form id="audit-form">` +
			`<div class="grid">` +
			`<div><label>Title:</label><input type="text" name="short_description" required></div>` +
			`<div><label>Audit Name:</label><input type="text" name="audit_name" required></div></div>` +
			`<div><label>Description:</label><textarea name="description" rows="3" required></textarea></div>` +
			`<div class="grid">` +
			`<div><label>Severity:</label><select name="severity" required>` +
			`<option value="High">High</option><option value="Medium">Medium</option><option value="Low">Low</option>` +
			`</select></div>` +
			`<div><label>Due Date:</label><input type="date" name="due_date" required></div></div>` +
			`<div><button type="submit" class="button">Create Audit Finding</button></div>` +
			`</form></div>` +
			`</div>` +

			// Control Tests tab
			`<div class="tab-content" id="control-tab">` +
			`<div class="card"><h2>Control Tests</h2><table id="control-table"><thead><tr>` +
			`<th>ID</th><th>Title</th><th>Control Name</th><th>Framework</th><th>Due Date</th><th>Status</th>` +
			`</tr></thead><tbody id="control-data"></tbody></table></div>` +
			`<div class="card"><h2>Add New Control Test</h2><form id="control-form">` +
			`<div class="grid">` +
			`<div><label>Title:</label><input type="text" name="short_description" required></div>` +
			`<div><label>Control Name:</label><input type="text" name="control_name" required></div></div>` +
			`<div><label>Description:</label><textarea name="description" rows="3" required></textarea></div>` +
			`<div class="grid">` +
			`<div><label>Framework:</label><select name="framework" required>` +
			`<option value="SOX">SOX</option><option value="NIST">NIST</option>` +
			`<option value="ISO 27001">ISO 27001</option><option value="PCI-DSS">PCI-DSS</option>` +
			`</select></div>` +
			`<div><label>Due Date:</label><input type="date" name="due_date" required></div></div>` +
			`<div><button type="submit" class="button">Create Control Test</button></div>` +
			`</form></div>` +
			`</div>` +

			// Vendor Risks tab
			`<div class="tab-content" id="vendor-tab">` +
			`<div class="card"><h2>Vendor Risks</h2><table id="vendor-table"><thead><tr>` +
			`<th>ID</th><th>Title</th><th>Vendor</th><th>Severity</th><th>Due Date</th><th>Status</th>` +
			`</tr></thead><tbody id="vendor-data"></tbody></table></div>` +
			`<div class="card"><h2>Add New Vendor Risk</h2><form id="vendor-form">` +
			`<div class="grid">` +
			`<div><label>Title:</label><input type="text" name="short_description" required></div>` +
			`<div><label>Vendor Name:</label><input type="text" name="vendor_name" required></div></div>` +
			`<div><label>Description:</label><textarea name="description" rows="3" required></textarea></div>` +
			`<div class="grid">` +
			`<div><label>Severity:</label><select name="severity" required>` +
			`<option value="High">High</option><option value="Medium">Medium</option><option value="Low">Low</option>` +
			`</select></div>` +
			`<div><label>Due Date:</label><input type="date" name="due_date" required></div></div>` +
			`<div><button type="submit" class="button">Create Vendor Risk</button></div>` +
			`</form></div>` +
			`</div>` +

			// Regulatory Changes tab
			`<div class="tab-content" id="regulatory-tab">` +
			`<div class="card"><h2>Regulatory Changes</h2><table id="regulatory-table"><thead><tr>` +
			`<th>ID</th><th>Title</th><th>Regulation</th><th>Jurisdiction</th><th>Effective Date</th><th>Status</th>` +
			`</tr></thead><tbody id="regulatory-data"></tbody></table></div>` +
			`<div class="card"><h2>Add New Regulatory Change</h2><form id="regulatory-form">` +
			`<div class="grid">` +
			`<div><label>Title:</label><input type="text" name="short_description" required></div>` +
			`<div><label>Regulation Name:</label><input type="text" name="regulation_name" required></div></div>` +
			`<div><label>Description:</label><textarea name="description" rows="3" required></textarea></div>` +
			`<div class="grid">` +
			`<div><label>Jurisdiction:</label><input type="text" name="jurisdiction" required></div>` +
			`<div><label>Effective Date:</label><input type="date" name="effective_date" required></div></div>` +
			`<div><button type="submit" class="button">Create Regulatory Change</button></div>` +
			`</form></div>` +
			`</div>` +

			`</div>` + // End of container

			`<script>
			// Tab switching
			document.addEventListener('DOMContentLoaded', function() {
				const tabs = document.querySelectorAll('.tab');
				tabs.forEach(tab => {
					tab.addEventListener('click', function() {
						// Remove active class from all tabs
						tabs.forEach(t => t.classList.remove('active'));
						
						// Add active class to clicked tab
						this.classList.add('active');
						
						// Hide all tab content
						document.querySelectorAll('.tab-content').forEach(content => {
							content.classList.remove('active');
						});
						
						// Show corresponding tab content
						const tabId = this.getAttribute('data-tab') + '-tab';
						document.getElementById(tabId).classList.add('active');
					});
				});
				
				// Load data for all tables
				loadAllData();
				loadSummary();
				
				// Risk form submission
				document.getElementById('risk-form').addEventListener('submit', function(e) {
					e.preventDefault();
					submitForm(this, '/servicenow/create_risk', function() {
						loadRisks();
						loadSummary();
					});
				});
				
				// Compliance form submission
				document.getElementById('compliance-form').addEventListener('submit', function(e) {
					e.preventDefault();
					submitForm(this, '/api/now/table/sn_compliance_task', function() {
						loadComplianceTasks();
						loadSummary();
					});
				});
				
				// Incident form submission
				document.getElementById('incident-form').addEventListener('submit', function(e) {
					e.preventDefault();
					submitForm(this, '/api/now/table/sn_si_incident', function() {
						loadIncidents();
						loadSummary();
					});
				});
				
				// Audit form submission
				document.getElementById('audit-form').addEventListener('submit', function(e) {
					e.preventDefault();
					submitForm(this, '/api/now/table/sn_audit_finding', function() {
						loadAuditFindings();
						loadSummary();
					});
				});
				
				// Control test form submission
				document.getElementById('control-form').addEventListener('submit', function(e) {
					e.preventDefault();
					submitForm(this, '/api/now/table/sn_policy_control_test', function() {
						loadControlTests();
						loadSummary();
					});
				});
				
				// Vendor risk form submission
				document.getElementById('vendor-form').addEventListener('submit', function(e) {
					e.preventDefault();
					submitForm(this, '/api/now/table/sn_vendor_risk', function() {
						loadVendorRisks();
						loadSummary();
					});
				});
				
				// Regulatory change form submission
				document.getElementById('regulatory-form').addEventListener('submit', function(e) {
					e.preventDefault();
					submitForm(this, '/api/now/table/sn_regulatory_change', function() {
						loadRegulatoryChanges();
						loadSummary();
					});
				});
			});
			
			// Generic form submission function
			function submitForm(form, url, callback) {
				const formData = new FormData(form);
				const data = {};
				
				formData.forEach((value, key) => {
					// Handle date format conversion
					if (key.includes('date') && value) {
						data[key] = new Date(value).toISOString();
					} else {
						data[key] = value;
					}
				});
				
				// If it's a ServiceNow form, add status
				if (!url.includes('create_risk')) {
					data.status = 'Open';
				}
				
				fetch(url, {
					method: 'POST', 
					headers: {'Content-Type': 'application/json'},
					body: JSON.stringify(data)
				})
				.then(response => response.json())
				.then(result => {
					alert('Item created successfully!');
					form.reset();
					if (callback) callback();
				})
				.catch(error => {
					console.error('Error creating item:', error);
					alert('Failed to create item. See console for details.');
				});
			}
			
			// Load all data
			function loadAllData() {
				loadRisks();
				loadComplianceTasks();
				loadIncidents();
				loadAuditFindings();
				loadControlTests();
				loadVendorRisks();
				loadRegulatoryChanges();
			}
			
			// Load risks
			// --------- REPLACE THESE EXISTING FUNCTIONS WITH THE NEW ONES ---------
function loadRisks() {
    fetchTableData('/api/now/table/sn_risk_risk', 'risk-data', 
        item => [
            item.number || item.sys_id,
            item.title || 'Untitled',
            item.severity || 'Unknown',
            item.category || 'Uncategorized',
            item.owner || 'Unassigned',
            item.status || 'New',
            new Date(item.created_on).toLocaleDateString() || 'Unknown'
        ],
        item => item.severity === 'Critical' || item.severity === 'High' ? 'risk-high' : 
               item.severity === 'Medium' ? 'risk-medium' : 'risk-low'
    );
}

function loadComplianceTasks() {
    fetchTableData('/api/now/table/sn_compliance_task', 'compliance-data', 
        item => [
            item.number || item.sys_id,
            item.short_description || 'Untitled',
            item.compliance_framework || 'Unknown',
            item.assigned_to || 'Unassigned',
            new Date(item.due_date).toLocaleDateString() || 'No date',
            item.status || 'New'
        ]
    );
}

// (Similar replacements for other load functions: loadIncidents, loadAuditFindings, etc.)
			
			// Load incidents
			function loadIncidents() {
				fetchTableData('/api/now/table/sn_si_incident', 'incident-data', 
					item => [
						item.number || item.sys_id,
						item.short_description || 'Untitled',
						item.severity || 'Unknown',
						item.category || 'Uncategorized',
						item.status || 'New',
						new Date(item.created_on).toLocaleDateString() || 'Unknown'
					],
					item => item.severity === 'Critical' || item.severity === 'High' ? 'risk-high' : 
						   item.severity === 'Medium' ? 'risk-medium' : 'risk-low'
				);
			}
			
			// Load audit findings
			function loadAuditFindings() {
				fetchTableData('/api/now/table/sn_audit_finding', 'audit-data', 
					item => [
						item.number || item.sys_id,
						item.short_description || 'Untitled',
						item.audit_name || 'Unknown',
						item.severity || 'Unknown',
						new Date(item.due_date).toLocaleDateString() || 'No date',
						item.status || 'New'
					],
					item => item.severity === 'High' ? 'risk-high' : 
						   item.severity === 'Medium' ? 'risk-medium' : 'risk-low'
				);
			}
			
			// Load control tests
			function loadControlTests() {
				fetchTableData('/api/now/table/sn_policy_control_test', 'control-data', 
					item => [
						item.number || item.sys_id,
						item.short_description || 'Untitled',
						item.control_name || 'Unknown',
						item.framework || 'Unknown',
						new Date(item.due_date).toLocaleDateString() || 'No date',
						item.test_status || 'Open'
					]
				);
			}
			
			// Load vendor risks
			function loadVendorRisks() {
				fetchTableData('/api/now/table/sn_vendor_risk', 'vendor-data', 
					item => [
						item.number || item.sys_id,
						item.short_description || 'Untitled',
						item.vendor_name || 'Unknown',
						item.severity || 'Unknown',
						new Date(item.due_date).toLocaleDateString() || 'No date',
						item.status || 'Open'
					],
					item => item.severity === 'High' ? 'risk-high' : 
						   item.severity === 'Medium' ? 'risk-medium' : 'risk-low'
				);
			}
			
			// Load regulatory changes
			function loadRegulatoryChanges() {
				fetchTableData('/api/now/table/sn_regulatory_change', 'regulatory-data', 
					item => [
						item.number || item.sys_id,
						item.short_description || 'Untitled',
						item.regulation_name || 'Unknown',
						item.jurisdiction || 'Unknown',
						new Date(item.effective_date).toLocaleDateString() || 'No date',
						item.status || 'Open'
					]
				);
			}
			
			// Generic function to fetch and display table data
			function fetchTableData(url, tableId, rowFormatter, rowClassFormatter) {
				fetch(url)
				.then(response => response.json())
				.then(data => {
					const tableBody = document.getElementById(tableId);
					tableBody.innerHTML = '';
					
					if (data.result && data.result.length) {
						data.result.forEach(item => {
							const row = document.createElement('tr');
							
							if (rowClassFormatter) {
								row.className = rowClassFormatter(item);
							}
							
							const cells = rowFormatter(item);
							cells.forEach(cellContent => {
								const cell = document.createElement('td');
								cell.textContent = cellContent;
								row.appendChild(cell);
							});
							
							tableBody.appendChild(row);
						});
					} else {
						const row = document.createElement('tr');
						const cell = document.createElement('td');
						cell.colSpan = '7';
						cell.style.textAlign = 'center';
						cell.textContent = 'No items found';
						row.appendChild(cell);
						tableBody.appendChild(row);
					}
				})
				.catch(error => {
					console.error('Error fetching data:', error);
					const tableBody = document.getElementById(tableId);
					tableBody.innerHTML = '<tr><td colspan="7" style="text-align: center;">Error loading data</td></tr>';
				});
			}
			
			// Load GRC summary data
			function loadSummary() {
				fetch('/api/now/table/sn_grc_summary')
				.then(response => response.json())
				.then(data => {
					const summary = data.result;
					const summaryDiv = document.getElementById('grc-summary');
					
					summaryDiv.innerHTML = '<div style="display: flex; flex-wrap: wrap; gap: 15px;">' +
					'<div style="flex: 1; text-align: center; padding: 15px; background: #e3f2fd; border-radius: 4px;">' +
					'<div style="font-size: 32px; font-weight: bold;">' + (summary.open_risks || 0) + '</div><div>Open Risks</div></div>' +
					
					'<div style="flex: 1; text-align: center; padding: 15px; background: #fff8e1; border-radius: 4px;">' +
					'<div style="font-size: 32px; font-weight: bold;">' + (summary.open_compliance_tasks || 0) + '</div><div>Compliance Tasks</div></div>' +
					
					'<div style="flex: 1; text-align: center; padding: 15px; background: #ffebee; border-radius: 4px;">' +
					'<div style="font-size: 32px; font-weight: bold;">' + (summary.open_incidents || 0) + '</div><div>Incidents</div></div>' +
					
					'<div style="flex: 1; text-align: center; padding: 15px; background: #e8f5e9; border-radius: 4px;">' +
					'<div style="font-size: 32px; font-weight: bold;">' + (summary.compliance_score || 0) + '%</div><div>Compliance Score</div></div>' +
					'</div>' +
					
					'<div style="display: flex; flex-wrap: wrap; gap: 15px; margin-top: 15px;">' +
					'<div style="flex: 1; text-align: center; padding: 15px; background: #f3e5f5; border-radius: 4px;">' +
					'<div style="font-size: 32px; font-weight: bold;">' + (summary.open_audit_findings || 0) + '</div><div>Audit Findings</div></div>' +
					
					'<div style="flex: 1; text-align: center; padding: 15px; background: #e0f2f1; border-radius: 4px;">' +
					'<div style="font-size: 32px; font-weight: bold;">' + (summary.control_tests_in_progress || 0) + '</div><div>Control Tests</div></div>' +
					
					'<div style="flex: 1; text-align: center; padding: 15px; background: #f1f8e9; border-radius: 4px;">' +
					'<div style="font-size: 32px; font-weight: bold;">' + (summary.open_vendor_risks || 0) + '</div><div>Vendor Risks</div></div>' +
					
					'<div style="flex: 1; text-align: center; padding: 15px; background: #fce4ec; border-radius: 4px;">' +
					'<div style="font-size: 32px; font-weight: bold;">' + (summary.overdue_items || 0) + '</div><div>Overdue Items</div></div>' +
					'</div>';
				})
				.catch(error => {
					console.error('Error loading summary:', error);
					document.getElementById('grc-summary').innerHTML = '<p>Error loading GRC summary data</p>';
				});
			}
			</script></body></html>`
		w.Write([]byte(html))
	})

	// Start server
	port := "3000"
	fmt.Printf("Starting mock ServiceNow server on port %s...\n", port)
	log.Fatal(http.ListenAndServe(":"+port, r))
}

// Generic table handlers
func handleRisks(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	risks := []interface{}{}
	for _, risk := range MockDatabase["risks"] {
		risks = append(risks, risk)
	}
	json.NewEncoder(w).Encode(ResponseResult{Result: risks})
}

func handleRiskByID(w http.ResponseWriter, r *http.Request) {
	handleGenericItemByID(w, r)
}

func handleComplianceTasks(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	tasks := []interface{}{}
	for _, task := range MockDatabase["compliance_tasks"] {
		tasks = append(tasks, task)
	}
	json.NewEncoder(w).Encode(ResponseResult{Result: tasks})
}

func handleComplianceTaskByID(w http.ResponseWriter, r *http.Request) {
	handleGenericItemByID(w, r)
}

func handleIncidents(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	incidents := []interface{}{}
	for _, incident := range MockDatabase["incidents"] {
		incidents = append(incidents, incident)
	}
	json.NewEncoder(w).Encode(ResponseResult{Result: incidents})
}

func handleIncidentByID(w http.ResponseWriter, r *http.Request) {
	handleGenericItemByID(w, r)
}

func handleControlTests(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	tests := []interface{}{}
	for _, test := range MockDatabase["control_tests"] {
		tests = append(tests, test)
	}
	json.NewEncoder(w).Encode(ResponseResult{Result: tests})
}

func handleControlTestByID(w http.ResponseWriter, r *http.Request) {
	handleGenericItemByID(w, r)
}

func handleAuditFindings(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	findings := []interface{}{}
	for _, finding := range MockDatabase["audit_findings"] {
		findings = append(findings, finding)
	}
	json.NewEncoder(w).Encode(ResponseResult{Result: findings})
}

func handleAuditFindingByID(w http.ResponseWriter, r *http.Request) {
	handleGenericItemByID(w, r)
}

func handleVendorRisks(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	vendorRisks := []interface{}{}
	for _, risk := range MockDatabase["vendor_risks"] {
		vendorRisks = append(vendorRisks, risk)
	}
	json.NewEncoder(w).Encode(ResponseResult{Result: vendorRisks})
}

func handleVendorRiskByID(w http.ResponseWriter, r *http.Request) {
	handleGenericItemByID(w, r)
}

func handleRegulatoryChanges(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	changes := []interface{}{}
	for _, change := range MockDatabase["regulatory_changes"] {
		changes = append(changes, change)
	}
	json.NewEncoder(w).Encode(ResponseResult{Result: changes})
}

func handleRegulatoryChangeByID(w http.ResponseWriter, r *http.Request) {
	handleGenericItemByID(w, r)
}

// Reset handler to clear MockDatabase
func resetHandler(w http.ResponseWriter, r *http.Request) {
	MockDatabase = map[string]map[string]interface{}{
		"risks":              {},
		"compliance_tasks":   {},
		"incidents":          {},
		"control_tests":      {},
		"audit_findings":     {},
		"vendor_risks":       {},
		"regulatory_changes": {},
	}
	log.Println("Mock ServiceNow server reset")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "reset"})
}

// Generic handler implementations
func handleGenericTable(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	tableName := strings.TrimPrefix(r.URL.Path, "/api/now/table/")
	if tableName == "" {
		http.Error(w, "Invalid table name", http.StatusBadRequest)
		return
	}

	// Map ServiceNow table names to internal mock table names
	tableNameMap := map[string]string{
		"sn_risk_risk":           "risks",
		"sn_compliance_task":     "compliance_tasks",
		"sn_si_incident":         "incidents",
		"sn_policy_control_test": "control_tests",
		"sn_audit_finding":       "audit_findings",
		"sn_vendor_risk":         "vendor_risks",
		"sn_regulatory_change":   "regulatory_changes",
	}
	internalTable := tableNameMap[tableName]

	if internalTable == "" {
		http.Error(w, "Invalid table name", http.StatusBadRequest)
		return
	}

	// Make sure the table's map is initialized
	if MockDatabase[internalTable] == nil {
		MockDatabase[internalTable] = make(map[string]interface{})
	}

	switch r.Method {
	case "GET":
		// This is handled by specific handlers now but keeping for completeness
		items := []interface{}{}
		for _, item := range MockDatabase[internalTable] {
			items = append(items, item)
		}
		json.NewEncoder(w).Encode(ResponseResult{Result: items})

	case "POST":
		var itemData map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&itemData); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		// Use provided sys_id if available, otherwise generate one
		sysID, ok := itemData["sys_id"].(string)
		if !ok || sysID == "" {
			sysID = fmt.Sprintf("%s_%d", internalTable, time.Now().UnixNano())
			itemData["sys_id"] = sysID
		}

		// Generate appropriate number if not provided
		if _, ok := itemData["number"].(string); !ok {
			switch internalTable {
			case "risks":
				itemData["number"] = fmt.Sprintf("RISK%d", len(MockDatabase[internalTable])+1001)
			case "compliance_tasks":
				itemData["number"] = fmt.Sprintf("TASK%d", len(MockDatabase[internalTable])+1001)
			case "incidents":
				itemData["number"] = fmt.Sprintf("INC%d", len(MockDatabase[internalTable])+1001)
			case "control_tests":
				itemData["number"] = fmt.Sprintf("TEST%d", len(MockDatabase[internalTable])+1001)
			case "audit_findings":
				itemData["number"] = fmt.Sprintf("AUDIT-%d", len(MockDatabase[internalTable])+1001)
			case "vendor_risks":
				itemData["number"] = fmt.Sprintf("VR%d", len(MockDatabase[internalTable])+1001)
			case "regulatory_changes":
				itemData["number"] = fmt.Sprintf("REG%d", len(MockDatabase[internalTable])+1001)
			}
		}

		// Add timestamps if not provided
		if _, ok := itemData["created_on"]; !ok {
			itemData["created_on"] = time.Now().Format(time.RFC3339)
		}
		if _, ok := itemData["updated_on"]; !ok {
			itemData["updated_on"] = time.Now().Format(time.RFC3339)
		}

		// Store the item (Map check redundant but kept for safety)
		if MockDatabase[internalTable] == nil {
			MockDatabase[internalTable] = make(map[string]interface{})
		}
		MockDatabase[internalTable][sysID] = itemData

		log.Printf("[POST] %s: Created item with sys_id %s", tableName, sysID)

		// Map from table types to entity types for notification
		entityTypeMap := map[string]string{
			"risks":              "risk",
			"compliance_tasks":   "compliance_task",
			"incidents":          "incident",
			"control_tests":      "control_test",
			"audit_findings":     "audit_finding",
			"vendor_risks":       "vendor_risk",
			"regulatory_changes": "regulatory_change",
		}

		// Send webhook
		go triggerWebhook(tableName, sysID, "insert", itemData)

		// Send Slack notification
		entityType := entityTypeMap[internalTable]
		go sendGenericSlackNotification(entityType, itemData)

		// Return the created item
		json.NewEncoder(w).Encode(ResponseResult{Result: itemData})

	case "PATCH":
		http.Error(w, "PATCH not allowed on collection", http.StatusMethodNotAllowed)
		return
	}
}
func handleGenericItemByID(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	vars := mux.Vars(r)
	id := vars["id"]
	tableName := strings.TrimPrefix(strings.TrimSuffix(r.URL.Path, "/"+id), "/api/now/table/")
	if tableName == "" {
		http.Error(w, "Invalid table name", http.StatusBadRequest)
		return
	}

	// Map ServiceNow table names to internal mock table names
	tableNameMap := map[string]string{
		"sn_risk_risk":           "risks",
		"sn_compliance_task":     "compliance_tasks",
		"sn_si_incident":         "incidents",
		"sn_policy_control_test": "control_tests",
		"sn_audit_finding":       "audit_findings",
		"sn_vendor_risk":         "vendor_risks",
		"sn_regulatory_change":   "regulatory_changes",
	}
	internalTable := tableNameMap[tableName]
	if internalTable == "" {
		http.Error(w, "Invalid table name", http.StatusBadRequest)
		return
	}

	// Check if item exists
	item, exists := MockDatabase[internalTable][id]
	if !exists {
		log.Printf("GET %s/%s: Item not found", tableName, id)
		http.Error(w, "Item not found", http.StatusNotFound)
		return
	}

	switch r.Method {
	case "GET":
		log.Printf("GET %s/%s: Returning item", tableName, id)
		json.NewEncoder(w).Encode(ResponseResult{Result: item})

	case "PATCH":
		var updateData map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&updateData); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		// Update the item
		itemMap, ok := item.(map[string]interface{})
		if !ok {
			http.Error(w, "Internal server error: item is not a map", http.StatusInternalServerError)
			return
		}

		for k, v := range updateData {
			itemMap[k] = v
		}

		// Update last modified time
		itemMap["sys_updated_on"] = time.Now().Format(time.RFC3339)
		MockDatabase[internalTable][id] = itemMap
		log.Printf("PATCH %s/%s: Updated item with fields %v", tableName, id, updateData)

		json.NewEncoder(w).Encode(ResponseResult{Result: itemMap})

	case "DELETE":
		delete(MockDatabase[internalTable], id)
		log.Printf("DELETE %s/%s: Item deleted", tableName, id)
		w.WriteHeader(http.StatusNoContent)
	}
}

// Special handlers for GRC dashboard data
func handleGRCSummary(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	summary := GRCSummary{
		OpenRisks:              len(MockDatabase["risks"]),
		OpenComplianceTasks:    len(MockDatabase["compliance_tasks"]),
		OpenIncidents:          len(MockDatabase["incidents"]),
		ControlTestsInProgress: len(MockDatabase["control_tests"]),
		OpenAuditFindings:      len(MockDatabase["audit_findings"]),
		OpenVendorRisks:        len(MockDatabase["vendor_risks"]),
		PendingRegChanges:      len(MockDatabase["regulatory_changes"]),
		OverdueItems:           0,   // Changed from fixed value 3 to 0
		ComplianceScore:        100, // Changed from fixed value 85 to 100 (perfect score when no issues)
	}

	json.NewEncoder(w).Encode(ResponseResult{Result: summary})
}

func handleRisksByCategory(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// Generate mock risk categories
	categories := []RiskByCategory{
		{Category: "Security", Count: 5},
		{Category: "Financial", Count: 3},
		{Category: "Operational", Count: 7},
		{Category: "Compliance", Count: 4},
		{Category: "Strategic", Count: 2},
	}

	json.NewEncoder(w).Encode(ResponseResult{Result: categories})
}

// Webhook trigger helper function
func triggerWebhook(tableName, sysID, actionType string, data map[string]interface{}) {
	webhookPayload := map[string]interface{}{
		"sys_id":      sysID,
		"table_name":  tableName,
		"action_type": actionType,
		"data":        data,
	}

	jsonPayload, err := json.Marshal(webhookPayload)
	if err != nil {
		log.Printf("Error marshaling webhook payload for %s/%s: %v", tableName, sysID, err)
		return
	}

	resp, err := http.Post("http://localhost:8081/api/webhooks/servicenow", "application/json", bytes.NewBuffer(jsonPayload))
	if err != nil {
		log.Printf("Error sending webhook to http://localhost:8081/api/webhooks/servicenow for %s/%s: %v", tableName, sysID, err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		log.Printf("Webhook sent successfully to http://localhost:8081/api/webhooks/servicenow for %s/%s", tableName, sysID)
	} else {
		body, _ := io.ReadAll(resp.Body)
		log.Printf("Webhook failed for %s/%s: %d, %s", tableName, sysID, resp.StatusCode, string(body))
	}
}

// Webhook trigger endpoint
func triggerWebhookHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	tableName := vars["table_name"]
	actionType := vars["action_type"]

	// Parse the data from the request
	var data map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
		log.Printf("Error decoding request body: %v", err)
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Map ServiceNow table names to internal mock table names
	tableNameMap := map[string]string{
		"sn_risk_risk":           "risks",
		"sn_compliance_task":     "compliance_tasks",
		"sn_si_incident":         "incidents",
		"sn_policy_control_test": "control_tests",
		"sn_audit_finding":       "audit_findings",
		"sn_vendor_risk":         "vendor_risks",
		"sn_regulatory_change":   "regulatory_changes",
	}

	// Check if the table is valid and get the internal table name
	internalTable, validTable := tableNameMap[tableName]
	if !validTable {
		log.Printf("Invalid table name: %s", tableName)
		http.Error(w, "Invalid table name", http.StatusBadRequest)
		return
	}

	// Use provided sys_id if available, otherwise generate one
	sysID, ok := data["sys_id"].(string)
	if !ok || sysID == "" {
		sysID = fmt.Sprintf("mock%d", time.Now().UnixNano())
		data["sys_id"] = sysID
	}

	// Add timestamps if not provided
	if _, ok := data["created_on"]; !ok {
		data["created_on"] = time.Now().Format(time.RFC3339)
	}
	if _, ok := data["updated_on"]; !ok {
		data["updated_on"] = time.Now().Format(time.RFC3339)
	}

	// Ensure the internal table map is initialized
	if MockDatabase[internalTable] == nil {
		MockDatabase[internalTable] = make(map[string]interface{})
	}

	// Store the data in the mock database
	MockDatabase[internalTable][sysID] = data
	log.Printf("Webhook trigger stored %s/%s: %v", tableName, sysID, data)

	// Send the webhook
	go triggerWebhook(tableName, sysID, actionType, data)

	// Return the status
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"status":      "success",
		"message":     "Webhook sent to http://localhost:8081/api/webhooks/servicenow",
		"webhook_id":  fmt.Sprintf("mock-webhook-%d", time.Now().UnixNano()),
		"table_name":  tableName,
		"action_type": actionType,
	})
}

// Optional: Handle Jira webhook directly (if bypassing port 8080)
func handleJiraWebhook(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	var payload map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	log.Printf("Received Jira webhook: %v", payload)

	// Extract relevant data from Jira webhook
	issue, ok := payload["issue"].(map[string]interface{})
	if !ok {
		http.Error(w, "Missing issue in webhook payload", http.StatusBadRequest)
		return
	}

	fields, ok := issue["fields"].(map[string]interface{})
	if !ok {
		http.Error(w, "Missing fields in issue", http.StatusBadRequest)
		return
	}

	serviceNowID, ok := fields["customfield_servicenow_id"].(string)
	if !ok || serviceNowID == "" {
		http.Error(w, "Missing or invalid customfield_servicenow_id", http.StatusBadRequest)
		return
	}

	statusMap, ok := fields["status"].(map[string]interface{})
	if !ok {
		http.Error(w, "Missing status in fields", http.StatusBadRequest)
		return
	}

	status, ok := statusMap["name"].(string)
	if !ok {
		http.Error(w, "Missing status name", http.StatusBadRequest)
		return
	}

	// Update the corresponding control test
	item, exists := MockDatabase["control_tests"][serviceNowID]
	if !exists {
		http.Error(w, "Control test not found", http.StatusNotFound)
		return
	}

	itemMap, ok := item.(map[string]interface{})
	if !ok {
		http.Error(w, "Internal server error: item is not a map", http.StatusInternalServerError)
		return
	}

	itemMap["test_status"] = status
	itemMap["sys_updated_on"] = time.Now().Format(time.RFC3339)
	MockDatabase["control_tests"][serviceNowID] = itemMap

	log.Printf("Updated control test %s with status %s from Jira webhook", serviceNowID, status)

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"status": "received",
	})
}

// Updated handleSlackCommands function for paste.txt (ServiceNow mock server)
func handleSlackCommands(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		log.Printf("[SERVICENOW] Error parsing Slack command form: %v", err)
		http.Error(w, "Invalid form data", http.StatusBadRequest)
		return
	}

	command := r.FormValue("command")
	text := r.FormValue("text")
	userID := r.FormValue("user_id")
	channelID := r.FormValue("channel_id")

	// Log more details for debugging
	log.Printf("[SERVICENOW] Received Slack command: %s with text: %s from user %s in channel %s",
		command, text, userID, channelID)

	// Process the command
	response := processSlashCommand(command, text, userID)

	// Log the response being sent back
	responseBytes, _ := json.Marshal(response)
	log.Printf("[SERVICENOW] Sending response to Slack: %s", string(responseBytes))

	// Return the response
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// Add direct testing function to update an incident without going through Slack
func directUpdateIncident(incidentID string, status string) bool {
	for sysID, item := range MockDatabase["incidents"] {
		if itemMap, ok := item.(map[string]interface{}); ok {
			if number, hasNumber := itemMap["number"].(string); hasNumber && (number == incidentID || "INC"+incidentID == number) {
				itemMap["status"] = status
				itemMap["updated_on"] = time.Now().Format(time.RFC3339)
				MockDatabase["incidents"][sysID] = itemMap
				log.Printf("[DIRECT UPDATE] Updated incident %s status to %s", incidentID, status)
				return true
			}
		}
	}
	return false
}

func processSlashCommand(command, text, userID string) map[string]interface{} {
	parts := strings.Fields(text)

	switch command {
	case "/grc-status":
		// Return GRC status summary
		return map[string]interface{}{
			"response_type": "in_channel",
			"text": fmt.Sprintf("🔍 *GRC Status Summary*\n\n• *%d* Open Risks\n• *%d* Open Incidents\n• *%d* Open Compliance Tasks\n• *%d* Control Tests In Progress\n• *%d* Open Audit Findings\n• *%d* Open Vendor Risks\n• *%d* Pending Regulatory Changes",
				len(MockDatabase["risks"]),
				len(MockDatabase["incidents"]),
				len(MockDatabase["compliance_tasks"]),
				len(MockDatabase["control_tests"]),
				len(MockDatabase["audit_findings"]),
				len(MockDatabase["vendor_risks"]),
				len(MockDatabase["regulatory_changes"])),
		}

	case "/upload-evidence":
		if len(parts) < 2 {
			return map[string]interface{}{
				"response_type": "ephemeral",
				"text":          "Usage: /upload-evidence TASK_ID URL",
			}
		}

		taskID := parts[0]
		evidenceURL := parts[1]

		// Update the task
		success := updateServiceNowItem("compliance_tasks", taskID, map[string]interface{}{
			"evidence_url": evidenceURL,
			"has_evidence": true,
		})

		if success {
			return map[string]interface{}{
				"response_type": "in_channel",
				"text":          fmt.Sprintf("📎 Evidence for task *%s* has been uploaded: %s", taskID, evidenceURL),
			}
		} else {
			return map[string]interface{}{
				"response_type": "ephemeral",
				"text":          fmt.Sprintf("❌ Failed to upload evidence for task %s", taskID),
			}
		}

	case "/incident-update":
		if len(parts) < 2 {
			return map[string]interface{}{
				"response_type": "ephemeral",
				"text":          "Usage: /incident-update INC_ID details",
			}
		}

		incidentID := parts[0]
		updateDetails := strings.Join(parts[1:], " ")

		// Update the incident
		success := updateServiceNowItem("incidents", incidentID, map[string]interface{}{
			"last_update": updateDetails,
			"updated_on":  time.Now().Format(time.RFC3339),
		})

		if success {
			return map[string]interface{}{
				"response_type": "in_channel",
				"text":          fmt.Sprintf("📝 Incident *%s* has been updated: %s", incidentID, updateDetails),
			}
		} else {
			return map[string]interface{}{
				"response_type": "ephemeral",
				"text":          fmt.Sprintf("❌ Failed to update incident %s", incidentID),
			}
		}

	case "/resolve-incident":
		if len(parts) < 2 {
			return map[string]interface{}{
				"response_type": "ephemeral",
				"text":          "Usage: /resolve-incident INC_ID resolution_details",
			}
		}

		incidentID := parts[0]
		resolution := strings.Join(parts[1:], " ")

		// Update the incident
		success := updateServiceNowItem("incidents", incidentID, map[string]interface{}{
			"status":     "Resolved",
			"resolution": resolution,
		})

		if success {
			return map[string]interface{}{
				"response_type": "in_channel",
				"text":          fmt.Sprintf("✅ Incident *%s* has been resolved: %s", incidentID, resolution),
			}
		} else {
			return map[string]interface{}{
				"response_type": "ephemeral",
				"text":          fmt.Sprintf("❌ Failed to resolve incident %s", incidentID),
			}
		}

	case "/submit-test":
		if len(parts) < 3 {
			return map[string]interface{}{
				"response_type": "ephemeral",
				"text":          "Usage: /submit-test TEST_ID PASS|FAIL details",
			}
		}

		testID := parts[0]
		result := parts[1]
		details := strings.Join(parts[2:], " ")

		// Validate result
		if result != "PASS" && result != "FAIL" {
			return map[string]interface{}{
				"response_type": "ephemeral",
				"text":          "Result must be either PASS or FAIL",
			}
		}

		// Determine status based on result
		var status string
		if result == "PASS" {
			status = "Completed"
		} else {
			status = "Failed"
		}

		// Update the test
		success := updateServiceNowItem("control_tests", testID, map[string]interface{}{
			"test_status":  result,
			"test_details": details,
			"status":       status,
		})

		if success {
			return map[string]interface{}{
				"response_type": "in_channel",
				"text":          fmt.Sprintf("🧪 Test *%s* result submitted: *%s* - %s", testID, result, details),
			}
		} else {
			return map[string]interface{}{
				"response_type": "ephemeral",
				"text":          fmt.Sprintf("❌ Failed to submit results for test %s", testID),
			}
		}

	// (Rest of the function remains the same as in the original code)
	// ... [other cases remain unchanged]

	default:
		return map[string]interface{}{
			"response_type": "ephemeral",
			"text":          "Unknown command. Available commands: /upload-evidence, /incident-update, /resolve-incident, /submit-test, /resolve-finding, /update-vendor, /assess-impact, /plan-implementation, /grc-status, /assign-owner",
		}
	}
}

// Improved updateServiceNowItem function to better handle different item types
// updateServiceNowItem handles updating or creating items in the mock ServiceNow database
func updateServiceNowItem(table, itemID string, updates map[string]interface{}) bool {
	log.Printf("[MOCK SERVICENOW] Attempting to update %s with ID %s with updates: %v", table, itemID, updates)

	// Ensure the table exists in MockDatabase
	if MockDatabase[table] == nil {
		MockDatabase[table] = make(map[string]interface{})
	}

	// Normalize item ID based on table type
	normalizedID := normalizeItemID(table, itemID)

	// Find existing item
	var matchedItem map[string]interface{}
	var matchedSysID string

	for sysID, item := range MockDatabase[table] {
		itemMap, ok := item.(map[string]interface{})
		if !ok {
			continue
		}

		// Check for matching number or sys_id
		if number, hasNumber := itemMap["number"].(string); hasNumber {
			if strings.EqualFold(number, normalizedID) ||
				strings.HasSuffix(number, strings.TrimPrefix(normalizedID, number[:3])) ||
				number == normalizedID {
				matchedItem = itemMap
				matchedSysID = sysID
				break
			}
		}

		// Check for matching sys_id if no number match
		if sysID == normalizedID {
			matchedItem = itemMap
			matchedSysID = sysID
			break
		}
	}

	// If no matching item found, create a new one
	if matchedItem == nil {
		log.Printf("[MOCK SERVICENOW] Creating new item in %s with ID %s", table, normalizedID)
		matchedSysID = fmt.Sprintf("%s_%d", table, time.Now().UnixNano())
		matchedItem = map[string]interface{}{
			"sys_id":     matchedSysID,
			"number":     normalizedID,
			"created_on": time.Now().Format(time.RFC3339),
		}
		MockDatabase[table][matchedSysID] = matchedItem
	}

	// Update the item with provided updates
	for k, v := range updates {
		matchedItem[k] = v
	}

	// Always update the updated_on timestamp
	matchedItem["updated_on"] = time.Now().Format(time.RFC3339)

	// Update the mock database
	MockDatabase[table][matchedSysID] = matchedItem

	log.Printf("[MOCK SERVICENOW] Successfully updated %s item %s", table, normalizedID)

	// Trigger notification to Slack
	go sendUpdateNotificationToSlack(table, matchedItem)

	return true
}
func normalizeItemID(table, itemID string) string {
	// Remove any prefix that might already exist
	cleanedID := strings.TrimPrefix(strings.TrimPrefix(strings.TrimPrefix(strings.TrimPrefix(
		strings.TrimPrefix(itemID, "INC"), "RISK"), "AUDIT-"), "TEST"), "VR")

	// Add appropriate prefix based on table
	switch table {
	case "incidents":
		return "INC" + cleanedID
	case "risks":
		return "RISK" + cleanedID
	case "audit_findings":
		return "AUDIT-" + cleanedID
	case "control_tests":
		return "TEST" + cleanedID
	case "vendor_risks":
		return "VR" + cleanedID
	case "regulatory_changes":
		return "REG" + cleanedID
	default:
		return cleanedID
	}
}

// Enhanced sendUpdateNotificationToSlack function with better message formatting
func sendUpdateNotificationToSlack(table string, itemData map[string]interface{}) {
	// Extract item details
	var itemNumber, title, emoji string

	if num, ok := itemData["number"].(string); ok && num != "" {
		itemNumber = num
	} else if id, ok := itemData["sys_id"].(string); ok {
		itemNumber = id
	}

	if t, ok := itemData["title"].(string); ok && t != "" {
		title = t
	} else if sd, ok := itemData["short_description"].(string); ok && sd != "" {
		title = sd
	} else {
		title = "Untitled"
	}

	// Choose emoji based on table type
	switch table {
	case "risks":
		emoji = "⚠️"
	case "incidents":
		emoji = "🚨"
	case "compliance_tasks":
		emoji = "📋"
	case "control_tests":
		emoji = "🧪"
	case "audit_findings":
		emoji = "🔍"
	case "vendor_risks":
		emoji = "🏢"
	case "regulatory_changes":
		emoji = "📜"
	default:
		emoji = "🔄"
	}

	// Determine item type for display
	itemType := strings.TrimSuffix(table, "s") // Simple singular form
	itemType = strings.ReplaceAll(itemType, "_", " ")

	// Create message
	message := fmt.Sprintf("%s *%s %s Updated*\n*Item:* %s", emoji, strings.Title(itemType), itemNumber, title)

	// Add status if it exists
	if status, ok := itemData["status"].(string); ok && status != "" {
		message += fmt.Sprintf("\n*Status:* %s", status)
	} else if testStatus, ok := itemData["test_status"].(string); ok && testStatus != "" {
		message += fmt.Sprintf("\n*Test Status:* %s", testStatus)
	}

	// Determine appropriate channel based on table
	channelID := "C12345" // default to general
	switch table {
	case "risks":
		channelID = "C67890" // risk-management
	case "incidents":
		channelID = "C22222" // incident-response
	case "compliance_tasks":
		channelID = "C11111" // compliance-team
	case "audit_findings":
		channelID = "C54321" // audit
	case "control_tests":
		channelID = "C66666" // control-testing
	case "vendor_risks":
		channelID = "C33333" // vendor-risk
	case "regulatory_changes":
		channelID = "C44444" // regulatory-updates
	}

	// Create request with proper headers
	data := map[string]interface{}{
		"channel": channelID,
		"text":    message,
	}

	jsonData, err := json.Marshal(data)
	if err != nil {
		log.Printf("[SERVICENOW] ERROR: Failed to marshal Slack notification: %v", err)
		return
	}

	log.Printf("[SERVICENOW] Sending notification to Slack: %s", string(jsonData))

	// Create a proper HTTP request with correct content-type headers
	req, err := http.NewRequest("POST", "http://localhost:3002/api/chat.postMessage", bytes.NewBuffer(jsonData))
	if err != nil {
		log.Printf("[SERVICENOW] ERROR: Failed to create Slack request: %v", err)
		return
	}

	// Set content-type header
	req.Header.Set("Content-Type", "application/json")

	// Create client with timeout and send request
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("[SERVICENOW] ERROR: Failed to send notification to Slack: %v", err)
		return // Continue execution - don't fail the whole update
	}
	defer resp.Body.Close()

	// Log response
	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		log.Printf("[SERVICENOW] WARNING: Slack API error: %s", string(respBody))
	} else {
		log.Printf("[SERVICENOW] Successfully sent notification to Slack")
	}
}

func handleSlackInteractions(w http.ResponseWriter, r *http.Request) {
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
	json.Unmarshal([]byte(payloadStr), &payload)

	// Process the interaction
	response := processSlackInteraction(payload)

	// Return the response
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// Modify processSlackInteraction to use updateServiceNowItem
func processSlackInteraction(payload map[string]interface{}) map[string]interface{} {
	// Extract action details
	actions, hasActions := payload["actions"].([]interface{})
	if !hasActions || len(actions) == 0 {
		return map[string]interface{}{
			"text": "No actions found",
		}
	}

	action := actions[0].(map[string]interface{})
	actionID, _ := action["action_id"].(string)
	actionValue, _ := action["value"].(string)

	log.Printf("[MOCK SERVICENOW] Processing action: %s with value: %s", actionID, actionValue)

	// Extract item ID from action value
	parts := strings.Split(actionValue, "_")
	var itemID string
	if len(parts) >= 2 {
		itemID = parts[len(parts)-1]
	} else {
		return map[string]interface{}{
			"text": fmt.Sprintf("Invalid action value format: %s", actionValue),
		}
	}

	log.Printf("[MOCK SERVICENOW] Extracted item ID: %s", itemID)

	// Determine table and updates based on action
	switch {
	case strings.HasPrefix(actionID, "acknowledge_incident"):
		updateServiceNowItem("incidents", itemID, map[string]interface{}{
			"status":     "Acknowledged",
			"updated_on": time.Now().Format(time.RFC3339),
		})
		return map[string]interface{}{
			"text":          fmt.Sprintf("✅ Incident *%s* has been acknowledged", itemID),
			"response_type": "in_channel",
		}

	case strings.HasPrefix(actionID, "update_incident"):
		updateServiceNowItem("incidents", itemID, map[string]interface{}{
			"status":     "In Progress",
			"updated_on": time.Now().Format(time.RFC3339),
		})
		return map[string]interface{}{
			"text":          fmt.Sprintf("📝 Incident *%s* has been updated", itemID),
			"response_type": "in_channel",
		}

	case strings.HasPrefix(actionID, "resolve_incident"):
		updateServiceNowItem("incidents", itemID, map[string]interface{}{
			"status":     "Resolved",
			"updated_on": time.Now().Format(time.RFC3339),
		})
		return map[string]interface{}{
			"text":          fmt.Sprintf("✅ Incident *%s* has been resolved", itemID),
			"response_type": "in_channel",
		}

	case strings.HasPrefix(actionID, "discuss_risk"):
		updateServiceNowItem("risks", itemID, map[string]interface{}{
			"status":     "Under Discussion",
			"updated_on": time.Now().Format(time.RFC3339),
		})
		return map[string]interface{}{
			"text":          fmt.Sprintf("💬 Risk *%s* is now under discussion", itemID),
			"response_type": "in_channel",
		}

	case strings.HasPrefix(actionID, "assign_risk"):
		updateServiceNowItem("risks", itemID, map[string]interface{}{
			"status":      "Assigned",
			"assigned_to": "jane.smith",
			"updated_on":  time.Now().Format(time.RFC3339),
		})
		return map[string]interface{}{
			"text":          fmt.Sprintf("👤 Risk *%s* has been assigned to *jane.smith*", itemID),
			"response_type": "in_channel",
		}

	case strings.HasPrefix(actionID, "assign_finding"):
		updateServiceNowItem("audit_findings", itemID, map[string]interface{}{
			"status":      "Assigned",
			"assigned_to": "john.doe",
			"updated_on":  time.Now().Format(time.RFC3339),
		})
		return map[string]interface{}{
			"text":          fmt.Sprintf("👤 Audit finding *%s* has been assigned to *john.doe*", itemID),
			"response_type": "in_channel",
		}

	case strings.HasPrefix(actionID, "resolve_finding"):
		updateServiceNowItem("audit_findings", itemID, map[string]interface{}{
			"status":     "Resolved",
			"updated_on": time.Now().Format(time.RFC3339),
		})
		return map[string]interface{}{
			"text":          fmt.Sprintf("✅ Audit finding *%s* has been resolved", itemID),
			"response_type": "in_channel",
		}

	case strings.HasPrefix(actionID, "submit_test_results"):
		updateServiceNowItem("control_tests", itemID, map[string]interface{}{
			"test_status": "PASS",
			"status":      "Completed",
			"updated_on":  time.Now().Format(time.RFC3339),
		})
		return map[string]interface{}{
			"text":          fmt.Sprintf("🧪 Test results submitted for *%s*: PASS", itemID),
			"response_type": "in_channel",
		}

	case strings.HasPrefix(actionID, "request_compliance_report"):
		updateServiceNowItem("vendor_risks", itemID, map[string]interface{}{
			"status":     "Report Requested",
			"updated_on": time.Now().Format(time.RFC3339),
		})
		return map[string]interface{}{
			"text":          fmt.Sprintf("📊 Compliance report requested for vendor risk *%s*", itemID),
			"response_type": "in_channel",
		}

	case strings.HasPrefix(actionID, "update_vendor_status"):
		updateServiceNowItem("vendor_risks", itemID, map[string]interface{}{
			"status":     "In Review",
			"updated_on": time.Now().Format(time.RFC3339),
		})
		return map[string]interface{}{
			"text":          fmt.Sprintf("🏢 Vendor risk *%s* status updated to *In Review*", itemID),
			"response_type": "in_channel",
		}

	default:
		log.Printf("[MOCK SERVICENOW] Unhandled action ID: %s", actionID)
		return map[string]interface{}{
			"text": fmt.Sprintf("Unhandled action: %s", actionID),
		}
	}
}
func createInitialTestData() {
	log.Println("Initial test data creation disabled")
}
