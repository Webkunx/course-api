package server

import (
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"

	"course-api/repository"
)

func handleRegister(repo repository.Repository) fiber.Handler {
	return func(c *fiber.Ctx) error {
		token := uuid.New().String()
		if err := repo.CreateUser(token, time.Now().Unix()); err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "registration failed"})
		}
		return c.Status(fiber.StatusCreated).JSON(fiber.Map{"access_token": token})
	}
}
