package main

import (
	"errors"
	"expvar"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/felixge/httpsnoop"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"golang.org/x/time/rate"
	"greenlight.mpdev.com/internal/data"
	"greenlight.mpdev.com/internal/validator"
)

func (app *application) recoverPanic(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Create a deferred function (which will always be run in the event of a panic
		// as Go unwinds the stack).
		defer func() {
			// Use the builtin recover function to check if there has been a panic or
			// not.
			if err := recover(); err != nil {
				// If there was a panic, set a "Connection: close" header on the
				// response. This acts as a trigger to make Go's HTTP server
				// automatically close the current connection after a response has been
				// sent.
				w.Header().Set("Connection", "close")
				// The value returned by recover() has the type any, so we use
				// fmt.Errorf() to normalize it into an error and call our
				// serverErrorResponse() helper. In turn, this will log the error using
				// our custom Logger type at the ERROR level and send the client a 500
				// Internal Server Error response.
				app.serverErrorResponse(w, r, fmt.Errorf("%s", err))
			}
		}()

		next.ServeHTTP(w, r)
	})
}

func (app *application) rateLimit(next http.Handler) http.Handler {
	type client struct {
		limiter  *rate.Limiter
		lastSeen time.Time
	}

	// Declare a mutex and a map to hold the clients' IP addresses and rate limiters.
	var (
		mu      sync.Mutex
		clients = make(map[string]*client)
	)

	go func() {
		for {

			time.Sleep(time.Minute)

			// Lock the mutex to prevent any rate limiter checks from happening while
			// the cleanup is taking place.
			mu.Lock()

			// Loop through all clients. If they haven't been seen within the last three
			// minutes, delete the corresponding entry from the map.
			for ip, client := range clients {
				if time.Since(client.lastSeen) > 3*time.Minute {
					delete(clients, ip)
				}
			}
			// Importantly, unlock the mutex when the cleanup is complete.
			mu.Unlock()
		}
	}()

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		if app.config.limiter.enabled {
			// Extract the client's IP address from the request.
			ip, _, err := net.SplitHostPort(r.RemoteAddr)
			if err != nil {
				app.serverErrorResponse(w, r, err)
				return
			}

			// Lock the mutex to prevent this code from being executed concurrently.
			mu.Lock()

			// Check to see if the IP address already exists in the map.  If it doesn't, then
			// initialize a new rate limiter and add the IP address and limiter to the map.
			if _, found := clients[ip]; !found {
				// Allow 2 requests per second, with a maximum of 4 requests in a burst.
				clients[ip] = &client{limiter: rate.NewLimiter(rate.Limit(app.config.limiter.rps), app.config.limiter.burst)}

			}
			// Update the last seen time for the client.
			clients[ip].lastSeen = time.Now()

			if !clients[ip].limiter.Allow() {
				mu.Unlock()
				app.rateLimitExceededResponse(w, r)
				return
			}

			// middleware have also returned.
			mu.Unlock()
		}
		next.ServeHTTP(w, r)
	})
}

func (app *application) authenticate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		w.Header().Add("Vary", "Authorization")
		authorizationHeader := r.Header.Get("Authorization")
		if authorizationHeader == "" {
			r = app.contextSetUser(r, data.AnonymousUser)
			next.ServeHTTP(w, r)
			return
		}
		headerParts := strings.Split(authorizationHeader, " ")
		if len(headerParts) != 2 || headerParts[0] != "Bearer" {
			app.invalidAuthenticationTokenResponse(w, r)
			return
		}

		// Extract the actual authentication token from the header parts.
		token := headerParts[1]

		v := validator.New()

		if data.ValidateTokenPlaintext(v, token); !v.Valid() {
			app.invalidAuthenticationTokenResponse(w, r)
			return
		}

		user, err := app.models.Users.GetForToken(data.ScopeAuthentication, token)

		if err != nil {
			switch {
			case errors.Is(err, data.ErrRecordNotFound):
				app.invalidAuthenticationTokenResponse(w, r)
			default:
				app.serverErrorResponse(w, r, err)
			}
			return
		}

		r = app.contextSetUser(r, user)

		// Call the next handler in the chain.
		next.ServeHTTP(w, r)
	})
}

func (app *application) requireAuthenticatedUser(next http.HandlerFunc) http.HandlerFunc {

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user := app.contextGetUser(r)
		if user.IsAnonymous() {
			app.authenticationRequiredResponse(w, r)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// requireActivatedUser>requireAuthenticatedUser
func (app *application) requireActivatedUser(next http.HandlerFunc) http.HandlerFunc {

	fn := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user := app.contextGetUser(r)
		// Check that a user is activated.
		if !user.Activated {
			app.inactiveAccountResponse(w, r)
			return
		}
		next.ServeHTTP(w, r)
	})
	return app.requireAuthenticatedUser(fn)
}

