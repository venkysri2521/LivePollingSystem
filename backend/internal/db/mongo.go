package db

import (
	"context"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type Mongo struct {
	Client *mongo.Client
	DB     *mongo.Database
}

func ConnectMongo(ctx context.Context, uri, dbName string) (*Mongo, error) {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	opts := options.Client().
		ApplyURI(uri).
		SetServerSelectionTimeout(10 * time.Second).
		SetMaxPoolSize(50)

	client, err := mongo.Connect(ctx, opts)
	if err != nil {
		return nil, fmt.Errorf("mongo connect: %w", err)
	}
	if err := client.Ping(ctx, nil); err != nil {
		return nil, fmt.Errorf("mongo ping: %w", err)
	}

	m := &Mongo{Client: client, DB: client.Database(dbName)}
	if err := m.ensureIndexes(ctx); err != nil {
		return nil, err
	}
	return m, nil
}

// ensureIndexes is the durable half of correctness. Redis guards the hot path,
// but these unique indexes mean a Redis flush can never let a duplicate email
// or a double vote slip into the database.
func (m *Mongo) ensureIndexes(ctx context.Context) error {
	_, err := m.Users().Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys:    bson.D{{Key: "email", Value: 1}},
		Options: options.Index().SetUnique(true).SetName("uniq_email"),
	})
	if err != nil {
		return fmt.Errorf("index users.email: %w", err)
	}

	_, err = m.Polls().Indexes().CreateMany(ctx, []mongo.IndexModel{
		{
			Keys:    bson.D{{Key: "slug", Value: 1}},
			Options: options.Index().SetUnique(true).SetName("uniq_slug"),
		},
		{
			Keys:    bson.D{{Key: "ownerId", Value: 1}, {Key: "createdAt", Value: -1}},
			Options: options.Index().SetName("owner_recent"),
		},
	})
	if err != nil {
		return fmt.Errorf("index polls: %w", err)
	}

	_, err = m.Votes().Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys:    bson.D{{Key: "pollId", Value: 1}, {Key: "voterKey", Value: 1}},
		Options: options.Index().SetUnique(true).SetName("one_vote_per_voter"),
	})
	if err != nil {
		return fmt.Errorf("index votes: %w", err)
	}
	return nil
}

func (m *Mongo) Users() *mongo.Collection { return m.DB.Collection("users") }
func (m *Mongo) Polls() *mongo.Collection { return m.DB.Collection("polls") }
func (m *Mongo) Votes() *mongo.Collection { return m.DB.Collection("votes") }

func (m *Mongo) Close(ctx context.Context) error { return m.Client.Disconnect(ctx) }
