package main

import (
	"context"
	"flag"
	"log/slog"
	"os"
	"sync"
	"time"

	_ "api.gradconnect.com/cmd/api/docs" // swagger docs
	"api.gradconnect.com/internal/data"
	"github.com/jackc/pgx/v5/pgxpool"
)

// application version
const version = "1.0.0"

type config struct {
	port int
	env  string
	db   struct {
		dsn          string
		maxOpenConns int
		minConns     int
		maxIdleConns int
		maxIdleTime  time.Duration
	}
}

type application struct {
	config config
	logger *slog.Logger
	models data.Models
	wg     sync.WaitGroup
}

// @title GradConnect API
// @version 1.0
// @description Nigeria's Graduate Career Intelligence Platform API
// @host localhost:4000
// @BasePath /api/v1
// @securityDefinitions.apikey SessionCookie
// @in cookie
// @name session_id
func main() {
	var cfg config

	// command line configuration values
	flag.IntVar(&cfg.port, "port", cfg.port, "API server port")
	flag.StringVar(&cfg.env, "env", "development", "Environment (development|staging|production)")

	flag.StringVar(&cfg.db.dsn, "db-dsn", "", "PostgreSQL DSN")
	flag.IntVar(&cfg.db.maxOpenConns, "db-max-open-conns", 25, "PostgreSQL max open connections")
	flag.IntVar(&cfg.db.minConns, "db-min-conns", 5, "PostgreSQL min connections")
	flag.IntVar(&cfg.db.maxIdleConns, "db-max-idle-conns", 25, "PostgreSQL max idle connections")
	flag.DurationVar(&cfg.db.maxIdleTime, "db-max-idle-time", 15*time.Minute, "PostgreSQL max connection idle time")

	flag.Parse()

	// logger
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	// database
	db, err := openDB(cfg)
	if err != nil {
		logger.Error(err.Error())
		os.Exit(1)
	}

	// initialise application
	app := &application{
		config: cfg,
		logger: logger,
		models: data.NewModels(db),
	}

	err = app.serve()
	if err != nil {
		logger.Error(err.Error())
		os.Exit(1)
	}

}

func openDB(cfg config) (*pgxpool.Pool, error) {
	config, err := pgxpool.ParseConfig(cfg.db.dsn)
	if err != nil {
		return nil, err
	}

	config.MaxConns = int32(cfg.db.maxOpenConns)
	config.MinConns = int32(cfg.db.minConns)
	config.MaxConnIdleTime = cfg.db.maxIdleTime

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, err
	}

	if err = pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, err
	}

	return pool, nil
}
