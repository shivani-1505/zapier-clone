# AuditCue

An automation platform that enables seamless connectivity between different business applications.

## Project Structure

```
auditcue/
├── cmd/                # Application entry points
├── configs/            # Configuration files
├── internal/           # Private application code
├── pkg/               # Public libraries
├── docs/              # Documentation
├── tests/             # Test files
└── web/              # Frontend assets
```

##Explanation of the Directory structure in detail

------------------------------------------------------------------------------------------------------------------------------------------------
------------------------------------------------------------------------------------------------------------------------------------------------

**The cmd Directory**
This is like the control center of our application. Inside cmd/server/main.go, we have our main application entry point. This is where everything starts - it's responsible for:

Initializing all components

Setting up the server

Connecting to the database

Starting background workers

Managing the application lifecycle

------------------------------------------------------------------------

**The configs Directory**
Think of this as the rulebook for our application. It contains different configuration files:

app.yaml: Base configuration that applies everywhere

development.yaml: Special rules for when we're developing

production.yaml: Rules for when the app is live for users

These files control things like database connections, API keys, and environment-specific settings.

------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------

**The internal Directory**
This is the heart of our application. The "internal" name is special in Go - it means these packages can only be used within our application, not by other projects. Let's break down each subdirectory:

auth/

handlers.go: Processes login/signup requests

middleware.go: Checks if users are allowed to access certain parts

service.go: Contains the core logic for authentication


connections/
This manages how our application talks to other services (like Google or Slack):

oauth/: Contains specific code for each service we connect to

handlers.go: Processes requests to connect/disconnect services

service.go: Manages the connection lifecycle


models/
These are our data blueprints:

user.go: Defines what information we store about users

connection.go: Defines how we store connection information

workflow.go: Defines how automated workflows are structured


database/
This handles all database operations:

migrations/: Contains files that set up and update our database structure

database.go: Manages database connections and operations


workflows/
This is our automation engine:

engine/: Contains the core automation logic

queue/: Manages scheduled and background tasks

handlers.go: Processes workflow-related requests

service.go: Contains the business logic for workflows

---------------------------------------------------------------------------------------------------

**The pkg Directory**
Unlike internal, this directory contains code that could be used by other projects. It includes:

logger/: Handles application logging

validator/: Validates user inputs

errors/: Defines custom error types

------------------------------------------------------------------------

**The docs Directory**
This is our project's knowledge base:

api/: API documentation (like Swagger files)

guides/: Instructions for developers and users

architecture/: Explains how the system is built

------------------------------------------------------------------------

**The scripts Directory**
Contains utility scripts that help with:

Setting up development environments

Database migrations

Deployment processes

------------------------------------------------------------------------

**The tests Directory**
Organized into two main types:

integration/: Tests how different parts work together

unit/: Tests individual components in isolation

------------------------------------------------------------------------

**The web Directory**
Contains frontend-related files:

static/: CSS, JavaScript, and images

templates/: HTML templates and email templates

------------------------------------------------------------------------

**Root-Level Files**

Dockerfile: Instructions for building our application container

docker-compose.yml: Defines our development environment

go.mod: Lists our Go dependencies

Makefile: Contains commands for building and running the app

README.md: Project overview and setup instructions

------------------------------------------------------------------------------------------------------------------------------------------------
------------------------------------------------------------------------------------------------------------------------------------------------

## Getting Started

1. Clone the repository
2. Install dependencies: `go mod tidy`
3. Run the application: `go run cmd/server/main.go`

## Development

See `docs/guides/development.md` for development guidelines.

## Deployment

See `docs/guides/deployment.md` for deployment instructions.

