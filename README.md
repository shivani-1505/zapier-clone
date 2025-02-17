# Zapier Clone

This project is a clone of Zapier built using GoLang for the backend and React for the frontend, with PostgreSQL as the database.

## Project Structure

```
zapier-clone
├── backend
│   ├── main.go
│   ├── controllers
│   │   └── userController.go
│   ├── models
│   │   └── user.go
│   ├── routes
│   │   └── userRoutes.go
│   ├── database
│   │   └── connection.go
│   └── go.mod
├── frontend
│   ├── src
│   │   ├── App.js
│   │   ├── index.js
│   │   ├── components
│   │   │   └── UserComponent.js
│   │   └── services
│   │       └── userService.js
│   ├── package.json
│   └── public
│       └── index.html
└── README.md
```

## Backend Setup

1. Navigate to the `backend` directory.
2. Run `go mod tidy` to install dependencies.
3. Set up your PostgreSQL database and update the connection settings in `backend/database/connection.go`.
4. Start the server by running `go run main.go`.

## Frontend Setup

1. Navigate to the `frontend` directory.
2. Run `npm install` to install dependencies.
3. Start the React application by running `npm start`.

## Usage

- The backend exposes RESTful API endpoints for user management.
- The frontend provides a user interface to interact with these endpoints.

## Contributing

Feel free to submit issues or pull requests for improvements and features.