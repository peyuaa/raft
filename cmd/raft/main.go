package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/peyuaa/raft/internal/raft"
)

const nodesCount = 5

func main() {
	r, err := raft.New(nodesCount)
	if err != nil {
		log.Fatalf("unable to create raft cluster: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{}, 1)

	go func() {
		defer func() { done <- struct{}{} }()
		_ = r.Run(ctx)
	}()

	h := raft.NewHandler(r)
	mux := http.NewServeMux()
	mux.HandleFunc("/nodes", h.Nodes)
	mux.HandleFunc("/journal", h.Journal)
	mux.HandleFunc("/request", h.Request)
	mux.HandleFunc("/kill", h.Kill)
	mux.HandleFunc("/recover", h.Recover)
	mux.HandleFunc("/dump", h.DumpMap)
	mux.HandleFunc("/get", h.Get)
	mux.HandleFunc("/connect", h.Connect)
	mux.HandleFunc("/disconnect", h.Disconnect)
	mux.HandleFunc("/topology", h.Topology)

	s := http.Server{
		Addr:    ":8080",
		Handler: mux,
	}

	ch := make(chan os.Signal, 1)
	signal.Notify(ch, os.Interrupt, syscall.SIGTERM, syscall.SIGINT)
	go func() {
		err = s.ListenAndServe()
		if err != nil {
			cancel()
		}
	}()
	<-ch
	log.Print("Shutting down...")

	err = s.Shutdown(ctx)
	if err != nil {
		log.Println("error shutting down:", err)
	}

	cancel()
	<-ctx.Done()
}
