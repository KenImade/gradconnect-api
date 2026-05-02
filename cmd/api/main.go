package main

import (
	"context"
	"log/slog"
	"net/url"
	"os"
	"time"

	"api.gradconnect.com/cmd/api/docs"
	_ "api.gradconnect.com/cmd/api/docs" // swagger docs
	"api.gradconnect.com/internal/app"
	"api.gradconnect.com/internal/mailer"
	"api.gradconnect.com/internal/storage"
	"github.com/getsentry/sentry-go"
)

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

	if u, err := url.Parse(cfg.baseURL); err == nil && u.Host != "" {
		docs.SwaggerInfo.Host = u.Host
		docs.SwaggerInfo.Schemes = []string{u.Scheme}
	}

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

	a := app.New(cfg.toAppConfig(), db, logger, m, storageClient)

	if err = a.Serve(); err != nil {
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
		Release:          app.Version,
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
