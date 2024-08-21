package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func (app *application) serve() error {
	srv := &http.Server{
		Addr:         fmt.Sprintf(":%d", app.config.port),
		Handler:      app.routes(),
		IdleTimeout:  time.Minute,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
	}

	// Create a shutdownError channel. We will use this to receive any errors returned
	shutdownError := make(chan error)

	// Start a background goroutine.
	go func() {

		// Create a quit channel which carries os.Signal values.
		// Use signal.Notify() to listen for incoming SIGINT and SIGTERM
		// Read the signal from the quit channel. This code will block until a signal is received.
		quit := make(chan os.Signal, 1)
		signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
		s := <-quit

		// Log a message to say that the signal has been caught.
		app.logger.Printf("shutting down server due to recieved signal: %s", s.String())

		// Create a context with a 5-second timeout.
		ctx, cancel := context.WithTimeout(context.Background(),
			5*time.Second)
		defer cancel()

		shutdownError <- srv.Shutdown(ctx)

		// Log a message to say that the signal has been caught.
		app.logger.Printf("Completing background tasks...")
		app.wg.Wait()
		shutdownError <- nil

	}()

	err := srv.ListenAndServe()
	if !errors.Is(err, http.ErrServerClosed) {
		return err
	}

	err = <-shutdownError
	if err != nil {
		return err
	}
	app.logger.Printf("stopped server on addr: %s", srv.Addr)
	// Start the server as normal, returning any error.
	return nil
}
