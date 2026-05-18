package server

import (
	"github.com/gofiber/fiber/v2"

	"course-api/repository"
	"course-api/services"
)

func AddRoutes(app *fiber.App, cs *services.CourseService, repo repository.Repository) {
	app.Get("/health", handleHealth(cs))
	app.Post("/register", handleRegister(repo))

	auth := authMiddleware(repo)
	app.Post("/next", auth, handleNext(cs))
	app.Post("/complete", auth, handleComplete(cs))
}
