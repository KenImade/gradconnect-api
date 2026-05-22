package main

import (
	"flag"
	"os"
	"strconv"
	"strings"
	"time"

	"api.gradconnect.com/internal/app"
)

type config struct {
	port int
	env  string
	cors struct {
		trustedOrigins []string
	}
	cookieDomain string
	db           struct {
		dsn          string
		maxOpenConns int
		minConns     int
		maxIdleConns int
		maxIdleTime  time.Duration
	}
	sesEvents struct {
		queueURL  string
		awsRegion string
	}
	smtp struct {
		host             string
		port             int
		username         string
		password         string
		sender           string
		configurationSet string
	}
	google struct {
		clientID     string
		clientSecret string
		redirectURL  string
	}
	frontendURL string
	baseURL     string
	sentry      struct {
		dsn              string
		env              string
		tracesSampleRate float64
	}
	r2 struct {
		accountID       string
		accessKeyID     string
		secretAccessKey string
		bucket          string
		publicURL       string
		endpoint        string
	}
}

// resolveDBDSN picks the DSN env var based on the current environment.
// It checks for environment-specific vars first (GRADCONNECT_DB_DSN_STAGING,
// GRADCONNECT_DB_DSN_PROD), falling back to the generic GRADCONNECT_DB_DSN.
func resolveDBDSN() string {
	switch os.Getenv("GRADCONNECT_ENV") {
	case "production":
		if v := os.Getenv("GRADCONNECT_DB_DSN_PROD"); v != "" {
			return v
		}
	case "staging":
		if v := os.Getenv("GRADCONNECT_DB_DSN_STAGING"); v != "" {
			return v
		}
	case "test":
		if v := os.Getenv("GRADCONNECT_DB_DSN_TEST"); v != "" {
			return v
		}
	}
	return os.Getenv("GRADCONNECT_DB_DSN")
}

