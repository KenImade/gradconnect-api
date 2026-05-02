package data_test

import (
	"testing"

	"api.gradconnect.com/internal/data"
	"api.gradconnect.com/internal/validator"
)

func TestValidateFilters(t *testing.T) {
	safelist := []string{"name", "-name", "created_at", "-created_at"}

	cases := []struct {
		name    string
		filters data.Filters
		wantErr bool
	}{
		{
			name:    "valid",
			filters: data.Filters{Page: 1, PageSize: 20, Sort: "name", SortSafeList: safelist},
		},
		{
			name:    "page zero",
			filters: data.Filters{Page: 0, PageSize: 20, Sort: "name", SortSafeList: safelist},
			wantErr: true,
		},
		{
			name:    "page too large",
			filters: data.Filters{Page: 10_000_001, PageSize: 20, Sort: "name", SortSafeList: safelist},
			wantErr: true,
		},
		{
			name:    "page_size zero",
			filters: data.Filters{Page: 1, PageSize: 0, Sort: "name", SortSafeList: safelist},
			wantErr: true,
		},
		{
			name:    "page_size over 100",
			filters: data.Filters{Page: 1, PageSize: 101, Sort: "name", SortSafeList: safelist},
			wantErr: true,
		},
		{
			name:    "invalid sort",
			filters: data.Filters{Page: 1, PageSize: 20, Sort: "invalid", SortSafeList: safelist},
			wantErr: true,
		},
		{
			name:    "descending sort",
			filters: data.Filters{Page: 1, PageSize: 20, Sort: "-name", SortSafeList: safelist},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			v := validator.New()
			data.ValidateFilters(v, tc.filters)
			if tc.wantErr && v.Valid() {
				t.Error("expected validation error, got none")
			}
			if !tc.wantErr && !v.Valid() {
				t.Errorf("expected no errors, got %v", v.Errors)
			}
		})
	}
}

func TestCalculateMetadata(t *testing.T) {
	cases := []struct {
		name         string
		total        int
		page         int
		pageSize     int
		wantLastPage int
		wantEmpty    bool
	}{
		{"no records", 0, 1, 20, 0, true},
		{"exact page", 20, 1, 20, 1, false},
		{"partial last page", 21, 1, 20, 2, false},
		{"middle page", 100, 3, 10, 10, false},
		{"single record", 1, 1, 20, 1, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := data.CalculateMetadata(tc.total, tc.page, tc.pageSize)
			if tc.wantEmpty {
				if m.TotalRecords != 0 || m.LastPage != 0 {
					t.Errorf("expected empty metadata, got %+v", m)
				}
				return
			}
			if m.CurrentPage != tc.page {
				t.Errorf("CurrentPage = %d, want %d", m.CurrentPage, tc.page)
			}
			if m.PageSize != tc.pageSize {
				t.Errorf("PageSize = %d, want %d", m.PageSize, tc.pageSize)
			}
			if m.LastPage != tc.wantLastPage {
				t.Errorf("LastPage = %d, want %d", m.LastPage, tc.wantLastPage)
			}
			if m.FirstPage != 1 {
				t.Errorf("FirstPage = %d, want 1", m.FirstPage)
			}
			if m.TotalRecords != tc.total {
				t.Errorf("TotalRecords = %d, want %d", m.TotalRecords, tc.total)
			}
		})
	}
}
