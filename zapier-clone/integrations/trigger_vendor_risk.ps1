Write-Host "===== Implementing Third-Party Risk Management Use Case ====="
$SN_URL = "http://localhost:3000"
$JIRA_URL = "http://localhost:3001"

# Step 1: Create a vendor risk in ServiceNow
Write-Host "1. Flagging 'Vendor failed SOC 2 audit' in ServiceNow..."
$vendorRisk = @{
    sys_id = "risk001"
    number = "RISK0001"
    risk_name = "Vendor failed SOC 2 audit"
    description = "Vendor X failed SOC 2 audit due to weak access controls"
    assigned_to = "procurement_user"
    state = "Open"
    sys_created_on = "2025-03-28T12:00:00Z"
}
$jsonBody = $vendorRisk | ConvertTo-Json

try {
    $response = Invoke-RestMethod -Uri "$SN_URL/api/now/table/sn_vendor_risk" `
        -Method Post `
        -Body $jsonBody `
        -ContentType "application/json"
    Write-Host "Vendor risk created successfully:"
    Write-Host ($response | ConvertTo-Json -Depth 10)
} catch {
    Write-Host "Error creating vendor risk in ServiceNow: $($_.Exception.Message)"
    exit 1
}

# Wait for synchronization to Jira
Start-Sleep -Seconds 5

# Step 2: Verify Jira ticket creation
Write-Host "2. Verifying Jira ticket creation..."
try {
    $jiraIssue = Invoke-RestMethod -Uri "$JIRA_URL/rest/api/2/search?jql=project=AUDIT AND customfield_servicenow_id=risk001" `
        -Method Get
    if ($jiraIssue.issues.Count -gt 0) {
        Write-Host "Found Jira issue for risk001:"
        Write-Host ($jiraIssue.issues[0] | ConvertTo-Json -Depth 10)
        $jiraKey = $jiraIssue.issues[0].key
    } else {
        Write-Host "Error: No Jira issue found for risk001. Synchronization may have failed."
        exit 1
    }
} catch {
    Write-Host "Error fetching Jira issue: $($_.Exception.Message)"
    exit 1
}

# Step 3: Update Jira ticket with resolution
Write-Host "3. Updating Jira ticket with resolution..."
$updateBody = @{
    fields = @{
        resolution = "Updated SOC 2 report received from Vendor X"
        status = @{ name = "Done" }
    }
} | ConvertTo-Json

try {
    Invoke-RestMethod -Uri "$JIRA_URL/rest/api/2/issue/$jiraKey" `
        -Method Put `
        -Body $updateBody `
        -ContentType "application/json"
    Write-Host "Successfully updated Jira issue $jiraKey"
} catch {
    Write-Host "Error updating Jira issue $jiraKey`: $($_.Exception.Message)"
    exit 1
}

# Wait for synchronization back to ServiceNow
Start-Sleep -Seconds 5

# Step 4: Check ServiceNow for updated vendor risk
Write-Host "4. Checking ServiceNow for updated vendor risk..."
try {
    $snRecord = Invoke-RestMethod -Uri "$SN_URL/api/now/table/sn_vendor_risk/risk001" `
        -Method Get
    Write-Host "ServiceNow record:"
    Write-Host ($snRecord | ConvertTo-Json -Depth 10)

    # Adjusted state check to accept "Resolved" or "Done" as valid sync states
    $validStates = @("Resolved", "Done")
    if ($validStates -contains $snRecord.result.state) {
        Write-Host "ServiceNow sync succeeded with state: $($snRecord.result.state)"
    } else {
        Write-Host "Error: ServiceNow sync failed (state is $($snRecord.result.state), expected one of: $validStates)"
        exit 1
    }
} catch {
    Write-Host "Error fetching ServiceNow record: $($_.Exception.Message)"
    exit 1
}

Write-Host "===== Use Case Setup Complete! ====="