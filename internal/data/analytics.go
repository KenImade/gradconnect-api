package data

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// AnalyticsCounts holds the top-of-dashboard scalar metrics.
// Field names are JSON-tagged to match the API spec verbatim — the handler
// can drop this straight into the envelope.
type AnalyticsCounts struct {
	UsersTotal               int `json:"users_total"`
	UsersVerified            int `json:"users_verified"`
	UsersRegisteredLast7Days int `json:"users_registered_last_7_days"`
	EmployersTotal           int `json:"employers_total"`
	EmployersVerified        int `json:"employers_verified"`
	OpportunitiesTotal       int `json:"opportunities_total"`
	OpportunitiesOpen        int `json:"opportunities_open"`
	ReviewsTotal             int `json:"reviews_total"`
	ReviewsPendingModeration int `json:"reviews_pending_moderation"`
	BookmarksTotal           int `json:"bookmarks_total"`
	ApplicationsTotal        int `json:"applications_total"`
	SessionsActive           int `json:"sessions_active"`
}

// TimeSeriesPoint is one bucket in a 30-day series. Date is YYYY-MM-DD
// in Africa/Lagos. Count is zero when nothing happened that day —
// the SQL fills gaps via generate_series so the chart never has holes.
type TimeSeriesPoint struct {
	Date  string `json:"date"`
	Count int    `json:"count"`
}

type TopEmployer struct {
	ID               string `json:"id"`
	Name             string `json:"name"`
	Slug             string `json:"slug"`
	BookmarkCount    int    `json:"bookmark_count"`
	ReviewCount      int    `json:"review_count"`
	OpportunityCount int    `json:"opportunity_count"`
}

type TopOpportunity struct {
	ID            string     `json:"id"`
	Title         string     `json:"title"`
	Slug          string     `json:"slug"`
	EmployerName  string     `json:"employer_name"`
	BookmarkCount int        `json:"bookmark_count"`
	Deadline      *time.Time `json:"deadline"`
}

// RecentJob is the latest cron_run per job_name. Status is derived:
// completed_at != null → "completed", else → "running".
type RecentJob struct {
	JobName         string     `json:"job_name"`
	LastRunAt       time.Time  `json:"last_run_at"`
	LastRunEnqueued int        `json:"last_run_enqueued"`
	LastRunStatus   string     `json:"last_run_status"`
	CompletedAt     *time.Time `json:"completed_at,omitempty"`
}

// Response Data Models
type AnalyticsResponse struct {
	Counts           *AnalyticsCounts    `json:"counts"`
	TimeSeries       AnalyticsTimeSeries `json:"time_series"`
	TopEmployers     []TopEmployer       `json:"top_employers"`
	TopOpportunities []TopOpportunity    `json:"top_opportunities"`
	RecentJobs       []RecentJob         `json:"recent_jobs"`
}

type AnalyticsTimeSeries struct {
	Registrations    []TimeSeriesPoint `json:"registrations"`
	Bookmarks        []TimeSeriesPoint `json:"bookmarks"`
	ReviewsSubmitted []TimeSeriesPoint `json:"reviews_submitted"`
}

type AnalyticsModel struct {
	DB *pgxpool.Pool
}

// GetCounts returns all top-line scalar metrics in a single round-trip.
// Each subquery is independent; combining them into one statement saves
// 11 round trips with no functional cost.
func (m AnalyticsModel) GetCounts(ctx context.Context) (*AnalyticsCounts, error) {
	const query = `
        SELECT
            (SELECT COUNT(*) FROM app_user) AS users_total,
            (SELECT COUNT(*) FROM app_user WHERE email_verified = true) AS users_verified,
            (SELECT COUNT(*) FROM app_user WHERE created_at >= now() - interval '7 days') AS users_registered_last_7_days,
            (SELECT COUNT(*) FROM employer) AS employers_total,
            (SELECT COUNT(*) FROM employer WHERE is_verified = true) AS employers_verified,
            (SELECT COUNT(*) FROM opportunity) AS opportunities_total,
            (SELECT COUNT(*) FROM opportunity WHERE is_active = true
                AND (opens_at IS NULL OR CURRENT_DATE >= opens_at)
                AND (deadline IS NULL OR CURRENT_DATE <= deadline)) AS opportunities_open,
            (SELECT COUNT(*) FROM review) AS reviews_total,
            (SELECT COUNT(*) FROM review WHERE status = 'pending') AS reviews_pending_moderation,
            (SELECT COUNT(*) FROM bookmark) AS bookmarks_total,
            (SELECT COUNT(*) FROM application_track) AS applications_total,
            (SELECT COUNT(*) FROM session WHERE expires_at > now()) AS sessions_active`

	c := &AnalyticsCounts{}
	err := m.DB.QueryRow(ctx, query).Scan(
		&c.UsersTotal,
		&c.UsersVerified,
		&c.UsersRegisteredLast7Days,
		&c.EmployersTotal,
		&c.EmployersVerified,
		&c.OpportunitiesTotal,
		&c.OpportunitiesOpen,
		&c.ReviewsTotal,
		&c.ReviewsPendingModeration,
		&c.BookmarksTotal,
		&c.ApplicationsTotal,
		&c.SessionsActive,
	)
	if err != nil {
		return nil, err
	}
	return c, nil
}

