package auth

import (
	"github.com/gofiber/fiber/v2"
)

func Setup(app *fiber.App) {

	app.Post("/api/register", auth.Register)
	app.Post("/api/login", auth.Login)
	app.Get("/api/user", auth.User)
	app.Post("/api/logout", auth.Logout)
	app.Post("/api/google-login", auth.GoogleLogin)
	app.Post("/api/google-register", auth.GoogleRegister)

}
