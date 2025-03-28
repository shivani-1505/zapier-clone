// mock_servicenow.go
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
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

	// Generate a risk ID and number
	sysID := fmt.Sprintf("risk_%d", time.Now().UnixNano())
	riskNumber := fmt.Sprintf("RISK%d", len(MockDatabase["risks"])+1001)

	// Add system fields
	riskData["sys_id"] = sysID
	riskData["number"] = riskNumber
	riskData["created_on"] = time.Now().Format(time.RFC3339)
	riskData["updated_on"] = time.Now().Format(time.RFC3339)

	// Make sure risks map exists
	if MockDatabase["risks"] == nil {
		MockDatabase["risks"] = make(map[string]interface{})
	}

	// Store the risk in our mock database
	MockDatabase["risks"][sysID] = riskData

	// Send a webhook to sync this with Jira
	go func() {
		// Build webhook payload
		webhookPayload := map[string]interface{}{
			"sys_id":      sysID,
			"table_name":  "sn_risk_risk",
			"action_type": "insert",
			"data":        riskData,
		}

		// Send the webhook to your backend server
		jsonPayload, _ := json.Marshal(webhookPayload)
		http.Post("http://localhost:8080/api/webhooks/servicenow",
			"application/json",
			bytes.NewBuffer(jsonPayload))
	}()

	// Return the created risk
	json.NewEncoder(w).Encode(ResponseResult{Result: riskData})
}

