package app

import (
	"context"
	"fmt"

	"api.gradconnect.com/internal/data"
	"api.gradconnect.com/internal/worker"
)

// SES event payload shapes. We only parse the fields we use; SES events
// carry a lot of metadata (headers, tags, MTA info) that we discard.
type sesBouncePayload struct {
	EventType string `json:"eventType"`
	Bounce    struct {
		BounceType        string `json:"bounceType"`
		BouncedRecipients []struct {
			EmailAddress string `json:"emailAddress"`
		} `json:"bouncedRecipients"`
	} `json:"bounce"`
}

type sesComplaintPayload struct {
	EventType string `json:"eventType"`
	Complaint struct {
		ComplainedRecipients []struct {
			EmailAddress string `json:"emailAddress"`
		} `json:"complainedRecipients"`
	} `json:"complaint"`
}

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
	dispatcher.Register("ses:bounce", app.handleSESBounce)
	dispatcher.Register("ses:complaint", app.handleSESComplaint)

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
	return app.processImport(ctx, load.ImportJobID)
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

// handleSESBounce marks bounced addresses as undeliverable. Only acts on
// permanent (hard) bounces — transient bounces aren't in our subscription,
// but we defensively check anyway in case the SES subscription is widened
// later without updating this handler.
func (app *App) handleSESBounce(ctx context.Context, _ string, payload []byte) error {
	load, err := worker.UnmarshalPayload[sesBouncePayload](payload)
	if err != nil {
		return err
	}

	if load.Bounce.BounceType != "Permanent" {
		app.logger.Info("ignoring non-permanent bounce", "bounce_type", load.Bounce.BounceType)
		return nil
	}

	for _, r := range load.Bounce.BouncedRecipients {
		if err := app.models.Users.MarkEmailStatus(ctx, app.db, r.EmailAddress, "bounced"); err != nil {
			return fmt.Errorf("marking %s as bounced: %w", r.EmailAddress, err)
		}
		app.logger.Info("marked address as bounced", "email", r.EmailAddress)
	}

	return nil
}

// handleSESComplaint marks complaining addresses as undeliverable. A
// complaint is the recipient flagging us as spam — even stronger signal
// than a bounce that we should stop sending.
func (app *App) handleSESComplaint(ctx context.Context, _ string, payload []byte) error {
	load, err := worker.UnmarshalPayload[sesComplaintPayload](payload)
	if err != nil {
		return err
	}

	for _, r := range load.Complaint.ComplainedRecipients {
		if err := app.models.Users.MarkEmailStatus(ctx, app.db, r.EmailAddress, "complained"); err != nil {
			return fmt.Errorf("marking %s as complained: %w", r.EmailAddress, err)
		}
		app.logger.Info("marked address as complained", "email", r.EmailAddress)
	}

	return nil
}
