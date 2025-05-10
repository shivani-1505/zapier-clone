#!/bin/bash
# Script to implement the cybersecurity risk use case

# Base URL for the mock ServiceNow server
MOCK_URL="http://localhost:3000"

echo "===== Implementing Cybersecurity Risk Use Case ====="

# Step 1: Create the specific unpatched servers risk in ServiceNow
echo "1. Creating 'Unpatched Servers' risk in ServiceNow GRC..."
curl -X POST "$MOCK_URL/servicenow/create_risk" \
  -H "Content-Type: application/json" \
  -d '{
    "title": "Unpatched servers",
    "description": "Critical security patches missing on production web servers",
    "severity": "High",
    "category": "Cybersecurity",
    "owner": "IT Security Team",
    "status": "Open",
    "dueDate": "2023-06-30"
  }'

# Wait for sync to happen
echo "Waiting for synchronization..."
sleep 2

# Step 2: Verify the risk was created in ServiceNow
echo "2. Verifying risk record in ServiceNow..."
RISK_ID=$(curl -s "$MOCK_URL/api/now/table/sn_risk_risk" | grep -o '"sys_id":"[^"]*' | head -1 | cut -d'"' -f4)
echo "   Risk created with ID: $RISK_ID"

# Step 3: Check if the corresponding Jira issue was created
echo "3. Checking Jira for created issue..."
JIRA_URL="http://localhost:3001/rest/api/2/search?jql=project=RM"
curl -s "$JIRA_URL"

# Step 4: Create associated tasks
echo "4. Creating tasks for the IT team in Jira..."
curl -X POST "http://localhost:3001/rest/api/2/issue" \
  -H "Content-Type: application/json" \
  -d '{
    "fields": {
      "project": {"key": "RM"},
      "summary": "Apply security patches to web servers",
      "description": "Apply the latest security patches to all production web servers to mitigate vulnerabilities",
      "issuetype": {"name": "Task"},
      "priority": {"name": "High"},
      "assignee": {"name": "jsmith"},
      "duedate": "2023-06-15"
    }
  }'

curl -X POST "http://localhost:3001/rest/api/2/issue" \
  -H "Content-Type: application/json" \
  -d '{
    "fields": {
      "project": {"key": "RM"},
      "summary": "Verify patched servers with vulnerability scan",
      "description": "Run vulnerability scans on patched servers to confirm vulnerabilities are resolved",
      "issuetype": {"name": "Task"},
      "priority": {"name": "Medium"},
      "assignee": {"name": "agarcia"},
      "duedate": "2023-06-20"
    }
  }'

# Step 5: Trigger a Slack notification
echo "5. Sending notification to Slack #security channel..."
curl -X POST "http://localhost:3002/api/chat.postMessage" \
  -H "Content-Type: application/json" \
  -d '{
    "channel": "C01",
    "text": "🚨 *High Risk Alert*: Unpatched servers",
    "blocks": [
      {
        "type": "header",
        "text": {
          "type": "plain_text",
          "text": "🚨 High Risk Alert: Unpatched Servers"
        }
      },
      {
        "type": "section",
        "text": {
          "type": "mrkdwn",
          "text": "*Critical security patches missing on production web servers*\n\nRisk ID: '"$RISK_ID"'\nSeverity: High\nOwner: IT Security Team"
        }
      },
      {
        "type": "actions",
        "elements": [
          {
            "type": "button",
            "text": {
              "type": "plain_text",
              "text": "View Details"
            },
            "style": "primary",
            "action_id": "view_risk_details",
            "value": "'"$RISK_ID"'"
          },
          {
            "type": "button",
            "text": {
              "type": "plain_text",
              "text": "Accept Task"
            },
            "style": "primary",
            "action_id": "accept_risk_mitigation",
            "value": "RM-101"
          },
          {
            "type": "button",
            "text": {
              "type": "plain_text",
              "text": "Mark Complete"
            },
            "action_id": "complete_risk_mitigation",
            "value": "RM-101"
          }
        ]
      }
    ]
  }'

echo ""
echo "===== Use Case Setup Complete! ====="
echo ""
echo "Demo flow:"
echo "1. Risk has been created in ServiceNow GRC"
echo "2. Risk has been synced to Jira as an issue"
echo "3. Tasks have been assigned to the IT team in Jira"
echo "4. Slack notification has been sent to #security channel with actionable buttons"
echo ""
echo "Next steps for demo:"
echo "- Show the risk details in ServiceNow: http://localhost:3000"
echo "- Show the Jira issue and tasks: http://localhost:3001"
echo "- Show the Slack notification: http://localhost:3002"
echo "- Demonstrate interaction by clicking 'Accept Task' in Slack"
echo "- Show the status update reflecting across all systems"