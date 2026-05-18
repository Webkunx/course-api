package server

import (
	"github.com/gofiber/fiber/v2"
	"github.com/sirupsen/logrus"

	"course-api/services"
)

func handleNext(cs *services.CourseService) fiber.Handler {
	return func(c *fiber.Ctx) error {
		userID := c.Locals("user_id").(string)

		res, err := cs.Next(userID)
		if err != nil {
			logrus.WithError(err).WithField("user_id", userID).Error("next failed")
			return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{"error": "service unavailable"})
		}

		if res.CourseComplete {
			return c.JSON(fiber.Map{"course_complete": true})
		}
		return c.JSON(res.Exercise)
	}
}
