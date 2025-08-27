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
	Clients     map[string]*Client
	Register    chan *Client
	Unregister  chan *Client
	Broadcast   chan models.Message
	Connections int
	mu          sync.RWMutex
}

var hub = &Hub{
	Clients:     make(map[string]*Client),
	Register:    make(chan *Client),
	Unregister:  make(chan *Client),
	Broadcast:   make(chan models.Message, 1000), // Buffer untuk broadcast
	Connections: 0,
}

func init() {
	go hub.run()
}

func (h *Hub) run() {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("Hub panic recovered: %v", r)
			// Restart hub
			go h.run()
		}
	}()

	for {
		select {
		case client := <-h.Register:
			h.mu.Lock()
			h.Clients[client.UserID] = client
			h.Connections++
			h.mu.Unlock()

			log.Printf("User %s connected. Total connections: %d", client.UserID, h.Connections)

			// Set user online dengan error handling
			go func(userID string) {
				_, err := config.DB.Collection("users").UpdateOne(context.Background(),
					bson.M{"_id": userID},
					bson.M{"$set": bson.M{"online": true, "last_seen": time.Now()}},
				)
				if err != nil {
					log.Printf("Failed to set user %s online: %v", userID, err)
				}
			}(client.UserID)

		case client := <-h.Unregister:
			h.mu.Lock()
			if _, ok := h.Clients[client.UserID]; ok {
				delete(h.Clients, client.UserID)
				h.Connections--
				close(client.Send)
				log.Printf("User %s disconnected. Total connections: %d", client.UserID, h.Connections)
			}
			h.mu.Unlock()

			// Set user offline dengan error handling
			go func(userID string) {
				_, err := config.DB.Collection("users").UpdateOne(context.Background(),
					bson.M{"_id": userID},
					bson.M{"$set": bson.M{"online": false, "last_seen": time.Now()}},
				)
				if err != nil {
					log.Printf("Failed to set user %s offline: %v", userID, err)
				}
			}(client.UserID)

		case message := <-h.Broadcast:
			h.mu.Lock()
			log.Printf("Processing broadcast message: %s -> %s", message.SenderID, message.ReceiverID)

			// Send to receiver
			if receiverClient, ok := h.Clients[message.ReceiverID]; ok {
				select {
				case receiverClient.Send <- message:
					log.Printf("Message sent to receiver: %s", message.ReceiverID)
				default:
					// Handle full channel
					delete(h.Clients, message.ReceiverID)
					close(receiverClient.Send)
					h.Connections--
					log.Printf("Receiver channel full, disconnected user: %s", message.ReceiverID)
				}
			} else {
				log.Printf("Receiver %s not connected", message.ReceiverID)
			}

			// Send to sender (for confirmation)
			if senderClient, ok := h.Clients[message.SenderID]; ok {
				select {
				case senderClient.Send <- message:
					log.Printf("Message confirmation sent to sender: %s", message.SenderID)
				default:
					delete(h.Clients, message.SenderID)
					close(senderClient.Send)
					h.Connections--
					log.Printf("Sender channel full, disconnected user: %s", message.SenderID)
				}
			} else {
				log.Printf("Sender %s not connected during broadcast", message.SenderID)
			}
			h.mu.Unlock()
		}
	}
}

func WebSocketChat(c *websocket.Conn) {
	// Get token from query param
	tokenStr := c.Query("token")
	if tokenStr == "" {
		log.Printf("WebSocket connection rejected: no token provided")
		c.Close()
		return
	}

	// Parse and validate token
	token, err := jwt.Parse(tokenStr, func(t *jwt.Token) (interface{}, error) {
		return []byte(os.Getenv("JWT_SECRET")), nil
	})

	if err != nil || !token.Valid {
		log.Printf("WebSocket connection rejected: invalid token - %v", err)
		c.Close()
		return
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		log.Printf("WebSocket connection rejected: invalid token claims")
		c.Close()
		return
	}

	userID, ok := claims["user_id"].(string)
	if !ok || userID == "" {
		log.Printf("WebSocket connection rejected: invalid user_id in token")
		c.Close()
		return
	}

	// Check if user already connected
	hub.mu.RLock()
	if existingClient, exists := hub.Clients[userID]; exists {
		log.Printf("User %s already connected, closing previous connection", userID)
		// Close previous connection
		existingClient.Conn.Close()
		close(existingClient.Send)
		delete(hub.Clients, userID)
	}
	hub.mu.RUnlock()

	// Create client dengan buffer yang lebih besar
	client := &Client{
		Conn:   c,
		UserID: userID,
		Send:   make(chan models.Message, 1024), // Increased buffer size
	}

	log.Printf("Registering user %s", userID)

	// Register client
	hub.Register <- client

	// Start goroutines
	go client.writePump()
	client.readPump() // readPump akan block sampai connection closed
}

