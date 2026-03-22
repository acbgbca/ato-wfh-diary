package main

import (
	"log"
	"net/http"
)

func main() {
	// TODO: load config (port, DB path, forward auth header name)
	// TODO: initialise database and run migrations
	// TODO: set up router with API routes and static file serving

	log.Println("Starting ATO WFH Diary server on :8080")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatal(err)
	}
}
