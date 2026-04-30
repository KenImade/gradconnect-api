package data

import (
	"context"
	"errors"
	"fmt"
	"time"

	"api.gradconnect.com/internal/validator"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// --- Domain types ---

type ApplicationStatus string

const (
	StatusInterested ApplicationStatus = "interested"
	StatusApplied    ApplicationStatus = "applied"
	StatusAssessment ApplicationStatus = "assessment"
	StatusInterview  ApplicationStatus = "interview"
	StatusOffer      ApplicationStatus = "offer"
	StatusRejected   ApplicationStatus = "rejected"
)

var permittedStatuses = []ApplicationStatus{
	StatusInterested, StatusApplied, StatusAssessment,
	StatusInterview, StatusOffer, StatusRejected,
}

// ApplicationTracker represents a row in the application_track table.
type ApplicationTracker struct {
	ID            string            `json:"id"`
	UserID        string            `json:"-"`
	OpportunityID string            `json:"opportunity_id"`
	Status        ApplicationStatus `json:"status"`
	Notes         *string           `json:"notes"`
	CreatedAt     time.Time         `json:"created_at"`
	UpdatedAt     time.Time         `json:"updated_at"`
}

// ApplicationTrackerListItem is the enriched shape returned by list endpoints,
// with joined opportunity and employer data.
type ApplicationTrackerListItem struct {
	ID          string                     `json:"id"`
	Status      ApplicationStatus          `json:"status"`
	Notes       *string                    `json:"notes"`
	UpdatedAt   time.Time                  `json:"updated_at"`
	Opportunity ApplicationOpportunityStub `json:"opportunity"`
}

type ApplicationOpportunityStub struct {
	ID       string                  `json:"id"`
	Title    string                  `json:"title"`
	Slug     string                  `json:"slug"`
	Type     string                  `json:"type"`
	Deadline time.Time               `json:"deadline"`
	Employer ApplicationEmployerStub `json:"employer"`
}

type ApplicationEmployerStub struct {
	Name    string `json:"name"`
	Slug    string `json:"slug"`
	LogoURL string `json:"logo_url"`
}

// --- Inputs ---

type CreateApplicationInput struct {
	OpportunityID string            `json:"opportunity_id"`
	Status        ApplicationStatus `json:"status"`
	Notes         *string           `json:"notes"`
}

type DeleteApplicationInput struct {
	ApplicationID string
}

type UpdateApplicationInput struct {
	Status *ApplicationStatus `json:"status"`
	Notes  *string            `json:"notes"`
}

// --- Input validators ---

func ValidateCreateApplicationInput(v *validator.Validator, input CreateApplicationInput) {
	v.Check(input.OpportunityID != "", "opportunity_id", "must be provided")
	v.Check(input.Status != "", "status", "must be provided")
	v.Check(validator.PermittedValue(input.Status, permittedStatuses...), "status", "invalid status")

	if input.Notes != nil {
		v.Check(len(*input.Notes) <= 5000, "notes", "must not be more than 5000 characters")
	}
}

func ValidateUpdateApplicationInput(v *validator.Validator, input UpdateApplicationInput) {
	if input.Status != nil {
		v.Check(validator.PermittedValue(*input.Status, permittedStatuses...), "status", "invalid status")
	}

	if input.Notes != nil {
		v.Check(len(*input.Notes) <= 5000, "notes", "must not be more than 5000 characters")
	}

	// At least one field must be provided — empty PATCH is a client error
	if input.Status == nil && input.Notes == nil {
		v.AddError("body", "at least one field must be provided")
	}
}

func ValidateApplicationStatusFilter(v *validator.Validator, status string) {
	if status == "" {
		return
	}
	v.Check(validator.PermittedValue(ApplicationStatus(status), permittedStatuses...), "status", "invalid status")
}

// --- Errors ---

var ErrDuplicateApplication = errors.New("application already exists")

// --- Model ---

type ApplicationTrackerModel struct {
	DB *pgxpool.Pool
}

func (m ApplicationTrackerModel) Add(ctx context.Context, db DBTX, userID string, input CreateApplicationInput) (*ApplicationTracker, error) {
	query := `
		INSERT INTO application_track (user_id, opportunity_id, status, notes)
		VALUES ($1, $2, $3, $4)
		RETURNING id, created_at, updated_at`

	tracker := &ApplicationTracker{
		UserID:        userID,
		OpportunityID: input.OpportunityID,
		Status:        input.Status,
		Notes:         input.Notes,
	}

	err := db.QueryRow(ctx, query, userID, input.OpportunityID, input.Status, input.Notes).
		Scan(&tracker.ID, &tracker.CreatedAt, &tracker.UpdatedAt)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			switch pgErr.Code {
			case "23505":
				return nil, ErrDuplicateApplication
			case "23503":
				return nil, ErrRecordNotFound
			}
		}
		return nil, err
	}

	return tracker, nil
}

