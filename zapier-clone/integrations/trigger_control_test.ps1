# PowerShell script to trigger the Policy and Control Testing use case

# Base URLs
$MOCK_URL = "http://localhost:3000"
$JIRA_URL = "http://localhost:3001"
$SLACK_URL = "http://localhost:3002"
$GRC_URL = "http://localhost:8080"

Write-Host "===== Implementing Policy and Control Testing Use Case ====="

# Step 1: Create a control test in ServiceNow
Write-Host "1. Creating 'SOX Access Controls' test in ServiceNow..."
$controlTestPayload = @{
    sys_id = "ctrl003"
    number = "CTRL0003"
    description = "Test access controls for SOX compliance"
    control_name = "SOX Access Controls"
    due_date = "2025-03-27"  # Adjusted to ensure overdue by March 29, 2025
    test_status = "Open"
    results = ""
    assigned_to = "audit_team"
    sys_created_on = "2025-03-27T12:00:00Z"
    sys_updated_on = "2025-03-27T12:00:00Z"
} | ConvertTo-Json

try {
    $webhookResponse = Invoke-RestMethod -Uri "$MOCK_URL/trigger_webhook/sn_policy_control_test/inserted?webhook_url=$GRC_URL/api/webhooks/servicenow" `
        -Method Post `
        -ContentType "application/json" `
        -Body $controlTestPayload
    Write-Host "Webhook triggered successfully:"
    Write-Host ($webhookResponse | ConvertTo-Json)
} catch {
    Write-Host "Error triggering webhook: $($_.Exception.Message)"
    exit 1
}

# Wait for sync
Write-Host "Waiting for synchronization..."
Start-Sleep -Seconds 5

# Step 2: Verify Jira ticket creation
Write-Host "2. Verifying Jira issue creation..."
try {
    $jiraSearch = Invoke-RestMethod -Uri "$JIRA_URL/rest/api/2/search?jql=project=AUDIT+AND+customfield_servicenow_id=ctrl003" -Method Get
    if ($jiraSearch.issues.Count -gt 0) {
        $jiraIssue = $jiraSearch.issues[0]
        Write-Host "Found Jira issue for ctrl003:"
        Write-Host ($jiraIssue | ConvertTo-Json -Depth 10)
        $jiraKey = $jiraIssue.key
    } else {
        Write-Host "Error: No Jira issue found for ctrl003. Synchronization may have failed."
        exit 1
    }
} catch {
    Write-Host "Error fetching Jira issue: $($_.Exception.Message)"
    exit 1
}

# Step 3: Simulate updating Jira ticket with test result
Write-Host "3. Updating Jira issue with test result..."
$jiraUpdatePayload = @{
    fields = @{
        description = "Test Result: Fail - weak passwords detected"
        status = @{ name = "Done" }
    }
} | ConvertTo-Json

try {
    Invoke-RestMethod -Uri "$JIRA_URL/rest/api/2/issue/$jiraKey" `
        -Method Put `
        -ContentType "application/json" `
        -Body $jiraUpdatePayload
    Write-Host "Successfully updated Jira issue $jiraKey"
} catch {
    Write-Host "Error updating Jira issue $jiraKey`: $($_.Exception.Message)"
    exit 1
}

# Wait for sync back to ServiceNow
Write-Host "Waiting for ServiceNow sync..."
Start-Sleep -Seconds 5

# Step 4: Verify ServiceNow update
Write-Host "4. Checking ServiceNow for updated control test..."
try {
    $snRecord = Invoke-RestMethod -Uri "$MOCK_URL/api/now/table/sn_policy_control_test/ctrl003" -Method Get
    Write-Host "ServiceNow record:"
    Write-Host ($snRecord | ConvertTo-Json -Depth 10)
    if ($snRecord.result.test_status -eq "Done") {
        Write-Host "ServiceNow record updated successfully with test result"
    } else {
        Write-Host "Error: ServiceNow sync failed (test_status still $($snRecord.result.test_status))"
        exit 1
    }
} catch {
    Write-Host "Error fetching ServiceNow record: $($_.Exception.Message)"
    exit 1
}

# Step 5: Note about overdue reminders
Write-Host "5. Overdue reminders should be sent to Slack (due date: 2025-03-27). Check Slack #audit channel in 60 seconds."

Write-Host "===== Use Case Setup Complete! ====="