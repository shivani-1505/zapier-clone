package routes

import (
	"go-auth-main/controllers"

	"github.com/gofiber/fiber/v2"
)

func Setup(app *fiber.App) {

	app.Post("/api/register", controllers.Register)
	app.Post("/api/login", controllers.Login)
	app.Get("/api/user", controllers.User)
	app.Post("/api/logout", controllers.Logout)
	app.Post("/api/google-login", controllers.GoogleLogin)
	app.Post("/api/google-register", controllers.GoogleRegister)

}