func (m ApplicationTrackerModel) Update(ctx context.Context, db DBTX, userID, trackerID string, input UpdateApplicationInput) (*ApplicationTracker, error) {
	// Fetch current values to merge partial update
	current, err := m.GetByID(ctx, db, trackerID, userID)
	if err != nil {
		return nil, err
	}

	if input.Status != nil {
		current.Status = *input.Status
	}
	if input.Notes != nil {
		current.Notes = input.Notes
	}

	query := `
		UPDATE application_track
		SET status = $1, notes = $2
		WHERE id = $3 AND user_id = $4
		RETURNING updated_at`

	err = db.QueryRow(ctx, query, current.Status, current.Notes, trackerID, userID).
		Scan(&current.UpdatedAt)
	if err != nil {
		return nil, err
	}

	return current, nil
}

func (m ApplicationTrackerModel) Remove(ctx context.Context, db DBTX, userID, trackerID string) error {
	query := `
		DELETE FROM application_track
		WHERE id = $1 AND user_id = $2`

	result, err := db.Exec(ctx, query, trackerID, userID)
	if err != nil {
		return err
	}

	if result.RowsAffected() == 0 {
		return ErrRecordNotFound
	}

	return nil
}

func (m ApplicationTrackerModel) GetByID(ctx context.Context, db DBTX, trackerID, userID string) (*ApplicationTracker, error) {
	query := `
		SELECT id, user_id, opportunity_id, status, notes, created_at, updated_at
		FROM application_track
		WHERE id = $1 AND user_id = $2`

	tracker := &ApplicationTracker{}
	err := db.QueryRow(ctx, query, trackerID, userID).Scan(
		&tracker.ID,
		&tracker.UserID,
		&tracker.OpportunityID,
		&tracker.Status,
		&tracker.Notes,
		&tracker.CreatedAt,
		&tracker.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrRecordNotFound
		}
		return nil, err
	}

	return tracker, nil
}

func (m ApplicationTrackerModel) List(ctx context.Context, db DBTX, userID, status string, filters Filters) ([]ApplicationTrackerListItem, Metadata, error) {
	query := fmt.Sprintf(`
		SELECT
			count(*) OVER(),
			apptk.id, apptk.status, apptk.notes, apptk.updated_at,
			o.id, o.title, o.slug, o.type, o.deadline,
			e.name, e.slug, e.logo_url
		FROM application_track apptk
		INNER JOIN opportunity o ON o.id = apptk.opportunity_id
		INNER JOIN employer e ON e.id = o.employer_id
		WHERE apptk.user_id = $1
		  AND ($2 = '' OR apptk.status = $2::application_status_type)
		ORDER BY apptk.%s %s, apptk.id ASC
		LIMIT $3 OFFSET $4
	`, filters.sortColumn(), filters.sortDirection())

	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	args := []any{userID, status, filters.limit(), filters.offset()}

	rows, err := db.Query(ctx, query, args...)
	if err != nil {
		return nil, Metadata{}, err
	}
	defer rows.Close()

	totalRecords := 0
	applications := []ApplicationTrackerListItem{}

	for rows.Next() {
		var app ApplicationTrackerListItem

		err := rows.Scan(
			&totalRecords,
			&app.ID, &app.Status, &app.Notes, &app.UpdatedAt,
			&app.Opportunity.ID, &app.Opportunity.Title, &app.Opportunity.Slug,
			&app.Opportunity.Type, &app.Opportunity.Deadline,
			&app.Opportunity.Employer.Name, &app.Opportunity.Employer.Slug,
			&app.Opportunity.Employer.LogoURL,
		)
		if err != nil {
			return nil, Metadata{}, err
		}

		applications = append(applications, app)
	}

	if err = rows.Err(); err != nil {
		return nil, Metadata{}, err
	}

	metadata := calculateMetadata(totalRecords, filters.Page, filters.PageSize)

	return applications, metadata, nil
}
