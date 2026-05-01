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

type Employer struct {
	ID                  string          `json:"id"`
	Name                string          `json:"name"`
	Slug                string          `json:"slug"`
	Industry            string          `json:"industry"`
	Size                *string         `json:"size"`
	HQLocation          *string         `json:"hq_location"`
	Offices             json.RawMessage `json:"offices"`
	LogoURL             *string         `json:"logo_url"`
	Overview            *string         `json:"overview"`
	Culture             *string         `json:"culture"`
	Website             *string         `json:"website"`
	SocialLinks         json.RawMessage `json:"social_links"`
	IsVerified          bool            `json:"is_verified"`
	AvgDifficultyRating *float64        `json:"avg_difficulty_rating"`
	AvgExperienceRating *float64        `json:"avg_experience_rating"`
	ReviewCount         int             `json:"review_count"`
	Version             int             `json:"version"`
	CreatedAt           time.Time       `json:"created_at"`
	UpdatedAt           time.Time       `json:"updated_at"`
}

// --- Inputs ---

type CreateEmployerInput struct {
	Name        string          `json:"name"`
	Slug        string          `json:"slug"`
	Industry    string          `json:"industry"`
	Size        *string         `json:"size"`
	HQLocation  *string         `json:"hq_location"`
	Offices     json.RawMessage `json:"offices"`
	LogoURL     *string         `json:"logo_url"`
	Overview    *string         `json:"overview"`
	Culture     *string         `json:"culture"`
	Website     *string         `json:"website"`
	SocialLinks json.RawMessage `json:"social_links"`
	IsVerified  bool            `json:"is_verified"`
}

type UpdateEmployerInput struct {
	Name        *string         `json:"name"`
	Slug        *string         `json:"slug"`
	Industry    *string         `json:"industry"`
	Size        *string         `json:"size"`
	HQLocation  *string         `json:"hq_location"`
	Offices     json.RawMessage `json:"offices"`
	LogoURL     *string         `json:"logo_url"`
	Overview    *string         `json:"overview"`
	Culture     *string         `json:"culture"`
	Website     *string         `json:"website"`
	SocialLinks json.RawMessage `json:"social_links"`
	IsVerified  *bool           `json:"is_verified"`
}

// --- Validators ---

func ValidateCreateEmployerInput(v *validator.Validator, input CreateEmployerInput) {
	v.Check(input.Name != "", "name", "must be provided")
	v.Check(len(input.Name) <= 255, "name", "must not be more than 255 characters")

	v.Check(input.Slug != "", "slug", "must be provided")
	v.Check(len(input.Slug) <= 255, "slug", "must not be more than 255 characters")
	v.Check(validator.Matches(input.Slug, validator.SlugRX), "slug", "must contain only lowercase letters, numbers, and hyphens")

	v.Check(input.Industry != "", "industry", "must be provided")
	v.Check(len(input.Industry) <= 100, "industry", "must not be more than 100 characters")

	if input.Size != nil {
		v.Check(len(*input.Size) <= 50, "size", "must not be more than 50 characters")
	}
	if input.HQLocation != nil {
		v.Check(len(*input.HQLocation) <= 255, "hq_location", "must not be more than 255 characters")
	}
	if input.LogoURL != nil {
		v.Check(len(*input.LogoURL) <= 512, "logo_url", "must not be more than 512 characters")
	}
	if input.Website != nil {
		v.Check(len(*input.Website) <= 512, "website", "must not be more than 512 characters")
	}
}

