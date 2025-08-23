package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type Message struct {
	ID         primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	SenderID   string             `bson:"sender_id" json:"sender_id"`
	ReceiverID string             `bson:"receiver_id" json:"receiver_id"`
	Content    string             `bson:"content" json:"content"`
	Type       string             `bson:"type" json:"type"` // "text", "image", etc
	Read       bool               `bson:"read" json:"read"`
	CreatedAt  time.Time          `bson:"created_at" json:"created_at"`
}

type SendMessageRequest struct {
	ReceiverID string `json:"receiver_id" validate:"required"`
	Content    string `json:"content" validate:"required,max=1000"`
	Type       string `json:"type" validate:"oneof=text image"`
}

func (r *SendMessageRequest) Validate() []string {
	var errors []string

	if r.ReceiverID == "" {
		errors = append(errors, "Receiver ID is required")
	}

	if r.Content == "" {
		errors = append(errors, "Message content is required")
	}

	if len(r.Content) > 1000 {
		errors = append(errors, "Message too long (max 1000 characters)")
	}

	if r.Type == "" {
		r.Type = "text"
	}

	return errors
}