// requirePermission>requireActivatedUser>requireAuthenticatedUser
func (app *application) requirePermission(code string, next http.HandlerFunc) http.HandlerFunc {

	fn := func(w http.ResponseWriter, r *http.Request) {
		// Retrieve the user from the request context.
		user := app.contextGetUser(r)
		// Get the slice of permissions for the user.
		permissions, err := app.models.Permissions.GetAllForUser(user.ID)
		if err != nil {
			app.serverErrorResponse(w, r, err)
			return
		}
		if !permissions.Include(code) {
			app.notPermittedResponse(w, r)
			return
		}
		next.ServeHTTP(w, r)
	}
	return app.requireActivatedUser(fn)
}

func (app *application) enableCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		next.ServeHTTP(w, r)
	})
}

func (app *application) metrics(next http.Handler) http.Handler {

	totalRequestsReceived := expvar.NewInt("total_requests_received")
	totalResponsesSent := expvar.NewInt("total_responses_sent")
	totalProcessingTimeMicroseconds := expvar.NewInt("total_processing_time_μs")
	totalResponsesSentByStatus := expvar.NewMap("total_responses_sent_by_status")

	var (
		opsProcessed = promauto.NewCounter(prometheus.CounterOpts{
			Namespace: "go_metrics",
			Subsystem: "prometheus",
			Name:      "processed_record_total",
			Help:      "process metrics count",
		})

		opsRequested = promauto.NewGauge(prometheus.GaugeOpts{
			Namespace: "go_metrics",
			Subsystem: "prometheus",
			Name:      "processed_record_count",
			Help:      "request record count",
		})
		promTotalRequestsReceived = promauto.NewGauge(prometheus.GaugeOpts{
			Namespace: "go_metrics",
			Subsystem: "prometheus",
			Name:      "total_requests_received",
			Help:      "total requests received",
		})

		promTotalResponsesSentByStatus = promauto.NewCounterVec(prometheus.CounterOpts{
			Namespace: "go_metrics",
			Subsystem: "prometheus",
			Name:      "http_requests_total",
			Help:      "Total number of API requests by HTTP code.",
		}, []string{"code", "method", "handler"})
	)

	// The following code will be run for every request...
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		totalRequestsReceived.Add(1)

		metrics := httpsnoop.CaptureMetrics(next, w, r)

		totalResponsesSent.Add(1)

		totalProcessingTimeMicroseconds.Add(metrics.Duration.Microseconds())

		totalResponsesSent.Add(1)

		totalResponsesSentByStatus.Add(strconv.Itoa(metrics.Code), 1)

		// prometheus metrics
		// Record the duration in microseconds
		//duration := float64(time.Since(start).Microseconds())
		//timer := prometheus.NewTimer(promHttpRequestDuration.WithLabelValues(strings.Split(r.RequestURI, "?")[0]))
		//defer timer.ObserveDuration()
		//fmt.Printf("hello from muiddleware on %s with %0.0f μs \n", strings.Split(r.RequestURI, "?")[0], float64(metrics.Duration.Microseconds()))
		//promHttpRequestDuration.WithLabelValues(strings.Split(r.RequestURI, "?")[0]).Observe(float64(metrics.Duration.Microseconds()))

		opsRequested.Add(1)
		opsProcessed.Add(1)

		promTotalRequestsReceived.Add(1)
		promTotalResponsesSentByStatus.WithLabelValues(strconv.Itoa(metrics.Code), r.Method, strings.Split(r.RequestURI, "?")[0]).Add(1)

	})

}
func (app *application) measureDuration(next http.Handler) http.Handler {
	var (
		requestDuration = prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: "go_metrics",
				Subsystem: "prometheus",
				Name:      "http_request_duration_seconds",
				Help:      "Histogram of request durations",
				Buckets:   []float64{.005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10},
			},
			[]string{"method", "handler"},
		)
	)
	prometheus.MustRegister(requestDuration)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		duration := time.Since(start)

		//fmt.Printf("hello from muiddleware on %s with %0.0f μs \n", r.URL.Path, float64(duration.Microseconds()))
		// Observe the duration with method and endpoint labels
		requestDuration.WithLabelValues(r.Method, r.URL.Path).Observe(float64(duration.Microseconds()))
	})
}
