package storage

import (
	"context"
	"fmt"

	"github.com/Damirka228/PerkBack/services/benefit-service/internal/config"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/sirupsen/logrus"
)

type Storage struct {
	pool *pgxpool.Pool
	log  *logrus.Logger
}

func NewStorage(ctx context.Context, cfg config.DB, log *logrus.Logger) (*Storage, error) {
	conn, err := pgxpool.ParseConfig(cfg.DSN)
	if err != nil {
		return nil, fmt.Errorf("error to parse config: %w", err)
	}

	conn.ConnConfig.ConnectTimeout = cfg.ConnectTimeout

	pool, err := pgxpool.NewWithConfig(ctx, conn)
	if err != nil {
		return nil, fmt.Errorf("error to get pool db: %w", err)
	}

	pingCtx, cancel := context.WithTimeout(ctx, cfg.ConnectTimeout)
	defer cancel()

	if err = pool.Ping(pingCtx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("err to ping db:%w", err)
	}
	return &Storage{
		pool: pool,
		log:  log,
	}, nil
}

func (s *Storage) Close() {
	if s.pool != nil {
		s.pool.Close()
	}
}
