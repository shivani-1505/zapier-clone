// backend/internal/integrations/jira/risk-jira-mapping.go
package jira

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// RiskJiraMapping stores the mapping between ServiceNow risks and Jira issues
type RiskJiraMapping struct {
	RiskIDToJiraKey map[string]string `json:"riskIdToJiraKey"`
	JiraKeyToRiskID map[string]string `json:"jiraKeyToRiskID"`
	mutex           sync.RWMutex
	filePath        string
}

// NewRiskJiraMapping creates a new mapping store
func NewRiskJiraMapping(storagePath string) (*RiskJiraMapping, error) {
	filePath := filepath.Join(storagePath, "risk_jira_mapping.json")

	mapping := &RiskJiraMapping{
		RiskIDToJiraKey: make(map[string]string),
		JiraKeyToRiskID: make(map[string]string),
		filePath:        filePath,
	}

	// Try to load existing mapping
	if _, err := os.Stat(filePath); err == nil {
		file, err := os.ReadFile(filePath)
		if err != nil {
			return nil, fmt.Errorf("error reading mapping file: %w", err)
		}

		if err := json.Unmarshal(file, mapping); err != nil {
			return nil, fmt.Errorf("error unmarshaling mapping: %w", err)
		}
	}

	return mapping, nil
}

// AddMapping adds a mapping between a risk ID and a Jira issue key
func (m *RiskJiraMapping) AddMapping(riskID, jiraKey string) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	m.RiskIDToJiraKey[riskID] = jiraKey
	m.JiraKeyToRiskID[jiraKey] = riskID

	return m.save()
}

// GetJiraKeyFromRiskID retrieves the Jira issue key for a risk ID
func (m *RiskJiraMapping) GetJiraKeyFromRiskID(riskID string) (string, bool) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	jiraKey, exists := m.RiskIDToJiraKey[riskID]
	return jiraKey, exists
}

// GetRiskIDFromJiraKey retrieves the risk ID for a Jira issue key
func (m *RiskJiraMapping) GetRiskIDFromJiraKey(jiraKey string) (string, bool) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	riskID, exists := m.JiraKeyToRiskID[jiraKey]
	return riskID, exists
}

// save persists the mapping to disk
func (m *RiskJiraMapping) save() error {
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return fmt.Errorf("error marshaling mapping: %w", err)
	}

	// Ensure directory exists
	dir := filepath.Dir(m.filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("error creating directory: %w", err)
	}

	if err := os.WriteFile(m.filePath, data, 0644); err != nil {
		return fmt.Errorf("error writing mapping file: %w", err)
	}

	return nil
}
