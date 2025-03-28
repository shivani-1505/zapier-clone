#!/bin/bash
# Script to trigger mock ServiceNow webhooks

# Base URL for the mock ServiceNow server
MOCK_URL="http://localhost:3000"
# Your GRC integration server URL
GRC_URL="http://localhost:8080"

# Trigger a Risk webhook
echo "Triggering a risk webhook (inserted)..."
curl -X POST "$MOCK_URL/trigger_webhook/risks/inserted?webhook_url=$GRC_URL/api/webhooks/servicenow" \
  -H "Content-Type: application/json" \
  -d '{
    "sys_id": "risk002",
    "number": "RISK0002",
    "short_description": "New unpatched vulnerabilities",
    "description": "Critical vulnerabilities found in web application servers",
    "category": "Security",
    "subcategory": "Vulnerability Management",
    "state": "new",
    "impact": "High",
    "likelihood": "High",
    "risk_score": 15.0,
    "assigned_to": "",
    "sys_created_on": "2023-05-16T09:00:00Z",
    "sys_updated_on": "2023-05-16T09:00:00Z",
    "due_date": "2023-06-01T23:59:59Z",
    "mitigation_plan": ""
  }'

# Trigger an Incident webhook
echo "Triggering an incident webhook (inserted)..."
curl -X POST "$MOCK_URL/trigger_webhook/incidents/inserted?webhook_url=$GRC_URL/api/webhooks/servicenow" \
  -H "Content-Type: application/json" \
  -d '{
    "sys_id": "inc002",
    "number": "INC0002",
    "short_description": "Unauthorized access attempt",
    "description": "Multiple failed login attempts detected from unusual IP ranges",
    "category": "Security",
    "subcategory": "Access Control",
    "state": "new",
    "priority": "High",
    "severity": "Critical",
    "impact": "Medium",
    "assigned_to": "",
    "assignment_group": "Security Team",
    "sys_created_on": "2023-05-16T10:30:00Z",
    "sys_updated_on": "2023-05-16T10:30:00Z",
    "resolution_notes": ""
  }'

# Trigger a Compliance Task webhook
echo "Triggering a compliance task webhook (inserted)..."
curl -X POST "$MOCK_URL/trigger_webhook/compliance_tasks/inserted?webhook_url=$GRC_URL/api/webhooks/servicenow" \
  -H "Content-Type: application/json" \
  -d '{
    "sys_id": "task002",
    "number": "COMP0002",
    "short_description": "PCI-DSS quarterly review",
    "description": "Review and validate PCI-DSS controls for Q2",
    "compliance_framework": "PCI-DSS",
    "regulation": "Requirement 10",
    "state": "open",
    "assigned_to": "",
    "sys_created_on": "2023-05-16T11:00:00Z",
    "sys_updated_on": "2023-05-16T11:00:00Z",
    "due_date": "2023-06-30T23:59:59Z"
  }'

# Trigger an Audit Finding webhook
echo "Triggering an audit finding webhook (inserted)..."
curl -X POST "$MOCK_URL/trigger_webhook/audit_findings/inserted?webhook_url=$GRC_URL/api/webhooks/servicenow" \
  -H "Content-Type: application/json" \
  -d '{
    "sys_id": "audit002",
    "number": "AUDIT0002",
    "short_description": "Excessive privileged access",
    "description": "Too many users have admin privileges on core systems",
    "audit_name": "Privileged Access Audit",
    "severity": "High",
    "state": "open",
    "assigned_to": "",
    "sys_created_on": "2023-05-16T13:45:00Z",
    "sys_updated_on": "2023-05-16T13:45:00Z",
    "due_date": "2023-06-15T23:59:59Z",
    "resolution": ""
  }'

# Trigger a Control Test webhook
echo "Triggering a control test webhook (inserted)..."
curl -X POST "$MOCK_URL/trigger_webhook/control_tests/inserted?webhook_url=$GRC_URL/api/webhooks/servicenow" \
  -H "Content-Type: application/json" \
  -d '{
    "sys_id": "ctrl002",
    "number": "CTRL0002",
    "short_description": "Data loss prevention control test",
    "description": "Test DLP controls to prevent unauthorized data exfiltration",
    "control_name": "Data Loss Prevention",
    "framework": "ISO 27001",
    "state": "open",
    "assigned_to": "",
    "sys_created_on": "2023-05-16T14:30:00Z",
    "sys_updated_on": "2023-05-16T14:30:00Z",
    "due_date": "2023-06-20T23:59:59Z",
    "results": "",
    "notes": "",
    "test_status": "In Progress"
  }'

# Trigger a Vendor Risk webhook