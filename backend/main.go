package main

import (
	"log"
	"net/http"
	"zapier-clone/backend/database"
	"zapier-clone/backend/routes"
    "github.com/gin-gonic/gin"
)

func main() {
    // Connect to the PostgreSQL database
    if err := database.Connect(); err != nil {
        log.Fatalf("Could not connect to the database: %v", err)
    }

    // Set up routes
    router := gin.Default()
    routes.SetupUserRoutes(router)

    // Start the server
    log.Println("Starting server on :8080")
    if err := http.ListenAndServe(":8080", router); err != nil {
        log.Fatalf("Could not start server: %v", err)
    }
}