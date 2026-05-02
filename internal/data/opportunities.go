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

type Opportunity struct {
	ID             string       `json:"id"`
	Title          string       `json:"title"`
	Slug           string       `json:"slug"`
	Type           string       `json:"type"`
	IntakeYear     int          `json:"intake_year"`
	Status         string       `json:"status"` // computed
	Description    string       `json:"description"`
	Requirements   *string      `json:"requirements"`
	Location       string       `json:"location"`
	DisciplineTags []string     `json:"discipline_tags"`
	OpensAt        *Date        `json:"opens_at"`
	Deadline       *Date        `json:"deadline"`
	DaysRemaining  *int         `json:"days_remaining"` // computed
	ApplicationURL string       `json:"application_url"`
	IsActive       bool         `json:"is_active"`
	SourceURL      *string      `json:"source_url"`
	CreatedAt      time.Time    `json:"created_at"`
	UpdatedAt      time.Time    `json:"updated_at"`
	Employer       EmployerStub `json:"employer"`
}

type EmployerStub struct {
	ID       string  `json:"id"`
	Name     string  `json:"name"`
	Slug     string  `json:"slug"`
	LogoURL  *string `json:"logo_url"`
	Industry string  `json:"industry"`
}

// --- Inputs ---

type CreateOpportunityInput struct {
	EmployerID     string   `json:"employer_id"`
	Title          string   `json:"title"`
	Slug           string   `json:"slug"`
	Type           string   `json:"type"`
	IntakeYear     int      `json:"intake_year"`
	Description    string   `json:"description"`
	Requirements   *string  `json:"requirements"`
	Location       string   `json:"location"`
	DisciplineTags []string `json:"discipline_tags"`
	OpensAt        *Date    `json:"opens_at"`
	Deadline       *Date    `json:"deadline"`
	ApplicationURL string   `json:"application_url"`
	IsActive       bool     `json:"is_active"`
	SourceURL      *string  `json:"source_url"`
}

type UpdateOpportunityInput struct {
	EmployerID     *string   `json:"employer_id"`
	Title          *string   `json:"title"`
	Slug           *string   `json:"slug"`
	Type           *string   `json:"type"`
	IntakeYear     *int      `json:"intake_year"`
	Description    *string   `json:"description"`
	Requirements   *string   `json:"requirements"`
	Location       *string   `json:"location"`
	DisciplineTags *[]string `json:"discipline_tags"`
	OpensAt        *Date     `json:"opens_at"`
	Deadline       *Date     `json:"deadline"`
	ApplicationURL *string   `json:"application_url"`
	SourceURL      *string   `json:"source_url"`
	IsActive       *bool     `json:"is_active"`
}

type OpportunityFilters struct {
	Search         string
	Type           string
	Status         string
	IntakeYear     int
	Industry       string
	Location       string
	Discipline     string
	EmployerSlug   string
	DeadlineBefore time.Time
	DeadlineAfter  time.Time
	Filters
}

// --- Validators ---

var permittedOpportunityTypes = []string{"graduate_trainee", "internship", "nysc", "industrial_attachment"}

