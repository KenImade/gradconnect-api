package app

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
func (app *App) registerUserHandler(w http.ResponseWriter, r *http.Request) {

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
		if err := app.models.Users.Insert(r.Context(), tx, user); err != nil {
			return err
		}

		var err error
		token, err = app.models.Tokens.New(r.Context(), tx, user.ID, 24*time.Hour, data.ScopeActivation)
		if err != nil {
			return err
		}

		taskPayload := map[string]any{
			"base_url":         app.config.BaseURL,
			"user_email":       user.Email,
			"first_name":       user.FirstName,
			"activation_token": token.Plaintext,
		}
		if err := app.models.Tasks.Insert(r.Context(), tx, "email:verify", taskPayload); err != nil {
			return err
		}

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
		Domain:   app.config.CookieDomain,
		Value:    session.ID,
		Expires:  session.ExpiresAt,
		HttpOnly: true,
		Secure:   app.config.Env == "production",
		SameSite: http.SameSiteLaxMode,
		Path:     "/",
	})

	headers := make(http.Header)
	headers.Set("Location", fmt.Sprintf("/api/v1/users/%s", user.ID))
	err = app.writeJSON(w, http.StatusCreated, envelope{"data": user}, headers)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}
}

// loginUserHandler godoc
// @Summary      Authenticate a user
// @Description  Validate credentials, create a new session, and set a session cookie. If the account is soft-deleted but still within the 30-day grace period, signing in restores it.
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
func (app *App) loginUserHandler(w http.ResponseWriter, r *http.Request) {
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

	limitKey := "login-failures:" + input.Email
	allowed, retryAfter := app.limiter.Peek(limitKey, 5, 15*time.Minute)
	if !allowed {
		app.rateLimitExceededResponse(w, r, retryAfter)
		return
	}

	// Lookup includes soft-deleted users so they can recover by signing in.
	user, err := app.models.Users.GetByEmailIncludingDeleted(r.Context(), app.db, input.Email)
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
		app.limiter.Allow(limitKey, 5, 15*time.Minute) // increment failure
		app.invalidCredentialsResponse(w, r)
		return
	}

	app.limiter.Reset(limitKey)

	ip, _, _ := net.SplitHostPort(r.RemoteAddr)

	// Restore + create session atomically. If restoration is needed, do
	// it inside the same transaction so we never end up with a session
	// pointing at a still-deleted user (or vice versa).
	var session *data.Session
	wasDeleted := user.DeletedAt != nil
	err = app.inTransaction(r.Context(), func(tx pgx.Tx) error {
		if wasDeleted {
			if err := app.models.Users.Restore(r.Context(), tx, user.ID); err != nil {
				return err
			}
			app.logger.Info("user account restored from soft-delete",
				"user_id", user.ID, "email", user.Email)

			taskPayload := map[string]any{
				"user_email": user.Email,
				"first_name": user.FirstName,
			}
			if err := app.models.Tasks.Insert(r.Context(), tx, "email:account_restored", taskPayload); err != nil {
				return err
			}
		}
		var err error
		session, err = app.models.Sessions.Create(r.Context(), tx, user.ID, ip, r.UserAgent())
		return err
	})
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "session_id",
		Domain:   app.config.CookieDomain,
		Value:    session.ID,
		Expires:  session.ExpiresAt,
		HttpOnly: true,
		Secure:   app.config.Env == "production",
		SameSite: http.SameSiteLaxMode,
		Path:     "/",
	})

	err = app.writeJSON(w, http.StatusOK, envelope{"data": user}, nil)
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
func (app *App) activateUserHandler(w http.ResponseWriter, r *http.Request) {
	tokenPlaintext := r.URL.Query().Get("token")

	v := validator.New()
	data.ValidateTokenPlaintext(v, tokenPlaintext)
	if !v.Valid() {
		http.Redirect(w, r, app.config.FrontendURL+"/verify-email?result=invalid", http.StatusFound)
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
			http.Redirect(w, r, app.config.FrontendURL+"/verify-email?result=invalid", http.StatusFound)
		// If you have specific expired / already-verified error sentinels, branch here:
		// case errors.Is(err, data.ErrTokenExpired):
		//     http.Redirect(w, r, app.config.FrontendURL+"/verify-email?result=expired", http.StatusFound)
		default:
			// On unexpected errors, still redirect rather than leak JSON
			app.logger.Error("activate user failed", "err", err)
			http.Redirect(w, r, app.config.FrontendURL+"/verify-email?result=error", http.StatusFound)
		}
		return
	}

	http.Redirect(w, r, app.config.FrontendURL+"/verify-email?result=success", http.StatusFound)
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
func (app *App) resendVerificationEmailHandler(w http.ResponseWriter, r *http.Request) {
	user := app.contextGetUser(r)

	limitKey := "resend-verification:" + user.ID
	allowed, retryAfter := app.limiter.Allow(limitKey, 1, 5*time.Minute)
	if !allowed {
		app.rateLimitExceededResponse(w, r, retryAfter)
		return
	}

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
			"base_url":         app.config.BaseURL,
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
func (app *App) googleAuthHandler(w http.ResponseWriter, r *http.Request) {
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
		ClientID:     app.config.Google.ClientID,
		ClientSecret: app.config.Google.ClientSecret,
		RedirectURL:  app.config.Google.RedirectURL,
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

	payload, err := idtoken.Validate(r.Context(), rawIDToken, app.config.Google.ClientID)
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

	user, err = app.models.Users.GetByEmailIncludingDeleted(r.Context(), app.db, email)
	if err != nil && !errors.Is(err, data.ErrRecordNotFound) {
		app.serverErrorResponse(w, r, err)
		return
	}

	if errors.Is(err, data.ErrRecordNotFound) {
		// NEW USER PATH.
		// Create the user, assign default permissions, queue welcome
		// email, create session — all in one transaction so a partial
		// failure doesn't leave a half-built account.
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
				// Race condition — another request just created this user.
				app.errorResponse(w, r, http.StatusConflict, "user creation conflict, please retry")
			default:
				app.serverErrorResponse(w, r, err)
			}
			return
		}
	} else {
		// EXISTING USER PATH.
		// Guard against provider collision, then restore if soft-deleted,
		// queue the restoration email, and create a session.
		if user.AuthProvider != "google" {
			app.errorResponse(w, r, http.StatusConflict, "this email is registered with a different sign-in method")
			return
		}

		wasDeleted := user.DeletedAt != nil
		err = app.inTransaction(r.Context(), func(tx pgx.Tx) error {
			if wasDeleted {
				if err := app.models.Users.Restore(r.Context(), tx, user.ID); err != nil {
					return err
				}
				app.logger.Info("user account restored from soft-delete via google auth",
					"user_id", user.ID, "email", user.Email)

				taskPayload := map[string]any{
					"user_email": user.Email,
					"first_name": user.FirstName,
				}
				if err := app.models.Tasks.Insert(r.Context(), tx, "email:account_restored", taskPayload); err != nil {
					return err
				}
			}
			var err error
			session, err = app.models.Sessions.Create(r.Context(), tx, user.ID, ip, r.UserAgent())
			return err
		})
		if err != nil {
			app.serverErrorResponse(w, r, err)
			return
		}
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "session_id",
		Domain:   app.config.CookieDomain,
		Value:    session.ID,
		Expires:  session.ExpiresAt,
		HttpOnly: true,
		Secure:   app.config.Env == "production",
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
func (app *App) logoutUserHandler(w http.ResponseWriter, r *http.Request) {
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
		Domain:   app.config.CookieDomain,
		Value:    "",
		Expires:  time.Unix(0, 0),
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   app.config.Env == "production",
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
func (app *App) forgotPasswordHandler(w http.ResponseWriter, r *http.Request) {
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

	limitKey := "forgot-password:" + input.Email
	allowed, retryAfter := app.limiter.Allow(limitKey, 3, time.Hour)
	if !allowed {
		app.rateLimitExceededResponse(w, r, retryAfter)
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
			"frontend_url": app.config.FrontendURL,
			"user_email":   user.Email,
			"first_name":   user.FirstName,
			"reset_token":  token.Plaintext,
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
func (app *App) resetPasswordHandler(w http.ResponseWriter, r *http.Request) {
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

// changePasswordHandler godoc
// @Summary      Change the current user's password
// @Description  Verify the current password, set a new one, and revoke all other sessions. A confirmation email is sent on success.
// @Tags         Auth
// @Accept       json
// @Produce      json
// @Param        body  body      data.ChangePasswordInput  true  "Current and new password"
// @Success      200   {object}  envelope
// @Failure      400   {object}  ErrorResponse
// @Failure      401   {object}  ErrorResponse  "Current password is incorrect"
// @Failure      409   {object}  ErrorResponse  "User signed in with an external provider"
// @Failure      422   {object}  ErrorResponse
// @Failure      429   {object}  ErrorResponse
// @Failure      500   {object}  ErrorResponse
// @Router       /me/password [post]
func (app *App) changePasswordHandler(w http.ResponseWriter, r *http.Request) {
	user := app.contextGetUser(r)

	// Reject for non-email providers. Google users don't have a password
	// at GradConnect — their authentication happens at Google.
	if user.AuthProvider != "email" {
		app.errorResponse(w, r, http.StatusConflict,
			"this account uses Google sign-in; manage your password via your Google account")
		return
	}

	var input data.ChangePasswordInput
	if err := app.readJSON(w, r, &input); err != nil {
		app.badRequestResponse(w, r, err)
		return
	}

	v := validator.New()
	if data.ValidateChangePasswordInput(v, input); !v.Valid() {
		app.failedValidationResponse(w, r, v.Errors)
		return
	}

	// Rate-limit password change attempts per user. A leaked session
	// token shouldn't allow unlimited guessing of the current password.
	limitKey := "change-password:" + user.ID
	allowed, retryAfter := app.limiter.Peek(limitKey, 5, time.Hour)
	if !allowed {
		app.rateLimitExceededResponse(w, r, retryAfter)
		return
	}

	// Verify current password before doing anything else.
	match, err := user.Password.Matches(input.CurrentPassword)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}
	if !match {
		app.limiter.Allow(limitKey, 5, time.Hour) // count this failure
		app.invalidCredentialsResponse(w, r)
		return
	}

	app.limiter.Reset(limitKey)

	hash, err := data.HashPassword(input.NewPassword)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	// Current session ID, so we can keep this device logged in while
	// revoking everywhere else.
	cookie, err := r.Cookie("session_id")
	if err != nil {
		// Should be impossible — the auth middleware just succeeded.
		// Treat as auth failure rather than 500 to fail safe.
		app.authenticationRequiredResponse(w, r)
		return
	}
	currentSessionID := cookie.Value

	err = app.inTransaction(r.Context(), func(tx pgx.Tx) error {
		if err := app.models.Users.UpdatePassword(r.Context(), tx, user.ID, hash); err != nil {
			return err
		}

		if err := app.models.Sessions.DeleteAllForUserExcept(r.Context(), tx, user.ID, currentSessionID); err != nil {
			return err
		}

		// Confirmation email: notifies the user that the change happened
		// so they can react if it wasn't them. Best-effort — failure here
		// shouldn't roll back the password change.
		taskPayload := map[string]any{
			"user_email": user.Email,
			"first_name": user.FirstName,
		}
		return app.models.Tasks.Insert(r.Context(), tx, "email:password_changed", taskPayload)
	})

	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	response := envelope{"data": envelope{
		"message": "Password updated. You've been signed out of all other devices.",
	}}
	if err := app.writeJSON(w, http.StatusOK, response, nil); err != nil {
		app.serverErrorResponse(w, r, err)
	}
}

type deleteAccountInput struct {
	Password     string `json:"password"`     // empty for Google users
	Confirmation string `json:"confirmation"` // must equal "DELETE"
	Reason       string `json:"reason"`       // optional
}

// deleteAccountHandler godoc
// @Summary      Delete the current user's account
// @Description  Soft-deletes the user's account. The account becomes
// @Description  inaccessible immediately and is permanently removed after
// @Description  30 days. The user can recover by logging in within the
// @Description  grace period (a separate undelete endpoint, not exposed
// @Description  in this version — they'd contact support).
// @Tags         Auth
// @Accept       json
// @Produce      json
// @Param        body  body      deleteAccountInput  true  "Deletion confirmation"
// @Success      204
// @Failure      400   {object}  ErrorResponse
// @Failure      401   {object}  ErrorResponse  "Password incorrect"
// @Failure      422   {object}  ErrorResponse  "Confirmation phrase missing or wrong"
// @Failure      500   {object}  ErrorResponse
// @Router       /me [delete]
func (app *App) deleteAccountHandler(w http.ResponseWriter, r *http.Request) {
	user := app.contextGetUser(r)

	var input deleteAccountInput
	if err := app.readJSON(w, r, &input); err != nil {
		app.badRequestResponse(w, r, err)
		return
	}

	if input.Confirmation != "DELETE" {
		v := validator.New()
		v.AddError("confirmation", "type DELETE to confirm")
		app.failedValidationResponse(w, r, v.Errors)
		return
	}

	if user.AuthProvider == "email" {
		if input.Password == "" {
			v := validator.New()
			v.AddError("password", "must be provided")
			app.failedValidationResponse(w, r, v.Errors)
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
	}

	err := app.inTransaction(r.Context(), func(tx pgx.Tx) error {
		if err := app.models.Users.SoftDelete(r.Context(), tx, user.ID, input.Reason); err != nil {
			return err
		}
		if err := app.models.Sessions.DeleteAllForUser(r.Context(), tx, user.ID); err != nil {
			return err
		}

		taskPayload := map[string]any{
			"user_email": user.Email,
			"first_name": user.FirstName,
		}
		return app.models.Tasks.Insert(r.Context(), tx, "email:account_deleted", taskPayload)
	})
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	// Clear the session cookie. The session row is gone from the DB;
	// clearing the cookie prevents the browser from sending stale
	// credentials on the next request.
	http.SetCookie(w, &http.Cookie{
		Name:     "session_id",
		Domain:   app.config.CookieDomain,
		Value:    "",
		Expires:  time.Unix(0, 0),
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   app.config.Env == "production",
		SameSite: http.SameSiteLaxMode,
		Path:     "/",
	})

	w.WriteHeader(http.StatusNoContent)
}
