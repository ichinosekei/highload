package platform

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	maxConns        = 20
	minConns        = 5
	maxConnIdleTime = 30 * time.Minute
)

type PostgresDB struct {
	*pgxpool.Pool
}

func NewPostgresDB(ctx context.Context, connString string) (*PostgresDB, error) {
	config, errParse := pgxpool.ParseConfig(connString)
	if errParse != nil {
		return nil, fmt.Errorf("postgres config parsing: %w", errParse)
	}

	config.MaxConns = maxConns
	config.MinConns = minConns
	config.MaxConnIdleTime = maxConnIdleTime

	pool, errPool := pgxpool.NewWithConfig(ctx, config)
	if errPool != nil {
		return nil, fmt.Errorf("postgres connection pool creation: %w", errPool)
	}

	if errPing := pool.Ping(ctx); errPing != nil {
		return nil, fmt.Errorf("postgres connection ping: %w", errPing)
	}

	return &PostgresDB{pool}, nil
}
