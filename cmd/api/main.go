package main

import (
	"context"
	"flag"
	"log/slog"
	"os"
	"strings"
	"sync"
	"time"

	_ "api.gradconnect.com/cmd/api/docs" // swagger docs
	"api.gradconnect.com/internal/data"
	"api.gradconnect.com/internal/mailer"
	"api.gradconnect.com/internal/ratelimit"
	"github.com/jackc/pgx/v5/pgxpool"
)

// application version
const version = "1.0.0"

type config struct {
	port int
	env  string
	cors struct {
		trustedOrigins []string
	}
	db struct {
		dsn          string
		maxOpenConns int
		minConns     int
		maxIdleConns int
		maxIdleTime  time.Duration
	}
	smtp struct {
		host     string
		port     int
		username string
		password string
		sender   string
	}
	google struct {
		clientID     string
		clientSecret string
		redirectURL  string
	}
	frontendURL string
	baseURL     string
	import_     struct {
		storageDir string
	}
}

type application struct {
	config  config
	db      *pgxpool.Pool
	limiter *ratelimit.MemoryLimiter
	logger  *slog.Logger
	mailer  *mailer.Mailer
	models  data.Models
	wg      sync.WaitGroup
}

// @title GradConnect API
// @version 1.0
// @description Nigeria's Graduate Career Intelligence Platform API
// @host localhost:4000
// @BasePath /api/v1
// @securityDefinitions.apikey SessionCookie
// @in cookie
// @name session_id
// @tag.name Opportunities
// @tag.description Public-facing graduate opportunities listings
// @tag.name Admin
// @tag.description Admin-only management endpoints for content seeding, editing, and moderation
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

	flag.StringVar(&cfg.smtp.host, "smtp-host", "sandbox.smtp.mailtrap.io", "SMTP host")
	flag.IntVar(&cfg.smtp.port, "smtp-port", 587, "SMTP port")
	flag.StringVar(&cfg.smtp.username, "smtp-username", "f111705a4bf447", "SMTP username")
	flag.StringVar(&cfg.smtp.password, "smtp-password", "eb96d8aef81e66", "SMTP password")
	flag.StringVar(&cfg.smtp.sender, "smtp-sender", "GradConnect <no-reply@gradconnect.ng>", "SMTP sender")

	flag.StringVar(&cfg.frontendURL, "frontend-url", os.Getenv("GRADCONNECT_FRONTEND_URL"), "Frontend URL")
	flag.StringVar(&cfg.baseURL, "base-url", os.Getenv("GRADCONNECT_BASE_URL"), "Base URL")

	flag.StringVar(&cfg.google.clientID, "google-client-id", "522466790021-q29p5hhcfenk8qrrr5dq5mskujduevq6.apps.googleusercontent.com", "Google OAuth client ID")
	flag.StringVar(&cfg.google.clientSecret, "google-client-secret", "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", "Google OAuth client secret")
	flag.StringVar(&cfg.google.redirectURL, "google-redirect-url", "http://localhost:3000", "Google OAuth redirect URL")

	flag.StringVar(&cfg.import_.storageDir, "import-storage-dir", "/tmp/gradconnect-imports", "Directory for CSV imports")

	flag.Func("cors-trusted-origins", "Trusted CORS origins (space separated)", func(val string) error {
		cfg.cors.trustedOrigins = strings.Fields(val)
		return nil
	})

	flag.Parse()

	// logger
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	// database
	db, err := openDB(cfg, logger)
	if err != nil {
		logger.Error(err.Error())
		os.Exit(1)
	}

	defer db.Close()

	logger.Info("database connection pool established")

	mailer, err := mailer.New(cfg.smtp.host, cfg.smtp.port, cfg.smtp.username, cfg.smtp.password, cfg.smtp.sender)
	if err != nil {
		logger.Error(err.Error())
		os.Exit(1)
	}

	// initialise application
	app := &application{
		config:  cfg,
		db:      db,
		limiter: ratelimit.NewMemoryLimiter(),
		logger:  logger,
		mailer:  mailer,
		models:  data.NewModels(db),
	}

	go app.runTaskWorker()

	err = app.serve()
	if err != nil {
		logger.Error(err.Error())
		os.Exit(1)
	}

}

func openDB(cfg config, logger *slog.Logger) (*pgxpool.Pool, error) {
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

	logger.Info("database connection pool established",
		"max_conns", config.MaxConns,
		"min_conns", config.MinConns,
		"max_idle_time", config.MaxConnIdleTime,
	)

	return pool, nil
}
