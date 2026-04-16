package data

import (
	"context"
	"fmt"
	"time"

	"api.gradconnect.com/internal/validator"
	"github.com/jackc/pgx/v5/pgxpool"
)

type OpportunityFilters struct {
	Search         string
	Type           string
	Status         string
	IntakeYear     int
	Industry       string
	Location       string
	Discipline     string
	DeadlineBefore time.Time
	DeadlineAfter  time.Time
	Filters
}

type Opportunity struct {
	ID             string        `json:"id"`
	EmployerID     string        `json:"employer_id"`
	Title          string        `json:"title"`
	Slug           string        `json:"slug"`
	Type           string        `json:"type"`
	IntakeYear     int           `json:"intake_year"`
	Status         string        `json:"status"` // computed
	Description    string        `json:"description"`
	Requirements   *string       `json:"requirements"`
	Location       string        `json:"location"`
	DisciplineTags []string      `json:"discipline_tags"`
	OpensAt        *time.Time    `json:"opens_at"`
	Deadline       *time.Time    `json:"deadline"`
	DaysRemaining  *int          `json:"days_remaining"` // computed
	ApplicationURL string        `json:"application_url"`
	IsActive       bool          `json:"is_active"`
	SourceURL      *string       `json:"source_url"`
	CreatedAt      time.Time     `json:"created_at"`
	Employer       *EmployerStub `json:"employer,omitempty"`
}

type EmployerStub struct {
	ID       string  `json:"id"`
	Name     string  `json:"name"`
	Slug     string  `json:"slug"`
	LogoURL  *string `json:"logo_url"`
	Industry string  `json:"industry"`
}

func ValidateOpportunity(v *validator.Validator, opportunity *Opportunity) {
	// Employer reference
	v.Check(validator.IsValidUUID(opportunity.EmployerID), "employer_id", "must be a valid UUID")

	// Title
	v.Check(opportunity.Title != "", "title", "must be provided")
	v.Check(len(opportunity.Title) <= 255, "title", "must not be more than 255 characters")

	// Slug
	v.Check(opportunity.Slug != "", "slug", "must be provided")
	v.Check(len(opportunity.Slug) <= 255, "slug", "must not be more than 255 characters")
	v.Check(validator.Matches(opportunity.Slug, validator.SlugRX), "slug", "must contain only lowercase letters, numbers, and hyphens")

	// Type enum
	v.Check(
		validator.PermittedValue(opportunity.Type, "graduate_trainee", "internship", "nysc", "industrial_attachment"),
		"type",
		"must be one of: graduate_trainee, internship, nysc, industrial_attachment",
	)

	// Intake year
	currentYear := time.Now().Year()
	v.Check(opportunity.IntakeYear >= currentYear-1, "intake_year", "must not be more than one year in the past")
	v.Check(opportunity.IntakeYear <= currentYear+2, "intake_year", "must not be more than two years in the future")

	// Description
	v.Check(opportunity.Description != "", "description", "must be provided")

	// Requirements (optional, length limit only)
	if opportunity.Requirements != nil {
		v.Check(len(*opportunity.Requirements) <= 5000, "requirements", "must not be more than 5000 characters")
	}

	// Location
	v.Check(opportunity.Location != "", "location", "must be provided")
	v.Check(len(opportunity.Location) <= 255, "location", "must not be more than 255 characters")

	// Discipline tags
	v.Check(len(opportunity.DisciplineTags) <= 20, "discipline_tags", "must not have more than 20 tags")
	for i, tag := range opportunity.DisciplineTags {
		v.Check(tag != "", fmt.Sprintf("discipline_tags[%d]", i), "must not be empty")
		v.Check(len(tag) <= 100, fmt.Sprintf("discipline_tags[%d]", i), "must not be more than 100 characters")
	}

	// Dates
	if opportunity.OpensAt != nil && opportunity.Deadline != nil {
		v.Check(
			opportunity.OpensAt.Before(*opportunity.Deadline) || opportunity.OpensAt.Equal(*opportunity.Deadline),
			"opens_at",
			"must be before or equal to deadline",
		)
	}

	// Application URL
	v.Check(opportunity.ApplicationURL != "", "application_url", "must be provided")
	v.Check(len(opportunity.ApplicationURL) <= 512, "application_url", "must not be more than 512 characters")
	v.Check(validator.Matches(opportunity.ApplicationURL, validator.URLRX), "application_url", "must be a valid URL")

	// Source URL (optional)
	if opportunity.SourceURL != nil {
		v.Check(len(*opportunity.SourceURL) <= 512, "source_url", "must not be more than 512 characters")
		v.Check(validator.Matches(*opportunity.SourceURL, validator.URLRX), "source_url", "must be a valid URL")
	}
}

type OpportunityModel struct {
	DB *pgxpool.Pool
}

