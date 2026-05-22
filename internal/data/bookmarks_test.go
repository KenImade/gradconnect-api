package data_test

import (
	"context"
	"errors"
	"regexp"
	"testing"
	"time"

	"api.gradconnect.com/internal/data"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/pashagolub/pgxmock/v4"
)

func newUUID(t *testing.T) uuid.UUID {
	t.Helper()
	id, err := uuid.NewRandom()
	if err != nil {
		t.Fatalf("uuid: %v", err)
	}
	return id
}

func strPtr(s string) *string { return &s }

// bookmarkSafeList is a representative sort safelist for bookmark listing.
var bookmarkSafeList = []string{"created_at", "deadline", "-created_at", "-deadline"}

func newBookmarkFilters() data.Filters {
	return data.Filters{
		Page:         1,
		PageSize:     20,
		Sort:         "-created_at",
		SortSafeList: bookmarkSafeList,
	}
}

// bookmarkRowCols matches the column order in GetAllForUser's SELECT.
var bookmarkRowCols = []string{
	"id", "created_at",
	"opp_id", "title", "slug", "type", "deadline",
	"emp_name", "emp_slug", "logo_url", "industry",
	"total",
}

// -------------------------------------------------------------------
// GetAllForUser
// -------------------------------------------------------------------

func TestBookmarkModel_GetAllForUser_DerivedFields(t *testing.T) {
	userID := "user-123"

	future := time.Now().Add(10 * 24 * time.Hour)
	past := time.Now().Add(-3 * 24 * time.Hour)
	today := time.Now()

	cases := []struct {
		name           string
		deadline       *time.Time
		wantDaysRemNil bool
		wantDaysRemMin int
		wantDaysRemMax int
		wantIsActive   bool
	}{
		{
			name:           "future deadline",
			deadline:       &future,
			wantDaysRemNil: false,
			wantDaysRemMin: 9,
			wantDaysRemMax: 10,
			wantIsActive:   true,
		},
		{
			name:           "past deadline clamps to zero",
			deadline:       &past,
			wantDaysRemNil: false,
			wantDaysRemMin: 0,
			wantDaysRemMax: 0,
			wantIsActive:   false,
		},
		{
			name:           "today deadline",
			deadline:       &today,
			wantDaysRemNil: false,
			wantDaysRemMin: 0,
			wantDaysRemMax: 0,
		},
		{
			name:           "nil deadline (rolling opportunity)",
			deadline:       nil,
			wantDaysRemNil: true,
			wantIsActive:   true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			mock, err := pgxmock.NewPool()
			if err != nil {
				t.Fatalf("creating mock pool: %v", err)
			}
			defer mock.Close()

			rows := pgxmock.NewRows(bookmarkRowCols).AddRow(
				"bm-1", time.Now(),
				"opp-1", "Software Engineer Intern", "swe-intern", "internship", tc.deadline,
				"Acme Corp", "acme", strPtr("https://example.com/logo.png"), "Technology",
				1,
			)

			mock.ExpectQuery("SELECT b.id, b.created_at").
				WithArgs(userID, 20, 0).
				WillReturnRows(rows)

			m := data.BookmarkModel{}
			bookmarks, meta, err := m.GetAllForUser(context.Background(), mock, userID, newBookmarkFilters())
			if err != nil {
				t.Fatalf("GetAllForUser: %v", err)
			}
			if len(bookmarks) != 1 {
				t.Fatalf("want 1 bookmark, got %d", len(bookmarks))
			}
			if meta.TotalRecords != 1 {
				t.Errorf("TotalRecords = %d, want 1", meta.TotalRecords)
			}

			opp := bookmarks[0].Opportunity

			if tc.wantDaysRemNil {
				if opp.DaysRemaining != nil {
					t.Errorf("DaysRemaining = %d, want nil", *opp.DaysRemaining)
				}
			} else {
				if opp.DaysRemaining == nil {
					t.Fatalf("DaysRemaining = nil, want value in [%d, %d]", tc.wantDaysRemMin, tc.wantDaysRemMax)
				}
				if *opp.DaysRemaining < tc.wantDaysRemMin || *opp.DaysRemaining > tc.wantDaysRemMax {
					t.Errorf("DaysRemaining = %d, want in [%d, %d]", *opp.DaysRemaining, tc.wantDaysRemMin, tc.wantDaysRemMax)
				}
			}

			// Only assert IsActive when the case actually pins it down.
			if tc.name != "today deadline" {
				if opp.IsActive != tc.wantIsActive {
					t.Errorf("IsActive = %v, want %v", opp.IsActive, tc.wantIsActive)
				}
			}

			if err := mock.ExpectationsWereMet(); err != nil {
				t.Errorf("unmet expectations: %v", err)
			}
		})
	}
}

