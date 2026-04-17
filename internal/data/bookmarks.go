package data

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrDuplicateBookmark = errors.New("bookmark already exists")

type OpportunityStub struct {
	ID            string       `json:"id"`
	Title         string       `json:"title"`
	Slug          string       `json:"slug"`
	Type          string       `json:"type"`
	Deadline      time.Time    `json:"deadline"`
	DaysRemaining int          `json:"days_remaining"`
	IsActive      bool         `json:"is_active"`
	Employer      EmployerStub `json:"employer"`
}

type Bookmark struct {
	ID          string          `json:"id"`
	UserID      string          `json:"-"`
	CreatedAt   time.Time       `json:"created_at"`
	Opportunity OpportunityStub `json:"opportunity"`
}

type BookmarkCreateResponse struct {
	ID            string    `json:"id"`
	OpportunityID string    `json:"opportunity_id"`
	CreatedAt     time.Time `json:"created_at"`
}

func NewOpportunityStub(id, title, slug, oppType string, deadline time.Time, employer EmployerStub) OpportunityStub {
	now := time.Now()
	days := int(deadline.Sub(now).Hours() / 24)
	if days < 0 {
		days = 0
	}
	return OpportunityStub{
		ID:            id,
		Title:         title,
		Slug:          slug,
		Type:          oppType,
		Deadline:      deadline,
		DaysRemaining: days,
		IsActive:      deadline.After(now),
	}
}

type BookmarkModel struct {
	DB *pgxpool.Pool
}

func (m BookmarkModel) Create(ctx context.Context, db DBTX, userID, opportunityID string) (*Bookmark, error) {
	query := `
        INSERT INTO bookmark (user_id, opportunity_id)
        VALUES ($1, $2)
        RETURNING id, created_at`

	bookmark := &Bookmark{
		UserID: userID,
	}

	err := db.QueryRow(ctx, query, userID, opportunityID).Scan(&bookmark.ID, &bookmark.CreatedAt)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			switch pgErr.Code {
			case "23505": // unique_violation
				return nil, ErrDuplicateBookmark
			case "23503": // foreign_key_violation — opportunity doesn't exist
				return nil, ErrRecordNotFound
			}
		}
		return nil, err
	}

	return bookmark, nil
}

func (m BookmarkModel) GetAllForUser(ctx context.Context, db DBTX, userID string, filters Filters) ([]Bookmark, Metadata, error) {
	query := fmt.Sprintf(`
        SELECT b.id, b.created_at,
               o.id, o.title, o.slug, o.type, o.deadline,
               e.name, e.slug, e.logo_url, e.industry,
               count(*) OVER()
        FROM bookmark b
        JOIN opportunity o ON o.id = b.opportunity_id
        JOIN employer e ON e.id = o.employer_id
        WHERE b.user_id = $1
        ORDER BY %s %s, b.id ASC
        LIMIT $2 OFFSET $3`, filters.sortColumn(), filters.sortDirection())

	rows, err := db.Query(ctx, query, userID, filters.limit(), filters.offset())
	if err != nil {
		return nil, Metadata{}, err
	}
	defer rows.Close()

	var totalRecords int
	bookmarks := []Bookmark{}

	for rows.Next() {
		var b Bookmark
		var opp OpportunityStub
		var emp EmployerStub

		err := rows.Scan(
			&b.ID, &b.CreatedAt,
			&opp.ID, &opp.Title, &opp.Slug, &opp.Type, &opp.Deadline,
			&emp.Name, &emp.Slug, &emp.LogoURL, &emp.Industry,
			&totalRecords,
		)
		if err != nil {
			return nil, Metadata{}, err
		}

		opp.Employer = emp
		// Derived fields
		now := time.Now()
		days := int(opp.Deadline.Sub(now).Hours() / 24)
		if days < 0 {
			days = 0
		}
		opp.DaysRemaining = days
		opp.IsActive = opp.Deadline.After(now)

		b.Opportunity = opp
		bookmarks = append(bookmarks, b)
	}

	if err := rows.Err(); err != nil {
		return nil, Metadata{}, err
	}

	metadata := calculateMetadata(totalRecords, filters.Page, filters.PageSize)
	return bookmarks, metadata, nil
}
