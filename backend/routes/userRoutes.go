package routes

import (
    "github.com/gin-gonic/gin"
    "zapier-clone/backend/controllers"
)

func SetupUserRoutes(router *gin.Engine) {
    userController := controllers.UserController{}

    router.POST("/users", userController.CreateUser)
    router.GET("/users/:id", userController.GetUser)
    router.PUT("/users/:id", userController.UpdateUser)
}