func main() {
	r := mux.NewRouter()

	// Add routes for different ServiceNow tables
	r.HandleFunc("/api/now/table/sn_risk_risk", handleRisks).Methods("GET", "POST", "PATCH")
	r.HandleFunc("/api/now/table/sn_risk_risk/{id}", handleRiskByID).Methods("GET", "PATCH", "DELETE")

	r.HandleFunc("/api/now/table/sn_compliance_task", handleComplianceTasks).Methods("GET", "POST", "PATCH")
	r.HandleFunc("/api/now/table/sn_compliance_task/{id}", handleComplianceTaskByID).Methods("GET", "PATCH", "DELETE")

	r.HandleFunc("/api/now/table/sn_si_incident", handleIncidents).Methods("GET", "POST", "PATCH")
	r.HandleFunc("/api/now/table/sn_si_incident/{id}", handleIncidentByID).Methods("GET", "PATCH", "DELETE")

	r.HandleFunc("/servicenow/create_risk", handleCreateRisk).Methods("POST")

	r.HandleFunc("/api/now/table/sn_policy_control_test", handleControlTests).Methods("GET", "POST", "PATCH")
	r.HandleFunc("/api/now/table/sn_policy_control_test/{id}", handleControlTestByID).Methods("GET", "PATCH", "DELETE")

	r.HandleFunc("/api/now/table/sn_audit_finding", handleAuditFindings).Methods("GET", "POST", "PATCH")
	r.HandleFunc("/api/now/table/sn_audit_finding/{id}", handleAuditFindingByID).Methods("GET", "PATCH", "DELETE")

	r.HandleFunc("/api/now/table/sn_vendor_risk", handleVendorRisks).Methods("GET", "POST", "PATCH")
	r.HandleFunc("/api/now/table/sn_vendor_risk/{id}", handleVendorRiskByID).Methods("GET", "PATCH", "DELETE")

	r.HandleFunc("/api/now/table/sn_regulatory_change", handleRegulatoryChanges).Methods("GET", "POST", "PATCH")
	r.HandleFunc("/api/now/table/sn_regulatory_change/{id}", handleRegulatoryChangeByID).Methods("GET", "PATCH", "DELETE")

	// Special endpoints for GRC dashboard data
	r.HandleFunc("/api/now/table/sn_grc_summary", handleGRCSummary).Methods("GET")
	r.HandleFunc("/api/now/table/sn_risk_by_category", handleRisksByCategory).Methods("GET")
	// Add this to your router setup
	r.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	}).Methods("GET")

	// Test webhook trigger endpoint (special mock endpoint to simulate ServiceNow sending webhooks)
	r.HandleFunc("/trigger_webhook/{table_name}/{action_type}", triggerWebhook).Methods("POST")

	// Add a simple UI handler for the root path
	r.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		html := `
    	<!DOCTYPE html>
	    <html>
    	<head>
	        <title>Mock ServiceNow GRC</title>
    	    <style>
        	    body { font-family: Arial, sans-serif; margin: 0; padding: 20px; }
	            .header { background: #0057a6; color: white; padding: 15px; }
    	        .container { max-width: 1200px; margin: 0 auto; }
        	    .card { border: 1px solid #ddd; border-radius: 4px; padding: 15px; margin-bottom: 15px; background: white; }
            	h2 { color: #0057a6; }
	            table { width: 100%; border-collapse: collapse; }
    	        table, th, td { border: 1px solid #ddd; }
        	    th, td { padding: 10px; text-align: left; }
            	th { background-color: #f2f2f2; }
	            .risk-high { background-color: #ffebee; }
    	        .risk-medium { background-color: #fff8e1; }
        	    .risk-low { background-color: #e8f5e9; }
            	.button { display: inline-block; padding: 8px 16px; background: #0057a6; color: white; text-decoration: none; border-radius: 4px; }
	        </style>
    	</head>
	    <body>
    	    <div class="header">
        	    <div class="container">
            	    <h1>ServiceNow GRC - Risk Management</h1>
	            </div>
    	    </div>
        
        	<div class="container">
            	<div class="card">
	                <h2>Risk Dashboard</h2>
    	            <div id="grc-summary"></div>
        	    </div>
            
            	<div class="card">
	                <h2>Risk Register</h2>
    	            <table id="risk-table">
        	            <thead>
            	            <tr>
                	            <th>ID</th>
                    	        <th>Title</th>
                        	    <th>Severity</th>
                            	<th>Category</th>
	                            <th>Owner</th>
    	                        <th>Status</th>
        	                    <th>Created</th>
            	            </tr>
                	    </thead>
                    	<tbody id="risk-data"></tbody>
	                </table>
    	        </div>
            
        	    <div class="card">
            	    <h2>Add New Risk</h2>
                	<form id="risk-form">
	                    <div style="display: flex; flex-wrap: wrap; gap: 10px;">
    	                    <div style="flex: 1;">
        	                    <label>Title:</label><br>
            	                <input type="text" name="title" style="width: 100%; padding: 8px;" required>
                	        </div>
                    	    <div style="flex: 1;">
                        	    <label>Severity:</label><br>
                            	<select name="severity" style="width: 100%; padding: 8px;" required>
	                                <option value="High">High</option>
    	                            <option value="Medium">Medium</option>
        	                        <option value="Low">Low</option>
            	                </select>
                	        </div>
                    	</div>
	                    <div style="margin-top: 10px;">
    	                    <label>Description:</label><br>
        	                <textarea name="description" style="width: 100%; padding: 8px;" rows="3" required></textarea>
            	        </div>
                	    <div style="display: flex; flex-wrap: wrap; gap: 10px; margin-top: 10px;">
                    	    <div style="flex: 1;">
                        	    <label>Category:</label><br>
                            	<select name="category" style="width: 100%; padding: 8px;" required>
                                	<option value="Cybersecurity">Cybersecurity</option>
	                                <option value="Financial">Financial</option>
    	                            <option value="Operational">Operational</option>
        	                        <option value="Compliance">Compliance</option>
            	                    <option value="Strategic">Strategic</option>
                	            </select>
                    	    </div>
                        	<div style="flex: 1;">
                            	<label>Owner:</label><br>
	                            <input type="text" name="owner" style="width: 100%; padding: 8px;" required>
    	                    </div>
        	            </div>
            	        <div style="margin-top: 15px;">
                	        <button type="submit" class="button">Create Risk</button>
                    	</div>
	                </form>
    	        </div>
        	</div>

	        <script>
    	        // Load risk data
        	    function loadRisks() {
            	    fetch('/api/now/table/sn_risk_risk')
                	    .then(response => response.json())
                    	.then(data => {
                        	const riskData = document.getElementById('risk-data');
	                        riskData.innerHTML = '';
		
	                        if (data.result && data.result.length) {
    	                        data.result.forEach(risk => {
        	                        const row = document.createElement('tr');
		
        	                        // Add severity class
            	                    if (risk.severity === 'High') {
                	                    row.className = 'risk-high';
                    	            } else if (risk.severity === 'Medium') {
                        	            row.className = 'risk-medium';
                            	    } else {
                                	    row.className = 'risk-low';
	                                }
                                
    	                            row.innerHTML = 
										<td>\${risk.number || risk.sys_id}</td>            	                        <td>\${risk.title || 'Untitled'}</td>
                	                    <td>${risk.severity || 'Unknown'}</td>
                    	                <td>${risk.category || 'Uncategorized'}</td>
                        	            <td>${risk.owner || 'Unassigned'}</td>
                            	        <td>${risk.status || 'New'}</td>
                                	    <td>${new Date(risk.created_on).toLocaleDateString() || 'Unknown'}</td>
	                                ;
	                                riskData.appendChild(row);
	                            });
    	                    } else {
        	                    riskData.innerHTML = '<tr><td colspan="7" style="text-align: center;">No risks found</td></tr>';
            	            }
                	    });
	            }
            
    	        // Load GRC summary
        	    function loadSummary() {
            	    fetch('/api/now/table/sn_grc_summary')
	                    .then(response => response.json())
    	                .then(data => {
        	                const summary = data.result;
            	            const summaryDiv = document.getElementById('grc-summary');
                        
                	        summaryDiv.innerHTML = 
                    	        <div style="display: flex; flex-wrap: wrap; gap: 15px;">
                        	        <div style="flex: 1; text-align: center; padding: 15px; background: #e3f2fd; border-radius: 4px;">
                            	        <div style="font-size: 32px; font-weight: bold;">${summary.open_risks || 0}</div>
                                	    <div>Open Risks</div>
	                                </div>
    	                            <div style="flex: 1; text-align: center; padding: 15px; background: #e8f5e9; border-radius: 4px;">
        	                            <div style="font-size: 32px; font-weight: bold;">${summary.compliance_score || 0}%</div>
            	                        <div>Compliance Score</div>
                	                </div>
                    	            <div style="flex: 1; text-align: center; padding: 15px; background: #fff8e1; border-radius: 4px;">
                        	            <div style="font-size: 32px; font-weight: bold;">${summary.open_compliance_tasks || 0}</div>
                            	        <div>Open Tasks</div>
                                	</div>
	                                <div style="flex: 1; text-align: center; padding: 15px; background: #fce4ec; border-radius: 4px;">
    	                                <div style="font-size: 32px; font-weight: bold;">${summary.overdue_items || 0}</div>
        	                            <div>Overdue Items</div>
            	                    </div>
                	            </div>
                    	    ;
	                    });
    	        }
            
        	    // Submit new risk form
            	document.getElementById('risk-form').addEventListener('submit', function(e) {
	                e.preventDefault();
                
	                const formData = new FormData(this);
    	            const risk = {
        	            title: formData.get('title'),
            	        description: formData.get('description'),
                	    severity: formData.get('severity'),
                    	category: formData.get('category'),
	                    owner: formData.get('owner'),
    	                status: 'Open'
        	        };
                
            	    fetch('/servicenow/create_risk', {
                	    method: 'POST',
                    	headers: {
	                        'Content-Type': 'application/json',
    	                },
        	            body: JSON.stringify(risk),
            	    })
                	.then(response => response.json())
	                .then(data => {
    	                alert('Risk created successfully!');
        	            this.reset();
            	        loadRisks();
                	    loadSummary();
	                })
    	            .catch(error => {
        	            console.error('Error creating risk:', error);
            	        alert('Failed to create risk. See console for details.');
                	});
	            });
            
	            // Initial data load
    	        loadRisks();
        	    loadSummary();
	        </script>
    	</body>
	    </html>
	    `
		w.Write([]byte(html))
	})

	// Start server
	port := "3000"
	fmt.Printf("Starting mock ServiceNow server on port %s...\n", port)
	log.Fatal(http.ListenAndServe(":"+port, r))
}

