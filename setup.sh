#!/bin/bash

# This line above is called a shebang - it tells the system this is a bash script

# First, let's make the script stop if any command fails
set -e

echo "Starting to create AuditCue project structure..."

# Create main application directories
echo "Creating main application directories..."
mkdir -p cmd/server
mkdir -p configs
mkdir -p scripts

# Create internal directory structure
echo "Creating internal package directories..."
mkdir -p internal/auth
mkdir -p internal/connections/oauth
mkdir -p internal/models
mkdir -p internal/database/migrations
mkdir -p internal/workflows/engine
mkdir -p internal/workflows/queue
mkdir -p internal/api/middleware
mkdir -p internal/api/routes
mkdir -p internal/api/handlers

# Create public package directory
echo "Creating public package directories..."
mkdir -p pkg/logger
mkdir -p pkg/validator
mkdir -p pkg/errors

# Create documentation directories
echo "Creating documentation directories..."
mkdir -p docs/api
mkdir -p docs/guides
mkdir -p docs/architecture

# Create test directories
echo "Creating test directories..."
mkdir -p tests/integration
mkdir -p tests/unit

# Create web assets directories
echo "Creating web asset directories..."
mkdir -p web/static/css
mkdir -p web/static/js
mkdir -p web/static/img
mkdir -p web/templates/emails

# Create initial files
echo "Creating initial files..."

# Create main application files
touch cmd/server/main.go

# Create config files
touch configs/app.yaml
touch configs/development.yaml
touch configs/production.yaml

# Create auth package files
touch internal/auth/handlers.go
touch internal/auth/middleware.go
touch internal/auth/service.go

# Create model files
touch internal/models/connection.go
touch internal/models/user.go
touch internal/models/workflow.go

# Create database files
touch internal/database/database.go

# Create workflow engine files
touch internal/workflows/engine/executor.go
touch internal/workflows/engine/scheduler.go
touch internal/workflows/handlers.go
touch internal/workflows/service.go

# Create utility package files
touch pkg/logger/logger.go
touch pkg/validator/validator.go
touch pkg/errors/errors.go

# Create documentation files
touch docs/api/swagger.yaml
touch docs/guides/development.md
touch docs/guides/deployment.md
touch docs/architecture/system-design.md

# Create root level files
touch Dockerfile
touch docker-compose.yml
touch Makefile
touch README.md
touch .gitignore

# Make the script executable
chmod +x setup.sh

echo "Project structure created successfully!"

# Initialize git repository if git is installed
if command -v git >/dev/null 2>&1; then
    echo "Initializing git repository..."
    git init
    
    # Create initial .gitignore content
    echo "Creating .gitignore content..."
    cat > .gitignore << EOL
# Binaries for programs and plugins
*.exe
*.exe~
*.dll
*.so
*.dylib

# Test binary, built with 'go test -c'
*.test

# Output of the go coverage tool, specifically when used with LiteIDE
*.out

# Dependency directories (remove the comment below to include it)
vendor/

# IDE specific files
.idea/
.vscode/
*.swp
*.swo

# OS specific files
.DS_Store
.DS_Store?
._*
.Spotlight-V100
.Trashes
ehthumbs.db
Thumbs.db
EOL

    # Create initial README content
    echo "Creating README.md content..."
    cat > README.md << EOL
# AuditCue

An automation platform that enables seamless connectivity between different business applications.

## Project Structure

\`\`\`
auditcue/
├── cmd/                # Application entry points
├── configs/            # Configuration files
├── internal/           # Private application code
├── pkg/               # Public libraries
├── docs/              # Documentation
├── tests/             # Test files
└── web/              # Frontend assets
\`\`\`

## Getting Started

1. Clone the repository
2. Install dependencies: \`go mod tidy\`
3. Run the application: \`go run cmd/server/main.go\`

## Development

See \`docs/guides/development.md\` for development guidelines.

## Deployment

See \`docs/guides/deployment.md\` for deployment instructions.
EOL
fi

# Initialize Go module if go is installed
if command -v go >/dev/null 2>&1; then
    echo "Initializing Go module..."
    go mod init github.com/yourusername/auditcue
    go mod tidy
fi

echo "Setup completed successfully!"