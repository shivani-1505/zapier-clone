# Define URLs for ServiceNow, GRC, and Jira mock servers
$SN_URL = "http://localhost:3000"
$GRC_URL = "http://localhost:8080"
$JIRA_URL = "http://localhost:3001"

# Create a regulatory change payload for ServiceNow with explicit sys_id
$regChangePayload = @{
    sys_id = "REG001"  # Explicitly set sys_id
    number = "REG001"
    description = "Update due to new GDPR regulation"
    change_name = "GDPR 2025"
    effective_date = "2025-06-01"
    assigned_to = "jane.doe"
} | ConvertTo-Json

# Create webhook payload to simulate ServiceNow insertion
$webhookPayload = @{
    sys_id = "REG001"
    table_name = "sn_regulatory_change"
    action_type = "inserted"
    data = ($regChangePayload | ConvertFrom-Json)
} | ConvertTo-Json

# Create regulatory change in ServiceNow and trigger webhook
Write-Host "Creating regulatory change in ServiceNow and triggering webhook..."
Invoke-RestMethod -Uri "$SN_URL/api/now/table/sn_regulatory_change" -Method Post -Body $regChangePayload -ContentType "application/json"
Invoke-RestMethod -Uri "$GRC_URL/api/webhooks/servicenow" -Method Post -Body $webhookPayload -ContentType "application/json"

# Wait for Jira issue creation
Start-Sleep -Seconds 10

# Find the dynamically created epic
$jiraSearch = Invoke-RestMethod -Uri "$JIRA_URL/rest/api/2/search?jql=project=AUDIT+AND+customfield_servicenow_id=REG001+AND+issuetype=Epic" -Method Get
$epicKey = $jiraSearch.issues[0].key
Write-Host "Found epic: $epicKey"

# Update the Jira epic to "Done"
$updatePayload = @{
    fields = @{
        status = @{ name = "Done" }
        description = "GDPR policy updated and staff trained"
    }
} | ConvertTo-Json

Write-Host "Updating Jira epic $epicKey to Done..."
Invoke-RestMethod -Uri "$JIRA_URL/rest/api/2/issue/$epicKey" -Method Put -Body $updatePayload -ContentType "application/json"

# Wait for ServiceNow sync
Start-Sleep -Seconds 10

# Verify ServiceNow record update
$snRecord = Invoke-RestMethod -Uri "$SN_URL/api/now/table/sn_regulatory_change/REG001" -Method Get
if ($snRecord.result.state -eq "Done") {
    Write-Host "ServiceNow record updated successfully!"
} else {
    Write-Host "ServiceNow sync failed: state is $($snRecord.result.state)"
}