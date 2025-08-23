// controllers/chat.go
package controllers

import (
	"context"
	"log"
	"os"
	"sync"
	"time"

	"github.com/Adisonsmn/ngobrolyuk/config"
	"github.com/Adisonsmn/ngobrolyuk/models"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/websocket/v2"
	"github.com/golang-jwt/jwt/v5"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type Client struct {
	Conn   *websocket.Conn
	UserID string
	Send   chan models.Message
}

type Hub struct {
	Clients    map[string]*Client
	Register   chan *Client
	Unregister chan *Client
	Broadcast  chan models.Message
	mu         sync.RWMutex
}

var hub = &Hub{
	Clients:    make(map[string]*Client),
	Register:   make(chan *Client),
	Unregister: make(chan *Client),
	Broadcast:  make(chan models.Message),
}

func init() {
	go hub.run()
}

func (h *Hub) run() {
	for {
		select {
		case client := <-h.Register:
			h.mu.Lock()
			h.Clients[client.UserID] = client
			h.mu.Unlock()

			// Set user online
			config.DB.Collection("users").UpdateOne(context.Background(),
				bson.M{"_id": client.UserID},
				bson.M{"$set": bson.M{"online": true, "last_seen": time.Now()}},
			)

			log.Printf("User %s connected", client.UserID)

		case client := <-h.Unregister:
			h.mu.Lock()
			if _, ok := h.Clients[client.UserID]; ok {
				delete(h.Clients, client.UserID)
				close(client.Send)

				// Set user offline
				config.DB.Collection("users").UpdateOne(context.Background(),
					bson.M{"_id": client.UserID},
					bson.M{"$set": bson.M{"online": false, "last_seen": time.Now()}},
				)

				log.Printf("User %s disconnected", client.UserID)
			}
			h.mu.Unlock()

		case message := <-h.Broadcast:
			h.mu.RLock()
			// Send to receiver
			if receiverClient, ok := h.Clients[message.ReceiverID]; ok {
				select {
				case receiverClient.Send <- message:
				default:
					delete(h.Clients, message.ReceiverID)
					close(receiverClient.Send)
				}
			}

			// Send to sender (for confirmation)
			if senderClient, ok := h.Clients[message.SenderID]; ok {
				select {
				case senderClient.Send <- message:
				default:
					delete(h.Clients, message.SenderID)
					close(senderClient.Send)
				}
			}
			h.mu.RUnlock()
		}
	}
}

func WebSocketChat(c *websocket.Conn) {
	// Get token from query param
	tokenStr := c.Query("token")
	if tokenStr == "" {
		c.Close()
		return
	}

	// Parse and validate token
	token, err := jwt.Parse(tokenStr, func(t *jwt.Token) (interface{}, error) {
		return []byte(os.Getenv("JWT_SECRET")), nil
	})

	if err != nil || !token.Valid {
		c.Close()
		return
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		c.Close()
		return
	}

	userID, ok := claims["user_id"].(string)
	if !ok || userID == "" {
		c.Close()
		return
	}

	// Create client
	client := &Client{
		Conn:   c,
		UserID: userID,
		Send:   make(chan models.Message, 256),
	}

	// Register client
	hub.Register <- client

	// Start goroutines
	go client.writePump()
	go client.readPump()
}

func (c *Client) writePump() {
	ticker := time.NewTicker(54 * time.Second)
	defer func() {
		ticker.Stop()
		c.Conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.Send:
			c.Conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if !ok {
				c.Conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			if err := c.Conn.WriteJSON(message); err != nil {
				return
			}

		case <-ticker.C:
			c.Conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := c.Conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

func (c *Client) readPump() {
	defer func() {
		hub.Unregister <- c
		c.Conn.Close()
	}()

	c.Conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	c.Conn.SetPongHandler(func(string) error {
		c.Conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	for {
		var msgReq models.SendMessageRequest
		if err := c.Conn.ReadJSON(&msgReq); err != nil {
			break
		}

		// Validate message
		if validationErrors := msgReq.Validate(); len(validationErrors) > 0 {
			continue
		}

		// Prevent self-messaging
		if msgReq.ReceiverID == c.UserID {
			continue
		}

		// Create message
		message := models.Message{
			ID:         primitive.NewObjectID(),
			SenderID:   c.UserID,
			ReceiverID: msgReq.ReceiverID,
			Content:    msgReq.Content,
			Type:       msgReq.Type,
			Read:       false,
			CreatedAt:  time.Now(),
		}

		// Save to database
		_, err := config.DB.Collection("messages").InsertOne(context.Background(), message)
		if err != nil {
			log.Printf("Failed to save message: %v", err)
			continue
		}

		// Update user's last seen
		config.DB.Collection("users").UpdateOne(context.Background(),
			bson.M{"_id": c.UserID},
			bson.M{"$set": bson.M{"last_seen": time.Now()}},
		)

		// Broadcast message
		hub.Broadcast <- message
	}
}

func GetMessages(c *fiber.Ctx) error {
	currentUserID := c.Locals("user_id").(string)
	otherUserID := c.Query("user_id")
	page := c.QueryInt("page", 1)
	limit := c.QueryInt("limit", 50)

	if otherUserID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "user_id parameter is required",
		})
	}

	if limit > 100 {
		limit = 100
	}

	skip := (page - 1) * limit

	// Find messages between users
	filter := bson.M{
		"$or": []bson.M{
			{"sender_id": currentUserID, "receiver_id": otherUserID},
			{"sender_id": otherUserID, "receiver_id": currentUserID},
		},
	}

	opts := options.Find().
		SetSort(bson.M{"created_at": -1}).
		SetSkip(int64(skip)).
		SetLimit(int64(limit))

	cursor, err := config.DB.Collection("messages").Find(context.Background(), filter, opts)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to fetch messages",
		})
	}
	defer cursor.Close(context.Background())

	var messages []models.Message
	if err := cursor.All(context.Background(), &messages); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to decode messages",
		})
	}

	// Reverse to get chronological order
	for i := len(messages)/2 - 1; i >= 0; i-- {
		opp := len(messages) - 1 - i
		messages[i], messages[opp] = messages[opp], messages[i]
	}

	// Mark messages as read
	go func() {
		config.DB.Collection("messages").UpdateMany(context.Background(),
			bson.M{
				"sender_id":   otherUserID,
				"receiver_id": currentUserID,
				"read":        false,
			},
			bson.M{"$set": bson.M{"read": true}},
		)
	}()

	return c.JSON(fiber.Map{
		"messages": messages,
		"pagination": fiber.Map{
			"page":  page,
			"limit": limit,
		},
	})
}

