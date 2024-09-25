package main

import (
	"flag"
	"log"
	"net/http"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func main() {

	// Make the server address configurable at runtime via a command-line flag.
	addr := flag.String("addr", ":9191", "Server address")

	flag.Parse()

	log.Printf("starting server on %s", *addr)

	// Start a HTTP server listening on the given address, which responds to all
	// requests with the webpage HTML above.
	err := http.ListenAndServe(*addr, promhttp.Handler())

	log.Fatal(err)
}
