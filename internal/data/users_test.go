package data_test

import (
	"testing"

	"api.gradconnect.com/internal/data"
	"api.gradconnect.com/internal/validator"
)

func TestValidateEmail(t *testing.T) {
	cases := []struct {
		email   string
		wantErr bool
	}{
		{"user@example.com", false},
		{"user+tag@sub.domain.ng", false},
		{"", true},
		{"notanemail", true},
		{"@nodomain.com", true},
		{"user@", true},
	}

	for _, tc := range cases {
		t.Run(tc.email, func(t *testing.T) {
			v := validator.New()
			data.ValidateEmail(v, tc.email)
			if tc.wantErr && v.Valid() {
				t.Errorf("email %q: expected validation error", tc.email)
			}
			if !tc.wantErr && !v.Valid() {
				t.Errorf("email %q: unexpected errors: %v", tc.email, v.Errors)
			}
		})
	}
}

func TestValidatePasswordPlaintext(t *testing.T) {
	cases := []struct {
		name     string
		password string
		wantErr  bool
	}{
		{"valid", "pa55word123", false},
		{"exactly 8 chars", "12345678", false},
		{"empty", "", true},
		{"too short", "abc123", true},
		{"too long", string(make([]byte, 73)), true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			v := validator.New()
			data.ValidatePasswordPlaintext(v, tc.password)
			if tc.wantErr && v.Valid() {
				t.Error("expected validation error, got none")
			}
			if !tc.wantErr && !v.Valid() {
				t.Errorf("unexpected errors: %v", v.Errors)
			}
		})
	}
}

func TestValidateCreateUserInput(t *testing.T) {
	validYear := 2025

	cases := []struct {
		name    string
		input   data.CreateUserInput
		wantErr bool
	}{
		{
			name: "valid minimal",
			input: data.CreateUserInput{
				FirstName: "Ada",
				LastName:  "Lovelace",
				Email:     "ada@example.com",
				Password:  "pa55word123",
			},
		},
		{
			name: "missing first name",
			input: data.CreateUserInput{
				LastName: "Lovelace",
				Email:    "ada@example.com",
				Password: "pa55word123",
			},
			wantErr: true,
		},
		{
			name: "missing last name",
			input: data.CreateUserInput{
				FirstName: "Ada",
				Email:     "ada@example.com",
				Password:  "pa55word123",
			},
			wantErr: true,
		},
		{
			name: "invalid email",
			input: data.CreateUserInput{
				FirstName: "Ada",
				LastName:  "Lovelace",
				Email:     "not-an-email",
				Password:  "pa55word123",
			},
			wantErr: true,
		},
		{
			name: "short password",
			input: data.CreateUserInput{
				FirstName: "Ada",
				LastName:  "Lovelace",
				Email:     "ada@example.com",
				Password:  "abc",
			},
			wantErr: true,
		},
		{
			name: "valid with optional fields",
			input: data.CreateUserInput{
				FirstName:      "Ada",
				LastName:       "Lovelace",
				Email:          "ada@example.com",
				Password:       "pa55word123",
				GraduationYear: &validYear,
			},
		},
		{
			name: "graduation year before 1990",
			input: func() data.CreateUserInput {
				y := 1985
				return data.CreateUserInput{
					FirstName: "Ada", LastName: "L",
					Email: "ada@example.com", Password: "pa55word123",
					GraduationYear: &y,
				}
			}(),
			wantErr: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			v := validator.New()
			data.ValidateCreateUserInput(v, tc.input)
			if tc.wantErr && v.Valid() {
				t.Error("expected validation error, got none")
			}
			if !tc.wantErr && !v.Valid() {
				t.Errorf("unexpected errors: %v", v.Errors)
			}
		})
	}
}

func TestUser_IsAnonymous(t *testing.T) {
	if !data.AnonymousUser.IsAnonymous() {
		t.Error("AnonymousUser.IsAnonymous() should be true")
	}

	u := &data.User{ID: "some-id"}
	if u.IsAnonymous() {
		t.Error("regular user should not be anonymous")
	}
}
