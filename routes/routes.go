package routes

import (
	"time"

	"github.com/Adisonsmn/ngobrolyuk/controllers"
	"github.com/Adisonsmn/ngobrolyuk/middleware"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/limiter"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/gofiber/websocket/v2"
)

func SetupRoutes(app *fiber.App) {
	// Global middleware
	app.Use(recover.New(recover.Config{
		EnableStackTrace: true,
	}))

	app.Use(logger.New(logger.Config{
		Format: "[${time}] ${status} - ${method} ${path} - ${latency}\n",
	}))

	// CORS configuration
	app.Use(cors.New(cors.Config{
		AllowOrigins:     "http://localhost:3000,http://localhost:5173", // Add your frontend URLs
		AllowCredentials: true,
		AllowMethods:     "GET,POST,PUT,DELETE,OPTIONS",
		AllowHeaders:     "Origin,Content-Type,Accept,Authorization,X-Requested-With",
	}))

	// Rate limiting for auth endpoints
	authLimiter := limiter.New(limiter.Config{
		Max:        15,
		Expiration: 15 * time.Minute,
		KeyGenerator: func(c *fiber.Ctx) string {
			return c.IP()
		},
		LimitReached: func(c *fiber.Ctx) error {
			return c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{
				"error": "Too many requests, please try again later",
			})
		},
	})

	// API routes
	api := app.Group("/api/v1")

	// Health check
	api.Get("/health", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"status":    "ok",
			"timestamp": time.Now().Unix(),
		})
	})

	// Public routes (with rate limiting)
	auth := api.Group("/auth")
	auth.Use(authLimiter)
	auth.Post("/register", controllers.Register)
	auth.Post("/login", controllers.Login)

	// Protected routes
	protected := api.Group("/", middleware.Protect)

	// Auth protected routes
	protected.Post("/auth/logout", controllers.Logout)
	protected.Post("/auth/refresh", controllers.RefreshToken)

	// User routes
	users := protected.Group("/users")
	users.Get("/", controllers.ListUsers)            // List users with filters
	users.Get("/online", controllers.GetOnlineUsers) // Get online users
	users.Get("/profile", controllers.GetProfile)    // Get own profile
	users.Put("/profile", controllers.UpdateProfile) // Update own profile
	users.Get("/:id", controllers.GetUserProfile)    // Get specific user profile

	// Chat routes
	chat := protected.Group("/chat")
	chat.Get("/messages", controllers.GetMessages)           // Get messages with user
	chat.Get("/conversations", controllers.GetConversations) // Get all conversations
	chat.Put("/read/:user_id", controllers.MarkMessagesRead) // Mark messages as read
	chat.Get("/unread", controllers.GetUnreadCount)          // Get unread count

	// WebSocket route (token in query param)
	app.Get("/ws", websocket.New(controllers.WebSocketChat))

	// 404 handler
	app.Use(func(c *fiber.Ctx) error {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Route not found",
		})
	})
}