// Generic table handlers

func handleRisks(w http.ResponseWriter, r *http.Request) {
	handleGenericTable(w, r, "risks")
}

func handleRiskByID(w http.ResponseWriter, r *http.Request) {
	handleGenericItemByID(w, r, "risks")
}

func handleComplianceTasks(w http.ResponseWriter, r *http.Request) {
	handleGenericTable(w, r, "compliance_tasks")
}

func handleComplianceTaskByID(w http.ResponseWriter, r *http.Request) {
	handleGenericItemByID(w, r, "compliance_tasks")
}

func handleIncidents(w http.ResponseWriter, r *http.Request) {
	handleGenericTable(w, r, "incidents")
}

func handleIncidentByID(w http.ResponseWriter, r *http.Request) {
	handleGenericItemByID(w, r, "incidents")
}

func handleControlTests(w http.ResponseWriter, r *http.Request) {
	handleGenericTable(w, r, "control_tests")
}

func handleControlTestByID(w http.ResponseWriter, r *http.Request) {
	handleGenericItemByID(w, r, "control_tests")
}

func handleAuditFindings(w http.ResponseWriter, r *http.Request) {
	handleGenericTable(w, r, "audit_findings")
}

func handleAuditFindingByID(w http.ResponseWriter, r *http.Request) {
	handleGenericItemByID(w, r, "audit_findings")
}