func (m OpportunityModel) Insert(opportunity *Opportunity) error { return nil }

func (m OpportunityModel) Get(id string) (*Opportunity, error) { return nil, nil }

func (m OpportunityModel) Update(Opportunity *Opportunity) error { return nil }

func (m OpportunityModel) Delete(id string) error { return nil }

func (m OpportunityModel) GetAll(input OpportunityFilters) ([]*Opportunity, Metadata, error) {
	query := fmt.Sprintf(`
        SELECT
            count(*) OVER(),
            o.id, o.employer_id, o.title, o.slug, o.type, o.intake_year,
            CASE
                WHEN o.is_active = false THEN 'withdrawn'
                WHEN o.opens_at IS NOT NULL AND CURRENT_DATE < o.opens_at THEN 'upcoming'
                WHEN o.deadline IS NOT NULL AND CURRENT_DATE > o.deadline THEN 'closed'
                ELSE 'open'
            END AS status,
            o.description, o.requirements, o.location, o.discipline_tags,
            o.opens_at, o.deadline,
            CASE WHEN o.deadline IS NULL THEN NULL ELSE (o.deadline - CURRENT_DATE)::int END AS days_remaining,
            o.application_url, o.is_active, o.source_url, o.created_at,
            e.id, e.name, e.slug, e.logo_url, e.industry
        FROM opportunity o
        INNER JOIN employer e ON e.id = o.employer_id
        WHERE (o.search_vector @@ plainto_tsquery('english', $1) OR $1 = '')
        AND ($2::opportunity_type IS NULL OR o.type = $2)
        AND (o.intake_year = $3 OR $3 = 0)
        AND (e.industry = $4 OR $4 = '')
        AND (o.location ILIKE '%%' || $5 || '%%' OR $5 = '')
        AND ($6 = '' OR $6 = ANY(o.discipline_tags))
        AND ($7::date IS NULL OR o.deadline <= $7)
        AND ($8::date IS NULL OR o.deadline >= $8)
        AND (
            ($9 = 'all')
            OR ($9 = 'withdrawn' AND o.is_active = false)
            OR ($9 = 'upcoming' AND o.is_active = true AND o.opens_at IS NOT NULL AND CURRENT_DATE < o.opens_at)
            OR ($9 = 'closed' AND o.is_active = true AND o.deadline IS NOT NULL AND CURRENT_DATE > o.deadline)
            OR ($9 = 'open' AND o.is_active = true
                AND (o.opens_at IS NULL OR CURRENT_DATE >= o.opens_at)
                AND (o.deadline IS NULL OR CURRENT_DATE <= o.deadline))
        )
        ORDER BY o.%s %s, o.intake_year DESC
        LIMIT $10 OFFSET $11
    `, input.Filters.sortColumn(), input.Filters.sortDirection())

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	var deadlineBefore, deadlineAfter any
	if !input.DeadlineBefore.IsZero() {
		deadlineBefore = input.DeadlineBefore
	}
	if !input.DeadlineAfter.IsZero() {
		deadlineAfter = input.DeadlineAfter
	}

	var typeFilter any
	if input.Type != "" {
		typeFilter = input.Type
	}

	args := []any{
		input.Search,
		typeFilter,
		input.IntakeYear,
		input.Industry,
		input.Location,
		input.Discipline,
		deadlineBefore,
		deadlineAfter,
		input.Status,
		input.Filters.limit(),
		input.Filters.offset(),
	}

	rows, err := m.DB.Query(ctx, query, args...)
	if err != nil {
		return nil, Metadata{}, err
	}
	defer rows.Close()

	totalRecords := 0
	opportunities := []*Opportunity{}

	for rows.Next() {
		var opportunity Opportunity
		opportunity.Employer = &EmployerStub{}

		err := rows.Scan(
			&totalRecords,
			&opportunity.ID, &opportunity.EmployerID, &opportunity.Title, &opportunity.Slug,
			&opportunity.Type, &opportunity.IntakeYear, &opportunity.Status,
			&opportunity.Description, &opportunity.Requirements, &opportunity.Location,
			&opportunity.DisciplineTags, &opportunity.OpensAt, &opportunity.Deadline,
			&opportunity.DaysRemaining, &opportunity.ApplicationURL, &opportunity.IsActive,
			&opportunity.SourceURL, &opportunity.CreatedAt,
			&opportunity.Employer.ID, &opportunity.Employer.Name, &opportunity.Employer.Slug,
			&opportunity.Employer.LogoURL, &opportunity.Employer.Industry,
		)
		if err != nil {
			return nil, Metadata{}, err
		}

		opportunities = append(opportunities, &opportunity)
	}

	if err = rows.Err(); err != nil {
		return nil, Metadata{}, err
	}

	metadata := calculateMetadata(totalRecords, input.Filters.Page, input.Filters.PageSize)

	return opportunities, metadata, nil
}
