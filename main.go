package main

import (
	"context"
	"log"
	"os"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
)

func main() {
	_ = godotenv.Load()
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		log.Fatal("DATABASE_URL is required")
	}

	ctx := context.Background()
	poolConfig, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		log.Fatalf("parse database config: %v", err)
	}
	poolConfig.MaxConns = 10
	poolConfig.MinConns = 1
	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		log.Fatalf("connect to database: %v", err)
	}
	defer pool.Close()

	pingCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if err := pool.Ping(pingCtx); err != nil {
		log.Fatalf("ping database: %v", err)
	}

	handler := NewTaskHandler(NewTaskRepository(pool), os.Getenv("SEED_FILE_PATH"))
	app := fiber.New(fiber.Config{ErrorHandler: ErrorHandler})
	app.Get("/health", func(c *fiber.Ctx) error { return c.JSON(fiber.Map{"status": "ok"}) })

	api := app.Group("/api/v1/tasks")
	api.Get("/", handler.List)
	api.Get("/:id", handler.Get)
	api.Post("/", handler.Create)
	api.Put("/:id", handler.Replace)
	api.Patch("/:id", handler.Update)
	api.Delete("/:id", handler.Delete)
	api.Post("/seed", handler.Seed)

	port := os.Getenv("PORT")
	if port == "" {
		port = "3000"
	}
	log.Printf("API listening on :%s", port)
	log.Fatal(app.Listen(":" + port))
}

func ErrorHandler(c *fiber.Ctx, err error) error {
	code := fiber.StatusInternalServerError
	if e, ok := err.(*fiber.Error); ok {
		code = e.Code
	}
	return c.Status(code).JSON(fiber.Map{"error": fiber.Map{"code": code, "message": err.Error()}})
}
