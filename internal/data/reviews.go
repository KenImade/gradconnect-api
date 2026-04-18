package data

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"api.gradconnect.com/internal/validator"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// --- Domain types ---

// Review represents a row in the review table.
type Review struct {
	ID               string          `json:"id"`
	EmployerID       string          `json:"employer_id"`
	UserID           string          `json:"-"`
	ProgrammeName    string          `json:"programme_name"`
	ApplicationYear  int             `json:"application_year"`
	Outcome          string          `json:"outcome"`
	StageBreakdown   json.RawMessage `json:"stage_breakdown"`
	DifficultyRating int             `json:"difficulty_rating"`
	ExperienceRating int             `json:"experience_rating"`
	Tips             *string         `json:"tips"`
	DegreeDiscipline *string         `json:"degree_discipline"`
	University       *string         `json:"university"`
	Status           string          `json:"status"`
	CreatedAt        time.Time       `json:"created_at"`
	UpdatedAt        time.Time       `json:"updated_at"`
}

// ReviewSubmissionResponse is the minimal response returned after a user
// submits a review, before moderation.
type ReviewSubmissionResponse struct {
	ID            string    `json:"id"`
	EmployerID    string    `json:"employer_id"`
	ProgrammeName string    `json:"programme_name"`
	Status        string    `json:"status"`
	CreatedAt     time.Time `json:"created_at"`
}

// --- Inputs ---

type CreateReviewInput struct {
	EmployerID       string          `json:"employer_id"`
	ProgrammeName    string          `json:"programme_name"`
	ApplicationYear  int             `json:"application_year"`
	Outcome          string          `json:"outcome"`
	StageBreakdown   json.RawMessage `json:"stage_breakdown"`
	DifficultyRating int             `json:"difficulty_rating"`
	ExperienceRating int             `json:"experience_rating"`
	Tips             *string         `json:"tips"`
	DegreeDiscipline *string         `json:"degree_discipline"`
	University       *string         `json:"university"`
}

type UpdateReviewInput struct {
	ProgrammeName    *string         `json:"programme_name"`
	ApplicationYear  *int            `json:"application_year"`
	Outcome          *string         `json:"outcome"`
	StageBreakdown   json.RawMessage `json:"stage_breakdown"`
	DifficultyRating *int            `json:"difficulty_rating"`
	ExperienceRating *int            `json:"experience_rating"`
	Tips             *string         `json:"tips"`
	DegreeDiscipline *string         `json:"degree_discipline"`
	University       *string         `json:"university"`
}

type ModerateReviewInput struct {
	Status string `json:"status"`
}

// --- Validators ---

var permittedReviewOutcomes = []string{"offer", "waitlisted", "rejected", "withdrew"}
var permittedReviewStatuses = []string{"approved", "rejected"}

func validateStageBreakdown(v *validator.Validator, raw json.RawMessage) {
	if len(raw) == 0 {
		v.AddError("stage_breakdown", "must be provided")
		return
	}

	var stages []map[string]any
	if err := json.Unmarshal(raw, &stages); err != nil {
		v.AddError("stage_breakdown", "must be a valid JSON array")
		return
	}

	v.Check(len(stages) >= 1, "stage_breakdown", "must contain at least one stage")
	v.Check(len(stages) <= 10, "stage_breakdown", "must not contain more than 10 stages")

	for i, stage := range stages {
		name, ok := stage["stage_name"].(string)
		if !ok || name == "" {
			v.AddError(fmt.Sprintf("stage_breakdown[%d].stage_name", i), "must be provided")
		}
	}
}

