package main

import (
	"go-auth-main/database"
	"go-auth-main/routes"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
)

func main() {
	database.Connect()
	app := fiber.New()

	app.Use(cors.New(cors.Config{
		AllowOrigins:     "http://localhost:3000",
		AllowCredentials: true,
	}))
	routes.Setup(app)

	app.Listen(":8000")
}