func TestBookmarkModel_GetAllForUser_EmptyResult(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("creating mock pool: %v", err)
	}
	defer mock.Close()

	rows := pgxmock.NewRows(bookmarkRowCols)
	mock.ExpectQuery("SELECT b.id, b.created_at").
		WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg()).
		WillReturnRows(rows)

	m := data.BookmarkModel{}
	bookmarks, meta, err := m.GetAllForUser(context.Background(), mock, "user-x", newBookmarkFilters())
	if err != nil {
		t.Fatalf("GetAllForUser: %v", err)
	}
	if len(bookmarks) != 0 {
		t.Errorf("want empty slice, got %d items", len(bookmarks))
	}
	if bookmarks == nil {
		t.Error("bookmarks slice is nil; want empty non-nil slice for [] JSON output")
	}
	if meta.TotalRecords != 0 {
		t.Errorf("TotalRecords = %d, want 0", meta.TotalRecords)
	}
}

func TestBookmarkModel_GetAllForUser_MixedDeadlines(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("creating mock pool: %v", err)
	}
	defer mock.Close()

	future := time.Now().Add(5 * 24 * time.Hour)
	logo := strPtr("https://example.com/logo.png")

	rows := pgxmock.NewRows(bookmarkRowCols).
		AddRow("bm-1", time.Now(),
			"opp-1", "With deadline", "with-deadline", "graduate", &future,
			"Acme", "acme", logo, "Tech", 2).
		AddRow("bm-2", time.Now(),
			"opp-2", "Rolling", "rolling", "internship", (*time.Time)(nil),
			"Beta", "beta", (*string)(nil), "Finance", 2)

	mock.ExpectQuery("SELECT b.id, b.created_at").
		WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg()).
		WillReturnRows(rows)

	m := data.BookmarkModel{}
	bookmarks, _, err := m.GetAllForUser(context.Background(), mock, "u", newBookmarkFilters())
	if err != nil {
		t.Fatalf("GetAllForUser: %v", err)
	}
	if len(bookmarks) != 2 {
		t.Fatalf("want 2 bookmarks, got %d", len(bookmarks))
	}

	if bookmarks[0].Opportunity.DaysRemaining == nil {
		t.Error("first opportunity: DaysRemaining unexpectedly nil")
	}
	if bookmarks[1].Opportunity.DaysRemaining != nil {
		t.Errorf("second opportunity: DaysRemaining = %d, want nil", *bookmarks[1].Opportunity.DaysRemaining)
	}
	if !bookmarks[1].Opportunity.IsActive {
		t.Error("rolling opportunity: IsActive = false, want true")
	}
}

func TestBookmarkModel_GetAllForUser_QueryError(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("creating mock pool: %v", err)
	}
	defer mock.Close()

	wantErr := errors.New("connection refused")
	mock.ExpectQuery("SELECT b.id, b.created_at").
		WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg()).
		WillReturnError(wantErr)

	m := data.BookmarkModel{}
	_, _, err = m.GetAllForUser(context.Background(), mock, "u", newBookmarkFilters())
	if !errors.Is(err, wantErr) {
		t.Errorf("err = %v, want %v", err, wantErr)
	}
}

// -------------------------------------------------------------------
// Create
// -------------------------------------------------------------------

