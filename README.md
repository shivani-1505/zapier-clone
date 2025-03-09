# Zapier Clone

This project is a clone of Zapier built using GoLang for the backend and React for the frontend, with PostgreSQL as the database.

## Project Structure

```
│-- backend
    │-- api
        │-- routes.go
    │-- apps
        │-- gdrive
        │-- gmail
        │-- jira
        │-- outlook
        │-- servicenow
        │-- slack
        │-- teams
    │-- cmd
        │-- server
            │-- main.go
    │-- internal
        │-- auth
        │-- config
        │-- database
        │-- http
        │-- models
        │-- store
    │-- .dockerignore
    │-- Dockerfile
    │-- go.mod
    │-- go.sum
    │-- railway.toml
│-- frontend
    │-- public
        │-- vite.svg
    │-- src
        │-- assets
        │-- App.css
        │-- App.tsx
        │-- index.css
        │-- main.jsx
    │-- .gitignore
    │-- eslint.config.js
    │-- index.html
    │-- package-lock.json
    │-- package.json
    │-- README.md
    │-- vite.config.js
│-- .DS_Store
│-- Procfile
│-- railway.json
│-- README.md

```

## Backend Setup

1. Navigate to the `backend` directory.
2. Run `go mod tidy` to install dependencies.
3. Start the server by running `go run main.go`.

## Frontend Setup

1. Navigate to the `frontend` directory.
2. Run `npm install` to install dependencies.
3. Start the React application by running `npm start`.

## Usage

- The backend exposes RESTful API endpoints for user management.
- The frontend provides a user interface to interact with these endpoints.

## Contributing

Feel free to submit issues or pull requests for improvements and features.
