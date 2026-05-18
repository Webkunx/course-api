package server

import (
	"github.com/gofiber/fiber/v2"

	"course-api/services"
)

func handleHealth(cs *services.CourseService) fiber.Handler {
	return func(c *fiber.Ctx) error {
		scale, seed := cs.GetMeta()
		return c.JSON(fiber.Map{
			"ok":    true,
			"scale": scale,
			"seed":  seed,
		})
	}
}