func TestBookmarkModel_Create(t *testing.T) {
	insertPattern := regexp.QuoteMeta("INSERT INTO bookmark")

	t.Run("success", func(t *testing.T) {
		mock, err := pgxmock.NewPool()
		if err != nil {
			t.Fatalf("creating mock pool: %v", err)
		}
		defer mock.Close()

		now := time.Now()
		rows := pgxmock.NewRows([]string{"id", "created_at"}).AddRow("bm-1", now)
		mock.ExpectQuery(insertPattern).
			WithArgs("user-1", "opp-1").
			WillReturnRows(rows)

		m := data.BookmarkModel{}
		bm, err := m.Create(context.Background(), mock, "user-1", "opp-1")
		if err != nil {
			t.Fatalf("Create: %v", err)
		}
		if bm.ID != "bm-1" {
			t.Errorf("ID = %q, want %q", bm.ID, "bm-1")
		}
		if bm.UserID != "user-1" {
			t.Errorf("UserID = %q, want %q", bm.UserID, "user-1")
		}
		if !bm.CreatedAt.Equal(now) {
			t.Errorf("CreatedAt = %v, want %v", bm.CreatedAt, now)
		}
	})

	t.Run("duplicate returns ErrDuplicateBookmark", func(t *testing.T) {
		mock, err := pgxmock.NewPool()
		if err != nil {
			t.Fatalf("creating mock pool: %v", err)
		}
		defer mock.Close()

		mock.ExpectQuery(insertPattern).
			WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg()).
			WillReturnError(&pgconn.PgError{Code: "23505"})

		m := data.BookmarkModel{}
		_, err = m.Create(context.Background(), mock, "user-1", "opp-1")
		if !errors.Is(err, data.ErrDuplicateBookmark) {
			t.Errorf("err = %v, want ErrDuplicateBookmark", err)
		}
	})

	t.Run("missing opportunity returns ErrRecordNotFound", func(t *testing.T) {
		mock, err := pgxmock.NewPool()
		if err != nil {
			t.Fatalf("creating mock pool: %v", err)
		}
		defer mock.Close()

		mock.ExpectQuery(insertPattern).
			WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg()).
			WillReturnError(&pgconn.PgError{Code: "23503"})

		m := data.BookmarkModel{}
		_, err = m.Create(context.Background(), mock, "user-1", "missing-opp")
		if !errors.Is(err, data.ErrRecordNotFound) {
			t.Errorf("err = %v, want ErrRecordNotFound", err)
		}
	})

	t.Run("other pg error bubbles up", func(t *testing.T) {
		mock, err := pgxmock.NewPool()
		if err != nil {
			t.Fatalf("creating mock pool: %v", err)
		}
		defer mock.Close()

		pgErr := &pgconn.PgError{Code: "08000", Message: "connection exception"}
		mock.ExpectQuery(insertPattern).
			WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg()).
			WillReturnError(pgErr)

		m := data.BookmarkModel{}
		_, err = m.Create(context.Background(), mock, "user-1", "opp-1")
		if !errors.Is(err, pgErr) {
			t.Errorf("err = %v, want %v", err, pgErr)
		}
	})
}

// -------------------------------------------------------------------
// Delete
// -------------------------------------------------------------------

func TestBookmarkModel_Delete(t *testing.T) {
	deletePattern := regexp.QuoteMeta("DELETE FROM bookmark")

	t.Run("success", func(t *testing.T) {
		mock, err := pgxmock.NewPool()
		if err != nil {
			t.Fatalf("creating mock pool: %v", err)
		}
		defer mock.Close()

		mock.ExpectExec(deletePattern).
			WithArgs("bm-1", "user-1").
			WillReturnResult(pgxmock.NewResult("DELETE", 1))

		m := data.BookmarkModel{}
		if err := m.Delete(context.Background(), mock, "bm-1", "user-1"); err != nil {
			t.Errorf("Delete: %v", err)
		}
	})

	t.Run("no rows returns ErrRecordNotFound", func(t *testing.T) {
		mock, err := pgxmock.NewPool()
		if err != nil {
			t.Fatalf("creating mock pool: %v", err)
		}
		defer mock.Close()

		mock.ExpectExec(deletePattern).
			WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg()).
			WillReturnResult(pgxmock.NewResult("DELETE", 0))

		m := data.BookmarkModel{}
		err = m.Delete(context.Background(), mock, "missing", "user-1")
		if !errors.Is(err, data.ErrRecordNotFound) {
			t.Errorf("err = %v, want ErrRecordNotFound", err)
		}
	})

	t.Run("db error bubbles up", func(t *testing.T) {
		mock, err := pgxmock.NewPool()
		if err != nil {
			t.Fatalf("creating mock pool: %v", err)
		}
		defer mock.Close()

		wantErr := errors.New("boom")
		mock.ExpectExec(deletePattern).
			WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg()).
			WillReturnError(wantErr)

		m := data.BookmarkModel{}
		err = m.Delete(context.Background(), mock, "bm-1", "user-1")
		if !errors.Is(err, wantErr) {
			t.Errorf("err = %v, want %v", err, wantErr)
		}
	})
}

