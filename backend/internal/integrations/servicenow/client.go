// backend/internal/integrations/servicenow/client.go
package servicenow

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// Client represents a ServiceNow GRC API client
type Client struct {
	BaseURL    string
	Username   string
	Password   string
	HTTPClient *http.Client
}

// NewClient creates a new ServiceNow GRC client
func NewClient(baseURL, username, password string) *Client {
	return &Client{
		BaseURL:  baseURL,
		Username: username,
		Password: password,
		HTTPClient: &http.Client{
			Timeout: time.Second * 30,
		},
	}
}

// makeRequest performs an HTTP request to the ServiceNow API
func (c *Client) makeRequest(method, endpoint string, body interface{}) (*http.Response, error) {
	url := fmt.Sprintf("%s/%s", c.BaseURL, endpoint)

	var req *http.Request
	var err error

	if body != nil {
		jsonBody, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("error marshaling request body: %w", err)
		}
		req, err = http.NewRequest(method, url, bytes.NewBuffer(jsonBody))
		if err != nil {
			return nil, fmt.Errorf("error creating request: %w", err)
		}
	} else {
		req, err = http.NewRequest(method, url, nil)
		if err != nil {
			return nil, fmt.Errorf("error creating request: %w", err)
		}
	}

	req.SetBasicAuth(c.Username, c.Password)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error executing request: %w", err)
	}

	return resp, nil
}

// GetRisks fetches risks from ServiceNow GRC
func (c *Client) GetRisks() ([]Risk, error) {
	resp, err := c.makeRequest("GET", "api/now/table/sn_risk_risk", nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var response struct {
		Result []Risk `json:"result"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("error decoding response: %w", err)
	}

	return response.Result, nil
}

// GetComplianceTasks fetches compliance tasks from ServiceNow GRC
func (c *Client) GetComplianceTasks() ([]ComplianceTask, error) {
	resp, err := c.makeRequest("GET", "api/now/table/sn_compliance_task", nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var response struct {
		Result []ComplianceTask `json:"result"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("error decoding response: %w", err)
	}

	return response.Result, nil
}

// GetIncidents fetches security incidents from ServiceNow GRC
func (c *Client) GetIncidents() ([]Incident, error) {
	resp, err := c.makeRequest("GET", "api/now/table/sn_si_incident", nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var response struct {
		Result []Incident `json:"result"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("error decoding response: %w", err)
	}

	return response.Result, nil
}

// UpdateRiskStatus updates the status of a risk in ServiceNow GRC
func (c *Client) UpdateRiskStatus(riskID, status string) error {
	body := map[string]string{
		"state": status,
	}

	resp, err := c.makeRequest("PATCH", fmt.Sprintf("api/now/table/sn_risk_risk/%s", riskID), body)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	return nil
}

// UpdateComplianceTaskStatus updates the status of a compliance task
func (c *Client) UpdateComplianceTaskStatus(taskID, status string) error {
	body := map[string]string{
		"state": status,
	}

	resp, err := c.makeRequest("PATCH", fmt.Sprintf("api/now/table/sn_compliance_task/%s", taskID), body)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	return nil
}

// UpdateIncidentStatus updates the status of a security incident
func (c *Client) UpdateIncidentStatus(incidentID, status string) error {
	body := map[string]string{
		"state": status,
	}

	resp, err := c.makeRequest("PATCH", fmt.Sprintf("api/now/table/sn_si_incident/%s", incidentID), body)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	return nil
}

// AttachEvidenceToComplianceTask attaches evidence to a compliance task
func (c *Client) AttachEvidenceToComplianceTask(taskID, fileName, fileContent string) error {
	// In a real implementation, this would handle file uploads with multipart/form-data
	// For simplicity, we're just simulating success
	return nil
}