func ValidateCreateOpportunityInput(v *validator.Validator, input CreateOpportunityInput) {
	v.Check(validator.IsValidUUID(input.EmployerID), "employer_id", "must be a valid UUID")

	v.Check(input.Title != "", "title", "must be provided")
	v.Check(len(input.Title) <= 255, "title", "must not be more than 255 characters")

	v.Check(input.Slug != "", "slug", "must be provided")
	v.Check(len(input.Slug) <= 255, "slug", "must not be more than 255 characters")
	v.Check(validator.Matches(input.Slug, validator.SlugRX), "slug", "must contain only lowercase letters, numbers, and hyphens")

	v.Check(validator.PermittedValue(input.Type, permittedOpportunityTypes...), "type", "must be one of: graduate_trainee, internship, nysc, industrial_attachment")

	currentYear := time.Now().Year()
	v.Check(input.IntakeYear >= currentYear-1, "intake_year", "must not be more than one year in the past")
	v.Check(input.IntakeYear <= currentYear+2, "intake_year", "must not be more than two years in the future")

	v.Check(input.Description != "", "description", "must be provided")

	if input.Requirements != nil {
		v.Check(len(*input.Requirements) <= 5000, "requirements", "must not be more than 5000 characters")
	}

	v.Check(input.Location != "", "location", "must be provided")
	v.Check(len(input.Location) <= 255, "location", "must not be more than 255 characters")

	v.Check(len(input.DisciplineTags) <= 20, "discipline_tags", "must not have more than 20 tags")
	for i, tag := range input.DisciplineTags {
		v.Check(tag != "", fmt.Sprintf("discipline_tags[%d]", i), "must not be empty")
		v.Check(len(tag) <= 100, fmt.Sprintf("discipline_tags[%d]", i), "must not be more than 100 characters")
	}

	if input.OpensAt != nil && input.Deadline != nil {
		v.Check(
			input.OpensAt.BeforeDate(*input.Deadline) || input.OpensAt.EqualDate(*input.Deadline),
			"opens_at",
			"must be before or equal to deadline",
		)
	}

	v.Check(input.ApplicationURL != "", "application_url", "must be provided")
	v.Check(len(input.ApplicationURL) <= 512, "application_url", "must not be more than 512 characters")
	v.Check(validator.Matches(input.ApplicationURL, validator.URLRX), "application_url", "must be a valid URL")

	if input.SourceURL != nil {
		v.Check(len(*input.SourceURL) <= 512, "source_url", "must not be more than 512 characters")
		v.Check(validator.Matches(*input.SourceURL, validator.URLRX), "source_url", "must be a valid URL")
	}
}

func ValidateUpdateOpportunityInput(v *validator.Validator, input UpdateOpportunityInput) {
	if input.Title != nil {
		v.Check(*input.Title != "", "title", "must not be empty")
		v.Check(len(*input.Title) <= 255, "title", "must not be more than 255 characters")
	}
	if input.Slug != nil {
		v.Check(*input.Slug != "", "slug", "must not be empty")
		v.Check(len(*input.Slug) <= 255, "slug", "must not be more than 255 characters")
		v.Check(validator.Matches(*input.Slug, validator.SlugRX), "slug", "must contain only lowercase letters, numbers, and hyphens")
	}
	if input.Type != nil {
		v.Check(validator.PermittedValue(*input.Type, permittedOpportunityTypes...), "type", "must be one of: graduate_trainee, internship, nysc, industrial_attachment")
	}
	if input.IntakeYear != nil {
		currentYear := time.Now().Year()
		v.Check(*input.IntakeYear >= currentYear-1, "intake_year", "must not be more than one year in the past")
		v.Check(*input.IntakeYear <= currentYear+2, "intake_year", "must not be more than two years in the future")
	}
	if input.Description != nil {
		v.Check(*input.Description != "", "description", "must not be empty")
	}
	if input.Requirements != nil {
		v.Check(len(*input.Requirements) <= 5000, "requirements", "must not be more than 5000 characters")
	}
	if input.Location != nil {
		v.Check(*input.Location != "", "location", "must not be empty")
		v.Check(len(*input.Location) <= 255, "location", "must not be more than 255 characters")
	}
	if input.DisciplineTags != nil {
		v.Check(len(*input.DisciplineTags) <= 20, "discipline_tags", "must not have more than 20 tags")
		for i, tag := range *input.DisciplineTags {
			v.Check(tag != "", fmt.Sprintf("discipline_tags[%d]", i), "must not be empty")
			v.Check(len(tag) <= 100, fmt.Sprintf("discipline_tags[%d]", i), "must not be more than 100 characters")
		}
	}
	if input.OpensAt != nil && input.Deadline != nil {
		v.Check(
			input.OpensAt.BeforeDate(*input.Deadline) || input.OpensAt.EqualDate(*input.Deadline),
			"opens_at",
			"must be before or equal to deadline",
		)
	}
	if input.ApplicationURL != nil {
		v.Check(*input.ApplicationURL != "", "application_url", "must not be empty")
		v.Check(len(*input.ApplicationURL) <= 512, "application_url", "must not be more than 512 characters")
		v.Check(validator.Matches(*input.ApplicationURL, validator.URLRX), "application_url", "must be a valid URL")
	}
	if input.SourceURL != nil {
		v.Check(len(*input.SourceURL) <= 512, "source_url", "must not be more than 512 characters")
		v.Check(validator.Matches(*input.SourceURL, validator.URLRX), "source_url", "must be a valid URL")
	}
}

