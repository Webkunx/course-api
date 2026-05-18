package server

import (
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"

	"course-api/repository"
)

func authMiddleware(repo repository.Repository) fiber.Handler {
	return func(c *fiber.Ctx) error {
		auth := c.Get("Authorization")
		token, ok := strings.CutPrefix(auth, "Bearer ")
		if !ok || token == "" {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "missing authorization"})
		}

		parsed, err := uuid.Parse(token)
		if err != nil || parsed.Version() != 4 {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "invalid token format"})
		}

		exists, err := repo.UserExists(token)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "internal error"})
		}
		if !exists {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "unauthorized"})
		}

		c.Locals("user_id", token)
		return c.Next()
	}
}
