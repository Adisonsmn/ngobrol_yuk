package controllers

import (
	"context"
	"os"
	"strings"
	"time"

	"github.com/Adisonsmn/ngobrolyuk/config"
	"github.com/Adisonsmn/ngobrolyuk/models"
	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"golang.org/x/crypto/bcrypt"
)

func Register(c *fiber.Ctx) error {
	var input models.RegisterRequest
	if err := c.BodyParser(&input); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request format",
		})
	}

	// Validate input
	if validationErrors := input.Validate(); len(validationErrors) > 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":  "Validation failed",
			"errors": validationErrors,
		})
	}

	// Normalize email
	input.Email = strings.ToLower(strings.TrimSpace(input.Email))
	input.Username = strings.TrimSpace(input.Username)

	// Check if email or username already exists
	var existingUser models.User
	err := config.DB.Collection("users").FindOne(context.Background(), bson.M{
		"$or": []bson.M{
			{"email": input.Email},
			{"username": input.Username},
		},
	}).Decode(&existingUser)

	if err == nil {
		if existingUser.Email == input.Email {
			return c.Status(fiber.StatusConflict).JSON(fiber.Map{
				"error": "Email already registered",
			})
		}
		if existingUser.Username == input.Username {
			return c.Status(fiber.StatusConflict).JSON(fiber.Map{
				"error": "Username already taken",
			})
		}
	} else if err != mongo.ErrNoDocuments {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Database error",
		})
	}

	// Hash password with higher cost for production
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(input.Password), 14)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to process password",
		})
	}

	// Create user
	user := models.User{
		ID:        config.GetNextUserID(),
		Username:  input.Username,
		Email:     input.Email,
		Password:  string(hashedPassword),
		Online:    false, // Set online via websocket
		LastSeen:  time.Now(),
		CreatedAt: time.Now(),
		Bio:       "",
		Avatar:    "",
	}

	_, err = config.DB.Collection("users").InsertOne(context.Background(), user)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to create user",
		})
	}

	// Generate JWT token
	token, err := generateJWT(user.ID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to generate token",
		})
	}

	// Set HTTP-only cookie
	setJWTCookie(c, token)

	// Return user info (without password)
	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"message": "Registration successful",
		"user": fiber.Map{
			"id":       user.ID,
			"username": user.Username,
			"email":    user.Email,
			"bio":      user.Bio,
			"avatar":   user.Avatar,
		},
	})
}

func Login(c *fiber.Ctx) error {
	var input models.LoginRequest
	if err := c.BodyParser(&input); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request format",
		})
	}

	// Basic validation
	if input.Email == "" || input.Password == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Email and password are required",
		})
	}

	// Normalize email
	input.Email = strings.ToLower(strings.TrimSpace(input.Email))

	// Find user
	var user models.User
	err := config.DB.Collection("users").FindOne(context.Background(),
		bson.M{"email": input.Email}).Decode(&user)

	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Invalid email or password",
		})
	}

	// Compare password
	if err = bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(input.Password)); err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Invalid email or password",
		})
	}

	// Update last seen
	config.DB.Collection("users").UpdateOne(context.Background(),
		bson.M{"_id": user.ID},
		bson.M{"$set": bson.M{"last_seen": time.Now()}},
	)

	// Generate JWT token
	token, err := generateJWT(user.ID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to generate token",
		})
	}

	// Set HTTP-only cookie
	setJWTCookie(c, token)

	// Return user info
	return c.JSON(fiber.Map{
		"message": "Login successful",
		"user": fiber.Map{
			"id":       user.ID,
			"username": user.Username,
			"email":    user.Email,
			"bio":      user.Bio,
			"avatar":   user.Avatar,
		},
	})
}

func Logout(c *fiber.Ctx) error {
	userID := c.Locals("user_id").(string)

	// Set user offline
	_, err := config.DB.Collection("users").UpdateOne(
		context.Background(),
		bson.M{"_id": userID},
		bson.M{"$set": bson.M{"online": false, "last_seen": time.Now()}},
	)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "failed to update user status",
		})
	}

	// Clear cookie dengan cara overwrite dan expired
	c.Cookie(&fiber.Cookie{
		Name:     "jwt",
		Value:    "",
		Expires:  time.Now().Add(-time.Hour), // expired
		HTTPOnly: true,
		Secure:   true, // sesuaikan dengan waktu set cookie
		SameSite: "Strict",
		Path:     "/", // harus sama seperti saat set cookie
	})

	return c.JSON(fiber.Map{
		"message": "Logged out successfully",
	})
}

func RefreshToken(c *fiber.Ctx) error {
	userID := c.Locals("user_id").(string)

	// Generate new token
	token, err := generateJWT(userID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to refresh token",
		})
	}

	// Set new cookie
	setJWTCookie(c, token)

	return c.JSON(fiber.Map{
		"message": "Token refreshed successfully",
	})
}

// Helper functions
func generateJWT(userID string) (string, error) {
	claims := jwt.MapClaims{
		"user_id": userID,
		"exp":     time.Now().Add(time.Hour * 72).Unix(),
		"iat":     time.Now().Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(os.Getenv("JWT_SECRET")))
}

func setJWTCookie(c *fiber.Ctx, token string) {
	isSecure := os.Getenv("ENVIRONMENT") == "production"

	c.Cookie(&fiber.Cookie{
		Name:     "jwt",
		Value:    token,
		Expires:  time.Now().Add(time.Hour * 72),
		HTTPOnly: true,
		Secure:   isSecure,
		SameSite: "Strict",
		Path:     "/",
	})
}
