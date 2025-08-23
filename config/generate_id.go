package config

import (
	"context"
	"fmt"
	"sync"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var idMutex sync.Mutex

func GetNextUserID() string {
	idMutex.Lock()
	defer idMutex.Unlock()

	filter := bson.M{"_id": "user_id"}
	update := bson.M{"$inc": bson.M{"seq": 1}}
	opts := options.FindOneAndUpdate().
		SetUpsert(true).
		SetReturnDocument(options.After)

	var result struct {
		Seq int `bson:"seq"`
	}

	ctx := context.Background()
	err := DB.Collection("counters").FindOneAndUpdate(ctx, filter, update, opts).Decode(&result)
	if err != nil {
		// Fallback to timestamp-based ID if counter fails
		return fmt.Sprintf("%d", time.Now().UnixNano()%1000)
	}

	// Format as 3-digit string with leading zeros
	return fmt.Sprintf("%03d", result.Seq)
}
