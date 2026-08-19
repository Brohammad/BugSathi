package redis

import (
	"context"
	"fmt"

	goredis "github.com/redis/go-redis/v9"
)

// Client wraps a shared Redis connection pool for optional multi-replica features.
type Client struct {
	inner *goredis.Client
}

func New(url string) (*Client, error) {
	if url == "" {
		return nil, fmt.Errorf("redis url is empty")
	}
	opt, err := goredis.ParseURL(url)
	if err != nil {
		return nil, fmt.Errorf("redis url: %w", err)
	}
	c := goredis.NewClient(opt)
	if err := c.Ping(context.Background()).Err(); err != nil {
		_ = c.Close()
		return nil, fmt.Errorf("redis ping: %w", err)
	}
	return &Client{inner: c}, nil
}

func (c *Client) Raw() *goredis.Client {
	if c == nil {
		return nil
	}
	return c.inner
}

func (c *Client) Ping(ctx context.Context) error {
	if c == nil || c.inner == nil {
		return fmt.Errorf("redis not configured")
	}
	return c.inner.Ping(ctx).Err()
}

func (c *Client) Close() error {
	if c == nil || c.inner == nil {
		return nil
	}
	return c.inner.Close()
}
