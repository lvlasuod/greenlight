package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/golang-migrate/migrate/v4"                     // New import
	_ "github.com/golang-migrate/migrate/v4/database/postgres" // New import
	_ "github.com/golang-migrate/migrate/v4/source/file"       // New import
	"github.com/jackc/pgx/v5/pgxpool"
	"greenlight.mpdev.com/internal/data"
)

// Declare a string containing the application version number. Later in the book we'll
// generate this automatically at build time, but for now we'll just store the version
// number as a hard-coded global constant.
const version = "1.0.0"

// Define a config struct to hold all the configuration settings for our application.
// For now, the only configuration settings will be the network port that we want the
// server to listen on, and the name of the current operating environment for the
// application (development, staging, production, etc.). We will read in these
// configuration settings from command-line flags when the application starts.
type config struct {
	port int
	env  string
	db   struct {
		dsn          string
		maxOpenConns int

		maxIdleTime string
	}
	limiter struct {
		rps     float64
		burst   int
		enabled bool
	}
}

// Define an application struct to hold the dependencies for our HTTP handlers, helpers,
// and middleware. At the moment this only contains a copy of the config struct and a
// logger, but it will grow to include a lot more as our build progresses.
type application struct {
	config config
	logger *log.Logger
	models data.Models
}

func main() {
	// Declare an instance of the config struct.
	var cfg config

	// Read the value of the port and env command-line flags into the config struct. We
	// default to using the port number 4000 and the environment "development" if no
	// corresponding flags are provided.
	flag.IntVar(&cfg.port, "port", 4000, "API server port")
	flag.StringVar(&cfg.env, "env", "development", "Environment (development|staging|production)")

	// Read the DSN value from the db-dsn command-line flag into the config struct. We
	// default to using our development DSN if no flag is provided.
	flag.StringVar(&cfg.db.dsn, "db-dsn", "postgres://greenlight:pa55word@localhost/greenlight?sslmode=disable", "PostgreSQL DSN")

	flag.IntVar(&cfg.db.maxOpenConns, "db-max-open-conns", 25, "PostgreSQL max open connections")

	flag.StringVar(&cfg.db.maxIdleTime, "db-max-idle-time", "15m", "PostgreSQL max connection idle time")

	flag.Float64Var(&cfg.limiter.rps, "limiter-rps", 2, "Rate limiter maximum requests per second")
	flag.IntVar(&cfg.limiter.burst, "limiter-burst", 4, "Rate limiter maximum burst")
	flag.BoolVar(&cfg.limiter.enabled, "limiter-enabled", true, "Enable rate limiter")

	flag.Parse()

	// Initialize a new structured logger which writes log entries to the standard out
	// stream.
	logger := log.New(os.Stdout, "", log.Ldate|log.Ltime)

	// application immediately.
	db, err := openDB(cfg)
	if err != nil {
		logger.Fatal(err)
	}

	// Defer a call to db.Close() so that the connection pool is closed before the
	// main() function exits.
	//defer db.Close()
	// Also log a message to say that the connection pool has been successfully
	// established.
	logger.Printf("Database connection pool established")

	migrator, err := migrate.New("file://migrations", "postgres://greenlight:pa55word@localhost/greenlight?sslmode=disable")

	if err != nil {
		logger.Fatal(err, nil)
	}
	err = migrator.Up()
	if err != nil && err != migrate.ErrNoChange {
		logger.Fatal(err, nil)
	}

	logger.Printf("Database migrations applied [up]")

	// Declare an instance of the application struct, containing the config struct and
	// the logger.
	app := &application{
		config: cfg,
		logger: logger,
		models: data.NewModels(db),
	}

	// Declare a HTTP server which listens on the port provided in the config struct,
	// uses the servemux we created above as the handler, has some sensible timeout
	// settings and writes any log messages to the structured logger at Error level.
	srv := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.port),
		Handler:      app.routes(),
		IdleTimeout:  time.Minute,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	logger.Printf("Starting server on localhost...")
	// Start the HTTP server.
	logger.Printf("Running %s server on %s", cfg.env, srv.Addr)

	err = srv.ListenAndServe()

	logger.Fatal(err)

}
func openDB(cfg config) (*pgxpool.Pool, error) {
	/*
		db, err := sql.Open("postgres", cfg.db.dsn)
		if err != nil {
			return nil, err
		}

		// Create a context with a 5-second timeout deadline.
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
	*/
	// Create a context with a 5-second timeout deadline.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn, err := pgxpool.New(ctx, cfg.db.dsn)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Unable to connect to database: %v\n", err)
		os.Exit(1)
	}

	// Set the maximum number of open (in-use + idle) connections in the pool
	conn.Config().MaxConns = int32(cfg.db.maxOpenConns)

	duration, err := time.ParseDuration(cfg.db.maxIdleTime)
	if err != nil {
		return nil, err
	}
	// Set the maximum idle timeout.
	conn.Config().MaxConnIdleTime = duration

	//defer conn.Close()
	// Use PingContext() to establish a new connection to the database, passing in the
	// context we created above as a parameter. If the connection couldn't be
	// established successfully within the 5 second deadline, then this will return an error.
	err = conn.Ping(ctx)
	if err != nil {
		return nil, err
	}

	// Return the sql.DB connection pool.
	return conn, nil
}