// -------------------------------------------------------------------
// FindDeadlineReminderRecipients
// -------------------------------------------------------------------

func TestBookmarkModel_FindDeadlineReminderRecipients(t *testing.T) {
	queryPattern := "SELECT"
	deadline := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)

	t.Run("groups bookmarks per user", func(t *testing.T) {
		mock, err := pgxmock.NewPool()
		if err != nil {
			t.Fatalf("creating mock pool: %v", err)
		}
		defer mock.Close()

		u1, u2 := newUUID(t), newUUID(t)
		rows := pgxmock.NewRows([]string{
			"id", "email", "first_name", "title", "employer_name", "slug", "deadline",
		}).
			AddRow(u1, "a@example.com", "Ada", "Job A1", "Acme", "job-a1", &deadline).
			AddRow(u1, "a@example.com", "Ada", "Job A2", "Beta", "job-a2", &deadline).
			AddRow(u2, "b@example.com", "Babs", "Job B1", "Gamma", "job-b1", &deadline)

		mock.ExpectQuery(queryPattern).
			WithArgs(pgxmock.AnyArg()).
			WillReturnRows(rows)

		m := data.BookmarkModel{}
		recipients, err := m.FindDeadlineReminderRecipients(context.Background(), mock, 7)
		if err != nil {
			t.Fatalf("FindDeadlineReminderRecipients: %v", err)
		}
		if len(recipients) != 2 {
			t.Fatalf("want 2 recipients, got %d", len(recipients))
		}
		if len(recipients[0].Bookmarks) != 2 {
			t.Errorf("user 1 bookmarks = %d, want 2", len(recipients[0].Bookmarks))
		}
		if len(recipients[1].Bookmarks) != 1 {
			t.Errorf("user 2 bookmarks = %d, want 1", len(recipients[1].Bookmarks))
		}
		if recipients[0].Bookmarks[0].DaysRemaining != 7 {
			t.Errorf("DaysRemaining = %d, want 7", recipients[0].Bookmarks[0].DaysRemaining)
		}
	})

	t.Run("empty result", func(t *testing.T) {
		mock, err := pgxmock.NewPool()
		if err != nil {
			t.Fatalf("creating mock pool: %v", err)
		}
		defer mock.Close()

		rows := pgxmock.NewRows([]string{
			"id", "email", "first_name", "title", "employer_name", "slug", "deadline",
		})
		mock.ExpectQuery(queryPattern).
			WithArgs(pgxmock.AnyArg()).
			WillReturnRows(rows)

		m := data.BookmarkModel{}
		recipients, err := m.FindDeadlineReminderRecipients(context.Background(), mock, 7)
		if err != nil {
			t.Fatalf("FindDeadlineReminderRecipients: %v", err)
		}
		if len(recipients) != 0 {
			t.Errorf("want 0 recipients, got %d", len(recipients))
		}
	})

	t.Run("query error wraps", func(t *testing.T) {
		mock, err := pgxmock.NewPool()
		if err != nil {
			t.Fatalf("creating mock pool: %v", err)
		}
		defer mock.Close()

		wantErr := errors.New("pg down")
		mock.ExpectQuery(queryPattern).
			WithArgs(pgxmock.AnyArg()).
			WillReturnError(wantErr)

		m := data.BookmarkModel{}
		_, err = m.FindDeadlineReminderRecipients(context.Background(), mock, 7)
		if !errors.Is(err, wantErr) {
			t.Errorf("err = %v, want chain to %v", err, wantErr)
		}
	})
}
