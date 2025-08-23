package middleware

import (
	"errors"
	"os"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"
)

func Protect(c *fiber.Ctx) error {
	// Get token from cookie
	tokenStr := c.Cookies("jwt")

	// If no cookie, try Authorization header
	if tokenStr == "" {
		authHeader := c.Get("Authorization")
		if authHeader != "" && len(authHeader) > 7 && authHeader[:7] == "Bearer " {
			tokenStr = authHeader[7:]
		}
	}

	if tokenStr == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Missing authentication token",
		})
	}

	// Parse and validate token
	token, err := jwt.Parse(tokenStr, func(t *jwt.Token) (interface{}, error) {
		// Validate signing method
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("invalid signing method")
		}
		return []byte(os.Getenv("JWT_SECRET")), nil
	})

	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "Token expired",
			})
		}
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Invalid token",
		})
	}

	if !token.Valid {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Invalid token",
		})
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Invalid token claims",
		})
	}

	// Validate required claims
	userID, ok := claims["user_id"].(string)
	if !ok || userID == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Invalid user ID in token",
		})
	}

	// Check expiration
	exp, ok := claims["exp"].(float64)
	if !ok || time.Now().Unix() > int64(exp) {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Token expired",
		})
	}

	// Store user info in context
	c.Locals("user_id", userID)
	c.Locals("jwt_exp", exp)

	return c.Next()
}

// Rate limiting middleware for WebSocket connections
func WebSocketRateLimit() fiber.Handler {
	connections := make(map[string]int)

	return func(c *fiber.Ctx) error {
		ip := c.IP()

		if connections[ip] >= 3 { // Max 3 connections per IP
			return c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{
				"error": "Too many WebSocket connections from this IP",
			})
		}

		connections[ip]++

		// Clean up on disconnect (this is simplified)
		defer func() {
			connections[ip]--
			if connections[ip] <= 0 {
				delete(connections, ip)
			}
		}()

		return c.Next()
	}
}
