package main

// func (app *application) createAuthenticationTokenHandler(w http.ResponseWriter, r *http.Request) {
// 	var input struct {
// 		Email    string `json:"email"`
// 		Password string `json:"password"`
// 	}

// 	err := app.readJSON(w, r, &input)
// 	if err != nil {
// 		app.badRequestResponse(w, r, err)
// 		return
// 	}

// 	// Validate the input
// 	v := validator.New()

// 	data.ValidateEmail(v, input.Email)
// 	data.ValidatePasswordPlaintext(v, input.Password)

// 	if !v.Valid() {
// 		app.failedValidationResponse(w, r, v.Errors)
// 		return
// 	}

// 	// Check for user
// 	user, err := app.models.Users.GetByEmail(input.Email)
// 	if err != nil {
// 		switch {
// 		case errors.Is(err, data.ErrRecordNotFound):
// 			app.invalidCredentialsResponse(w, r)
// 		default:
// 			app.serverErrorResponse(w, r, err)
// 		}
// 		return
// 	}

// 	match, err := user.Password.Matches(input.Password)
// 	if err != nil {
// 		app.serverErrorResponse(w, r, err)
// 		return
// 	}

// 	if !match {
// 		app.invalidCredentialsResponse(w, r)
// 		return
// 	}

// 	// 1. Create a session record (Table 5)
// 	// We don't hash these; the UUID ID itself is the token
// 	session, err := app.models.Sessions.New(user.ID, r.RemoteAddr, r.UserAgent())
// 	if err != nil {
// 		app.serverErrorResponse(w, r, err)
// 		return
// 	}

// 	// 2. Return the session ID to the client
// 	// The client will send this back in a cookie or Authorization header
// 	err = app.writeJSON(w, http.StatusCreated, envelope{"session_token": session.ID}, nil)
// }
