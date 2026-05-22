package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

// Serve starts the HTTP server and the background worker loops. All are
// torn down together when SIGINT or SIGTERM is received: the HTTP server
// finishes in-flight requests, then the worker context is cancelled and
// we wait for the background goroutines to drain.
func (app *App) Serve() error {
	ctx, cancel := context.WithCancel(context.Background())

	app.worker = app.buildWorkerPool()

	// Task queue worker.
	app.wg.Add(1)
	go func() {
		defer app.wg.Done()
		app.worker.Run(ctx)
	}()

	// Cron loop.
	app.wg.Add(1)
	go func() {
		defer app.wg.Done()
		app.worker.Cron(ctx, app.config.BaseURL, app.config.FrontendURL)
	}()

	// SES events poller. Disabled (no-op) when sqsClient or queueURL
	// are unset — useful for local dev and integration tests where SQS
	// isn't configured.
	app.wg.Add(1)
	go func() {
		defer app.wg.Done()
		app.worker.PollSESEvents(ctx, app.sqsClient, app.sesEventsQueueURL)
	}()

	srv := &http.Server{
		Addr:         fmt.Sprintf(":%d", app.config.Port),
		Handler:      app.routes(),
		IdleTimeout:  time.Minute,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		ErrorLog:     slog.NewLogLogger(app.logger.Handler(), slog.LevelError),
	}

	shutdownError := make(chan error)

	go func() {
		quit := make(chan os.Signal, 1)
		signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

		s := <-quit

		app.logger.Info("shutting down server", "signal", s.String())

		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer shutdownCancel()

		if err := srv.Shutdown(shutdownCtx); err != nil {
			shutdownError <- err
			return
		}

		app.logger.Info("stopping background workers")
		cancel()

		app.logger.Info("waiting for background tasks", "addr", srv.Addr)
		app.wg.Wait()

		shutdownError <- nil
	}()

	app.logger.Info("starting server", "addr", srv.Addr, "env", app.config.Env)

	err := srv.ListenAndServe()
	if !errors.Is(err, http.ErrServerClosed) {
		cancel()
		return err
	}

	if err := <-shutdownError; err != nil {
		return err
	}

	app.logger.Info("stopped server", "addr", srv.Addr)
	return nil
}
