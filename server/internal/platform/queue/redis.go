package queue

import (
	"context"

	"github.com/redis/go-redis/v9"
)

type Client struct {
	*redis.Client
}

// New returns the shared Redis client. Day02 only uses Ping; Day08 will add
// Asynq using this same platform boundary.
func New(address, password string) *Client {
	return &Client{Client: redis.NewClient(&redis.Options{
		Addr:         address,
		Password:     password,
		DB:           0,
		PoolSize:     10,
		MinIdleConns: 1,
	})}
}

// Ping normalizes go-redis' StatusCmd API to the platform Pinger contract.
func (c *Client) Ping(ctx context.Context) error {
	return c.Client.Ping(ctx).Err()
}
