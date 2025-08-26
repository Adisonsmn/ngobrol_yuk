// controllers/user.go
package controllers

import (
	"context"
	"time"

	"github.com/Adisonsmn/ngobrolyuk/config"
	"github.com/Adisonsmn/ngobrolyuk/models"
	"github.com/gofiber/fiber/v2"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func GetProfile(c *fiber.Ctx) error {
	userID := c.Locals("user_id").(string)

	var user models.User
	err := config.DB.Collection("users").FindOne(context.Background(),
		bson.M{"_id": userID}).Decode(&user)

	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "User not found",
		})
	}

	return c.JSON(fiber.Map{
		"id":         user.ID,
		"username":   user.Username,
		"email":      user.Email,
		"bio":        user.Bio,
		"avatar":     user.Avatar,
		"online":     user.Online,
		"last_seen":  user.LastSeen,
		"created_at": user.CreatedAt,
	})
}

func UpdateProfile(c *fiber.Ctx) error {
	userID := c.Locals("user_id").(string)

	var input models.UpdateProfileRequest
	if err := c.BodyParser(&input); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request format",
		})
	}

	// Build update document
	updateDoc := bson.M{}

	if input.Username != "" {
		// Check if username is already taken by another user
		count, _ := config.DB.Collection("users").CountDocuments(context.Background(),
			bson.M{
				"username": input.Username,
				"_id":      bson.M{"$ne": userID},
			})

		if count > 0 {
			return c.Status(fiber.StatusConflict).JSON(fiber.Map{
				"error": "Username already taken",
			})
		}

		updateDoc["username"] = input.Username
	}

	if input.Bio != "" {
		if len(input.Bio) > 500 {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "Bio too long (max 500 characters)",
			})
		}
		updateDoc["bio"] = input.Bio
	}

	if input.Avatar != "" {
		updateDoc["avatar"] = input.Avatar
	}

	if len(updateDoc) == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "No fields to update",
		})
	}

	// Update user
	_, err := config.DB.Collection("users").UpdateOne(context.Background(),
		bson.M{"_id": userID},
		bson.M{"$set": updateDoc},
	)

	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to update profile",
		})
	}

	return c.JSON(fiber.Map{
		"message": "Profile updated successfully",
	})
}
func ListUsers(c *fiber.Ctx) error {
	userID, ok := c.Locals("user_id").(string)
	if !ok || userID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid user id",
		})
	}

	// Query parameters
	online := c.Query("online")
	search := c.Query("search")
	page := c.QueryInt("page", 1)
	limit := c.QueryInt("limit", 20)

	if limit > 100 {
		limit = 100 // Prevent large queries
	}
	skip := (page - 1) * limit

	// Build filter
	filter := bson.M{}

	// Exclude current user
	if objID, err := primitive.ObjectIDFromHex(userID); err == nil {
		filter["_id"] = bson.M{"$ne": objID}
	} else {
		filter["_id"] = bson.M{"$ne": userID}
	}

	if online == "true" {
		filter["online"] = true
	}
	if search != "" {
		filter["$or"] = []bson.M{
			{"username": bson.M{"$regex": search, "$options": "i"}},
			{"email": bson.M{"$regex": search, "$options": "i"}},
		}
	}

	// ✅ Use bson.D for sorting (ordered)
	opts := options.Find().
		SetSkip(int64(skip)).
		SetLimit(int64(limit)).
		SetSort(bson.D{
			{Key: "online", Value: -1},
			{Key: "last_seen", Value: -1},
		})

	cursor, err := config.DB.Collection("users").Find(context.Background(), filter, opts)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error":   "Failed to fetch users",
			"details": err.Error(),
		})
	}
	defer cursor.Close(context.Background())

	var users []fiber.Map
	for cursor.Next(context.Background()) {
		var raw bson.M
		if err := cursor.Decode(&raw); err != nil {
			continue
		}

		// Convert _id ke string
		id := ""
		if oid, ok := raw["_id"].(primitive.ObjectID); ok {
			id = oid.Hex()
		} else if str, ok := raw["_id"].(string); ok {
			id = str
		}

		users = append(users, fiber.Map{
			"id":        id,
			"username":  raw["username"],
			"bio":       raw["bio"],
			"avatar":    raw["avatar"],
			"online":    raw["online"],
			"last_seen": raw["last_seen"],
		})
	}

	// Total count
	total, _ := config.DB.Collection("users").CountDocuments(context.Background(), filter)

	return c.JSON(fiber.Map{
		"users": users,
		"pagination": fiber.Map{
			"page":        page,
			"limit":       limit,
			"total":       total,
			"total_pages": (total + int64(limit) - 1) / int64(limit),
		},
	})
}

func GetUserProfile(c *fiber.Ctx) error {
	userID := c.Params("id")

	var user models.User
	err := config.DB.Collection("users").FindOne(context.Background(),
		bson.M{"_id": userID}).Decode(&user)

	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "User not found",
		})
	}

	return c.JSON(fiber.Map{
		"id":        user.ID,
		"username":  user.Username,
		"bio":       user.Bio,
		"avatar":    user.Avatar,
		"online":    user.Online,
		"last_seen": user.LastSeen,
	})
}

func GetOnlineUsers(c *fiber.Ctx) error {
	currentUserID := c.Locals("user_id").(string)

	// Get users that are online and active within last 5 minutes
	filter := bson.M{
		"_id":    bson.M{"$ne": currentUserID},
		"online": true,
		"last_seen": bson.M{
			"$gte": time.Now().Add(-5 * time.Minute),
		},
	}

	cursor, err := config.DB.Collection("users").Find(context.Background(), filter)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to fetch online users",
		})
	}
	defer cursor.Close(context.Background())

	var users []fiber.Map
	for cursor.Next(context.Background()) {
		var user models.User
		if err := cursor.Decode(&user); err != nil {
			continue
		}

		users = append(users, fiber.Map{
			"id":       user.ID,
			"username": user.Username,
			"avatar":   user.Avatar,
		})
	}

	return c.JSON(fiber.Map{
		"online_users": users,
		"count":        len(users),
	})
}
