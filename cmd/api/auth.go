package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"time"

	"api.gradconnect.com/internal/data"
	"api.gradconnect.com/internal/validator"
	"github.com/jackc/pgx/v5"
)

type registerUserInput struct {
	FirstName          string          `json:"first_name" example:"John"`
	LastName           string          `json:"last_name" example:"Doe"`
	Email              string          `json:"email" example:"john@example.com"`
	Password           string          `json:"password" example:"pa55word"`
	DegreeDiscipline   *string         `json:"degree_discipline" example:"Computer Science"`
	GraduationYear     *int            `json:"graduation_year" example:"2025"`
	TargetIndustries   []string        `json:"target_industries" example:"[\"Finance\", \"Tech\"]"`
	PreferredLocations []string        `json:"preferred_locations" example:"[\"Lagos\", \"Abuja\"]"`
	Preferences        json.RawMessage `json:"preferences" swaggertype:"object"`
}

// registerUserHandler godoc
// @Summary      Register a new user
// @Description  Create a new user account, assign default permissions, and queue an activation email.
// @Tags         Auth
// @Accept       json
// @Produce      json
// @Param        user  body      registerUserInput  true  "User Registration Details"
// @Success      201   {object}  data.User
// @Failure      400   {object}  ErrorResponse
// @Failure      422   {object}  ErrorResponse
// @Failure      500   {object}  ErrorResponse
// @Router      /auth/register [post]
func (app *application) registerUserHandler(w http.ResponseWriter, r *http.Request) {

	var input registerUserInput

	err := app.readJSON(w, r, &input)
	if err != nil {
		app.badRequestResponse(w, r, err)
		return
	}

	user := &data.User{
		Email:              input.Email,
		FirstName:          input.FirstName,
		LastName:           input.LastName,
		AuthProvider:       "email",
		EmailVerified:      false,
		DegreeDiscipline:   input.DegreeDiscipline,
		GraduationYear:     input.GraduationYear,
		TargetIndustries:   input.TargetIndustries,
		PreferredLocations: input.PreferredLocations,
		Preferences:        input.Preferences,
	}

	err = user.Password.Set(input.Password)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	v := validator.New()

	if data.ValidateUser(v, user); !v.Valid() {
		app.failedValidationResponse(w, r, v.Errors)
		return
	}

	var token *data.Token
	var session *data.Session

	ip, _, _ := net.SplitHostPort(r.RemoteAddr)

	// --- TRANSACTION START ---
	err = app.inTransaction(r.Context(), func(tx pgx.Tx) error {
		// Insert the user
		if err := app.models.Users.Insert(r.Context(), tx, user); err != nil {
			return err
		}

		// Create activation token
		var err error
		token, err = app.models.Tokens.New(r.Context(), tx, user.ID, 24*time.Hour, data.ScopeActivation)
		if err != nil {
			return err
		}

		// Queue the welcome email task
		taskPayload := map[string]any{
			"user_email":       user.Email,
			"first_name":       user.FirstName,
			"activation_token": token.Plaintext,
		}
		if err := app.models.Tasks.Insert(r.Context(), tx, "email:verify", taskPayload); err != nil {
			return err
		}

		// Create session
		session, err = app.models.Sessions.Create(r.Context(), tx, user.ID, ip, r.UserAgent())
		return err
	})
	// --- TRANSACTION END ---

	if err != nil {
		switch {
		case errors.Is(err, data.ErrDuplicateEmail):
			v.AddError("email", "a user with this email address already exists")
			app.failedValidationResponse(w, r, v.Errors)
		default:
			app.serverErrorResponse(w, r, err)
		}
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "session_id",
		Value:    session.ID,
		Expires:  session.ExpiresAt,
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
		Path:     "/",
	})

	headers := make(http.Header)
	headers.Set("Location", fmt.Sprintf("/api/v1/users/%s", user.ID))
	err = app.writeJSON(w, http.StatusCreated, envelope{"user": user}, headers)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}
}

