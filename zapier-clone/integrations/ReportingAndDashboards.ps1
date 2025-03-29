# ReportingAndDashboards.ps1

# Define URLs for ServiceNow, GRC, and Jira mock servers
$SN_URL = "http://localhost:3000"
$GRC_URL = "http://localhost:8080"
$JIRA_URL = "http://localhost:3001"

# Fetch initial dashboard data
Write-Host "Fetching initial dashboard data..."
$initialDashboard = Invoke-RestMethod -Uri "$GRC_URL/api/dashboard" -Method Get
Write-Host "Initial dashboard: OpenRisks=$($initialDashboard.open_risks), CompletedControls=$($initialDashboard.completed_controls), OpenRegulatoryTasks=$($initialDashboard.open_regulatory_tasks)"

# Create a control test payload for ServiceNow
$controlPayload = @{
    number = "ctrl002"
    description = "Test PCI DSS control"
    control_name = "PCI DSS 3.2"
    due_date = "2025-04-15"
    assigned_to = "john.doe"
} | ConvertTo-Json

# Create webhook payload to simulate ServiceNow insertion
$webhookPayload = @{
    sys_id = "ctrl002"
    table_name = "sn_policy_control_test"
    action_type = "inserted"
    data = ($controlPayload | ConvertFrom-Json)
} | ConvertTo-Json

# Create control test in ServiceNow and trigger webhook
Write-Host "Creating control test in ServiceNow and triggering webhook..."
Invoke-RestMethod -Uri "$SN_URL/api/now/table/sn_policy_control_test" -Method Post -Body $controlPayload -ContentType "application/json"
Invoke-RestMethod -Uri "$GRC_URL/api/webhooks/servicenow" -Method Post -Body $webhookPayload -ContentType "application/json"

# Wait 5 minutes for dashboard to update (simulating periodic update)
Write-Host "Waiting 1 minutes for dashboard to update..."
Start-Sleep -Seconds 60

# Fetch updated dashboard data
$updatedDashboard = Invoke-RestMethod -Uri "$GRC_URL/api/dashboard" -Method Get
Write-Host "Updated dashboard: OpenRisks=$($updatedDashboard.open_risks), CompletedControls=$($updatedDashboard.completed_controls), OpenRegulatoryTasks=$($updatedDashboard.open_regulatory_tasks)"

# Verify dashboard update
if ($updatedDashboard.completed_controls -gt $initialDashboard.completed_controls) {
    Write-Host "Dashboard updated successfully!"
} else {
    Write-Host "Dashboard update failed."
}