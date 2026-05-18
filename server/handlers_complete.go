package server

import (
	"github.com/gofiber/fiber/v2"
	"github.com/sirupsen/logrus"

	"course-api/services"
)

func handleComplete(cs *services.CourseService) fiber.Handler {
	return func(c *fiber.Ctx) error {
		userID := c.Locals("user_id").(string)

		if err := cs.Complete(userID); err != nil {
			logrus.WithError(err).WithField("user_id", userID).Error("complete failed")
			return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{"error": "service unavailable"})
		}

		return c.JSON(fiber.Map{"ok": true})
	}
}
