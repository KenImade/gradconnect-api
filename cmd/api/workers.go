package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

func (app *application) runTaskWorker() {
	app.logger.Info("starting background task worker")

	for {
		// Poll for a pending task
		// We use FOR UPDATE SKIP LOCKED to handle concurrency safely
		ctx := context.Background()

		var taskID string
		var jobType string
		var payload []byte

		query := `
            UPDATE task_queue
            SET status = 'processing', locked_at = now()
            WHERE id = (
                SELECT id FROM task_queue
                WHERE status = 'pending' AND run_at <= now()
                ORDER BY run_at ASC
                LIMIT 1
                FOR UPDATE SKIP LOCKED
            )
            RETURNING id, job_type, payload`

		err := app.db.QueryRow(ctx, query).Scan(&taskID, &jobType, &payload)

		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				// No tasks to process, sleep for a bit
				time.Sleep(5 * time.Second)
				continue
			}
			app.logger.Error("worker poll error", "error", err)
			time.Sleep(5 * time.Second)
			continue
		}

		// Process the task
		err = app.processTask(jobType, payload)

		if err != nil {
			app.logger.Error("task failed", "id", taskID, "error", err)

			// Increment attempts and check if it should become 'dead'
			query := `
				UPDATE task_queue 
				SET 
					status = CASE 
						WHEN attempts + 1 >= max_attempts THEN 'dead'::task_status_type 
						ELSE 'pending'::task_status_type 
					END,
					attempts = attempts + 1,
					last_error = $1,
					run_at = CASE 
						WHEN attempts + 1 >= max_attempts THEN run_at -- don't reschedule
						ELSE now() + (pow(2, attempts + 1) * interval '1 minute') -- exponential backoff
					END
				WHERE id = $2`

			app.db.Exec(ctx, query, err.Error(), taskID)
		} else {
			// Task succeeded
			app.db.Exec(ctx, "UPDATE task_queue SET status = 'completed', completed_at = now() WHERE id = $1", taskID)
		}
	}
}

func (app *application) processTask(jobType string, payload []byte) error {
	switch jobType {
	case "email:verify":
		var data struct {
			Email           string `json:"user_email"`
			FirstName       string `json:"first_name"`
			ActivationToken string `json:"activation_token"`
		}
		if err := json.Unmarshal(payload, &data); err != nil {
			return err
		}
		return app.mailer.Send(data.Email, "email_verify.tmpl", data)

	case "email:welcome":
		var data struct {
			Email     string `json:"user_email"`
			FirstName string `json:"first_name"`
		}
		if err := json.Unmarshal(payload, &data); err != nil {
			return err
		}
		return app.mailer.Send(data.Email, "user_welcome.tmpl", data)

	case "email:password_reset":
		var data struct {
			Email      string `json:"user_email"`
			FirstName  string `json:"first_name"`
			ResetToken string `json:"reset_token"`
		}
		if err := json.Unmarshal(payload, &data); err != nil {
			return err
		}
		return app.mailer.Send(data.Email, "password_reset.tmpl", data)

	default:
		return fmt.Errorf("unknown job type: %s", jobType)
	}
}
