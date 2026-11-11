package storage

import (
	"context"
	"fmt"

	"github.com/Damirka228/PerkBack/services/rules-service/internal/config"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Storage struct {
	pool *pgxpool.Pool
}

func NewStorage(ctx context.Context, cfg config.DB) (*Storage, error) {
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
	}, nil
}

func (s *Storage) Close() {
	if s.pool != nil {
		//fmt.Println("closing db pool....") TODO: заменить на логи
		s.pool.Close()
	}
	// fmt.Println("pool is closed") TODO: заменить на логи
}