// loginUserHandler godoc
// @Summary      Authenticate a user
// @Description  Validate credentials, create a new session, and set a session cookie.
// @Tags         Auth
// @Accept       json
// @Produce      json
// @Param        credentials  body      object{email=string,password=string}  true  "Login Credentials"
// @Success      200          {object}  data.User
// @Failure      400          {object}  ErrorResponse
// @Failure      401          {object}  ErrorResponse
// @Failure      422          {object}  ErrorResponse
// @Failure      500          {object}  ErrorResponse
// @Router       /auth/login [post]
func (app *application) loginUserHandler(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}

	err := app.readJSON(w, r, &input)
	if err != nil {
		app.badRequestResponse(w, r, err)
		return
	}

	v := validator.New()
	data.ValidateEmail(v, input.Email)
	data.ValidatePasswordPlaintext(v, input.Password)
	if !v.Valid() {
		app.failedValidationResponse(w, r, v.Errors)
		return
	}

	user, err := app.models.Users.GetByEmail(r.Context(), app.db, input.Email)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrRecordNotFound):
			app.invalidCredentialsResponse(w, r)
		default:
			app.serverErrorResponse(w, r, err)
		}
		return
	}

	match, err := user.Password.Matches(input.Password)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}
	if !match {
		app.invalidCredentialsResponse(w, r)
		return
	}

	ip, _, _ := net.SplitHostPort(r.RemoteAddr)

	session, err := app.models.Sessions.Create(r.Context(), app.db, user.ID, ip, r.UserAgent())
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "session_id",
		Value:    session.ID,
		Expires:  session.ExpiresAt,
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
		Path:     "/",
	})

	err = app.writeJSON(w, http.StatusOK, envelope{"user": user}, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}
}

// activateUserHandler godoc
// @Summary      Activate a user account
// @Description  Verify a user's email address using the activation token sent by email.
// @Tags         Auth
// @Accept       json
// @Produce      json
// @Param        token  query     string  true  "Activation token"
// @Success      302    "Redirect to frontend success page"
// @Failure      302    "Redirect to frontend failure page on invalid/expired token"
// @Failure      422    {object}  ErrorResponse  "Missing or malformed token"
// @Failure      500    {object}  ErrorResponse
// @Router       /auth/verify-email [get]
func (app *application) activateUserHandler(w http.ResponseWriter, r *http.Request) {
	tokenPlaintext := r.URL.Query().Get("token")

	v := validator.New()
	v.Check(tokenPlaintext != "", "token", "must be provided")
	v.Check(len(tokenPlaintext) == 26, "token", "must be 26 bytes long")
	if !v.Valid() {
		app.failedValidationResponse(w, r, v.Errors)
		return
	}

	var user *data.User

	err := app.inTransaction(r.Context(), func(tx pgx.Tx) error {
		userID, err := app.models.Tokens.GetUserForToken(r.Context(), tx, data.ScopeActivation, tokenPlaintext)
		if err != nil {
			return err
		}

		if err := app.models.Users.Activate(r.Context(), tx, userID); err != nil {
			return err
		}

		permissions := []string{"review:submit", "review:edit"}
		if err := app.models.Permissions.AddForUser(r.Context(), tx, userID, permissions...); err != nil {
			return err
		}

		if err := app.models.Tokens.DeleteAllForUser(r.Context(), tx, data.ScopeActivation, userID); err != nil {
			return err
		}

		user, err = app.models.Users.GetByID(r.Context(), tx, userID)
		if err != nil {
			return err
		}

		// Queue the welcome email task
		welcomePayload := map[string]any{
			"user_email": user.Email,
			"first_name": user.FirstName,
		}
		return app.models.Tasks.Insert(r.Context(), tx, "email:welcome", welcomePayload)
	})

	if err != nil {
		switch {
		case errors.Is(err, data.ErrRecordNotFound):
			http.Redirect(w, r, app.config.frontendURL+"/verify/failed", http.StatusFound)
		default:
			app.serverErrorResponse(w, r, err)
		}
		return
	}

	http.Redirect(w, r, app.config.frontendURL+"/verify/success", http.StatusFound)
}

// resendVerificationEmailHandler godoc
// @Summary      Resend email verification link
// @Description  Deletes any existing verification token for the authenticated user and queues a new verification email.
// @Tags         Auth
// @Accept       json
// @Produce      json
// @Success      200  {object}  envelope
// @Failure      401  {object}  ErrorResponse
// @Failure      409  {object}  ErrorResponse  "Email already verified"
// @Failure      500  {object}  ErrorResponse
// @Router       /auth/resend-verification [post]
func (app *application) resendVerificationEmailHandler(w http.ResponseWriter, r *http.Request) {
	user := app.contextGetUser(r)

	if user.EmailVerified {
		app.errorResponse(w, r, http.StatusConflict, "email is already verified")
		return
	}

	err := app.inTransaction(r.Context(), func(tx pgx.Tx) error {
		token, err := app.models.Tokens.New(r.Context(), tx, user.ID, 24*time.Hour, data.ScopeActivation)
		if err != nil {
			return err
		}

		taskPayload := map[string]any{
			"user_email":       user.Email,
			"first_name":       user.FirstName,
			"activation_token": token.Plaintext,
		}
		return app.models.Tasks.Insert(r.Context(), tx, "email:verify", taskPayload)
	})

	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	err = app.writeJSON(w, http.StatusOK, envelope{"message": "verification email sent"}, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}
}
