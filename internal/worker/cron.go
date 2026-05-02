package worker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
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

// Time-of-day targets (Africa/Lagos hour) for daily jobs.
// Cleanup runs every 6 hours, so it has its own hour-set check.
const (
	cronTargetHourReminders = 18
	cronTargetHourRecalc    = 3
)

// cronJobName — identifier used in the cron_run table for idempotency.
const (
	cronJobName                = "deadline_reminders"
	cronJobNameRecalcRatings   = "recalculate_ratings"
	cronJobNameCleanupSessions = "cleanup_sessions"
)

// daysAhead — bookmarks with deadlines exactly this many days from
// today are included in the reminder digest.
const daysAhead = 3

// cleanupHours — hours of the Lagos day at which session cleanup runs.
// Every 6 hours: 00, 06, 12, 18.
var cleanupHours = map[int]bool{0: true, 6: true, 12: true, 18: true}

// Cron starts the periodic check loop. It blocks until ctx is cancelled.
// Designed to run as a goroutine alongside the worker pool.
func (p *Pool) Cron(ctx context.Context, baseURL, frontendURL string) {
	ticker := time.NewTicker(cronInterval)
	defer ticker.Stop()

	p.logger.Info("cron loop started", "interval", cronInterval)

	// Run a check immediately on startup in case we're starting up
	// during the target window after a restart.
	p.runScheduledChecks(ctx, baseURL, frontendURL)

	for {
		select {
		case <-ctx.Done():
			p.logger.Info("cron loop stopped")
			return
		case <-ticker.C:
			p.runScheduledChecks(ctx, baseURL, frontendURL)
		}
	}
}

// runScheduledChecks fires every cronInterval. Each maybe* function
// guards itself by time-of-day and idempotency, so this is a thin
// dispatcher that doesn't need its own scheduling logic.
func (p *Pool) runScheduledChecks(ctx context.Context, baseURL, frontendURL string) {
	p.maybeRunDeadlineReminders(ctx, baseURL, frontendURL)
	p.maybeRunRecalcRatings(ctx)
	p.maybeRunCleanupSessions(ctx)
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
	if now.Hour() != cronTargetHourReminders {
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

// maybeRunRecalcRatings recomputes employer rating aggregates daily
// at 03:00 Lagos. Same once-per-day idempotency pattern as reminders.
func (p *Pool) maybeRunRecalcRatings(ctx context.Context) {
	lagos, err := time.LoadLocation("Africa/Lagos")
	if err != nil {
		p.logger.Error("loading Africa/Lagos timezone", "err", err)
		return
	}

	now := time.Now().In(lagos)
	if now.Hour() != cronTargetHourRecalc {
		return
	}

	today := now.Format("2006-01-02")

	var runID string
	err = p.db.QueryRow(ctx, `
        INSERT INTO cron_run (job_name, run_date)
        VALUES ($1, $2::date)
        ON CONFLICT (job_name, run_date) DO NOTHING
        RETURNING id
    `, cronJobNameRecalcRatings, today).Scan(&runID)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return
		}
		p.logger.Error("claiming recalc cron run", "err", err)
		return
	}

	p.logger.Info("running ratings recalc cron", "run_id", runID, "date", today)

	count := p.recalculateAllEmployerRatings(ctx)

	_, err = p.db.Exec(ctx, `
        UPDATE cron_run
        SET completed_at = now(), enqueued_count = $1
        WHERE id = $2
    `, count, runID)
	if err != nil {
		p.logger.Error("marking recalc cron run complete", "err", err, "run_id", runID)
	}

	p.logger.Info("ratings recalc cron complete", "run_id", runID, "recalculated", count)
}

// maybeRunCleanupSessions deletes expired sessions every 6 hours.
// Unlike daily jobs, this uses ON CONFLICT DO UPDATE — multiple runs per
// day are expected and each "claims" the row freshly so cron_run shows
// the most recent run, not the first one of the day.
func (p *Pool) maybeRunCleanupSessions(ctx context.Context) {
	lagos, err := time.LoadLocation("Africa/Lagos")
	if err != nil {
		p.logger.Error("loading Africa/Lagos timezone", "err", err)
		return
	}

	now := time.Now().In(lagos)
	if !cleanupHours[now.Hour()] {
		return
	}

	// We only want to fire once per target hour, not once every 15min
	// during that hour. Cheap guard: check whether the most recent
	// cron_run for this job started within the current hour.
	var lastStarted time.Time
	err = p.db.QueryRow(ctx, `
        SELECT started_at FROM cron_run
        WHERE job_name = $1
        ORDER BY started_at DESC
        LIMIT 1
    `, cronJobNameCleanupSessions).Scan(&lastStarted)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		p.logger.Error("checking last cleanup run", "err", err)
		return
	}

	// If we ran within the current Lagos hour, skip.
	if !lastStarted.IsZero() && lastStarted.In(lagos).Hour() == now.Hour() &&
		lastStarted.In(lagos).YearDay() == now.YearDay() {
		return
	}

	today := now.Format("2006-01-02")

	var runID string
	err = p.db.QueryRow(ctx, `
        INSERT INTO cron_run (job_name, run_date)
        VALUES ($1, $2::date)
        ON CONFLICT (job_name, run_date) DO UPDATE
            SET started_at = now(),
                completed_at = NULL,
                enqueued_count = 0
        RETURNING id
    `, cronJobNameCleanupSessions, today).Scan(&runID)
	if err != nil {
		p.logger.Error("recording cleanup cron run", "err", err)
		return
	}

	p.logger.Info("running session cleanup cron", "run_id", runID)

	tag, err := p.db.Exec(ctx, `DELETE FROM session WHERE expires_at < now()`)
	if err != nil {
		p.logger.Error("deleting expired sessions", "err", err)
		return
	}
	deleted := int(tag.RowsAffected())

	_, err = p.db.Exec(ctx, `
        UPDATE cron_run
        SET completed_at = now(), enqueued_count = $1
        WHERE id = $2
    `, deleted, runID)
	if err != nil {
		p.logger.Error("marking cleanup cron run complete", "err", err, "run_id", runID)
	}

	p.logger.Info("session cleanup cron complete", "run_id", runID, "deleted", deleted)
}

