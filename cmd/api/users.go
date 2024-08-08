package main

import (
	"errors"
	"net/http"

	"greenlight.mpdev.com/internal/data"
	"greenlight.mpdev.com/internal/validator"
)

func (app *application) registerUserHandler(w http.ResponseWriter, r *http.Request) {
	// decode destination*.
	var input struct {
		Name     string `json:"name"`
		Email    string `json:"email"`
		Password string `json:"password"`
	}

	// Parse the request body into the anonymous struct.
	err := app.readJSON(w, r, &input)
	if err != nil {
		app.badRequestResponse(w, r, err)
		return
	}

	user := &data.User{
		Name:      input.Name,
		Email:     input.Email,
		Activated: false,
	}

	err = user.Password.Set(input.Password)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	// Initialize a new Validator instance.
	v := validator.New()

	if data.ValidateUser(v, user); !v.Valid() {
		app.failedValidationResponse(w, r, v.Errors)
		return
	}

	err = app.models.Users.Insert(user)
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

	//headers := make(http.Header)
	//headers.Set("Location", fmt.Sprintf("/v1/users/%d", user.ID))

	err = app.writeJSON(w, http.StatusCreated, envelope{"user": user},
		nil, r)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}
}
