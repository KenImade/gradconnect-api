package data

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"api.gradconnect.com/internal/validator"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Review struct {
	ID               string          `json:"id"`
	EmployerID       string          `json:"employer_id"`
	UserID           string          `json:"-"`
	ProgrammeName    string          `json:"programme_name"`
	ApplicationYear  int             `json:"application_year"`
	Outcome          string          `json:"outcome"`
	StageBreakDown   json.RawMessage `json:"stage_breakdown"`
	DifficultyRating int             `json:"difficulty_rating"`
	ExperienceRating int             `json:"experience_rating"`
	Tips             *string         `json:"tips"`
	DegreeDiscipline *string         `json:"degree_discipline"`
	University       *string         `json:"university"`
	Status           string          `json:"status"`
	CreatedAt        time.Time       `json:"created_at"`
}

func validateStageBreakdown(v *validator.Validator, raw json.RawMessage) {
	var stages []map[string]interface{}
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

func ValidateReview(v *validator.Validator, review *Review) {
	v.Check(validator.IsValidUUID(review.EmployerID), "employer_id", "must be a valid UUID")
	v.Check(review.ProgrammeName != "", "programme_name", "must be provided")
	v.Check(len(review.ProgrammeName) <= 255, "programme_name", "must not be more than 255 characters")
	v.Check(review.ApplicationYear >= 2015, "application_year", "must not be before 2015")
	v.Check(review.ApplicationYear <= time.Now().Year(), "application_year", "must not be in the future")
	v.Check(validator.PermittedValue(review.Outcome, "offer", "waitlisted", "rejected", "withdrew"), "outcome", "must be one of: offer, waitlisted, rejected, withdrew")
	v.Check(review.DifficultyRating >= 1 && review.DifficultyRating <= 5, "difficulty_rating", "must be between 1 and 5")
	v.Check(review.ExperienceRating >= 1 && review.ExperienceRating <= 5, "experience_rating", "must be between 1 and 5")
	validateStageBreakdown(v, review.StageBreakDown)

	if review.Tips != nil {
		v.Check(len(*review.Tips) <= 5000, "tips", "must not be more than 5000 characters")
	}
	if review.DegreeDiscipline != nil {
		v.Check(len(*review.DegreeDiscipline) <= 255, "degree_discipline", "must not be more than 255 characters")
	}
	if review.University != nil {
		v.Check(len(*review.University) <= 255, "university", "must not be more than 255 characters")
	}
}

func ValidateReviewModeration(v *validator.Validator, status string) {
	v.Check(validator.PermittedValue(status, "approved", "rejected"), "status", "must be one of: approved, rejected")
}

type ReviewModel struct {
	DB *pgxpool.Pool
}

func (m ReviewModel) Insert(review *Review) error { return nil }

func (m ReviewModel) Get(id string) (*Review, error) { return nil, nil }

func (m ReviewModel) Update(review *Review) error { return nil }

func (m ReviewModel) Delete(id string) error { return nil }

func (m ReviewModel) GetAllByEmployerSlug(slug string, filters Filters) ([]*Review, Metadata, error) {
	query := fmt.Sprintf(`
		SELECT count(*) OVER(),
			r.id, r.employer_id, r.programme_name,
			r.application_year, r.outcome, r.stage_breakdown,
			r.difficulty_rating, r.experience_rating, r.tips,
			r.degree_discipline, r.university, r.created_at
		FROM review r
		INNER JOIN employer e ON e.id = r.employer_id
		WHERE e.slug = $1 AND r.status = 'approved'
		ORDER BY r.%s %s
		LIMIT $2 OFFSET $3
	`, filters.sortColumn(), filters.sortDirection())

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	args := []any{slug, filters.limit(), filters.offset()}

	rows, err := m.DB.Query(ctx, query, args...)
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
			&review.StageBreakDown,
			&review.DifficultyRating,
			&review.ExperienceRating,
			&review.Tips,
			&review.DegreeDiscipline,
			&review.University,
			&review.CreatedAt,
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

func (m ReviewModel) GetAllForModeration() {}
