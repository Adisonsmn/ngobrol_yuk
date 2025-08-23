package config

import (
	"context"
	"log"
	"os"
	"time"

	"github.com/joho/godotenv"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/v2/bson"
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
	createIndexes()

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

func createIndexes() {
	ctx := context.Background()

	// Users collection indexes
	userIndexes := []mongo.IndexModel{
		{
			Keys:    bson.D{{"email", 1}},
			Options: options.Index().SetUnique(true),
		},
		{
			Keys:    bson.D{{"username", 1}},
			Options: options.Index().SetUnique(true),
		},
		{
			Keys: bson.D{{"online", 1}, {"last_seen", -1}},
		},
	}

	_, err := DB.Collection("users").Indexes().CreateMany(ctx, userIndexes)
	if err != nil {
		log.Printf("Failed to create user indexes: %v", err)
	}

	// Messages collection indexes
	messageIndexes := []mongo.IndexModel{
		{
			Keys: bson.D{
				{"sender_id", 1},
				{"receiver_id", 1},
				{"created_at", -1},
			},
		},
		{
			Keys: bson.D{
				{"receiver_id", 1},
				{"read", 1},
			},
		},
		{
			Keys: bson.D{{"created_at", -1}},
		},
	}

	_, err = DB.Collection("messages").Indexes().CreateMany(ctx, messageIndexes)
	if err != nil {
		log.Printf("Failed to create message indexes: %v", err)
	}

	log.Println("Database indexes created successfully")
}