func ValidateUpdateEmployerInput(v *validator.Validator, input UpdateEmployerInput) {
	if input.Name != nil {
		v.Check(*input.Name != "", "name", "must not be empty")
		v.Check(len(*input.Name) <= 255, "name", "must not be more than 255 characters")
	}
	if input.Slug != nil {
		v.Check(*input.Slug != "", "slug", "must not be empty")
		v.Check(len(*input.Slug) <= 255, "slug", "must not be more than 255 characters")
		v.Check(validator.Matches(*input.Slug, validator.SlugRX), "slug", "must contain only lowercase letters, numbers, and hyphens")
	}
	if input.Industry != nil {
		v.Check(*input.Industry != "", "industry", "must not be empty")
		v.Check(len(*input.Industry) <= 100, "industry", "must not be more than 100 characters")
	}
	if input.Size != nil {
		v.Check(len(*input.Size) <= 50, "size", "must not be more than 50 characters")
	}
	if input.HQLocation != nil {
		v.Check(len(*input.HQLocation) <= 255, "hq_location", "must not be more than 255 characters")
	}
	if input.LogoURL != nil {
		v.Check(len(*input.LogoURL) <= 512, "logo_url", "must not be more than 512 characters")
	}
	if input.Website != nil {
		v.Check(len(*input.Website) <= 512, "website", "must not be more than 512 characters")
	}
}

// --- Errors ---

var ErrDuplicateEmployerSlug = errors.New("employer with this slug already exists")

// --- Model ---

type EmployerModel struct {
	DB *pgxpool.Pool
}

func (m EmployerModel) Insert(ctx context.Context, db DBTX, input CreateEmployerInput) (*Employer, error) {
	query := `
		INSERT INTO employer
			(name, slug, industry, size, hq_location, offices,
			 logo_url, overview, culture, website, social_links)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		RETURNING id, name, slug, industry, size, hq_location, offices,
		          logo_url, overview, culture, website, social_links,
		          is_verified, avg_difficulty_rating, avg_experience_rating, review_count,
		          version, created_at, updated_at`

	employer := &Employer{}
	err := db.QueryRow(ctx, query,
		input.Name,
		input.Slug,
		input.Industry,
		input.Size,
		input.HQLocation,
		input.Offices,
		input.LogoURL,
		input.Overview,
		input.Culture,
		input.Website,
		input.SocialLinks,
	).Scan(
		&employer.ID,
		&employer.Name,
		&employer.Slug,
		&employer.Industry,
		&employer.Size,
		&employer.HQLocation,
		&employer.Offices,
		&employer.LogoURL,
		&employer.Overview,
		&employer.Culture,
		&employer.Website,
		&employer.SocialLinks,
		&employer.IsVerified,
		&employer.AvgDifficultyRating,
		&employer.AvgExperienceRating,
		&employer.ReviewCount,
		&employer.Version,
		&employer.CreatedAt,
		&employer.UpdatedAt,
	)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return nil, ErrDuplicateEmployerSlug
		}
		return nil, err
	}

	return employer, nil
}

func (m EmployerModel) GetByID(ctx context.Context, db DBTX, id string) (*Employer, error) {
	query := `
		SELECT id, name, slug, industry, size, hq_location, offices,
		       logo_url, overview, culture, website, social_links,
		       is_verified, avg_difficulty_rating, avg_experience_rating, review_count,
		       version, created_at, updated_at
		FROM employer
		WHERE id = $1`

	var employer Employer
	err := db.QueryRow(ctx, query, id).Scan(
		&employer.ID,
		&employer.Name,
		&employer.Slug,
		&employer.Industry,
		&employer.Size,
		&employer.HQLocation,
		&employer.Offices,
		&employer.LogoURL,
		&employer.Overview,
		&employer.Culture,
		&employer.Website,
		&employer.SocialLinks,
		&employer.IsVerified,
		&employer.AvgDifficultyRating,
		&employer.AvgExperienceRating,
		&employer.ReviewCount,
		&employer.Version,
		&employer.CreatedAt,
		&employer.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrRecordNotFound
		}
		return nil, err
	}

	return &employer, nil
}

func (m EmployerModel) GetBySlug(ctx context.Context, db DBTX, slug string) (*Employer, error) {
	query := `
		SELECT id, name, slug, industry, size, hq_location, offices,
		       logo_url, overview, culture, website, social_links,
		       is_verified, avg_difficulty_rating, avg_experience_rating, review_count,
		       version, created_at, updated_at
		FROM employer
		WHERE slug = $1`

	var employer Employer
	err := db.QueryRow(ctx, query, slug).Scan(
		&employer.ID,
		&employer.Name,
		&employer.Slug,
		&employer.Industry,
		&employer.Size,
		&employer.HQLocation,
		&employer.Offices,
		&employer.LogoURL,
		&employer.Overview,
		&employer.Culture,
		&employer.Website,
		&employer.SocialLinks,
		&employer.IsVerified,
		&employer.AvgDifficultyRating,
		&employer.AvgExperienceRating,
		&employer.ReviewCount,
		&employer.Version,
		&employer.CreatedAt,
		&employer.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrRecordNotFound
		}
		return nil, err
	}

	return &employer, nil
}

