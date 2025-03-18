#!/bin/bash
# Script to populate mock ServiceNow data

# Base URL for the mock ServiceNow server
MOCK_URL="http://localhost:3000"

# Create a mock risk
echo "Creating mock risk..."
curl -X POST "$MOCK_URL/api/now/table/sn_risk_risk" \
  -H "Content-Type: application/json" \
  -d '{
    "sys_id": "risk001",
    "number": "RISK0001",
    "short_description": "Security breach in customer database",
    "description": "Potential exposure of sensitive customer information due to weak encryption",
    "category": "Security",
    "subcategory": "Data Protection",
    "state": "new",
    "impact": "High",
    "likelihood": "Medium",
    "risk_score": 12.5,
    "assigned_to": "",
    "sys_created_on": "2023-05-15T10:00:00Z",
    "sys_updated_on": "2023-05-15T10:00:00Z",
    "due_date": "2023-06-15T23:59:59Z",
    "mitigation_plan": ""
  }'

# Create a mock compliance task
echo "Creating mock compliance task..."
curl -X POST "$MOCK_URL/api/now/table/sn_compliance_task" \
  -H "Content-Type: application/json" \
  -d '{
    "sys_id": "task001",
    "number": "COMP0001",
    "short_description": "Quarterly SOX control assessment",
    "description": "Perform testing of key financial controls as required by SOX compliance",
    "compliance_framework": "SOX",
    "regulation": "Section 404",
    "state": "open",
    "assigned_to": "",
    "sys_created_on": "2023-05-15T10:00:00Z",
    "sys_updated_on": "2023-05-15T10:00:00Z",
    "due_date": "2023-06-15T23:59:59Z"
  }'

# Create a mock incident
echo "Creating mock incident..."
curl -X POST "$MOCK_URL/api/now/table/sn_si_incident" \
  -H "Content-Type: application/json" \
  -d '{
    "sys_id": "inc001",
    "number": "INC0001",
    "short_description": "DDoS attack on public website",
    "description": "Distributed denial of service attack detected on company website",
    "category": "Security",
    "subcategory": "Network",
    "state": "new",
    "priority": "High",
    "severity": "Critical",
    "impact": "High",
    "assigned_to": "",
    "assignment_group": "Security Team",
    "sys_created_on": "2023-05-15T10:00:00Z",
    "sys_updated_on": "2023-05-15T10:00:00Z",
    "resolution_notes": ""
  }'

# Create a mock control test
echo "Creating mock control test..."
curl -X POST "$MOCK_URL/api/now/table/sn_policy_control_test" \
  -H "Content-Type: application/json" \
  -d '{
    "sys_id": "ctrl001",
    "number": "CTRL0001",
    "short_description": "Firewall rule validation",
    "description": "Test firewall configurations against security policy requirements",
    "control_name": "Network Perimeter Security",
    "framework": "NIST 800-53",
    "state": "open",
    "assigned_to": "",
    "sys_created_on": "2023-05-15T10:00:00Z",
    "sys_updated_on": "2023-05-15T10:00:00Z",
    "due_date": "2023-06-15T23:59:59Z",
    "results": "",
    "notes": "",
    "test_status": "In Progress"
  }'

# Create a mock audit finding
echo "Creating mock audit finding..."
curl -X POST "$MOCK_URL/api/now/table/sn_audit_finding" \
  -H "Content-Type: application/json" \
  -d '{
    "sys_id": "audit001",
    "number": "AUDIT0001",
    "short_description": "Incomplete access review documentation",
    "description": "Access review was performed but not properly documented in accordance with policy",
    "audit_name": "Q1 IT General Controls Audit",
    "severity": "Medium",
    "state": "open",
    "assigned_to": "",
    "sys_created_on": "2023-05-15T10:00:00Z",
    "sys_updated_on": "2023-05-15T10:00:00Z",
    "due_date": "2023-06-15T23:59:59Z",
    "resolution": ""
  }'

# Create a mock vendor risk
echo "Creating mock vendor risk..."
curl -X POST "$MOCK_URL/api/now/table/sn_vendor_risk" \
  -H "Content-Type: application/json" \
  -d '{
    "sys_id": "vendor001",
    "number": "VENDOR0001",
    "short_description": "Critical SaaS provider missing security certification",
    "description": "Our key SaaS provider is missing current SOC 2 Type 2 certification",
    "vendor_name": "Cloud Services Inc.",
    "category": "Compliance",
    "severity": "High",
    "state": "open",
    "assigned_to": "",
    "sys_created_on": "2023-05-15T10:00:00Z",
    "sys_updated_on": "2023-05-15T10:00:00Z",
    "due_date": "2023-06-15T23:59:59Z",
    "compliance_status": "At Risk",
    "mitigation_plan": ""
  }'

# Create a mock regulatory change
echo "Creating mock regulatory change..."
curl -X POST "$MOCK_URL/api/now/table/sn_regulatory_change" \
  -H "Content-Type: application/json" \
  -d '{
    "sys_id": "reg001",
    "number": "REG0001",
    "short_description": "EU DORA compliance",
    "description": "New Digital Operational Resilience Act requirements for financial services",
    "regulation_name": "DORA",
    "jurisdiction": "European Union",
    "effective_date": "2024-01-01T00:00:00Z",
    "state": "new",
    "assigned_to": "",
    "sys_created_on": "2023-05-15T10:00:00Z",
    "sys_updated_on": "2023-05-15T10:00:00Z",
    "impact_assessment": "",
    "implementation_plan": ""
  }'

echo "Mock data creation complete!"