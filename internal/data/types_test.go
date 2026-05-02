package data_test

import (
	"encoding/json"
	"testing"
	"time"

	"api.gradconnect.com/internal/data"
)

func TestDate_UnmarshalJSON(t *testing.T) {
	cases := []struct {
		name    string
		input   string
		wantErr bool
		wantNil bool // zero value expected
		wantStr string
	}{
		{"valid date", `"2025-06-15"`, false, false, "2025-06-15"},
		{"null literal", `null`, false, true, ""},
		{"empty string", `""`, false, true, ""},
		{"wrong format", `"15-06-2025"`, true, false, ""},
		{"not a date", `"foobar"`, true, false, ""},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var d data.Date
			err := json.Unmarshal([]byte(tc.input), &d)
			if tc.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tc.wantNil {
				if !d.Time.IsZero() {
					t.Errorf("expected zero time, got %v", d.Time)
				}
				return
			}
			if got := d.Time.Format("2006-01-02"); got != tc.wantStr {
				t.Errorf("got %q, want %q", got, tc.wantStr)
			}
		})
	}
}

func TestDate_MarshalJSON(t *testing.T) {
	t.Run("non-zero date", func(t *testing.T) {
		d := data.Date{Time: time.Date(2025, 6, 15, 0, 0, 0, 0, time.UTC)}
		b, err := json.Marshal(d)
		if err != nil {
			t.Fatal(err)
		}
		if string(b) != `"2025-06-15"` {
			t.Errorf("got %s, want %q", b, "2025-06-15")
		}
	})

	t.Run("zero date marshals as null", func(t *testing.T) {
		var d data.Date
		b, err := json.Marshal(d)
		if err != nil {
			t.Fatal(err)
		}
		if string(b) != "null" {
			t.Errorf("got %s, want null", b)
		}
	})
}

func TestDate_RoundTrip(t *testing.T) {
	original := `"2024-12-31"`
	var d data.Date
	if err := json.Unmarshal([]byte(original), &d); err != nil {
		t.Fatal(err)
	}
	b, err := json.Marshal(d)
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != original {
		t.Errorf("round-trip: got %s, want %s", b, original)
	}
}

func TestParseDate(t *testing.T) {
	d, err := data.ParseDate("2025-01-15")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if d.Time.Format("2006-01-02") != "2025-01-15" {
		t.Errorf("got %v", d.Time)
	}

	if _, err := data.ParseDate("not-a-date"); err == nil {
		t.Error("expected error for invalid date")
	}
}

func TestDate_BeforeDate(t *testing.T) {
	early, _ := data.ParseDate("2024-01-01")
	late, _ := data.ParseDate("2025-01-01")
	same, _ := data.ParseDate("2024-01-01")

	if !early.BeforeDate(late) {
		t.Error("early should be before late")
	}
	if late.BeforeDate(early) {
		t.Error("late should not be before early")
	}
	if early.BeforeDate(same) {
		t.Error("equal dates: BeforeDate should return false")
	}
}

func TestDate_EqualDate(t *testing.T) {
	a, _ := data.ParseDate("2024-06-01")
	b, _ := data.ParseDate("2024-06-01")
	c, _ := data.ParseDate("2024-06-02")

	if !a.EqualDate(b) {
		t.Error("same dates should be equal")
	}
	if a.EqualDate(c) {
		t.Error("different dates should not be equal")
	}
}