func (m EmployerModel) Update(ctx context.Context, db DBTX, id string, input UpdateEmployerInput) (*Employer, error) {
	current, err := m.GetByID(ctx, db, id)
	if err != nil {
		return nil, err
	}

	if input.Name != nil {
		current.Name = *input.Name
	}
	if input.Slug != nil {
		current.Slug = *input.Slug
	}
	if input.Industry != nil {
		current.Industry = *input.Industry
	}
	if input.Size != nil {
		current.Size = input.Size
	}
	if input.HQLocation != nil {
		current.HQLocation = input.HQLocation
	}
	if len(input.Offices) > 0 {
		current.Offices = input.Offices
	}
	if input.LogoURL != nil {
		current.LogoURL = input.LogoURL
	}
	if input.Overview != nil {
		current.Overview = input.Overview
	}
	if input.Culture != nil {
		current.Culture = input.Culture
	}
	if input.Website != nil {
		current.Website = input.Website
	}
	if len(input.SocialLinks) > 0 {
		current.SocialLinks = input.SocialLinks
	}
	if input.IsVerified != nil {
		current.IsVerified = *input.IsVerified
	}

	query := `
		UPDATE employer
		SET name = $1, slug = $2, industry = $3, size = $4, hq_location = $5,
		    offices = $6, logo_url = $7, overview = $8, culture = $9,
		    website = $10, social_links = $11, is_verified = $12,
		    version = version + 1
		WHERE id = $13 AND version = $14
		RETURNING version, updated_at`

	err = db.QueryRow(ctx, query,
		current.Name,
		current.Slug,
		current.Industry,
		current.Size,
		current.HQLocation,
		current.Offices,
		current.LogoURL,
		current.Overview,
		current.Culture,
		current.Website,
		current.SocialLinks,
		current.IsVerified,
		id,
		current.Version,
	).Scan(&current.Version, &current.UpdatedAt)
	if err != nil {
		switch {
		case errors.Is(err, pgx.ErrNoRows):
			return nil, ErrEditConflict
		default:
			var pgErr *pgconn.PgError
			if errors.As(err, &pgErr) && pgErr.Code == "23505" {
				return nil, ErrDuplicateEmployerSlug
			}
			return nil, err
		}
	}

	return current, nil
}

func (m EmployerModel) Delete(ctx context.Context, db DBTX, id string) error {
	query := `DELETE FROM employer WHERE id = $1`

	result, err := db.Exec(ctx, query, id)
	if err != nil {
		return err
	}

	if result.RowsAffected() == 0 {
		return ErrRecordNotFound
	}

	return nil
}

