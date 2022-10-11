package main

import (
	"flag"
	"os"
	"os/signal"
	"syscall"

	"github.com/form3tech-oss/github-team-approver/internal/api"
)

func main() {
	// buffering shutdown channel as recommended
	// https://golang.org/pkg/os/signal/#Notify
	// golangci-lint SA1017
	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, syscall.SIGINT, syscall.SIGTERM)
	ready := make(chan struct{})

	bindAddress := flag.String("bind-address", ":8080", "The 'host:port' pair to bind to.")
	flag.Parse()

	api.Start(*bindAddress, shutdown, ready)
}
