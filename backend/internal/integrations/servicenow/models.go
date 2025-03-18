// backend/internal/integrations/servicenow/models.go
package servicenow

import "time"

// Risk represents a risk record in ServiceNow GRC
type Risk struct {
	ID             string    `json:"sys_id"`
	Number         string    `json:"number"`
	ShortDesc      string    `json:"short_description"`
	Description    string    `json:"description"`
	Category       string    `json:"category"`
	Subcategory    string    `json:"subcategory"`
	State          string    `json:"state"`
	Impact         string    `json:"impact"`
	Likelihood     string    `json:"likelihood"`
	RiskScore      float64   `json:"risk_score"`
	AssignedTo     string    `json:"assigned_to"`
	CreatedOn      time.Time `json:"sys_created_on"`
	LastUpdated    time.Time `json:"sys_updated_on"`
	DueDate        time.Time `json:"due_date"`
	MitigationPlan string    `json:"mitigation_plan"`
}

// ComplianceTask represents a compliance task in ServiceNow GRC
type ComplianceTask struct {
	ID          string    `json:"sys_id"`
	Number      string    `json:"number"`
	ShortDesc   string    `json:"short_description"`
	Description string    `json:"description"`
	Framework   string    `json:"compliance_framework"`
	Regulation  string    `json:"regulation"`
	State       string    `json:"state"`
	AssignedTo  string    `json:"assigned_to"`
	CreatedOn   time.Time `json:"sys_created_on"`
	LastUpdated time.Time `json:"sys_updated_on"`
	DueDate     time.Time `json:"due_date"`
	Evidence    []string  `json:"evidence_list,omitempty"`
}

// Incident represents a security incident in ServiceNow GRC
type Incident struct {
	ID              string    `json:"sys_id"`
	Number          string    `json:"number"`
	ShortDesc       string    `json:"short_description"`
	Description     string    `json:"description"`
	Category        string    `json:"category"`
	Subcategory     string    `json:"subcategory"`
	State           string    `json:"state"`
	Priority        string    `json:"priority"`
	Severity        string    `json:"severity"`
	Impact          string    `json:"impact"`
	AssignedTo      string    `json:"assigned_to"`
	AssignmentGrp   string    `json:"assignment_group"`
	CreatedOn       time.Time `json:"sys_created_on"`
	LastUpdated     time.Time `json:"sys_updated_on"`
	ResolutionNotes string    `json:"resolution_notes"`
}

// WebhookPayload represents the incoming webhook payload from ServiceNow
type WebhookPayload struct {
	ID         string                 `json:"sys_id"`
	TableName  string                 `json:"table_name"`
	ActionType string                 `json:"action_type"` // inserted, updated, deleted
	Data       map[string]interface{} `json:"data"`
}

// RiskSeverity maps risk scores to severity levels
func RiskSeverity(score float64) string {
	switch {
	case score >= 15:
		return "Critical"
	case score >= 10:
		return "High"
	case score >= 5:
		return "Medium"
	default:
		return "Low"
	}
}