// timeSeries runs a 30-day windowed count, filling gaps with zero.
// table is the source table; dateCol is the column to bucket by.
// Caller is responsible for ensuring inputs are safe (these are not
// user-supplied — they're hardcoded in the call sites below).
func (m AnalyticsModel) timeSeries(ctx context.Context, table, dateCol string, extraWhere string) ([]TimeSeriesPoint, error) {
	// generate_series produces every day in the 30-day window,
	// LEFT JOIN to the actual data fills missing days with 0.
	// Days are bucketed in Africa/Lagos to match how operators think.
	query := `
        WITH days AS (
            SELECT generate_series(
                (now() AT TIME ZONE 'Africa/Lagos')::date - interval '29 days',
                (now() AT TIME ZONE 'Africa/Lagos')::date,
                interval '1 day'
            )::date AS day
        )
        SELECT
            to_char(d.day, 'YYYY-MM-DD') AS date,
            COALESCE(COUNT(t.` + dateCol + `), 0)::int AS count
        FROM days d
        LEFT JOIN ` + table + ` t
            ON (t.` + dateCol + ` AT TIME ZONE 'Africa/Lagos')::date = d.day
            ` + extraWhere + `
        GROUP BY d.day
        ORDER BY d.day`

	rows, err := m.DB.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	points := make([]TimeSeriesPoint, 0, 30)
	for rows.Next() {
		var p TimeSeriesPoint
		if err := rows.Scan(&p.Date, &p.Count); err != nil {
			return nil, err
		}
		points = append(points, p)
	}
	return points, rows.Err()
}

func (m AnalyticsModel) RegistrationsTimeSeries(ctx context.Context) ([]TimeSeriesPoint, error) {
	return m.timeSeries(ctx, "app_user", "created_at", "")
}

func (m AnalyticsModel) BookmarksTimeSeries(ctx context.Context) ([]TimeSeriesPoint, error) {
	return m.timeSeries(ctx, "bookmark", "created_at", "")
}

func (m AnalyticsModel) ReviewsTimeSeries(ctx context.Context) ([]TimeSeriesPoint, error) {
	// Only count submission, not moderation date — the question is "are
	// graduates contributing" not "is the moderation queue moving."
	return m.timeSeries(ctx, "review", "created_at", "")
}

// TopEmployers returns the 10 employers with the most bookmarks across
// their opportunities. Joins three tables but the query is tight at
// MVP scale — count(*) over a join with appropriate indexes.
func (m AnalyticsModel) TopEmployers(ctx context.Context, limit int) ([]TopEmployer, error) {
	const query = `
        SELECT
            e.id,
            e.name,
            e.slug,
            COALESCE(b.bookmark_count, 0) AS bookmark_count,
            COALESCE(r.review_count, 0) AS review_count,
            COALESCE(o.opportunity_count, 0) AS opportunity_count
        FROM employer e
        LEFT JOIN (
            SELECT o.employer_id, COUNT(b.id) AS bookmark_count
            FROM opportunity o
            INNER JOIN bookmark b ON b.opportunity_id = o.id
            GROUP BY o.employer_id
        ) b ON b.employer_id = e.id
        LEFT JOIN (
            SELECT employer_id, COUNT(*) AS review_count
            FROM review
            WHERE status = 'approved'
            GROUP BY employer_id
        ) r ON r.employer_id = e.id
        LEFT JOIN (
            SELECT employer_id, COUNT(*) AS opportunity_count
            FROM opportunity
            GROUP BY employer_id
        ) o ON o.employer_id = e.id
        ORDER BY bookmark_count DESC, e.name ASC
        LIMIT $1`

	rows, err := m.DB.Query(ctx, query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]TopEmployer, 0, limit)
	for rows.Next() {
		var t TopEmployer
		if err := rows.Scan(&t.ID, &t.Name, &t.Slug, &t.BookmarkCount, &t.ReviewCount, &t.OpportunityCount); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

func (m AnalyticsModel) TopOpportunities(ctx context.Context, limit int) ([]TopOpportunity, error) {
	const query = `
        SELECT
            o.id,
            o.title,
            o.slug,
            e.name AS employer_name,
            COUNT(b.id) AS bookmark_count,
            o.deadline
        FROM opportunity o
        INNER JOIN employer e ON e.id = o.employer_id
        LEFT JOIN bookmark b ON b.opportunity_id = o.id
        GROUP BY o.id, e.name
        ORDER BY bookmark_count DESC, o.title ASC
        LIMIT $1`

	rows, err := m.DB.Query(ctx, query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]TopOpportunity, 0, limit)
	for rows.Next() {
		var t TopOpportunity
		if err := rows.Scan(&t.ID, &t.Title, &t.Slug, &t.EmployerName, &t.BookmarkCount, &t.Deadline); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// RecentJobs returns the most recent cron_run row per job_name.
// Status is derived: completed_at != null → completed, else running.
func (m AnalyticsModel) RecentJobs(ctx context.Context) ([]RecentJob, error) {
	const query = `
        SELECT DISTINCT ON (job_name)
            job_name,
            started_at,
            enqueued_count,
            completed_at,
            CASE WHEN completed_at IS NOT NULL THEN 'completed' ELSE 'running' END AS status
        FROM cron_run
        ORDER BY job_name, started_at DESC`

	rows, err := m.DB.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []RecentJob{}
	for rows.Next() {
		var j RecentJob
		if err := rows.Scan(&j.JobName, &j.LastRunAt, &j.LastRunEnqueued, &j.CompletedAt, &j.LastRunStatus); err != nil {
			return nil, err
		}
		out = append(out, j)
	}
	return out, rows.Err()
}