func GetConversations(c *fiber.Ctx) error {
	currentUserID := c.Locals("user_id").(string)

	// Aggregation pipeline to get latest message for each conversation
	pipeline := []bson.M{
		{
			"$match": bson.M{
				"$or": []bson.M{
					{"sender_id": currentUserID},
					{"receiver_id": currentUserID},
				},
			},
		},
		{
			"$sort": bson.M{"created_at": -1},
		},
		{
			"$group": bson.M{
				"_id": bson.M{
					"$cond": []interface{}{
						bson.M{"$eq": []interface{}{"$sender_id", currentUserID}},
						"$receiver_id",
						"$sender_id",
					},
				},
				"last_message": bson.M{"$first": "$ROOT"},
				"unread_count": bson.M{
					"$sum": bson.M{
						"$cond": []interface{}{
							bson.M{
								"$and": []bson.M{
									{"$eq": []interface{}{"$receiver_id", currentUserID}},
									{"$eq": []interface{}{"$read", false}},
								},
							},
							1,
							0,
						},
					},
				},
			},
		},
		{
			"$sort": bson.M{"last_message.created_at": -1},
		},
	}

	cursor, err := config.DB.Collection("messages").Aggregate(context.Background(), pipeline)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to fetch conversations",
		})
	}
	defer cursor.Close(context.Background())

	var conversations []fiber.Map
	for cursor.Next(context.Background()) {
		var result struct {
			ID          string         `bson:"_id"`
			LastMessage models.Message `bson:"last_message"`
			UnreadCount int            `bson:"unread_count"`
		}

		if err := cursor.Decode(&result); err != nil {
			continue
		}

		// Get user info
		var user models.User
		config.DB.Collection("users").FindOne(context.Background(),
			bson.M{"_id": result.ID}).Decode(&user)

		conversations = append(conversations, fiber.Map{
			"user": fiber.Map{
				"id":       user.ID,
				"username": user.Username,
				"avatar":   user.Avatar,
				"online":   user.Online,
			},
			"last_message": fiber.Map{
				"content":    result.LastMessage.Content,
				"created_at": result.LastMessage.CreatedAt,
				"sender_id":  result.LastMessage.SenderID,
			},
			"unread_count": result.UnreadCount,
		})
	}

	return c.JSON(fiber.Map{
		"conversations": conversations,
	})
}

func MarkMessagesRead(c *fiber.Ctx) error {
	currentUserID := c.Locals("user_id").(string)
	otherUserID := c.Params("user_id")

	if otherUserID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "user_id parameter is required",
		})
	}

	// Mark all messages from other user as read
	result, err := config.DB.Collection("messages").UpdateMany(context.Background(),
		bson.M{
			"sender_id":   otherUserID,
			"receiver_id": currentUserID,
			"read":        false,
		},
		bson.M{"$set": bson.M{"read": true}},
	)

	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to mark messages as read",
		})
	}

	return c.JSON(fiber.Map{
		"message":          "Messages marked as read",
		"messages_updated": result.ModifiedCount,
	})
}

func GetUnreadCount(c *fiber.Ctx) error {
	currentUserID := c.Locals("user_id").(string)

	count, err := config.DB.Collection("messages").CountDocuments(context.Background(),
		bson.M{
			"receiver_id": currentUserID,
			"read":        false,
		},
	)

	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to get unread count",
		})
	}

	return c.JSON(fiber.Map{
		"unread_count": count,
	})
}