// --- Errors ---

var ErrDuplicateOpportunitySlug = errors.New("opportunity with this slug already exists")

// --- Model ---

type OpportunityModel struct {
	DB *pgxpool.Pool
}

// selectOpportunityColumns is the shared column list for SELECT queries that return
// the full enriched Opportunity shape (with computed status + days_remaining + employer).
const selectOpportunityColumns = `
	o.id, o.title, o.slug, o.type, o.intake_year,
	CASE
		WHEN o.is_active = false THEN 'withdrawn'
		WHEN o.opens_at IS NOT NULL AND CURRENT_DATE < o.opens_at THEN 'upcoming'
		WHEN o.deadline IS NOT NULL AND CURRENT_DATE > o.deadline THEN 'closed'
		ELSE 'open'
	END AS status,
	o.description, o.requirements, o.location, o.discipline_tags,
	o.opens_at, o.deadline,
	CASE WHEN o.deadline IS NULL THEN NULL ELSE (o.deadline - CURRENT_DATE)::int END AS days_remaining,
	o.application_url, o.is_active, o.source_url, o.created_at, o.updated_at,
	e.id, e.name, e.slug, e.logo_url, e.industry`

// scanOpportunity reads the columns defined in selectOpportunityColumns into an Opportunity.
// Caller passes any extra leading scan targets (e.g. count(*) OVER()).
func scanOpportunity(row pgx.Row, extra ...any) (*Opportunity, error) {
	var opportunity Opportunity
	scanTargets := append(extra,
		&opportunity.ID, &opportunity.Title, &opportunity.Slug,
		&opportunity.Type, &opportunity.IntakeYear, &opportunity.Status,
		&opportunity.Description, &opportunity.Requirements, &opportunity.Location,
		&opportunity.DisciplineTags, &opportunity.OpensAt, &opportunity.Deadline,
		&opportunity.DaysRemaining, &opportunity.ApplicationURL, &opportunity.IsActive,
		&opportunity.SourceURL, &opportunity.CreatedAt, &opportunity.UpdatedAt,
		&opportunity.Employer.ID, &opportunity.Employer.Name, &opportunity.Employer.Slug,
		&opportunity.Employer.LogoURL, &opportunity.Employer.Industry,
	)
	err := row.Scan(scanTargets...)
	if err != nil {
		return nil, err
	}
	return &opportunity, nil
}

func (m OpportunityModel) Insert(ctx context.Context, db DBTX, input CreateOpportunityInput) (*Opportunity, error) {
	query := `
		INSERT INTO opportunity
			(employer_id, title, slug, type, intake_year, description, requirements,
			 location, discipline_tags, opens_at, deadline, application_url, source_url)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
		RETURNING id`

	var id string
	err := db.QueryRow(ctx, query,
		input.EmployerID,
		input.Title,
		input.Slug,
		input.Type,
		input.IntakeYear,
		input.Description,
		input.Requirements,
		input.Location,
		input.DisciplineTags,
		input.OpensAt,
		input.Deadline,
		input.ApplicationURL,
		input.SourceURL,
	).Scan(&id)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			switch pgErr.Code {
			case "23505":
				return nil, ErrDuplicateOpportunitySlug
			case "23503":
				return nil, ErrRecordNotFound // employer doesn't exist
			}
		}
		return nil, err
	}

	// Re-fetch with joins to return the full enriched shape
	return m.GetByID(ctx, db, id)
}

func (m OpportunityModel) GetByID(ctx context.Context, db DBTX, id string) (*Opportunity, error) {
	query := fmt.Sprintf(`
		SELECT %s
		FROM opportunity o
		INNER JOIN employer e ON e.id = o.employer_id
		WHERE o.id = $1`, selectOpportunityColumns)

	opportunity, err := scanOpportunity(db.QueryRow(ctx, query, id))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrRecordNotFound
		}
		return nil, err
	}

	return opportunity, nil
}

