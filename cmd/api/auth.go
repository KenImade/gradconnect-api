package main

import (
	"errors"
	"fmt"
	"net"
	"net/http"
	"time"

	"api.gradconnect.com/internal/data"
	"api.gradconnect.com/internal/validator"
	"github.com/jackc/pgx/v5"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/idtoken"
)

// registerUserHandler godoc
// @Summary      Register a new user
// @Description  Create a new user account, assign default permissions, and queue an activation email.
// @Tags         Auth
// @Accept       json
// @Produce      json
// @Param        user  body      data.CreateUserInput  true  "User Registration Details"
// @Success      201   {object}  data.User
// @Failure      400   {object}  ErrorResponse
// @Failure      422   {object}  ErrorResponse
// @Failure      500   {object}  ErrorResponse
// @Router      /auth/register [post]
func (app *application) registerUserHandler(w http.ResponseWriter, r *http.Request) {

	var input data.CreateUserInput

	err := app.readJSON(w, r, &input)
	if err != nil {
		app.badRequestResponse(w, r, err)
		return
	}

	v := validator.New()
	if data.ValidateCreateUserInput(v, input); !v.Valid() {
		app.failedValidationResponse(w, r, v.Errors)
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
	var input data.LoginUserInput

	err := app.readJSON(w, r, &input)
	if err != nil {
		app.badRequestResponse(w, r, err)
		return
	}

	v := validator.New()
	data.ValidateLoginUserInput(v, input)
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
	data.ValidateTokenPlaintext(v, tokenPlaintext)
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

// googleAuthHandler godoc
// @Summary      Authenticate with Google OAuth
// @Description  Exchange a Google OAuth authorization code for a session. Creates a new user if one doesn't exist.
// @Tags         Auth
// @Accept       json
// @Produce      json
// @Param        body  body      data.GoogleAuthInput  true  "Google OAuth authorization code"
// @Success      200   {object}  data.User  "Existing user logged in"
// @Success      201   {object}  data.User  "New user created"
// @Failure      400   {object}  ErrorResponse
// @Failure      401   {object}  ErrorResponse
// @Failure      500   {object}  ErrorResponse
// @Router       /auth/google [post]
func (app *application) googleAuthHandler(w http.ResponseWriter, r *http.Request) {
	var input data.GoogleAuthInput

	err := app.readJSON(w, r, &input)
	if err != nil {
		app.badRequestResponse(w, r, err)
		return
	}

	if input.Code == "" {
		app.badRequestResponse(w, r, errors.New("authorization code is required"))
		return
	}

	oauthConfig := oauth2.Config{
		ClientID:     app.config.google.clientID,
		ClientSecret: app.config.google.clientSecret,
		RedirectURL:  app.config.google.redirectURL,
		Scopes:       []string{"openid", "email", "profile"},
		Endpoint:     google.Endpoint,
	}

	token, err := oauthConfig.Exchange(r.Context(), input.Code)
	if err != nil {
		app.errorResponse(w, r, http.StatusUnauthorized, "failed to exchange authorization code")
		return
	}

	rawIDToken, ok := token.Extra("id_token").(string)
	if !ok {
		app.errorResponse(w, r, http.StatusUnauthorized, "no id_token in oauth response")
		return
	}

	payload, err := idtoken.Validate(r.Context(), rawIDToken, app.config.google.clientID)
	if err != nil {
		app.errorResponse(w, r, http.StatusUnauthorized, "invalid id_token")
		return
	}

	email, _ := payload.Claims["email"].(string)
	emailVerified, _ := payload.Claims["email_verified"].(bool)
	givenName, _ := payload.Claims["given_name"].(string)
	familyName, _ := payload.Claims["family_name"].(string)

	if email == "" || !emailVerified {
		app.errorResponse(w, r, http.StatusUnauthorized, "email not verified by google")
		return
	}

	ip, _, _ := net.SplitHostPort(r.RemoteAddr)

	var user *data.User
	var session *data.Session
	isNewUser := false

	// Check if user exists
	user, err = app.models.Users.GetByEmail(r.Context(), app.db, email)
	if err != nil && !errors.Is(err, data.ErrRecordNotFound) {
		app.serverErrorResponse(w, r, err)
		return
	}

	if errors.Is(err, data.ErrRecordNotFound) {
		isNewUser = true

		user = &data.User{
			Email:         email,
			FirstName:     givenName,
			LastName:      familyName,
			AuthProvider:  "google",
			EmailVerified: true,
		}

		err = app.inTransaction(r.Context(), func(tx pgx.Tx) error {
			if err := app.models.Users.Insert(r.Context(), tx, user); err != nil {
				return err
			}

			if err := app.models.Permissions.AddForUser(r.Context(), tx, user.ID, "review:submit", "review:edit"); err != nil {
				return err
			}

			welcomePayload := map[string]any{
				"user_email": user.Email,
				"first_name": user.FirstName,
			}
			if err := app.models.Tasks.Insert(r.Context(), tx, "email:welcome", welcomePayload); err != nil {
				return err
			}

			session, err = app.models.Sessions.Create(r.Context(), tx, user.ID, ip, r.UserAgent())
			return err
		})

		if err != nil {
			switch {
			case errors.Is(err, data.ErrDuplicateEmail):
				// Race condition — another request just created this user. Retry login path.
				app.errorResponse(w, r, http.StatusConflict, "user creation conflict, please retry")
			default:
				app.serverErrorResponse(w, r, err)
			}
			return
		}
	} else {
		// Existing user — guard against email/google account collision
		if user.AuthProvider != "google" {
			app.errorResponse(w, r, http.StatusConflict, "this email is registered with a different sign-in method")
			return
		}
		session, err = app.models.Sessions.Create(r.Context(), app.db, user.ID, ip, r.UserAgent())
		if err != nil {
			app.serverErrorResponse(w, r, err)
			return
		}
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

	status := http.StatusOK
	if isNewUser {
		status = http.StatusCreated
	}

	err = app.writeJSON(w, status, envelope{"data": user}, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}
}

// logoutUserHandler godoc
// @Summary      Log out the current user
// @Description  Delete the current session and clear the session cookie.
// @Tags         Auth
// @Produce      json
// @Success      204
// @Failure      401  {object}  ErrorResponse
// @Failure      500  {object}  ErrorResponse
// @Router       /auth/logout [post]
func (app *application) logoutUserHandler(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie("session_id")
	if err != nil {
		app.authenticationRequiredResponse(w, r)
		return
	}

	err = app.models.Sessions.Delete(r.Context(), app.db, cookie.Value)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "session_id",
		Value:    "",
		Expires:  time.Unix(0, 0),
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   app.config.env == "production",
		SameSite: http.SameSiteLaxMode,
		Path:     "/",
	})

	w.WriteHeader(http.StatusNoContent)
}

// forgotPasswordHandler godoc
// @Summary      Request a password reset email
// @Description  Sends a password reset email if an account with the given email exists. Always returns 200 to prevent email enumeration.
// @Tags         Auth
// @Accept       json
// @Produce      json
// @Param        body  body      data.ForgotPasswordInput  true  "Email address"
// @Success      200   {object}  envelope
// @Failure      400   {object}  ErrorResponse
// @Failure      422   {object}  ErrorResponse
// @Failure      500   {object}  ErrorResponse
// @Router       /auth/forgot-password [post]
func (app *application) forgotPasswordHandler(w http.ResponseWriter, r *http.Request) {
	var input data.ForgotPasswordInput

	err := app.readJSON(w, r, &input)
	if err != nil {
		app.badRequestResponse(w, r, err)
		return
	}

	v := validator.New()
	data.ValidateEmail(v, input.Email)
	if !v.Valid() {
		app.failedValidationResponse(w, r, v.Errors)
		return
	}

	response := envelope{"data": envelope{
		"message": "If an account with that email exists, a reset link has been sent.",
	}}

	user, err := app.models.Users.GetByEmail(r.Context(), app.db, input.Email)
	if err != nil {
		if errors.Is(err, data.ErrRecordNotFound) {
			// Silently succeed to prevent email enumeration
			app.writeJSON(w, http.StatusOK, response, nil)
			return
		}
		app.serverErrorResponse(w, r, err)
		return
	}

	// Skip for non-email auth providers (Google users don't have passwords here)
	if user.AuthProvider != "email" {
		app.writeJSON(w, http.StatusOK, response, nil)
		return
	}

	err = app.inTransaction(r.Context(), func(tx pgx.Tx) error {
		token, err := app.models.Tokens.New(r.Context(), tx, user.ID, 1*time.Hour, data.ScopePasswordReset)
		if err != nil {
			return err
		}

		taskPayload := map[string]any{
			"user_email":  user.Email,
			"first_name":  user.FirstName,
			"reset_token": token.Plaintext,
		}
		return app.models.Tasks.Insert(r.Context(), tx, "email:password_reset", taskPayload)
	})

	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	err = app.writeJSON(w, http.StatusOK, response, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}
}

// resetPasswordHandler godoc
// @Summary      Reset password using a reset token
// @Description  Validates a password reset token and sets a new password. Revokes all existing sessions.
// @Tags         Auth
// @Accept       json
// @Produce      json
// @Param        body  body      data.ResetPasswordInput  true  "Reset token and new password"
// @Success      200   {object}  envelope
// @Failure      400   {object}  ErrorResponse  "Invalid or expired token"
// @Failure      422   {object}  ErrorResponse
// @Failure      500   {object}  ErrorResponse
// @Router       /auth/reset-password [post]
func (app *application) resetPasswordHandler(w http.ResponseWriter, r *http.Request) {
	var input data.ResetPasswordInput

	err := app.readJSON(w, r, &input)
	if err != nil {
		app.badRequestResponse(w, r, err)
		return
	}

	v := validator.New()
	data.ValidateTokenPlaintext(v, input.Token)
	data.ValidatePasswordPlaintext(v, input.NewPassword)
	if !v.Valid() {
		app.failedValidationResponse(w, r, v.Errors)
		return
	}

	hash, err := data.HashPassword(input.NewPassword)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	err = app.inTransaction(r.Context(), func(tx pgx.Tx) error {
		userID, err := app.models.Tokens.GetUserForToken(r.Context(), tx, data.ScopePasswordReset, input.Token)
		if err != nil {
			return err
		}

		if err := app.models.Users.UpdatePassword(r.Context(), tx, userID, hash); err != nil {
			return err
		}

		if err := app.models.Tokens.DeleteAllForUser(r.Context(), tx, data.ScopePasswordReset, userID); err != nil {
			return err
		}

		return app.models.Sessions.DeleteAllForUser(r.Context(), tx, userID)
	})

	if err != nil {
		switch {
		case errors.Is(err, data.ErrRecordNotFound):
			app.errorResponse(w, r, http.StatusBadRequest, "invalid or expired reset token")
		default:
			app.serverErrorResponse(w, r, err)
		}
		return
	}

	response := envelope{"data": envelope{
		"message": "Password updated successfully. Please log in.",
	}}
	err = app.writeJSON(w, http.StatusOK, response, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}
}
