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

// Serve starts the HTTP server and the background worker pool. Both are torn
// down together when SIGINT or SIGTERM is received.
func (app *App) Serve() error {
	ctx, cancel := context.WithCancel(context.Background())

	app.worker = app.buildWorkerPool()
	go app.worker.Run(ctx)

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

		err := srv.Shutdown(shutdownCtx)
		if err != nil {
			shutdownError <- err
			return
		}

		app.logger.Info("stopping background workers")
		cancel()

		app.logger.Info("completing background tasks", "addr", srv.Addr)
		app.wg.Wait()

		shutdownError <- nil
	}()

	app.logger.Info("starting server", "addr", srv.Addr, "env", app.config.Env)

	err := srv.ListenAndServe()
	if !errors.Is(err, http.ErrServerClosed) {
		cancel()
		return err
	}

	err = <-shutdownError
	if err != nil {
		return err
	}

	app.logger.Info("stopped server", "addr", srv.Addr)
	return nil
}
