# AuditCue Integration Framework

The AuditCue Integration Framework is a platform that enables seamless connectivity between the AuditCue GRC platform and various third-party business applications, similar to Zapier's functionality. This framework allows users to create automated workflows between AuditCue and external systems without requiring extensive programming knowledge.

## Features

- **Visual Workflow Builder**: Create workflows with a drag-and-drop interface
- **Connection Management**: Securely store and manage connections to third-party services
- **Data Mapping**: Map fields between different services
- **Authentication**: OAuth 2.0 and API Key support for connecting to external services
- **Execution Engine**: Reliable execution of workflows with retry capability
- **Monitoring**: Track workflow execution status and history

## Supported Integrations

- Jira
- Slack
- Microsoft Teams
- Google Workspace
- Microsoft 365
- ServiceNow

## Architecture

The framework is built using a modern stack:

- **Backend**: Go with RESTful API design
- **Frontend**: React with Tailwind CSS
- **Database**: SQLite (default) with support for PostgreSQL
- **Queue System**: Redis for job processing

## Directory Structure

```
.
├── backend/                 # Go backend code
│   ├── cmd/                 # Application entry points
│   ├── internal/            # Internal packages
│   │   ├── api/             # API handlers and routes
│   │   ├── auth/            # Authentication logic
│   │   ├── config/          # Configuration management
│   │   ├── db/              # Database access
│   │   ├── integrations/    # Integration implementations
│   │   ├── queue/           # Job queue
│   │   └── workflow/        # Workflow engine
│   └── pkg/                 # Shared packages
├── frontend/                # React frontend code
│   ├── public/              # Static assets
│   └── src/                 # React components and logic
├── config/                  # Configuration files
├── data/                    # Data storage (SQLite database)
├── docker/                  # Docker configurations
└── scripts/                 # Utility scripts
```

## Getting Started

### Prerequisites

- Go 1.18 or later
- Node.js 16 or later
- Redis (for job processing)

### Development Setup

1. Clone the repository:

   ```bash
   git clone https://github.com/auditcue/integration-framework.git
   cd integration-framework
   ```

2. Install dependencies:

   ```bash
   # Install backend dependencies
   cd backend
   go mod download

   # Install frontend dependencies
   cd ../frontend
   npm install
   ```

3. Configure the application:

   ```bash
   cp config.example.yaml config/config.yaml
   # Edit the configuration file as needed
   ```

4. Start the development servers:

   ```bash
   # For convenience, use the development script
   ./scripts/dev.sh

   # Or run the servers manually
   # Terminal 1 - Backend
   cd backend
   go run cmd/server/main.go

   # Terminal 2 - Frontend
   cd frontend
   npm start
   ```

5. Access the application:
   - Frontend: http://localhost:3000
   - Backend API: http://localhost:8081
     

### Running with Docker

```bash
# Build and start the containers
cd docker
docker compose up -d

# Access the application at http://localhost:3000
```

## Configuration

The framework is configured using a YAML file located at `config/config.yaml`. You can configure:

- Database connection details
- Job queue settings
- Authentication settings
- Integration service credentials

## Developing New Integrations

To add a new integration:

1. Create a new package in `backend/internal/integrations/`
2. Implement the required interfaces:
   - `ServiceProvider`
   - `AuthHandler`
   - `TriggerHandler`
   - `ActionHandler`
3. Register the new service provider in `backend/internal/api/routes.go`
4. Add UI components in the frontend to support the new integration

## License

MIT License - See LICENSE file for details.

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.