func ValidateCreateReviewInput(v *validator.Validator, input CreateReviewInput) {
	v.Check(validator.IsValidUUID(input.EmployerID), "employer_id", "must be a valid UUID")
	v.Check(input.ProgrammeName != "", "programme_name", "must be provided")
	v.Check(len(input.ProgrammeName) <= 255, "programme_name", "must not be more than 255 characters")
	v.Check(input.ApplicationYear >= 2015, "application_year", "must not be before 2015")
	v.Check(input.ApplicationYear <= time.Now().Year(), "application_year", "must not be in the future")
	v.Check(validator.PermittedValue(input.Outcome, permittedReviewOutcomes...), "outcome", "must be one of: offer, waitlisted, rejected, withdrew")
	v.Check(input.DifficultyRating >= 1 && input.DifficultyRating <= 5, "difficulty_rating", "must be between 1 and 5")
	v.Check(input.ExperienceRating >= 1 && input.ExperienceRating <= 5, "experience_rating", "must be between 1 and 5")

	validateStageBreakdown(v, input.StageBreakdown)

	if input.Tips != nil {
		v.Check(len(*input.Tips) <= 5000, "tips", "must not be more than 5000 characters")
	}
	if input.DegreeDiscipline != nil {
		v.Check(len(*input.DegreeDiscipline) <= 255, "degree_discipline", "must not be more than 255 characters")
	}
	if input.University != nil {
		v.Check(len(*input.University) <= 255, "university", "must not be more than 255 characters")
	}
}

func ValidateUpdateReviewInput(v *validator.Validator, input UpdateReviewInput) {
	if input.ProgrammeName != nil {
		v.Check(*input.ProgrammeName != "", "programme_name", "must not be empty")
		v.Check(len(*input.ProgrammeName) <= 255, "programme_name", "must not be more than 255 characters")
	}
	if input.ApplicationYear != nil {
		v.Check(*input.ApplicationYear >= 2015, "application_year", "must not be before 2015")
		v.Check(*input.ApplicationYear <= time.Now().Year(), "application_year", "must not be in the future")
	}
	if input.Outcome != nil {
		v.Check(validator.PermittedValue(*input.Outcome, permittedReviewOutcomes...), "outcome", "must be one of: offer, waitlisted, rejected, withdrew")
	}
	if input.DifficultyRating != nil {
		v.Check(*input.DifficultyRating >= 1 && *input.DifficultyRating <= 5, "difficulty_rating", "must be between 1 and 5")
	}
	if input.ExperienceRating != nil {
		v.Check(*input.ExperienceRating >= 1 && *input.ExperienceRating <= 5, "experience_rating", "must be between 1 and 5")
	}
	if len(input.StageBreakdown) > 0 {
		validateStageBreakdown(v, input.StageBreakdown)
	}
	if input.Tips != nil {
		v.Check(len(*input.Tips) <= 5000, "tips", "must not be more than 5000 characters")
	}
	if input.DegreeDiscipline != nil {
		v.Check(len(*input.DegreeDiscipline) <= 255, "degree_discipline", "must not be more than 255 characters")
	}
	if input.University != nil {
		v.Check(len(*input.University) <= 255, "university", "must not be more than 255 characters")
	}
}

func ValidateModerateReviewInput(v *validator.Validator, input ModerateReviewInput) {
	v.Check(input.Status != "", "status", "must be provided")
	v.Check(validator.PermittedValue(input.Status, permittedReviewStatuses...), "status", "must be one of: approved, rejected")
}

// --- Errors ---

var ErrDuplicateReview = errors.New("review already exists for this user and programme")

// --- Model ---

type ReviewModel struct {
	DB *pgxpool.Pool
}

func (m ReviewModel) Insert(ctx context.Context, db DBTX, userID string, input CreateReviewInput) (*Review, error) {
	query := `
		INSERT INTO review
			(employer_id, user_id, programme_name, application_year, outcome,
			 stage_breakdown, difficulty_rating, experience_rating,
			 tips, degree_discipline, university, status)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, 'pending')
		RETURNING id, employer_id, user_id, programme_name, application_year,
		          outcome, stage_breakdown, difficulty_rating, experience_rating,
		          tips, degree_discipline, university, status, created_at, updated_at`

	review := &Review{}
	err := db.QueryRow(ctx, query,
		input.EmployerID,
		userID,
		input.ProgrammeName,
		input.ApplicationYear,
		input.Outcome,
		input.StageBreakdown,
		input.DifficultyRating,
		input.ExperienceRating,
		input.Tips,
		input.DegreeDiscipline,
		input.University,
	).Scan(
		&review.ID,
		&review.EmployerID,
		&review.UserID,
		&review.ProgrammeName,
		&review.ApplicationYear,
		&review.Outcome,
		&review.StageBreakdown,
		&review.DifficultyRating,
		&review.ExperienceRating,
		&review.Tips,
		&review.DegreeDiscipline,
		&review.University,
		&review.Status,
		&review.CreatedAt,
		&review.UpdatedAt,
	)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			switch pgErr.Code {
			case "23503": // foreign_key_violation — employer doesn't exist
				return nil, ErrRecordNotFound
			case "23505": // unique_violation — duplicate review
				return nil, ErrDuplicateReview
			}
		}
		return nil, err
	}

	return review, nil
}

