package main

import (
	"expvar"
	"net/http"

	"github.com/julienschmidt/httprouter"
	"github.com/prometheus/client_golang/prometheus/promhttp"
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
	router.HandlerFunc(http.MethodPost, "/v1/movies", app.requirePermission("movies:write", app.createMovieHandler)) // Create a new movie

	router.HandlerFunc(http.MethodGet, "/v1/movies", app.requirePermission("movies:read", app.listMoviesHandler))    // Show the details of all Movies
	router.HandlerFunc(http.MethodGet, "/v1/movies/:id", app.requirePermission("movies:read", app.showMovieHandler)) // Show the details of a specific movie

	router.HandlerFunc(http.MethodPatch, "/v1/movies/:id", app.requirePermission("movies:write", app.updateMovieHandler)) // Update the details of a specific movie

	router.HandlerFunc(http.MethodDelete, "/v1/movies/:id", app.requirePermission("movies:write", app.deleteMovieHandler)) // Delete a specific movie

	// Users
	router.HandlerFunc(http.MethodPost, "/v1/users", app.registerUserHandler)          // Register a new user
	router.HandlerFunc(http.MethodPut, "/v1/users/activated", app.activateUserHandler) //Activate a specific user

	// POST /v1/tokens/activation endpoint.
	router.HandlerFunc(http.MethodPost, "/v1/tokens/activation", app.createActivationTokenHandler) //Generate a new activation token

	router.HandlerFunc(http.MethodPost, "/v1/tokens/authentication", app.createAuthenticationTokenHandler) //Generate a new authentication token

	router.Handler(http.MethodGet, "/v1/metrics", expvar.Handler())

	router.Handler(http.MethodGet, "/metrics", promhttp.Handler())

	// Wrap the router with the panic recovery middleware.
	return app.metrics(app.recoverPanic(app.enableCORS(app.rateLimit(app.authenticate(router)))))
}
