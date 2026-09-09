package main

import (
	"context"
	"fmt"
	"os"

	"github.com/jackc/pgx/v5"
)

// connect opens a connection using DATABASE_URL. A single connection (not
// a pool) is enough for a one-shot import CLI.
func connect(ctx context.Context) (*pgx.Conn, error) {
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		return nil, fmt.Errorf("DATABASE_URL is not set")
	}
	return pgx.Connect(ctx, url)
}
