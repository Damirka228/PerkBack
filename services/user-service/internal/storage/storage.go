package storage

import (
	"context"
	"fmt"

	"github.com/Damirka228/PerkBack/services/user-service/internal/config"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Storage struct {
	pool *pgxpool.Pool
}

func NewStorage(ctx context.Context, cfg config.DB) (*Storage, error) {
	cfgPool, err := pgxpool.ParseConfig(cfg.DSN)
	if err != nil {
		return nil, fmt.Errorf("parse to config: %w", err)
	}
	cfgPool.ConnConfig.ConnectTimeout = cfg.ConnectTimeout

	pool, err := pgxpool.NewWithConfig(ctx, cfgPool)
	if err != nil {
		return nil, fmt.Errorf("get pool db: %w", err)
	}

	pingCtx, cancel := context.WithTimeout(ctx, cfg.ConnectTimeout)
	defer cancel()

	if err := pool.Ping(pingCtx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping db: %w", err)
	}

	return &Storage{
		pool: pool,
	}, nil
}

func (s *Storage) Close() {
	if s.pool != nil {
		s.pool.Close()
	}
}
