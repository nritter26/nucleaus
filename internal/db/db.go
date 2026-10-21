package db

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Client struct{ Pool *pgxpool.Pool }

func NewClient(ctx context.Context, dsn string) (*Client, error) {
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, err
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, err
	}
	return &Client{Pool: pool}, nil
}

func (c *Client) Close() { c.Pool.Close() }