package data

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"api.gradconnect.com/internal/validator"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Employer struct {
	ID          string          `json:"id"`
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
	Version     int             `json:"version"`
	CreatedAt   time.Time       `json:"created_at"`
	UpdatedAt   time.Time       `json:"updated_at"`
}

func ValidateEmployer(v *validator.Validator, employer *Employer) {
	v.Check(employer.Name != "", "name", "must be provided")
	v.Check(len(employer.Name) <= 255, "name", "must not be more than 255 characters")

	v.Check(employer.Slug != "", "slug", "must be provided")
	v.Check(len(employer.Slug) <= 255, "slug", "must not be more than 255 characters")
	v.Check(validator.Matches(employer.Slug, validator.SlugRX), "slug", "must contain only lowercase letters, numbers, and hyphens")

	v.Check(employer.Industry != "", "industry", "must be provided")
	v.Check(len(employer.Industry) <= 100, "industry", "must not be more than 100 characters")

	if employer.Website != nil {
		v.Check(len(*employer.Website) <= 512, "website", "must not be more than 512 characters")
	}

	if employer.LogoURL != nil {
		v.Check(len(*employer.LogoURL) <= 512, "logo_url", "must not be more than 512 characters")
	}
}

type EmployerModel struct {
	DB *pgxpool.Pool
}

func (m EmployerModel) Insert(employer *Employer) error {
	return nil
}

func (m EmployerModel) GetByID(id string) (*Employer, error) {
	return nil, nil
}

func (m EmployerModel) GetBySlug(slug string) (*Employer, error) {
	return nil, nil
}

func (m EmployerModel) Update(employer *Employer) error {
	return nil
}

func (m EmployerModel) Delete(id string) error {
	return nil
}

func (m EmployerModel) GetAll(search string, industry string, isVerified *bool, filters Filters) ([]*Employer, Metadata, error) {
	query := fmt.Sprintf(`
		SELECT 
			count(*) OVER(),
			id, name, slug, industry, size, hq_location, offices,
			logo_url, overview, culture, website, social_links,
			is_verified, version, created_at, updated_at
		FROM employer
		WHERE (to_tsvector('english', name || ' ' || COALESCE(overview, '') || ' ' || industry || ' ' || COALESCE(hq_location, '')) @@ plainto_tsquery('english', $1) OR $1 = '')
		AND (industry = $2 OR $2 = '')
		AND (is_verified = $3 OR $3 IS NULL)
		ORDER BY %s %s, name ASC
		LIMIT $4 OFFSET $5
	`, filters.sortColumn(), filters.sortDirection())

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	args := []any{search, industry, isVerified, filters.limit(), filters.offset()}

	rows, err := m.DB.Query(ctx, query, args...)
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
			&employer.IsVerified, &employer.Version,
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
