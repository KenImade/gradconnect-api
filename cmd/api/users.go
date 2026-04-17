package main

import (
	"encoding/json"
	"net/http"
	"time"

	_ "api.gradconnect.com/internal/data"
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

func (app *application) updaterUserHandler(w http.ResponseWriter, r *http.Request) {}

func (app *application) deleteUserHandler(w http.ResponseWriter, r *http.Request) {}
