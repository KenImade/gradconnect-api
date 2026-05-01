package worker

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"api.gradconnect.com/internal/data"
	"github.com/jackc/pgx/v5"
)

var ErrAlreadyRanToday = errors.New("cron job already ran today")

// cronInterval — how often the cron loop wakes up to check whether any
// scheduled job is due. 15 minutes is a sensible balance: tight enough
// that a job firing at 18:00 happens within ~15min of that time, loose
// enough not to thrash.
const cronInterval = 15 * time.Minute

// cronTargetHour — the hour (Africa/Lagos) when the daily reminder
// digest is sent. Set to 18 for evening.
const cronTargetHour = 18

// cronJobName — identifier used in the cron_run table for idempotency.
const cronJobName = "deadline_reminders"

// daysAhead — bookmarks with deadlines exactly this many days from
// today are included in the reminder digest.
const daysAhead = 3

// Cron starts the periodic check loop. It blocks until ctx is cancelled.
// Designed to run as a goroutine alongside the worker pool.
func (p *Pool) Cron(ctx context.Context, baseURL, frontendURL string) {
	ticker := time.NewTicker(cronInterval)
	defer ticker.Stop()

	p.logger.Info("cron loop started", "interval", cronInterval, "target_hour", cronTargetHour)

	// Run a check immediately on startup in case we're starting up
	// during the target window after a restart.
	p.maybeRunDeadlineReminders(ctx, baseURL, frontendURL)

	for {
		select {
		case <-ctx.Done():
			p.logger.Info("cron loop stopped")
			return
		case <-ticker.C:
			p.maybeRunDeadlineReminders(ctx, baseURL, frontendURL)
		}
	}
}

// maybeRunDeadlineReminders enqueues today's reminder emails if:
//  1. Current Lagos time is within the target hour
//  2. We haven't already run today (checked via cron_run table)
func (p *Pool) maybeRunDeadlineReminders(ctx context.Context, baseURL, frontendURL string) {
	lagos, err := time.LoadLocation("Africa/Lagos")
	if err != nil {
		p.logger.Error("loading Africa/Lagos timezone", "err", err)
		return
	}

	now := time.Now().In(lagos)

	// Only proceed during the target hour.
	if now.Hour() != cronTargetHour {
		return
	}

	today := now.Format("2006-01-02")

	// Try to claim today's run with INSERT ... ON CONFLICT DO NOTHING.
	// If another worker already inserted, RowsAffected() == 0 and we exit.
	var runID string
	err = p.db.QueryRow(ctx, `
        INSERT INTO cron_run (job_name, run_date)
        VALUES ($1, $2::date)
        ON CONFLICT (job_name, run_date) DO NOTHING
        RETURNING id
    `, cronJobName, today).Scan(&runID)

	if err != nil {
		if err == pgx.ErrNoRows {
			// Already ran today — silent success.
			return
		}
		p.logger.Error("claiming cron run", "err", err)
		return
	}

	p.logger.Info("running deadline reminder cron", "run_id", runID, "date", today)

	enqueued := p.enqueueDeadlineReminders(ctx, baseURL, frontendURL)

	// Mark complete.
	_, err = p.db.Exec(ctx, `
        UPDATE cron_run
        SET completed_at = now(), enqueued_count = $1
        WHERE id = $2
    `, enqueued, runID)
	if err != nil {
		p.logger.Error("marking cron run complete", "err", err, "run_id", runID)
	}

	p.logger.Info("deadline reminder cron complete", "run_id", runID, "enqueued", enqueued)
}

// enqueueDeadlineReminders queries for recipients and inserts one
// task_queue row per user. Returns the count of tasks enqueued.
func (p *Pool) enqueueDeadlineReminders(ctx context.Context, baseURL, frontendURL string) int {
	bookmarks := data.BookmarkModel{}
	recipients, err := bookmarks.FindDeadlineReminderRecipients(ctx, p.db, daysAhead)
	if err != nil {
		p.logger.Error("finding deadline reminder recipients", "err", err)
		return 0
	}

	enqueued := 0
	for _, r := range recipients {
		payload, err := json.Marshal(map[string]any{
			"recipient":    r.Email,
			"first_name":   r.FirstName,
			"base_url":     baseURL,
			"frontend_url": frontendURL,
			"bookmarks":    r.Bookmarks,
		})
		if err != nil {
			p.logger.Error("marshaling reminder payload",
				"err", err, "user_id", r.UserID)
			continue
		}

		_, err = p.db.Exec(ctx, `
            INSERT INTO task_queue (job_type, payload, run_at)
            VALUES ('email:deadline_reminder', $1::jsonb, now())
        `, payload)
		if err != nil {
			p.logger.Error("enqueuing reminder task",
				"err", err, "user_id", r.UserID)
			continue
		}

		enqueued++
	}

	return enqueued
}