func handleVendorRisks(w http.ResponseWriter, r *http.Request) {
	handleGenericTable(w, r, "vendor_risks")
}

func handleVendorRiskByID(w http.ResponseWriter, r *http.Request) {
	handleGenericItemByID(w, r, "vendor_risks")
}

func handleRegulatoryChanges(w http.ResponseWriter, r *http.Request) {
	handleGenericTable(w, r, "regulatory_changes")
}

func handleRegulatoryChangeByID(w http.ResponseWriter, r *http.Request) {
	handleGenericItemByID(w, r, "regulatory_changes")
}

// Generic handler implementations

func handleGenericTable(w http.ResponseWriter, r *http.Request, tableName string) {
	w.Header().Set("Content-Type", "application/json")

	switch r.Method {
	case "GET":
		// Convert map values to a slice
		var results []interface{}
		for _, v := range MockDatabase[tableName] {
			results = append(results, v)
		}
		json.NewEncoder(w).Encode(ResponseResult{Result: results})

	case "POST":
		var itemData map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&itemData); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		// Generate a random ID if not provided
		if _, ok := itemData["sys_id"]; !ok {
			itemData["sys_id"] = fmt.Sprintf("mock%d", time.Now().UnixNano())
		}

		id := itemData["sys_id"].(string)
		MockDatabase[tableName][id] = itemData

		json.NewEncoder(w).Encode(ResponseResult{Result: itemData})

	case "PATCH":
		http.Error(w, "PATCH not allowed on collection", http.StatusMethodNotAllowed)
		return
	}
}

func handleGenericItemByID(w http.ResponseWriter, r *http.Request, tableName string) {
	w.Header().Set("Content-Type", "application/json")
	vars := mux.Vars(r)
	id := vars["id"]

	// Check if item exists
	item, exists := MockDatabase[tableName][id]
	if !exists {
		http.Error(w, "Item not found", http.StatusNotFound)
		return
	}

	switch r.Method {
	case "GET":
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
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		for k, v := range updateData {
			itemMap[k] = v
		}

		// Update last modified time
		itemMap["sys_updated_on"] = time.Now().Format(time.RFC3339)

		MockDatabase[tableName][id] = itemMap
		json.NewEncoder(w).Encode(ResponseResult{Result: itemMap})

	case "DELETE":
		delete(MockDatabase[tableName], id)
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
		OverdueItems:           3,  // Mock value
		ComplianceScore:        85, // Mock value
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

// Webhook trigger (special mock function)

func triggerWebhook(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	tableName := vars["table_name"]
	actionType := vars["action_type"]

	// Parse the data from the request
	var data map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Map table names to their ServiceNow equivalents
	tableNameMap := map[string]string{
		"risks":              "sn_risk_risk",
		"compliance_tasks":   "sn_compliance_task",
		"incidents":          "sn_si_incident",
		"control_tests":      "sn_policy_control_test",
		"audit_findings":     "sn_audit_finding",
		"vendor_risks":       "sn_vendor_risk",
		"regulatory_changes": "sn_regulatory_change",
	}

	sn_table := tableNameMap[tableName]
	if sn_table == "" {
		http.Error(w, "Invalid table name", http.StatusBadRequest)
		return
	}

	// Build webhook payload
	webhookPayload := map[string]interface{}{
		"sys_id":      data["sys_id"],
		"table_name":  sn_table,
		"action_type": actionType,
		"data":        data,
	}

	// Get the webhook URL from the query parameter or use a default
	webhookURL := r.URL.Query().Get("webhook_url")
	if webhookURL == "" {
		webhookURL = "http://localhost:8080/api/webhooks/servicenow"
	}

	// Send the webhook
	jsonPayload, _ := json.Marshal(webhookPayload)
	resp, err := http.Post(webhookURL, "application/json", strings.NewReader(string(jsonPayload)))
	if err != nil {
		http.Error(w, fmt.Sprintf("Error sending webhook: %v", err), http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	// Return the status
	if resp.StatusCode == http.StatusOK {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{
			"status":      "success",
			"message":     fmt.Sprintf("Webhook sent to %s", webhookURL),
			"webhook_id":  fmt.Sprintf("mock-webhook-%d", time.Now().UnixNano()),
			"table_name":  sn_table,
			"action_type": actionType,
		})
	} else {
		http.Error(w, fmt.Sprintf("Webhook target returned status: %d", resp.StatusCode), http.StatusInternalServerError)
	}
}
