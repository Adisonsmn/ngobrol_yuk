package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/Adisonsmn/ngobrolyuk/config"
	"github.com/Adisonsmn/ngobrolyuk/routes"
	"github.com/gofiber/fiber/v2"
)

func main() {
	// Connect to database
	config.ConnectDB()
	defer config.DisconnectDB()

	// Create Fiber app
	app := fiber.New(fiber.Config{
		ErrorHandler: func(c *fiber.Ctx, err error) error {
			code := fiber.StatusInternalServerError
			message := "Internal Server Error"

			// Check if it's a Fiber error
			if e, ok := err.(*fiber.Error); ok {
				code = e.Code
				message = e.Message
			}

			// Log error for debugging
			log.Printf("Error: %v", err)

			return c.Status(code).JSON(fiber.Map{
				"error": message,
			})
		},
		ServerHeader: "NgobrolYuk API",
		AppName:      "NgobrolYuk v1.0",
	})

	// Setup routes
	routes.SetupRoutes(app)

	// Get port from environment
	port := config.GetEnvWithDefault("PORT", "8080")

	// Setup graceful shutdown
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-c
		log.Println("Shutting down server...")
		app.Shutdown()
	}()

	// Start server
	log.Printf("Server starting on port %s", port)
	if err := app.Listen(":" + port); err != nil {
		log.Fatal("Server failed to start:", err)
	}
}
