package validator_test

import (
	"testing"

	"api.gradconnect.com/internal/validator"
)

func TestValidator_Valid(t *testing.T) {
	v := validator.New()
	if !v.Valid() {
		t.Error("new validator should be valid")
	}

	v.AddError("field", "something wrong")
	if v.Valid() {
		t.Error("validator with errors should not be valid")
	}
}

func TestValidator_AddError_FirstWins(t *testing.T) {
	v := validator.New()
	v.AddError("field", "first error")
	v.AddError("field", "second error")

	if v.Errors["field"] != "first error" {
		t.Errorf("got %q, want first error to win", v.Errors["field"])
	}
}

func TestValidator_Check(t *testing.T) {
	v := validator.New()
	v.Check(true, "ok", "should not appear")
	v.Check(false, "bad", "should appear")

	if _, ok := v.Errors["ok"]; ok {
		t.Error("passing check should not add error")
	}
	if v.Errors["bad"] != "should appear" {
		t.Error("failing check should add error")
	}
}

func TestMatches(t *testing.T) {
	cases := []struct {
		input string
		want  bool
	}{
		{"user@example.com", true},
		{"user+tag@sub.domain.com", true},
		{"notanemail", false},
		{"@nodomain.com", false},
		{"", false},
	}

	for _, tc := range cases {
		got := validator.Matches(tc.input, validator.EmailRX)
		if got != tc.want {
			t.Errorf("Matches(%q) = %v, want %v", tc.input, got, tc.want)
		}
	}
}

func TestSlugRX(t *testing.T) {
	cases := []struct {
		input string
		want  bool
	}{
		{"valid-slug", true},
		{"slug123", true},
		{"a", true},
		{"Invalid-Slug", false},
		{"has space", false},
		{"double--dash", false},
		{"-leading", false},
		{"trailing-", false},
	}

	for _, tc := range cases {
		got := validator.Matches(tc.input, validator.SlugRX)
		if got != tc.want {
			t.Errorf("SlugRX(%q) = %v, want %v", tc.input, got, tc.want)
		}
	}
}

func TestUnique(t *testing.T) {
	if !validator.Unique([]string{"a", "b", "c"}) {
		t.Error("unique slice should return true")
	}
	if validator.Unique([]string{"a", "b", "a"}) {
		t.Error("slice with duplicates should return false")
	}
	if !validator.Unique([]string{}) {
		t.Error("empty slice should return true")
	}
}

func TestPermittedValue(t *testing.T) {
	if !validator.PermittedValue("foo", "foo", "bar", "baz") {
		t.Error("value in permitted list should return true")
	}
	if validator.PermittedValue("qux", "foo", "bar", "baz") {
		t.Error("value not in permitted list should return false")
	}
}

func TestIsValidUUID(t *testing.T) {
	cases := []struct {
		input string
		want  bool
	}{
		{"550e8400-e29b-41d4-a716-446655440000", true},
		{"00000000-0000-0000-0000-000000000000", true},
		{"not-a-uuid", false},
		{"", false},
		{"550e8400-e29b-41d4-a716-44665544000Z", false},
	}

	for _, tc := range cases {
		got := validator.IsValidUUID(tc.input)
		if got != tc.want {
			t.Errorf("IsValidUUID(%q) = %v, want %v", tc.input, got, tc.want)
		}
	}
}
