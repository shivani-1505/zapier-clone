# ServiceNow GRC - Slack Integration Implementation Guide

This guide provides step-by-step instructions for implementing the ServiceNow GRC and Slack integration using the AuditCue Integration Framework.

## Prerequisites

- ServiceNow GRC instance with admin access
- Slack workspace with admin privileges
- Server or cloud environment for hosting the integration
- Go 1.18+ for development

## 1. Set Up Slack App

### Create a Slack App

1. Go to [https://api.slack.com/apps](https://api.slack.com/apps)
2. Click "Create New App" → "From scratch"
3. Enter a name (e.g., "GRC Integration") and select your workspace
4. Click "Create App"

### Configure Bot Permissions

1. Navigate to "OAuth & Permissions" in the sidebar
2. Under "Scopes" → "Bot Token Scopes", add the following:
   - `channels:read` - View channels
   - `chat:write` - Send messages
   - `commands` - Add slash commands
   - `files:read` - Access files
   - `users:read` - View user information
   - `reactions:write` - Add reactions to messages

### Create Slash Commands

1. Navigate to "Slash Commands" in the sidebar
2. Click "Create New Command" and add the following commands:
   - `/assign-owner` - Description: "Assign an owner to a GRC item"
   - `/update-status` - Description: "Update the status of a GRC item"
   - `/upload-evidence` - Description: "Upload evidence for a compliance task"
   - `/grc-status` - Description: "Get current GRC metrics"

3. Set the Request URL for each command to your server URL + `/api/slack/commands/slack-workspace`
   (e.g., `https://integration.example.com/api/slack/commands/slack-workspace`)

### Configure Interactive Components

1. Navigate to "Interactivity & Shortcuts" in the sidebar
2. Enable interactivity and set the Request URL to your server URL + `/api/slack/interactions/slack-workspace`
   (e.g., `https://integration.example.com/api/slack/interactions/slack-workspace`)

### Install the App

1. Navigate to "Install App" in the sidebar
2. Click "Install to Workspace"
3. Review and authorize the permissions
4. Save the "Bot User OAuth Token" (begins with `xoxb-`) and "Signing Secret" for later

### Create Slack Channels

Create the following channels in your Slack workspace:
- `#risk-management` - For risk-related notifications
- `#incident-response` - For security incident notifications
- `#compliance-team` - For compliance task notifications
- `#audit-team` - For audit finding notifications
- `#vendor-risk` - For vendor risk notifications
- `#regulatory-updates` - For regulatory change notifications
- `#grc-reports` - For GRC reports and metrics

## 2. Configure ServiceNow

### Create an Integration User

1. In ServiceNow, navigate to User Administration
2. Create a new user with appropriate permissions for GRC modules
3. Assign the user the following roles:
   - `sn_risk_mgmt.risk_manager`
   - `sn_compliance.compliance_manager`
   - `sn_audit.audit_manager`
   - If using OAuth, also assign `oauth_admin`

### Set Up Outbound REST Message (Optional)

To enable real-time notifications:

1. Navigate to System Web Services → Outbound → REST Message
2. Create a new REST Message:
   - Name: "GRC Slack Integration"
   - Endpoint: Your server URL + `/api/webhooks/servicenow/servicenow-grc`
   (e.g., `https://integration.example.com/api/webhooks/servicenow/servicenow-grc`)

3. Create HTTP Methods for:
   - POST Risk
   - POST Incident
   - POST Compliance Task
   - POST Audit Finding

### Create Business Rules for Notifications

For each GRC table (Risk, Incident, Compliance Task, etc.):

1. Navigate to System Definition → Business Rules
2. Create a new Business Rule:
   - Name: "Notify Slack on [Table] Update"
   - Table: The GRC table (e.g., sn_rm_risk)
   - When: After Insert and Update
   - Action: Call the REST Message to send data to the integration

## 3. Deploy the Integration

### Configure Environment Variables

Create a `.env` file with the following variables:
```
# General
PORT=8080
JWT_SECRET=your-jwt-secret

# Database
DATABASE_URL=postgresql://user:password@localhost:5432/auditcue

# ServiceNow
SERVICENOW_INSTANCE_URL=https://your-instance.service-now.com
SERVICENOW_USERNAME=integration_user
SERVICENOW_PASSWORD=your-password

# Slack
SLACK_BOT_TOKEN=xoxb-your-bot-token
SLACK_SIGNING_SECRET=your-signing-secret
SLACK_APP_ID=A12345ABCDE

# Slack Channels
SLACK_RISK_CHANNEL_ID=C12345ABCDE
SLACK_INCIDENT_CHANNEL_ID=C23456BCDEF
SLACK_COMPLIANCE_CHANNEL_ID=C34567CDEFG
SLACK_AUDIT_CHANNEL_ID=C45678DEFGH
SLACK_VENDOR_CHANNEL_ID=C56789EFGHI
SLACK_REGULATORY_CHANNEL_ID=C67890FGHIJ
SLACK_REPORTS_CHANNEL_ID=C78901GHIJK
```

### Build and Run

With Docker:
```bash
# Build the Docker image
docker build -f docker/Dockerfile.backend -t auditcue-integration .

# Run the container
docker run -p 8080:8080 --env-file .env auditcue-integration
```

Without Docker:
```bash
cd backend
go build -o auditcue-server ./cmd/server
./auditcue-server
```

## 4. Testing the Integration

### Test ServiceNow to Slack Flow

1. Create a new risk in ServiceNow GRC
2. Verify a notification appears in the `#risk-management` channel
3. Check the message includes severity, description, and action buttons

### Test Slack Commands

1. In the `#risk-management` channel, use the command: `/assign-owner RISK001 @username`
2. Verify the command is processed and the risk is assigned in ServiceNow
3. Try other commands like `/update-status` and `/grc-status`

### Test Interactive Buttons

1. Find a risk notification in Slack
2. Click the "Assign Owner" button
3. Complete the dialog that appears
4. Verify the risk is updated in ServiceNow

## 5. Troubleshooting

### Check Logs

Monitor the integration server logs for errors:
```bash
docker logs auditcue-integration
```

### Verify Webhook Connectivity

1. Use a tool like Postman to send a test webhook to your server
2. Check the server logs for the webhook receipt
3. Verify the server can connect to both ServiceNow and Slack APIs

### Common Issues

- **Permissions**: Ensure the ServiceNow user and Slack bot have sufficient permissions
- **Network**: Check firewall rules if the integration server cannot reach ServiceNow or Slack
- **Authentication**: Verify that tokens and credentials are correct and not expired

## 6. Production Considerations

### Security

- Use HTTPS for all endpoints
- Store secrets in a secure vault, not environment variables
- Implement IP restrictions if possible
- Regularly rotate credentials

### Monitoring

- Set up logging to a centralized system
- Create alerts for integration failures
- Monitor API rate limits for both ServiceNow and Slack

### Scaling

- Use a database with proper connection pooling
- Implement message queuing for high-volume environments
- Consider multiple instances behind a load balancer for large deployments

## 7. Extending the Integration

### Add Custom Workflows

1. Define new workflows in `config.yaml`
2. Add new triggers or actions in code if needed
3. Restart the service to apply changes

### Integrate with Additional Systems

The framework can be extended to connect with:
- JIRA
- Microsoft Teams
- Email systems
- CI/CD pipelines
- Other GRC platforms

## Support and Feedback

For questions or issues, please contact the development team or open an issue in the repository.