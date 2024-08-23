package main

import (
	"fmt"
	"net/http"
	"time"

	"github.com/jackc/pgx"
	"greenlight.mpdev.com/internal/data"
	"greenlight.mpdev.com/internal/validator"
)

func (app *application) createActivationTokenHandler(w http.ResponseWriter, r *http.Request) {

	// Parse and validate the user's email address.
	var input struct {
		Email string `json:"email"`
	}

	err := app.readJSON(w, r, &input)
	if err != nil {
		app.badRequestResponse(w, r, err)
		return
	}

	v := validator.New()
	if data.ValidateEmail(v, input.Email); !v.Valid() {
		app.failedValidationResponse(w, r, v.Errors)
		return
	}

	// Try to retrieve the corresponding user record for the email address.
	user, err := app.models.Users.GetByEmail(input.Email)
	if err != nil {
		switch {
		case err.Error() == pgx.ErrNoRows.Error():
			v.AddError("email", "no matching email address found")
			app.failedValidationResponse(w, r, v.Errors)
		default:
			app.serverErrorResponse(w, r, err)
		}
		return
	}

	// Return an error if the user has already been activated.
	if user.Activated {
		v.AddError("email", "user has already been activated")
		app.failedValidationResponse(w, r, v.Errors)
		return
	}

	// Otherwise, create a new activation token.
	token, err := app.models.Tokens.New(user.ID, 3*24*time.Hour, data.ScopeActivation)

	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	// Email the user with their additional activation token.
	app.background(func() {
		data := map[string]interface{}{
			"activationToken": token.Plaintext,
		}

		// remove the below line in production
		fmt.Printf("Token %s\n", token.Plaintext)
		// Since email addresses MAY be case sensitive, notice that we are sending this
		// email using the address stored in our database for the user ---  not to the
		// input.Email address provided by the client in this request.
		err = app.mailer.Send(user.Email, "token_activation.tmpl", data)
		if err != nil {
			app.logger.Printf(err.Error(), nil)
		}
	})

	// Send the 202 Accepted response and confirmation message to the client.
	env := envelope{"message": "an email will be sent to you containing activation instructions"}

	err = app.writeJSON(w, http.StatusAccepted, env, nil, r)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}
}

func (app *application) createAuthenticationTokenHandler(w http.ResponseWriter, r *http.Request) {

	// Parse the email and password from the request body.
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

	// Try to retrieve the corresponding user record for the email address.
	user, err := app.models.Users.GetByEmail(input.Email)
	if err != nil {
		switch {
		case err.Error() == pgx.ErrNoRows.Error():

			app.invalidCredentialsResponse(w, r)

		default:
			app.serverErrorResponse(w, r, err)
		}
		return
	}

	// Check if the provided password matches the actual password for the user.
	match, err := user.Password.Matches(input.Password)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	if !match {
		app.invalidCredentialsResponse(w, r)
		return
	}

	// Otherwise, if the password is correct, we generate a new token with a 24-hour expiry time and the scope 'authentication'.
	token, err := app.models.Tokens.New(user.ID, 24*time.Hour, data.ScopeAuthentication)

	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	env := envelope{"authentication_token": token}

	err = app.writeJSON(w, http.StatusCreated, env, nil, r)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}
}
