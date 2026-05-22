package app

import (
	"context"

	"api.gradconnect.com/internal/data"
	"api.gradconnect.com/internal/worker"
)

// buildWorkerPool constructs the background job dispatcher and worker
// pool for this application. Each dispatcher.Register binds a job_type
// string (matching what's stored in task_queue rows) to a handler that
// closes over `app` for access to mailer, models, importer, etc.
//
// The returned pool is not started — the caller is responsible for
// running it (typically `go pool.Run(ctx)`) and for cancelling its
// context on shutdown.
func (app *App) buildWorkerPool() *worker.Pool {
	dispatcher := worker.NewDispatcher()

	dispatcher.Register("email:verify", app.handleEmailVerify)
	dispatcher.Register("email:welcome", app.handleEmailWelcome)
	dispatcher.Register("email:password_reset", app.handleEmailPasswordReset)
	dispatcher.Register("email:deadline_reminder", app.handleEmailDeadlineReminder)
	dispatcher.Register("admin:import", app.handleAdminImport)
	dispatcher.Register("employer:recalc_ratings", app.handleEmployerRecalcRatings)

	return worker.New(app.db, app.logger, dispatcher)
}

// --- Job handlers ---

func (app *App) handleEmailVerify(ctx context.Context, _ string, payload []byte) error {
	load, err := worker.UnmarshalPayload[struct {
		BaseURL         string `json:"base_url"`
		Email           string `json:"user_email"`
		FirstName       string `json:"first_name"`
		ActivationToken string `json:"activation_token"`
	}](payload)
	if err != nil {
		return err
	}
	return app.sendIfDeliverable(ctx, load.Email, "email_verify.tmpl", load)
}

func (app *App) handleEmailWelcome(ctx context.Context, _ string, payload []byte) error {
	load, err := worker.UnmarshalPayload[struct {
		Email     string `json:"user_email"`
		FirstName string `json:"first_name"`
	}](payload)
	if err != nil {
		return err
	}
	return app.sendIfDeliverable(ctx, load.Email, "user_welcome.tmpl", load)
}

func (app *App) handleEmailPasswordReset(ctx context.Context, _ string, payload []byte) error {
	load, err := worker.UnmarshalPayload[struct {
		FrontendURL string `json:"frontend_url"`
		Email       string `json:"user_email"`
		FirstName   string `json:"first_name"`
		ResetToken  string `json:"reset_token"`
	}](payload)
	if err != nil {
		return err
	}
	return app.sendIfDeliverable(ctx, load.Email, "password_reset.tmpl", load)
}

func (app *App) handleAdminImport(ctx context.Context, _ string, payload []byte) error {
	load, err := worker.UnmarshalPayload[struct {
		ImportJobID string `json:"import_job_id"`
	}](payload)
	if err != nil {
		return err
	}
	return app.processImport(load.ImportJobID)
}

func (app *App) handleEmployerRecalcRatings(ctx context.Context, _ string, payload []byte) error {
	load, err := worker.UnmarshalPayload[struct {
		EmployerID string `json:"employer_id"`
	}](payload)
	if err != nil {
		return err
	}
	return app.models.Employers.RecalculateRatings(ctx, app.db, load.EmployerID)
}

func (app *App) handleEmailDeadlineReminder(ctx context.Context, _ string, payload []byte) error {
	load, err := worker.UnmarshalPayload[struct {
		Recipient   string                          `json:"recipient"`
		FirstName   string                          `json:"first_name"`
		BaseURL     string                          `json:"base_url"`
		FrontendURL string                          `json:"frontend_url"`
		Bookmarks   []data.DeadlineReminderBookmark `json:"bookmarks"`
	}](payload)
	if err != nil {
		return err
	}
	return app.sendIfDeliverable(ctx, load.Recipient, "deadline_reminder.tmpl", load)
}
