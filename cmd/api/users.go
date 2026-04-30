package main

import (
	"errors"
	"net/http"

	"api.gradconnect.com/internal/data"
	"api.gradconnect.com/internal/validator"
)

// getCurrentUserHandler godoc
// @Summary      Get the currently authenticated user
// @Description  Returns the user associated with the current session, including their permissions.
// @Tags         Users
// @Produce      json
// @Success      200  {object}  data.CurrentUserResponse
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

	response := data.CurrentUserResponse{
		ID:                 user.ID,
		Email:              user.Email,
		FirstName:          user.FirstName,
		LastName:           user.LastName,
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

// updateUserHandler godoc
// @Summary      Update the current user's profile
// @Description  Update the authenticated user's profile fields. Email cannot be changed.
// @Tags         Users
// @Accept       json
// @Produce      json
// @Param        user  body      data.UpdateUserInput  true  "Profile fields to update"
// @Success      200   {object}  data.User
// @Failure      400   {object}  ErrorResponse
// @Failure      409   {object}  ErrorResponse  "Edit conflict"
// @Failure      422   {object}  ErrorResponse
// @Failure      500   {object}  ErrorResponse
// @Router       /me [patch]
func (app *application) updateUserHandler(w http.ResponseWriter, r *http.Request) {
	var input data.UpdateUserInput

	err := app.readJSON(w, r, &input)
	if err != nil {
		app.badRequestResponse(w, r, err)
		return
	}

	v := validator.New()
	data.ValidateUpdateUserInput(v, input)
	if !v.Valid() {
		app.failedValidationResponse(w, r, v.Errors)
		return
	}

	user := app.contextGetUser(r)

	// Apply the input to the user
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