func (m OpportunityModel) GetBySlug(ctx context.Context, db DBTX, slug string) (*Opportunity, error) {
	query := fmt.Sprintf(`
		SELECT %s
		FROM opportunity o
		INNER JOIN employer e ON e.id = o.employer_id
		WHERE o.slug = $1`, selectOpportunityColumns)

	opportunity, err := scanOpportunity(db.QueryRow(ctx, query, slug))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrRecordNotFound
		}
		return nil, err
	}

	return opportunity, nil
}

func (m OpportunityModel) Update(ctx context.Context, db DBTX, id string, input UpdateOpportunityInput) (*Opportunity, error) {
	current, err := m.GetByID(ctx, db, id)
	if err != nil {
		return nil, err
	}

	if input.Title != nil {
		current.Title = *input.Title
	}
	if input.Slug != nil {
		current.Slug = *input.Slug
	}
	if input.Type != nil {
		current.Type = *input.Type
	}
	if input.IntakeYear != nil {
		current.IntakeYear = *input.IntakeYear
	}
	if input.Description != nil {
		current.Description = *input.Description
	}
	if input.Requirements != nil {
		current.Requirements = input.Requirements
	}
	if input.Location != nil {
		current.Location = *input.Location
	}
	if input.DisciplineTags != nil {
		current.DisciplineTags = *input.DisciplineTags
	}
	if input.OpensAt != nil {
		current.OpensAt = input.OpensAt
	}
	if input.Deadline != nil {
		current.Deadline = input.Deadline
	}
	if input.ApplicationURL != nil {
		current.ApplicationURL = *input.ApplicationURL
	}
	if input.SourceURL != nil {
		current.SourceURL = input.SourceURL
	}
	if input.IsActive != nil {
		current.IsActive = *input.IsActive
	}

	query := `
        UPDATE opportunity
        SET title = $1, slug = $2, type = $3, intake_year = $4,
            description = $5, requirements = $6, location = $7,
            discipline_tags = $8, opens_at = $9, deadline = $10,
            application_url = $11, source_url = $12, is_active = $13
        WHERE id = $14`

	result, err := db.Exec(ctx, query,
		current.Title, current.Slug, current.Type, current.IntakeYear,
		current.Description, current.Requirements, current.Location,
		current.DisciplineTags, current.OpensAt, current.Deadline,
		current.ApplicationURL, current.SourceURL, current.IsActive,
		id,
	)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return nil, ErrDuplicateOpportunitySlug
		}
		return nil, err
	}

	if result.RowsAffected() == 0 {
		return nil, ErrRecordNotFound
	}

	// Re-fetch to pick up updated_at and computed fields
	return m.GetByID(ctx, db, id)
}

func (m OpportunityModel) Delete(ctx context.Context, db DBTX, id string) error {
	result, err := db.Exec(ctx, `DELETE FROM opportunity WHERE id = $1`, id)
	if err != nil {
		return err
	}

	if result.RowsAffected() == 0 {
		return ErrRecordNotFound
	}

	return nil
}