// recalculateAllEmployerRatings runs the per-employer recalc query
// for every employer and returns the count successfully processed.
// Per-row failures are logged and don't abort the run — partial
// success is the right outcome here.
func (p *Pool) recalculateAllEmployerRatings(ctx context.Context) int {
	rows, err := p.db.Query(ctx, `SELECT id FROM employer`)
	if err != nil {
		p.logger.Error("listing employers for recalc", "err", err)
		return 0
	}
	defer rows.Close()

	const updateQuery = `
        UPDATE employer
        SET avg_difficulty_rating = sub.avg_diff,
            avg_experience_rating = sub.avg_exp,
            review_count = sub.cnt
        FROM (
            SELECT
                AVG(difficulty_rating)::numeric(3,2) AS avg_diff,
                AVG(experience_rating)::numeric(3,2) AS avg_exp,
                COUNT(*) AS cnt
            FROM review
            WHERE employer_id = $1 AND status = 'approved'
        ) AS sub
        WHERE id = $1`

	count := 0
	for rows.Next() {
		var employerID string
		if err := rows.Scan(&employerID); err != nil {
			p.logger.Error("scanning employer id for recalc", "err", err)
			continue
		}

		if _, err := p.db.Exec(ctx, updateQuery, employerID); err != nil {
			p.logger.Error("recalculating ratings for employer",
				"employer_id", employerID, "err", err)
			continue
		}
		count++
	}

	if err := rows.Err(); err != nil {
		p.logger.Error("iterating employers for recalc", "err", err)
	}

	return count
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

// RunRecalcRatingsNow recomputes employer ratings immediately, bypassing
// the time-of-day check. The daily idempotency guard via cron_run still
// applies — returns ErrAlreadyRanToday if today's run already happened.
func (p *Pool) RunRecalcRatingsNow(ctx context.Context) (int, error) {
	lagos, err := time.LoadLocation("Africa/Lagos")
	if err != nil {
		return 0, fmt.Errorf("loading timezone: %w", err)
	}
	today := time.Now().In(lagos).Format("2006-01-02")

	var runID string
	err = p.db.QueryRow(ctx, `
        INSERT INTO cron_run (job_name, run_date)
        VALUES ($1, $2::date)
        ON CONFLICT (job_name, run_date) DO NOTHING
        RETURNING id
    `, cronJobNameRecalcRatings, today).Scan(&runID)

	if errors.Is(err, pgx.ErrNoRows) {
		return 0, ErrAlreadyRanToday
	}
	if err != nil {
		return 0, fmt.Errorf("claiming cron run: %w", err)
	}

	count := p.recalculateAllEmployerRatings(ctx)

	_, err = p.db.Exec(ctx, `
        UPDATE cron_run
        SET completed_at = now(), enqueued_count = $1
        WHERE id = $2
    `, count, runID)
	if err != nil {
		p.logger.Error("marking recalc cron run complete", "err", err)
	}

	return count, nil
}

// RunCleanupSessionsNow deletes expired sessions immediately. Naturally
// idempotent — second run finds no expired sessions. Records the run in
// cron_run for the recent_jobs panel.
func (p *Pool) RunCleanupSessionsNow(ctx context.Context) (int, error) {
	lagos, err := time.LoadLocation("Africa/Lagos")
	if err != nil {
		return 0, fmt.Errorf("loading timezone: %w", err)
	}
	today := time.Now().In(lagos).Format("2006-01-02")

	var runID string
	err = p.db.QueryRow(ctx, `
        INSERT INTO cron_run (job_name, run_date)
        VALUES ($1, $2::date)
        ON CONFLICT (job_name, run_date) DO UPDATE
            SET started_at = now(),
                completed_at = NULL,
                enqueued_count = 0
        RETURNING id
    `, cronJobNameCleanupSessions, today).Scan(&runID)
	if err != nil {
		return 0, fmt.Errorf("recording cron run: %w", err)
	}

	tag, err := p.db.Exec(ctx, `DELETE FROM session WHERE expires_at < now()`)
	if err != nil {
		return 0, fmt.Errorf("deleting expired sessions: %w", err)
	}
	deleted := int(tag.RowsAffected())

	_, err = p.db.Exec(ctx, `
        UPDATE cron_run
        SET completed_at = now(), enqueued_count = $1
        WHERE id = $2
    `, deleted, runID)
	if err != nil {
		p.logger.Error("marking cleanup cron run complete", "err", err)
	}

	return deleted, nil
}
