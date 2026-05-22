package app

import (
	"log/slog"
	"sync"

	"api.gradconnect.com/internal/data"
	"api.gradconnect.com/internal/imagegen"
	"api.gradconnect.com/internal/mailer"
	"api.gradconnect.com/internal/ratelimit"
	"api.gradconnect.com/internal/storage"
	"api.gradconnect.com/internal/worker"
	"github.com/jackc/pgx/v5/pgxpool"
)

const Version = "1.0.0"

// Config holds the runtime configuration needed by the application layer.
// Initialisation-only settings (DB pool sizes, SMTP credentials, R2 keys, Sentry DSN)
// are handled by cmd/api and never reach this package.
type Config struct {
	Port         int
	CookieDomain string
	Env          string
	FrontendURL  string
	BaseURL      string
	CORS         struct {
		TrustedOrigins []string
	}
	Google struct {
		ClientID     string
		ClientSecret string
		RedirectURL  string
	}
}

// App is the central dependency container. All HTTP handlers and middleware
// are methods on this type, keeping them co-located with their dependencies
// without relying on global state.
type App struct {
	config            Config
	db                *pgxpool.Pool
	imagegen          *imagegen.Generator
	limiter           *ratelimit.MemoryLimiter
	logger            *slog.Logger
	mailer            *mailer.Mailer
	models            data.Models
	storage           storage.Storage
	worker            *worker.Pool
	wg                sync.WaitGroup
	sqsClient         worker.SQSClient // nil disables SES events polling
	sesEventsQueueURL string           // empty disables SES events polling
}

// New constructs an App with all dependencies wired up. The worker pool is
// not started here — Serve() starts it alongside the HTTP server so both
// are torn down together on shutdown.
//
// Pass sqsClient=nil and sesEventsQueueURL="" to disable SES events polling
// (typical for local dev and integration tests).
func New(
	cfg Config,
	db *pgxpool.Pool,
	ig *imagegen.Generator,
	logger *slog.Logger,
	m *mailer.Mailer,
	s storage.Storage,
	sqsClient worker.SQSClient,
	sesEventsQueueURL string,
) *App {
	return &App{
		config:            cfg,
		db:                db,
		imagegen:          ig,
		limiter:           ratelimit.NewMemoryLimiter(),
		logger:            logger,
		mailer:            m,
		models:            data.NewModels(db),
		storage:           s,
		sqsClient:         sqsClient,
		sesEventsQueueURL: sesEventsQueueURL,
	}
}
