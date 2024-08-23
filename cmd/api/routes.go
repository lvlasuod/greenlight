package main

import (
	"net/http"

	"github.com/julienschmidt/httprouter"
)

func (app *application) routes() http.Handler {
	// Initialize a new httprouter router instance.
	router := httprouter.New()

	// Convert the notFoundResponse() helper to a http.Handler using the
	// http.HandlerFunc() adapter, and then set it as the custom error handler for 404
	// Not Found responses.
	router.NotFound = http.HandlerFunc(app.notFoundResponse)

	// Likewise, convert the methodNotAllowedResponse() helper to a http.Handler and set
	// it as the custom error handler for 405 Method Not Allowed responses.
	router.MethodNotAllowed = http.HandlerFunc(app.methodNotAllowedResponse)

	// Serving routers
	router.HandlerFunc(http.MethodGet, "/v1/healthcheck", app.healthcheckHandler) //Show application information

	// Movies
	router.HandlerFunc(http.MethodPost, "/v1/movies", app.createMovieHandler) // Create a new movie

	router.HandlerFunc(http.MethodGet, "/v1/movies", app.listMoviesHandler)    // Show the details of all Movies
	router.HandlerFunc(http.MethodGet, "/v1/movies/:id", app.showMovieHandler) // Show the details of a specific movie

	router.HandlerFunc(http.MethodPatch, "/v1/movies/:id", app.updateMovieHandler) // Update the details of a specific movie

	router.HandlerFunc(http.MethodDelete, "/v1/movies/:id", app.deleteMovieHandler) // Delete a specific movie

	// Users
	router.HandlerFunc(http.MethodPost, "/v1/users", app.registerUserHandler)          // Register a new user
	router.HandlerFunc(http.MethodPut, "/v1/users/activated", app.activateUserHandler) //Activate a specific user

	// POST /v1/tokens/activation endpoint.
	router.HandlerFunc(http.MethodPost, "/v1/tokens/activation", app.createActivationTokenHandler) //Generate a new activation token

	router.HandlerFunc(http.MethodPost, "/v1/tokens/authentication", app.createAuthenticationTokenHandler) //Generate a new authentication token

	// Wrap the router with the panic recovery middleware.
	return app.recoverPanic(app.rateLimit(app.authenticate(router)))
}