func parseConfig() config {
	var cfg config

	defaultPort := 8080
	if v := os.Getenv("PORT"); v != "" {
		if p, err := strconv.Atoi(v); err == nil {
			defaultPort = p
		}
	}

	defaultSMTPHost := "localhost"
	if v := os.Getenv("GRADCONNECT_SMTP_HOST"); v != "" {
		defaultSMTPHost = v
	}

	defaultSMTPPort := 1025
	if v := os.Getenv("GRADCONNECT_SMTP_PORT"); v != "" {
		if p, err := strconv.Atoi(v); err == nil {
			defaultSMTPPort = p
		}
	}

	flag.IntVar(&cfg.port, "port", defaultPort, "API server port")

	flag.StringVar(&cfg.env, "env", os.Getenv("GRADCONNECT_ENV"), "Environment (development|staging|production)")

	flag.StringVar(&cfg.db.dsn, "db-dsn", resolveDBDSN(), "PostgreSQL DSN")
	flag.IntVar(&cfg.db.maxOpenConns, "db-max-open-conns", 50, "PostgreSQL max open connections")
	flag.IntVar(&cfg.db.minConns, "db-min-conns", 10, "PostgreSQL min connections")
	flag.IntVar(&cfg.db.maxIdleConns, "db-max-idle-conns", 50, "PostgreSQL max idle connections")
	flag.DurationVar(&cfg.db.maxIdleTime, "db-max-idle-time", 15*time.Minute, "PostgreSQL max connection idle time")

	flag.StringVar(&cfg.cookieDomain, "cookie-domain",
		os.Getenv("GRADCONNECT_COOKIE_DOMAIN"),
		"Cookie domain (e.g. gradconnect.ng for production; leave empty for localhost dev)")

	flag.StringVar(&cfg.smtp.configurationSet, "smtp-configuration-set", os.Getenv("GRADCONNECT_SMTP_CONFIGURATION_SET"), "SMTP configuration set")
	flag.StringVar(&cfg.smtp.host, "smtp-host", defaultSMTPHost, "SMTP host")
	flag.IntVar(&cfg.smtp.port, "smtp-port", defaultSMTPPort, "SMTP port")
	flag.StringVar(&cfg.smtp.username, "smtp-username", os.Getenv("GRADCONNECT_SMTP_USERNAME"), "SMTP username")
	flag.StringVar(&cfg.smtp.password, "smtp-password", os.Getenv("GRADCONNECT_SMTP_PASSWORD"), "SMTP password")
	flag.StringVar(&cfg.smtp.sender, "smtp-sender", os.Getenv("GRADCONNECT_SMTP_SENDER"), "SMTP sender")

	flag.StringVar(&cfg.frontendURL, "frontend-url", os.Getenv("GRADCONNECT_FRONTEND_URL"), "Frontend URL")
	flag.StringVar(&cfg.baseURL, "base-url", os.Getenv("GRADCONNECT_BASE_URL"), "Base URL")

	flag.StringVar(&cfg.google.clientID, "google-client-id", os.Getenv("GRADCONNECT_GOOGLE_CLIENT_ID"), "Google OAuth client ID")
	flag.StringVar(&cfg.google.clientSecret, "google-client-secret", os.Getenv("GRADCONNECT_GOOGLE_CLIENT_SECRET"), "Google OAuth client secret")
	flag.StringVar(&cfg.google.redirectURL, "google-redirect-url", os.Getenv("GRADCONNECT_GOOGLE_REDIRECT_URL"), "Google OAuth redirect URL")

	flag.StringVar(&cfg.sentry.dsn, "sentry-dsn", os.Getenv("GRADCONNECT_SENTRY_DSN"), "Sentry DSN")
	flag.StringVar(&cfg.sentry.env, "sentry-env", os.Getenv("GRADCONNECT_SENTRY_ENV"), "Sentry environment tag")
	flag.Float64Var(&cfg.sentry.tracesSampleRate, "sentry-traces-rate", 0.1, "Sentry traces sample rate")

	flag.StringVar(&cfg.r2.accountID, "r2-account-id", os.Getenv("GRADCONNECT_R2_ACCOUNT_ID"), "Cloudflare R2 account ID")
	flag.StringVar(&cfg.r2.accessKeyID, "r2-access-key-id", os.Getenv("GRADCONNECT_R2_ACCESS_KEY_ID"), "Cloudflare R2 access key ID")
	flag.StringVar(&cfg.r2.secretAccessKey, "r2-secret-access-key", os.Getenv("GRADCONNECT_R2_SECRET_ACCESS_KEY"), "Cloudflare R2 secret access key")
	flag.StringVar(&cfg.r2.bucket, "r2-bucket", os.Getenv("GRADCONNECT_R2_BUCKET"), "Cloudflare R2 bucket name")
	flag.StringVar(&cfg.r2.publicURL, "r2-public-url", os.Getenv("GRADCONNECT_R2_PUBLIC_URL"), "Cloudflare R2 public bucket URL")
	flag.StringVar(&cfg.r2.endpoint, "r2-endpoint", os.Getenv("GRADCONNECT_R2_ENDPOINT"), "Cloudflare R2 S3 endpoint")

	flag.StringVar(&cfg.sesEvents.queueURL, "ses-events-queue-url",
		os.Getenv("GRADCONNECT_SES_EVENTS_QUEUE_URL"),
		"SQS queue URL for SES bounce/complaint events")
	flag.StringVar(&cfg.sesEvents.awsRegion, "aws-region",
		os.Getenv("GRADCONNECT_AWS_REGION"),
		"AWS region for SQS access (e.g. eu-west-1)")

	flag.Func("cors-trusted-origins", "Trusted CORS origins (space separated)", func(val string) error {
		cfg.cors.trustedOrigins = strings.Fields(val)
		return nil
	})

	flag.Parse()

	if len(cfg.cors.trustedOrigins) == 0 {
		if v := os.Getenv("GRADCONNECT_CORS_TRUSTED_ORIGINS"); v != "" {
			cfg.cors.trustedOrigins = strings.Fields(v)
		}
	}

	if cfg.sentry.env == "" {
		cfg.sentry.env = cfg.env
	}

	return cfg
}

// toAppConfig converts the CLI config into the subset needed by internal/app.
func (c config) toAppConfig() app.Config {
	var ac app.Config
	ac.Port = c.port
	ac.Env = c.env
	ac.CookieDomain = c.cookieDomain
	ac.FrontendURL = c.frontendURL
	ac.BaseURL = c.baseURL
	ac.CORS.TrustedOrigins = c.cors.trustedOrigins
	ac.Google.ClientID = c.google.clientID
	ac.Google.ClientSecret = c.google.clientSecret
	ac.Google.RedirectURL = c.google.redirectURL
	return ac
}
