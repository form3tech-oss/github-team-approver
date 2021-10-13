package main

import (
	"flag"
	gta "github.com/form3tech-oss/github-team-approver/internal/api"
	"log"
	"net/http"
)

func main() {
	bindAddress := flag.String("bind-address", ":8080", "The 'host:port' pair to bind to.")
	flag.Parse()

	m := http.NewServeMux()
	m.HandleFunc("/events", gta.Handle)
	m.HandleFunc("/function/github-team-approver", gta.Handle) // Keep backwards-compatibility.
	if err := http.ListenAndServe(*bindAddress, m); err != nil && err != http.ErrServerClosed {
		log.Fatalf("Failed to serve HTTP: %v", err)
	}
}