func (m ReviewModel) GetByID(ctx context.Context, db DBTX, reviewID string) (*Review, error) {
	query := `
		SELECT id, employer_id, user_id, programme_name, application_year,
		       outcome, stage_breakdown, difficulty_rating, experience_rating,
		       tips, degree_discipline, university, status, created_at, updated_at
		FROM review
		WHERE id = $1`

	review := &Review{}
	err := db.QueryRow(ctx, query, reviewID).Scan(
		&review.ID,
		&review.EmployerID,
		&review.UserID,
		&review.ProgrammeName,
		&review.ApplicationYear,
		&review.Outcome,
		&review.StageBreakdown,
		&review.DifficultyRating,
		&review.ExperienceRating,
		&review.Tips,
		&review.DegreeDiscipline,
		&review.University,
		&review.Status,
		&review.CreatedAt,
		&review.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrRecordNotFound
		}
		return nil, err
	}

	return review, nil
}

func (m ReviewModel) Update(ctx context.Context, db DBTX, userID, reviewID string, input UpdateReviewInput) (*Review, error) {
	// Fetch current values to merge partial update, scoped to the owner
	current, err := m.GetByID(ctx, db, reviewID)
	if err != nil {
		return nil, err
	}

	if current.UserID != userID {
		return nil, ErrRecordNotFound
	}

	if input.ProgrammeName != nil {
		current.ProgrammeName = *input.ProgrammeName
	}
	if input.ApplicationYear != nil {
		current.ApplicationYear = *input.ApplicationYear
	}
	if input.Outcome != nil {
		current.Outcome = *input.Outcome
	}
	if len(input.StageBreakdown) > 0 {
		current.StageBreakdown = input.StageBreakdown
	}
	if input.DifficultyRating != nil {
		current.DifficultyRating = *input.DifficultyRating
	}
	if input.ExperienceRating != nil {
		current.ExperienceRating = *input.ExperienceRating
	}
	if input.Tips != nil {
		current.Tips = input.Tips
	}
	if input.DegreeDiscipline != nil {
		current.DegreeDiscipline = input.DegreeDiscipline
	}
	if input.University != nil {
		current.University = input.University
	}

	// Edits require re-moderation
	current.Status = "pending"

	query := `
		UPDATE review
		SET programme_name = $1, application_year = $2, outcome = $3,
		    stage_breakdown = $4, difficulty_rating = $5, experience_rating = $6,
		    tips = $7, degree_discipline = $8, university = $9, status = $10
		WHERE id = $11 AND user_id = $12
		RETURNING updated_at`

	err = db.QueryRow(ctx, query,
		current.ProgrammeName,
		current.ApplicationYear,
		current.Outcome,
		current.StageBreakdown,
		current.DifficultyRating,
		current.ExperienceRating,
		current.Tips,
		current.DegreeDiscipline,
		current.University,
		current.Status,
		reviewID,
		userID,
	).Scan(&current.UpdatedAt)
	if err != nil {
		return nil, err
	}

	return current, nil
}

func (m ReviewModel) Delete(ctx context.Context, db DBTX, userID, reviewID string) error {
	query := `
		DELETE FROM review
		WHERE id = $1 AND user_id = $2`

	result, err := db.Exec(ctx, query, reviewID, userID)
	if err != nil {
		return err
	}

	if result.RowsAffected() == 0 {
		return ErrRecordNotFound
	}

	return nil
}

