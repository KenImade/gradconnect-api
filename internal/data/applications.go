package data

import (
	"context"
	"fmt"
	"time"

	"api.gradconnect.com/internal/validator"
	"github.com/jackc/pgx/v5/pgxpool"
)

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

type ApplicationTracker struct {
	ID            string            `json:"id"`
	UserID        string            `json:"-"`
	OpportunityID string            `json:"opportunity_id"`
	Status        ApplicationStatus `json:"status"`
	Notes         *string           `json:"notes"`
	CreatedAt     time.Time         `json:"created_at"`
	UpdatedAt     time.Time         `json:"updated_at"`
}

func ValidateApplicationTracker(v *validator.Validator, appTracker *ApplicationTracker) {
	v.Check(appTracker.Status != "", "status", "must be provided")
	v.Check(validator.PermittedValue(appTracker.Status, permittedStatuses...), "status", "invalid status")

	if appTracker.Notes != nil {
		v.Check(len(*appTracker.Notes) <= 5000, "notes", "must not be more than 5000 characters")
	}
}

type ApplicationTrackerModel struct {
	DB *pgxpool.Pool
}

func (m ApplicationTrackerModel) Add() {}

func (m ApplicationTrackerModel) Remove() {}

func (m ApplicationTrackerModel) Update() {}

type ApplicationTrackerResponse struct {
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

func (m ApplicationTrackerModel) List(ctx context.Context, db DBTX, userID, status string, filters Filters) ([]ApplicationTrackerResponse, Metadata, error) {
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
	applications := []ApplicationTrackerResponse{}

	for rows.Next() {
		var app ApplicationTrackerResponse

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
