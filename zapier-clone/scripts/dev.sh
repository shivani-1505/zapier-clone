#!/bin/bash

# This script sets up a development environment for the AuditCue Integration Framework

# Exit on error
set -e

# Create necessary directories
mkdir -p data

# Check if Go is installed
if ! command -v go &> /dev/null; then
    echo "Go is not installed. Please install Go 1.18 or later."
    exit 1
fi

# Check if Node.js is installed
if ! command -v node &> /dev/null; then
    echo "Node.js is not installed. Please install Node.js 16 or later."
    exit 1
fi

# Check if npm is installed
if ! command -v npm &> /dev/null; then
    echo "npm is not installed. Please install npm."
    exit 1
fi

# Check if Docker is installed (optional)
if ! command -v docker &> /dev/null; then
    echo "Warning: Docker is not installed. Docker is recommended for Redis."
    DOCKER_INSTALLED=false
else
    DOCKER_INSTALLED=true
fi

# Check if Docker is installed for database services
if [ "$DOCKER_INSTALLED" = true ]; then
    # Check if Redis is running
    if ! docker ps | grep -q auditcue-redis; then
        echo "Starting Redis using Docker..."
        docker run --name auditcue-redis -p 6379:6379 -d redis:7-alpine
    else
        echo "Redis is already running."
    fi

    # Check if PostgreSQL is running
    if ! docker ps | grep -q auditcue-postgres; then
        echo "Starting PostgreSQL using Docker..."
        docker run --name auditcue-postgres -e POSTGRES_USER=auditcue -e POSTGRES_PASSWORD=auditcue -e POSTGRES_DB=auditcue -p 5432:5432 -d postgres:14-alpine
        
        # Wait for PostgreSQL to start
        echo "Waiting for PostgreSQL to start..."
        sleep 5
    else
        echo "PostgreSQL is already running."
    fi
else
    echo "Docker not installed. Make sure Redis and PostgreSQL are running manually."
fi

# Install Go dependencies
echo "Installing Go dependencies..."
cd backend
go mod download
cd ..

# Install frontend dependencies
echo "Installing frontend dependencies..."
cd frontend
npm install
cd ..

# Create a default config file if it doesn't exist
if [ ! -f config/config.yaml ]; then
    echo "Creating default config file..."
    mkdir -p config
    cp config.example.yaml config/config.yaml
fi

# Run the backend in the background
echo "Starting backend server..."
cd backend
go run cmd/server/main.go &
BACKEND_PID=$!
cd ..

# Wait for backend to start
echo "Waiting for backend to start..."
sleep 5

# Run the frontend
echo "Starting frontend server..."
cd frontend
npm start

# Clean up on exit
trap "kill $BACKEND_PID" EXIT