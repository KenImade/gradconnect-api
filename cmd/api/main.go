package main

import (
	"context"
	"log/slog"
	"os"
	"sync"
	"time"

	_ "api.gradconnect.com/cmd/api/docs" // swagger docs
	"api.gradconnect.com/internal/data"
	"api.gradconnect.com/internal/mailer"
	"api.gradconnect.com/internal/ratelimit"
	"api.gradconnect.com/internal/storage"
	"api.gradconnect.com/internal/worker"
	"github.com/getsentry/sentry-go"
	"github.com/jackc/pgx/v5/pgxpool"
)

// application version
const version = "1.0.0"

type application struct {
	config       config
	db           *pgxpool.Pool
	limiter      *ratelimit.MemoryLimiter
	logger       *slog.Logger
	mailer       *mailer.Mailer
	models       data.Models
	storage      storage.Storage
	worker       *worker.Pool
	workerCtx    context.Context
	workerCancel context.CancelFunc
	wg           sync.WaitGroup
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
	cfg := parseConfig()

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	if err := initSentry(cfg, logger); err != nil {
		logger.Error("sentry init failed", "err", err)
		os.Exit(1)
	}

	db, err := openDB(cfg, logger)
	if err != nil {
		logger.Error(err.Error())
		os.Exit(1)
	}
	defer db.Close()

	m, err := mailer.New(cfg.smtp.host, cfg.smtp.port, cfg.smtp.username, cfg.smtp.password, cfg.smtp.sender)
	if err != nil {
		logger.Error(err.Error())
		os.Exit(1)
	}

	storageClient, err := storage.NewR2Storage(context.Background(), storage.R2Config{
		AccountID:       cfg.r2.accountID,
		AccessKeyID:     cfg.r2.accessKeyID,
		SecretAccessKey: cfg.r2.secretAccessKey,
		Bucket:          cfg.r2.bucket,
		PublicURL:       cfg.r2.publicURL,
		Endpoint:        cfg.r2.endpoint,
	})
	if err != nil {
		logger.Error("storage init failed", "err", err)
		os.Exit(1)
	}
	logger.Info("storage initialised", "bucket", cfg.r2.bucket)

	app := &application{
		config:  cfg,
		db:      db,
		limiter: ratelimit.NewMemoryLimiter(),
		logger:  logger,
		mailer:  m,
		models:  data.NewModels(db),
		storage: storageClient,
	}

	ctx, cancel := context.WithCancel(context.Background())
	app.workerCtx = ctx
	app.workerCancel = cancel

	app.worker = app.buildWorkerPool()
	go app.worker.Run(ctx)

	if err = app.serve(); err != nil {
		logger.Error(err.Error())
		os.Exit(1)
	}
}

func initSentry(cfg config, logger *slog.Logger) error {
	if cfg.sentry.dsn == "" {
		logger.Warn("sentry not configured; SENTRY_DSN missing")
		return nil
	}

	err := sentry.Init(sentry.ClientOptions{
		Dsn:              cfg.sentry.dsn,
		Environment:      cfg.sentry.env,
		Release:          version,
		EnableTracing:    true,
		TracesSampleRate: cfg.sentry.tracesSampleRate,
		BeforeSend: func(event *sentry.Event, hint *sentry.EventHint) *sentry.Event {
			if cfg.env == "development" {
				return nil
			}
			return event
		},
	})
	if err != nil {
		return err
	}

	defer sentry.Flush(2 * time.Second)
	logger.Info("sentry enabled", "env", cfg.sentry.env)
	return nil
}