func (m EmployerModel) GetAll(ctx context.Context, db DBTX, search, industry string, isVerified *bool, filters Filters) ([]*Employer, Metadata, error) {
	query := fmt.Sprintf(`
		SELECT
			count(*) OVER(),
			id, name, slug, industry, size, hq_location, offices,
			logo_url, overview, culture, website, social_links,
			is_verified, avg_difficulty_rating, avg_experience_rating, review_count,
			version, created_at, updated_at
		FROM employer
		WHERE (to_tsvector('english', name || ' ' || COALESCE(overview, '') || ' ' || industry || ' ' || COALESCE(hq_location, '')) @@ plainto_tsquery('english', $1) OR $1 = '')
		  AND (industry = $2 OR $2 = '')
		  AND (is_verified = $3 OR $3 IS NULL)
		ORDER BY %s %s, name ASC
		LIMIT $4 OFFSET $5
	`, filters.sortColumn(), filters.sortDirection())

	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	args := []any{search, industry, isVerified, filters.limit(), filters.offset()}

	rows, err := db.Query(ctx, query, args...)
	if err != nil {
		return nil, Metadata{}, err
	}
	defer rows.Close()

	totalRecords := 0
	employers := []*Employer{}

	for rows.Next() {
		var employer Employer

		err := rows.Scan(
			&totalRecords,
			&employer.ID, &employer.Name, &employer.Slug, &employer.Industry,
			&employer.Size, &employer.HQLocation, &employer.Offices,
			&employer.LogoURL, &employer.Overview, &employer.Culture,
			&employer.Website, &employer.SocialLinks,
			&employer.IsVerified,
			&employer.AvgDifficultyRating, &employer.AvgExperienceRating, &employer.ReviewCount,
			&employer.Version,
			&employer.CreatedAt, &employer.UpdatedAt,
		)
		if err != nil {
			return nil, Metadata{}, err
		}

		employers = append(employers, &employer)
	}

	if err = rows.Err(); err != nil {
		return nil, Metadata{}, err
	}

	metadata := calculateMetadata(totalRecords, filters.Page, filters.PageSize)

	return employers, metadata, nil
}

func (m EmployerModel) RecalculateRatings(ctx context.Context, db DBTX, employerID string) error {
	query := `
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

	result, err := db.Exec(ctx, query, employerID)
	if err != nil {
		return err
	}

	if result.RowsAffected() == 0 {
		return ErrRecordNotFound
	}

	return nil
}

// Upsert inserts a new employer or updates an existing one matched by slug.
//
// Used by the bulk CSV import path where re-uploading a corrected CSV
// should refresh editable fields without losing manually-managed state
// (verification flag, computed review aggregates, optimistic lock version).
//
// Protected fields not touched by upsert:
//   - is_verified         (admin-toggled, manual workflow)
//   - avg_difficulty_rating, avg_experience_rating, review_count
//     (computed by recalc_ratings worker)
//   - version             (managed by the optimistic-lock pattern;
//     bumped here only on actual conflict update)
//
// Slug collisions are the merge key — there is no way to call this with
// the intent "create even if slug exists." Use Insert for that.
func (m EmployerModel) Upsert(ctx context.Context, db DBTX, input CreateEmployerInput) (*Employer, error) {
	query := `
        INSERT INTO employer
            (name, slug, industry, size, hq_location, offices,
             logo_url, overview, culture, website, social_links)
        VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
        ON CONFLICT (slug) DO UPDATE SET
            name         = EXCLUDED.name,
            industry     = EXCLUDED.industry,
            size         = EXCLUDED.size,
            hq_location  = EXCLUDED.hq_location,
            offices      = EXCLUDED.offices,
            logo_url     = EXCLUDED.logo_url,
            overview     = EXCLUDED.overview,
            culture      = EXCLUDED.culture,
            website      = EXCLUDED.website,
            social_links = EXCLUDED.social_links,
            version      = employer.version + 1,
            updated_at   = now()
        RETURNING id, name, slug, industry, size, hq_location, offices,
                  logo_url, overview, culture, website, social_links,
                  is_verified, avg_difficulty_rating, avg_experience_rating, review_count,
                  version, created_at, updated_at`

	employer := &Employer{}
	err := db.QueryRow(ctx, query,
		input.Name,
		input.Slug,
		input.Industry,
		input.Size,
		input.HQLocation,
		input.Offices,
		input.LogoURL,
		input.Overview,
		input.Culture,
		input.Website,
		input.SocialLinks,
	).Scan(
		&employer.ID,
		&employer.Name,
		&employer.Slug,
		&employer.Industry,
		&employer.Size,
		&employer.HQLocation,
		&employer.Offices,
		&employer.LogoURL,
		&employer.Overview,
		&employer.Culture,
		&employer.Website,
		&employer.SocialLinks,
		&employer.IsVerified,
		&employer.AvgDifficultyRating,
		&employer.AvgExperienceRating,
		&employer.ReviewCount,
		&employer.Version,
		&employer.CreatedAt,
		&employer.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	return employer, nil
}
