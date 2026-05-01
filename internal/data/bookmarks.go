package data

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
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

type DeadlineReminderRecipient struct {
	UserID    uuid.UUID
	Email     string
	FirstName string
	Bookmarks []DeadlineReminderBookmark
}

type DeadlineReminderBookmark struct {
	Title           string
	EmployerName    string
	OpportunitySlug string
	Deadline        time.Time
	DaysRemaining   int
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

func (m BookmarkModel) Delete(ctx context.Context, db DBTX, bookmarkID, userID string) error {
	query := `
        DELETE FROM bookmark
        WHERE id = $1 AND user_id = $2`

	result, err := db.Exec(ctx, query, bookmarkID, userID)
	if err != nil {
		return err
	}

	if result.RowsAffected() == 0 {
		return ErrRecordNotFound
	}

	return nil
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

func (m BookmarkModel) FindDeadlineReminderRecipients(
	ctx context.Context,
	db DBTX,
	daysAhead int,
) ([]DeadlineReminderRecipient, error) {
	lagos, err := time.LoadLocation("Africa/Lagos")
	if err != nil {
		return nil, fmt.Errorf("loading Africa/Lagos timezone: %w", err)
	}
	targetDate := time.Now().In(lagos).AddDate(0, 0, daysAhead).Format("2006-01-02")

	query := `
		SELECT
			u.id,
			u.email,
			u.first_name,
			o.title,
			e.name AS employer_name,
			o.slug,
			o.deadline
		FROM bookmark b
		INNER JOIN opportunity o ON o.id = b.opportunity_id
		INNER JOIN employer e ON e.id = o.employer_id
		INNER JOIN app_user u ON u.id = b.user_id
		WHERE
			o.deadline = $1::date
			AND o.is_active = true
			AND u.email_verified = true
		ORDER BY u.id, o.deadline ASC, o.title ASC
	`

	rows, err := db.Query(ctx, query, targetDate)
	if err != nil {
		return nil, fmt.Errorf("querying reminder recipients: %w", err)
	}
	defer rows.Close()

	var recipients []DeadlineReminderRecipient
	var current *DeadlineReminderRecipient

	for rows.Next() {
		var (
			userID               uuid.UUID
			email, firstName     string
			title, empName, slug string
			deadline             time.Time
		)

		if err := rows.Scan(&userID, &email, &firstName, &title, &empName, &slug, &deadline); err != nil {
			return nil, fmt.Errorf("scanning reminder row: %w", err)
		}

		if current == nil || current.UserID != userID {
			recipients = append(recipients, DeadlineReminderRecipient{
				UserID:    userID,
				Email:     email,
				FirstName: firstName,
				Bookmarks: nil,
			})
			current = &recipients[len(recipients)-1]
		}

		current.Bookmarks = append(current.Bookmarks, DeadlineReminderBookmark{
			Title:           title,
			EmployerName:    empName,
			OpportunitySlug: slug,
			Deadline:        deadline,
			DaysRemaining:   daysAhead,
		})
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating reminder rows: %w", err)
	}

	return recipients, nil
}
