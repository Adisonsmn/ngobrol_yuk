package config

import (
	"context"
	"log"
	"os"
	"time"

	"github.com/joho/godotenv"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var DB *mongo.Database
var Client *mongo.Client

func ConnectDB() {
	// Load environment variables
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, using system environment variables")
	}

	// Get MongoDB URI
	mongoURI := os.Getenv("MONGO_URI")
	if mongoURI == "" {
		log.Fatal("MONGO_URI environment variable is required")
	}

	// Create client options
	clientOptions := options.Client().
		ApplyURI(mongoURI).
		SetMaxPoolSize(20).
		SetMinPoolSize(5).
		SetMaxConnIdleTime(30 * time.Second).
		SetServerSelectionTimeout(5 * time.Second)

	// Create client
	client, err := mongo.NewClient(clientOptions)
	if err != nil {
		log.Fatal("Failed to create MongoDB client:", err)
	}

	// Connect with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := client.Connect(ctx); err != nil {
		log.Fatal("Failed to connect to MongoDB:", err)
	}

	// Ping database
	if err := client.Ping(ctx, nil); err != nil {
		log.Fatal("Failed to ping MongoDB:", err)
	}

	// Set global variables
	Client = client
	DB = client.Database("ngobrolyuk")

	// Create indexes
	createIndexes(DB)

	log.Println("Successfully connected to MongoDB")
}

func DisconnectDB() {
	if Client != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := Client.Disconnect(ctx); err != nil {
			log.Printf("Error disconnecting from MongoDB: %v", err)
		} else {
			log.Println("Disconnected from MongoDB")
		}
	}
}
func createIndexes(db *mongo.Database) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	userCollection := db.Collection("users")
	messageCollection := db.Collection("messages")

	// ✅ Indexes untuk users
	userIndexes := []mongo.IndexModel{
		{
			Keys:    bson.D{{Key: "username", Value: 1}},
			Options: options.Index().SetUnique(true),
		},
		{
			Keys:    bson.D{{Key: "email", Value: 1}},
			Options: options.Index().SetUnique(true),
		},
		{
			Keys: bson.D{{Key: "online", Value: 1}, {Key: "last_seen", Value: -1}},
		},
	}
	if _, err := userCollection.Indexes().CreateMany(ctx, userIndexes); err != nil {
		log.Printf("Failed to create user indexes: %v", err)
		return err
	}

	// ✅ Indexes untuk messages
	messageIndexes := []mongo.IndexModel{
		{
			Keys: bson.D{
				{Key: "sender_id", Value: 1},
				{Key: "receiver_id", Value: 1},
				{Key: "created_at", Value: -1},
			},
		},
		{
			Keys: bson.D{
				{Key: "receiver_id", Value: 1},
				{Key: "read", Value: 1},
			},
		},
		{
			Keys: bson.D{{Key: "created_at", Value: -1}},
		},
	}
	if _, err := messageCollection.Indexes().CreateMany(ctx, messageIndexes); err != nil {
		log.Printf("Failed to create message indexes: %v", err)
		return err
	}

	return nil
}
