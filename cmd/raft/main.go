package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"gopkg.in/yaml.v3"

	"github.com/peyuaa/raft/internal/cluster"
)

type Config struct {
	NodesNumber int `yaml:"nodes_number"`
}

const (
	configFile = "config.yaml"
)

func main() {
	yamlFile, err := os.ReadFile(configFile)
	if err != nil {
		log.Fatalf("unable to read config file: %v", err)
	}

	var cfg Config
	err = yaml.Unmarshal(yamlFile, &cfg)
	if err != nil {
		log.Fatalf("unable to parse config file: %v", err)
	}

	r, err := cluster.New(cfg.NodesNumber)
	if err != nil {
		log.Fatalf("unable to create raft cluster: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{}, 1)

	go func() {
		defer func() { done <- struct{}{} }()

		err = r.Run(ctx)
		if err != nil {
			log.Fatalf("error from r.Run: %v", err)
		}
	}()

	h := cluster.NewHandler(r)
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
		if err != nil && !errors.Is(http.ErrServerClosed, err) {
			log.Printf("error in ListenAndServe: %v", err)
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