func (m OpportunityModel) GetAll(ctx context.Context, db DBTX, input OpportunityFilters) ([]*Opportunity, Metadata, error) {
	query := fmt.Sprintf(`
		SELECT
			count(*) OVER(),
			%s
		FROM opportunity o
		INNER JOIN employer e ON e.id = o.employer_id
		WHERE (o.search_vector @@ plainto_tsquery('english', $1) OR $1 = '')
			AND ($2::opportunity_type IS NULL OR o.type = $2)
			AND (o.intake_year = $3 OR $3 = 0)
			AND (e.industry = $4 OR $4 = '')
			AND (e.slug = $5 OR $5 = '')
			AND (o.location ILIKE '%%' || $6 || '%%' OR $6 = '')
			AND ($7 = '' OR $7 = ANY(o.discipline_tags))
			AND ($8::date IS NULL OR o.deadline <= $8)
			AND ($9::date IS NULL OR o.deadline >= $9)
			AND (
					($10 = 'all')                             
					OR ($10 = 'withdrawn' AND o.is_active = false)
					OR ($10 = 'upcoming' AND o.is_active = true AND o.opens_at IS NOT NULL AND CURRENT_DATE < o.opens_at)
					OR ($10 = 'closed' AND o.is_active = true AND o.deadline IS NOT NULL AND CURRENT_DATE > o.deadline)
					OR ($10 = 'open' AND o.is_active = true
						AND (o.opens_at IS NULL OR CURRENT_DATE >= o.opens_at)
						AND (o.deadline IS NULL OR CURRENT_DATE <= o.deadline))
					OR ($10 = 'open_or_upcoming' AND o.is_active = true
						AND (o.deadline IS NULL OR CURRENT_DATE <= o.deadline))
				)
			ORDER BY o.%s %s, o.id ASC
			LIMIT $11 OFFSET $12
	`, selectOpportunityColumns, input.Filters.sortColumn(), input.Filters.sortDirection())

	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
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
		input.EmployerSlug,
		input.Location,
		input.Discipline,
		deadlineBefore,
		deadlineAfter,
		input.Status,
		input.Filters.limit(),
		input.Filters.offset(),
	}

	rows, err := db.Query(ctx, query, args...)
	if err != nil {
		return nil, Metadata{}, err
	}
	defer rows.Close()

	totalRecords := 0
	opportunities := []*Opportunity{}

	for rows.Next() {
		opportunity, err := scanOpportunity(rows, &totalRecords)
		if err != nil {
			return nil, Metadata{}, err
		}
		opportunities = append(opportunities, opportunity)
	}

	if err = rows.Err(); err != nil {
		return nil, Metadata{}, err
	}

	metadata := calculateMetadata(totalRecords, input.Filters.Page, input.Filters.PageSize)

	return opportunities, metadata, nil
}

// Upsert inserts a new opportunity or updates an existing one matched by
// (employer_id, title, intake_year). That triple is the natural dedup key
// for graduate programmes — same employer, same role title, same recruitment
// cycle. Re-uploading a corrected CSV refreshes editable fields without
// creating phantom duplicates.
//
// Protected fields not touched by upsert:
//   - search_vector  (maintained by trigger; never written directly)
//
// Note: opportunity has no `version` column, no review aggregates, and no
// admin-toggled flags (is_active is editable via upsert; this is the
// intended behaviour — re-uploading a CSV with is_active=false should
// withdraw the listing).
func (m OpportunityModel) Upsert(ctx context.Context, db DBTX, input CreateOpportunityInput) (*Opportunity, error) {
	query := `
        INSERT INTO opportunity
            (employer_id, title, slug, type, intake_year, description, requirements,
             location, discipline_tags, opens_at, deadline, application_url, source_url,
             is_active)
        VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
        ON CONFLICT (employer_id, title, intake_year) DO UPDATE SET
            slug            = EXCLUDED.slug,
            type            = EXCLUDED.type,
            description     = EXCLUDED.description,
            requirements    = EXCLUDED.requirements,
            location        = EXCLUDED.location,
            discipline_tags = EXCLUDED.discipline_tags,
            opens_at        = EXCLUDED.opens_at,
            deadline        = EXCLUDED.deadline,
            application_url = EXCLUDED.application_url,
            source_url      = EXCLUDED.source_url,
            is_active       = EXCLUDED.is_active,
            updated_at      = now()
        RETURNING id`

	var id string
	err := db.QueryRow(ctx, query,
		input.EmployerID,
		input.Title,
		input.Slug,
		input.Type,
		input.IntakeYear,
		input.Description,
		input.Requirements,
		input.Location,
		input.DisciplineTags,
		input.OpensAt,
		input.Deadline,
		input.ApplicationURL,
		input.SourceURL,
		input.IsActive,
	).Scan(&id)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			switch pgErr.Code {
			case "23505":
				// Slug collision (different employer/title/year combo wants
				// the same slug). Conflict target above only handles the
				// (employer_id, title, intake_year) constraint.
				return nil, ErrDuplicateOpportunitySlug
			case "23503":
				return nil, ErrRecordNotFound // employer doesn't exist
			}
		}
		return nil, err
	}

	// Re-fetch with joins to return the full enriched shape (status,
	// days_remaining, employer stub).
	return m.GetByID(ctx, db, id)
}