func (m ReviewModel) Moderate(ctx context.Context, db DBTX, reviewID, status string) (*Review, error) {
	query := `
		UPDATE review
		SET status = $1
		WHERE id = $2
		RETURNING id, employer_id, user_id, programme_name, application_year,
		          outcome, stage_breakdown, difficulty_rating, experience_rating,
		          tips, degree_discipline, university, status, created_at, updated_at`

	review := &Review{}
	err := db.QueryRow(ctx, query, status, reviewID).Scan(
		&review.ID,
		&review.EmployerID,
		&review.UserID,
		&review.ProgrammeName,
		&review.ApplicationYear,
		&review.Outcome,
		&review.StageBreakdown,
		&review.DifficultyRating,
		&review.ExperienceRating,
		&review.Tips,
		&review.DegreeDiscipline,
		&review.University,
		&review.Status,
		&review.CreatedAt,
		&review.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrRecordNotFound
		}
		return nil, err
	}

	return review, nil
}

func (m ReviewModel) GetAllByEmployerSlug(ctx context.Context, db DBTX, slug string, filters Filters) ([]*Review, Metadata, error) {
	query := fmt.Sprintf(`
		SELECT count(*) OVER(),
		       r.id, r.employer_id, r.programme_name,
		       r.application_year, r.outcome, r.stage_breakdown,
		       r.difficulty_rating, r.experience_rating, r.tips,
		       r.degree_discipline, r.university, r.status,
		       r.created_at, r.updated_at
		FROM review r
		INNER JOIN employer e ON e.id = r.employer_id
		WHERE e.slug = $1 AND r.status = 'approved'
		ORDER BY r.%s %s, r.id ASC
		LIMIT $2 OFFSET $3
	`, filters.sortColumn(), filters.sortDirection())

	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	args := []any{slug, filters.limit(), filters.offset()}

	rows, err := db.Query(ctx, query, args...)
	if err != nil {
		return nil, Metadata{}, err
	}
	defer rows.Close()

	totalRecords := 0
	reviews := []*Review{}

	for rows.Next() {
		var review Review

		err := rows.Scan(
			&totalRecords,
			&review.ID,
			&review.EmployerID,
			&review.ProgrammeName,
			&review.ApplicationYear,
			&review.Outcome,
			&review.StageBreakdown,
			&review.DifficultyRating,
			&review.ExperienceRating,
			&review.Tips,
			&review.DegreeDiscipline,
			&review.University,
			&review.Status,
			&review.CreatedAt,
			&review.UpdatedAt,
		)
		if err != nil {
			return nil, Metadata{}, err
		}

		reviews = append(reviews, &review)
	}

	if err = rows.Err(); err != nil {
		return nil, Metadata{}, err
	}

	metadata := calculateMetadata(totalRecords, filters.Page, filters.PageSize)

	return reviews, metadata, nil
}

func (m ReviewModel) GetAllForModeration(ctx context.Context, db DBTX, filters Filters) ([]*Review, Metadata, error) {
	query := fmt.Sprintf(`
		SELECT count(*) OVER(),
		       id, employer_id, user_id, programme_name,
		       application_year, outcome, stage_breakdown,
		       difficulty_rating, experience_rating, tips,
		       degree_discipline, university, status,
		       created_at, updated_at
		FROM review
		WHERE status = 'pending'
		ORDER BY %s %s, id ASC
		LIMIT $1 OFFSET $2
	`, filters.sortColumn(), filters.sortDirection())

	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	rows, err := db.Query(ctx, query, filters.limit(), filters.offset())
	if err != nil {
		return nil, Metadata{}, err
	}
	defer rows.Close()

	totalRecords := 0
	reviews := []*Review{}

	for rows.Next() {
		var review Review

		err := rows.Scan(
			&totalRecords,
			&review.ID,
			&review.EmployerID,
			&review.UserID,
			&review.ProgrammeName,
			&review.ApplicationYear,
			&review.Outcome,
			&review.StageBreakdown,
			&review.DifficultyRating,
			&review.ExperienceRating,
			&review.Tips,
			&review.DegreeDiscipline,
			&review.University,
			&review.Status,
			&review.CreatedAt,
			&review.UpdatedAt,
		)
		if err != nil {
			return nil, Metadata{}, err
		}

		reviews = append(reviews, &review)
	}

	if err = rows.Err(); err != nil {
		return nil, Metadata{}, err
	}

	metadata := calculateMetadata(totalRecords, filters.Page, filters.PageSize)

	return reviews, metadata, nil
}