func (c *Client) writePump() {
	ticker := time.NewTicker(30 * time.Second)
	defer func() {
		ticker.Stop()
		c.Conn.Close()
		log.Printf("Write pump stopped for user %s", c.UserID)
	}()

	for {
		select {
		case message, ok := <-c.Send:
			c.Conn.SetWriteDeadline(time.Now().Add(15 * time.Second))
			if !ok {
				// Channel closed
				c.Conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			if err := c.Conn.WriteJSON(message); err != nil {
				log.Printf("Write error for user %s: %v", c.UserID, err)
				return
			}

			log.Printf("Message written to websocket for user %s", c.UserID)

		case <-ticker.C:
			c.Conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := c.Conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				log.Printf("Ping error for user %s: %v", c.UserID, err)
				return
			}
			log.Printf("Ping sent to user %s", c.UserID)
		}
	}
}

func (c *Client) readPump() {
	defer func() {
		log.Printf("Read pump stopping for user %s", c.UserID)
		hub.Unregister <- c
		c.Conn.Close()
	}()

	c.Conn.SetReadLimit(512 * 1024) // Set read limit
	c.Conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	c.Conn.SetPongHandler(func(string) error {
		c.Conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		log.Printf("Pong received from user %s", c.UserID)
		return nil
	})

	for {
		var msgReq models.SendMessageRequest
		if err := c.Conn.ReadJSON(&msgReq); err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("WebSocket read error for user %s: %v", c.UserID, err)
			} else {
				log.Printf("WebSocket closed normally for user %s", c.UserID)
			}
			break
		}

		log.Printf("Message received from user %s: %s", c.UserID, msgReq.Content)

		// Validate message
		if validationErrors := msgReq.Validate(); len(validationErrors) > 0 {
			log.Printf("Message validation failed for user %s: %v", c.UserID, validationErrors)
			continue
		}

		// Prevent self-messaging
		if msgReq.ReceiverID == c.UserID {
			log.Printf("User %s attempted to send message to themselves", c.UserID)
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

		// Save to database dengan timeout
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		_, err := config.DB.Collection("messages").InsertOne(ctx, message)
		if err != nil {
			log.Printf("Failed to save message from user %s: %v", c.UserID, err)
			continue
		}

		log.Printf("Message saved to database: %s -> %s", c.UserID, msgReq.ReceiverID)

		// Update user's last seen
		go func(userID string) {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			_, err := config.DB.Collection("users").UpdateOne(ctx,
				bson.M{"_id": userID},
				bson.M{"$set": bson.M{"last_seen": time.Now()}},
			)
			if err != nil {
				log.Printf("Failed to update last_seen for user %s: %v", userID, err)
			}
		}(c.UserID)

		// Broadcast message
		select {
		case hub.Broadcast <- message:
			log.Printf("Message broadcast to hub: %s -> %s", message.SenderID, message.ReceiverID)
		case <-time.After(5 * time.Second):
			log.Printf("Broadcast channel full, message dropped: %s -> %s", message.SenderID, message.ReceiverID)
		}
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

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cursor, err := config.DB.Collection("messages").Find(ctx, filter, opts)
	if err != nil {
		log.Printf("Failed to fetch messages: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to fetch messages",
		})
	}
	defer cursor.Close(ctx)

	var messages []models.Message
	if err := cursor.All(ctx, &messages); err != nil {
		log.Printf("Failed to decode messages: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to decode messages",
		})
	}

	// Reverse to get chronological order
	for i := len(messages)/2 - 1; i >= 0; i-- {
		opp := len(messages) - 1 - i
		messages[i], messages[opp] = messages[opp], messages[i]
	}

	// Mark messages as read dengan goroutine
	go func(currentUserID, otherUserID string) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		result, err := config.DB.Collection("messages").UpdateMany(ctx,
			bson.M{
				"sender_id":   otherUserID,
				"receiver_id": currentUserID,
				"read":        false,
			},
			bson.M{"$set": bson.M{"read": true}},
		)

		if err != nil {
			log.Printf("Failed to mark messages as read: %v", err)
		} else {
			log.Printf("Marked %d messages as read", result.ModifiedCount)
		}
	}(currentUserID, otherUserID)

	return c.JSON(fiber.Map{
		"messages": messages,
		"pagination": fiber.Map{
			"page":  page,
			"limit": limit,
			"total": len(messages),
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
				"last_message": bson.M{"$first": "$$ROOT"},
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

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	cursor, err := config.DB.Collection("messages").Aggregate(ctx, pipeline)
	if err != nil {
		log.Printf("Failed to fetch conversations: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to fetch conversations",
		})
	}
	defer cursor.Close(ctx)

	var conversations []fiber.Map
	for cursor.Next(ctx) {
		var result struct {
			ID          string         `bson:"_id"`
			LastMessage models.Message `bson:"last_message"`
			UnreadCount int            `bson:"unread_count"`
		}

		if err := cursor.Decode(&result); err != nil {
			log.Printf("Failed to decode conversation: %v", err)
			continue
		}

		// Get user info
		var user models.User
		userCtx, userCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer userCancel()

		err := config.DB.Collection("users").FindOne(userCtx,
			bson.M{"_id": result.ID}).Decode(&user)
		if err != nil {
			log.Printf("Failed to find user %s: %v", result.ID, err)
			continue
		}

		conversations = append(conversations, fiber.Map{
			"user": fiber.Map{
				"id":        user.ID,
				"username":  user.Username,
				"avatar":    user.Avatar,
				"online":    user.Online,
				"last_seen": user.LastSeen,
			},
			"last_message": fiber.Map{
				"id":         result.LastMessage.ID,
				"content":    result.LastMessage.Content,
				"type":       result.LastMessage.Type,
				"created_at": result.LastMessage.CreatedAt,
				"sender_id":  result.LastMessage.SenderID,
				"read":       result.LastMessage.Read,
			},
			"unread_count": result.UnreadCount,
		})
	}

	if err := cursor.Err(); err != nil {
		log.Printf("Cursor error: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to process conversations",
		})
	}

	return c.JSON(fiber.Map{
		"conversations": conversations,
		"total":         len(conversations),
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

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Mark all messages from other user as read
	result, err := config.DB.Collection("messages").UpdateMany(ctx,
		bson.M{
			"sender_id":   otherUserID,
			"receiver_id": currentUserID,
			"read":        false,
		},
		bson.M{"$set": bson.M{"read": true}},
	)

	if err != nil {
		log.Printf("Failed to mark messages as read: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to mark messages as read",
		})
	}

	log.Printf("Marked %d messages as read from %s to %s", result.ModifiedCount, otherUserID, currentUserID)

	return c.JSON(fiber.Map{
		"message":          "Messages marked as read",
		"messages_updated": result.ModifiedCount,
	})
}

func GetUnreadCount(c *fiber.Ctx) error {
	currentUserID := c.Locals("user_id").(string)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	count, err := config.DB.Collection("messages").CountDocuments(ctx,
		bson.M{
			"receiver_id": currentUserID,
			"read":        false,
		},
	)

	if err != nil {
		log.Printf("Failed to get unread count: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to get unread count",
		})
	}

	return c.JSON(fiber.Map{
		"unread_count": count,
	})
}

// GetConnectionStatus untuk monitoring
func GetConnectionStatus(c *fiber.Ctx) error {
	hub.mu.RLock()
	defer hub.mu.RUnlock()

	connectedUsers := make([]string, 0, len(hub.Clients))
	for userID := range hub.Clients {
		connectedUsers = append(connectedUsers, userID)
	}

	return c.JSON(fiber.Map{
		"total_connections": hub.Connections,
		"connected_users":   connectedUsers,
		"timestamp":         time.Now(),
	})
}
