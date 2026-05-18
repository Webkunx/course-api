package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/compress"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/sirupsen/logrus"

	"course-api/config"
	"course-api/db"
	"course-api/repository"
	"course-api/server"
	"course-api/services"
)

func main() {
	v := config.CreateViper()

	database := db.Open(v.GetString(config.DB_URL), v)
	defer database.Close()

	db.Migrate(database, v.GetString(config.MIGRATIONS_DIR))

	repo := repository.New(database)
	cs := services.NewCourseService(repo, v)

	app := fiber.New()
	app.Use(recover.New(recover.Config{
		EnableStackTrace: true,
		StackTraceHandler: func(c *fiber.Ctx, e interface{}) {
			logrus.WithField("panic", e).Error("recovered from panic")
		},
	}))
	// Negotiates gzip/deflate/brotli with the client. Exercise payloads
	// (~4 KB JSON) compress ~4x; this cuts free-tier egress and dominates
	// the latency budget on small dynos.
	app.Use(compress.New(compress.Config{
		Level: compress.LevelDefault,
	}))
	app.Use(logger.New(logger.Config{
		Format: "[${time}] ${status} ${method} ${path} ${latency}\n",
	}))

	server.AddRoutes(app, cs, repo)

	go func() {
		if err := app.Listen(":" + v.GetString(config.PORT)); err != nil {
			logrus.WithError(err).Panic("server listen failed")
		}
	}()

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	<-c

	fmt.Println("Gracefully shutting down...")
	time.Sleep(5 * time.Second)
	_ = app.Shutdown()
}
