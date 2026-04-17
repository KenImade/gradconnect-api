package main

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"api.gradconnect.com/internal/data"
	_ "api.gradconnect.com/internal/data"
	"api.gradconnect.com/internal/validator"
)

type currentUserResponse struct {
	ID                 string          `json:"id"`
	Email              string          `json:"email"`
	Name               string          `json:"name"`
	AuthProvider       string          `json:"auth_provider"`
	EmailVerified      bool            `json:"email_verified"`
	DegreeDiscipline   *string         `json:"degree_discipline"`
	GraduationYear     *int            `json:"graduation_year"`
	TargetIndustries   []string        `json:"target_industries"`
	PreferredLocations []string        `json:"preferred_locations"`
	Preferences        json.RawMessage `json:"preferences"`
	Version            int             `json:"version"`
	Permissions        []string        `json:"permissions"`
	CreatedAt          time.Time       `json:"created_at"`
	UpdatedAt          time.Time       `json:"updated_at"`
}

// getCurrentUserHandler godoc
// @Summary      Get the currently authenticated user
// @Description  Returns the user associated with the current session, including their permissions.
// @Tags         Users
// @Produce      json
// @Success      200  {object}  currentUserResponse
// @Failure      401  {object}  ErrorResponse
// @Failure      500  {object}  ErrorResponse
// @Router       /me [get]
func (app *application) getCurrentUserHandler(w http.ResponseWriter, r *http.Request) {
	user := app.contextGetUser(r)

	permissions, err := app.models.Permissions.GetAllForUser(r.Context(), app.db, user.ID)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	response := currentUserResponse{
		ID:                 user.ID,
		Email:              user.Email,
		Name:               user.FirstName + " " + user.LastName,
		AuthProvider:       user.AuthProvider,
		EmailVerified:      user.EmailVerified,
		DegreeDiscipline:   user.DegreeDiscipline,
		GraduationYear:     user.GraduationYear,
		TargetIndustries:   user.TargetIndustries,
		PreferredLocations: user.PreferredLocations,
		Preferences:        user.Preferences,
		Version:            user.Version,
		Permissions:        permissions,
		CreatedAt:          user.CreatedAt,
		UpdatedAt:          user.UpdatedAt,
	}

	err = app.writeJSON(w, http.StatusOK, envelope{"data": response}, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}
}

type updateUserInput struct {
	FirstName          *string         `json:"first_name"`
	LastName           *string         `json:"last_name"`
	DegreeDiscipline   *string         `json:"degree_discipline"`
	GraduationYear     *int            `json:"graduation_year"`
	TargetIndustries   *[]string       `json:"target_industries"`
	PreferredLocations *[]string       `json:"preferred_locations"`
	Preferences        json.RawMessage `json:"preferences"`
}

// updateUserHandler godoc
// @Summary      Update the current user's profile
// @Description  Update the authenticated user's profile fields. Email cannot be changed.
// @Tags         Users
// @Accept       json
// @Produce      json
// @Param        user  body      updateUserInput  true  "Profile fields to update"
// @Success      200   {object}  data.User
// @Failure      400   {object}  ErrorResponse
// @Failure      409   {object}  ErrorResponse  "Edit conflict"
// @Failure      422   {object}  ErrorResponse
// @Failure      500   {object}  ErrorResponse
// @Router       /me [patch]
func (app *application) updateUserHandler(w http.ResponseWriter, r *http.Request) {
	var input updateUserInput

	err := app.readJSON(w, r, &input)
	if err != nil {
		app.badRequestResponse(w, r, err)
		return
	}

	user := app.contextGetUser(r)

	if input.FirstName != nil {
		user.FirstName = *input.FirstName
	}

	if input.LastName != nil {
		user.LastName = *input.LastName
	}

	if input.DegreeDiscipline != nil {
		user.DegreeDiscipline = input.DegreeDiscipline
	}

	if input.GraduationYear != nil {
		user.GraduationYear = input.GraduationYear
	}

	if input.TargetIndustries != nil {
		user.TargetIndustries = *input.TargetIndustries
	}
	if input.PreferredLocations != nil {
		user.PreferredLocations = *input.PreferredLocations
	}
	if input.Preferences != nil {
		user.Preferences = input.Preferences
	}

	v := validator.New()
	if data.ValidateUser(v, user); !v.Valid() {
		app.failedValidationResponse(w, r, v.Errors)
		return
	}

	err = app.models.Users.Update(r.Context(), app.db, user)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrEditConflict):
			app.errorResponse(w, r, http.StatusConflict, "edit conflict, please retry")
		default:
			app.serverErrorResponse(w, r, err)
		}
		return
	}

	err = app.writeJSON(w, http.StatusOK, envelope{"data": user}, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}

}

func (app *application) deleteUserHandler(w http.ResponseWriter, r *http.Request) {}
