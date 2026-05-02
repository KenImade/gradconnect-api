package main

import (
	"context"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func openDB(cfg config, logger *slog.Logger) (*pgxpool.Pool, error) {
	poolCfg, err := pgxpool.ParseConfig(cfg.db.dsn)
	if err != nil {
		return nil, err
	}

	poolCfg.MaxConns = int32(cfg.db.maxOpenConns)
	poolCfg.MinConns = int32(cfg.db.minConns)
	poolCfg.MaxConnIdleTime = cfg.db.maxIdleTime

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		return nil, err
	}

	if err = pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, err
	}

	logger.Info("database connection pool established",
		"max_conns", poolCfg.MaxConns,
		"min_conns", poolCfg.MinConns,
		"max_idle_time", poolCfg.MaxConnIdleTime,
	)

	return pool, nil
}
