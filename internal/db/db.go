package db

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/lib/pq"
)

// connectTimeout bounds the startup ping, so an unreachable database fails
// the deploy quickly instead of hanging it.
const connectTimeout = 10 * time.Second

// Connect opens a connection pool to Postgres and verifies it with a ping.
func Connect(databaseURL string) (*sql.DB, error) {
	if databaseURL == "" {
		return nil, fmt.Errorf("DATABASE_URL is not set")
	}

	// sql.Open("postgres", ...) would dial through lib/pq's context-less
	// Driver.Open, so a context deadline couldn't interrupt connecting to an
	// unreachable host. A Connector passes the context through to the dial.
	connector, err := pq.NewConnector(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("parsing DATABASE_URL: %w", err)
	}
	conn := sql.OpenDB(connector)

	conn.SetMaxOpenConns(10)
	conn.SetMaxIdleConns(5)
	conn.SetConnMaxLifetime(30 * time.Minute)

	ctx, cancel := context.WithTimeout(context.Background(), connectTimeout)
	defer cancel()
	if err := conn.PingContext(ctx); err != nil {
		conn.Close()
		return nil, fmt.Errorf("pinging db: %w", err)
	}

	return conn, nil
}
