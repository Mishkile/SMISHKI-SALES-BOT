// Package db opens the MongoDB connection used by the repositories.
package db

import (
	"context"
	"log"
	"net/url"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// DefaultURI is used when MONGO_URI is unset; Docker should always provide it.
const DefaultURI = "mongodb://localhost:27017/SalesBotDB"

// DefaultDatabase is used when the URI carries no database name.
const DefaultDatabase = "SalesBotDB"

const retryDelay = 5 * time.Second

// Connect dials uri and keeps retrying every 5 seconds until the server
// answers a ping or ctx is cancelled, mirroring the original bot's behaviour of
// waiting for MongoDB instead of exiting.
func Connect(ctx context.Context, uri string) (*mongo.Client, error) {
	for {
		client, err := mongo.Connect(options.Client().ApplyURI(uri))
		if err == nil {
			pingCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
			err = client.Ping(pingCtx, nil)
			cancel()
			if err == nil {
				log.Println("✅ Connected to MongoDB")
				return client, nil
			}
			_ = client.Disconnect(context.Background())
		}
		log.Printf("❌ MongoDB connection error: %v", err)
		log.Printf("Retrying connection in %s...", retryDelay)
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(retryDelay):
		}
	}
}

// DatabaseName extracts the database from a Mongo URI path
// (mongodb://host/SalesBotDB), falling back to DefaultDatabase.
func DatabaseName(uri string) string {
	u, err := url.Parse(uri)
	if err != nil || u.Scheme == "" {
		return DefaultDatabase
	}
	name := strings.Trim(u.Path, "/")
	if name == "" {
		return DefaultDatabase
	}
	return name
}
