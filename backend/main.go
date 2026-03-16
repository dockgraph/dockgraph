package main

import (
	"context"
	"io/fs"
	"log"
	"net/http"
	"os/signal"
	"syscall"

	"github.com/dockgraph/docker-flow/api"
	"github.com/dockgraph/docker-flow/collector"
	"github.com/dockgraph/docker-flow/frontend"
	"github.com/dockgraph/docker-flow/state"
	"github.com/docker/docker/client"
)

func main() {
	cfg := LoadConfig()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	dockerCli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		log.Fatalf("failed to create Docker client: %v", err)
	}
	defer dockerCli.Close()

	mgr := state.NewManager()

	dc := collector.NewDockerCollector(dockerCli, cfg.PollInterval)
	if err := dc.Start(ctx); err != nil {
		log.Fatalf("docker collector start failed: %v", err)
	}
	defer dc.Stop()

	cc := collector.NewComposeCollector(cfg.ComposeDir)
	if err := cc.Start(ctx); err != nil {
		log.Printf("compose collector start failed (continuing without): %v", err)
	} else {
		defer cc.Stop()
	}

	go func() {
		for {
			select {
			case update := <-dc.Updates():
				mgr.HandleUpdate("docker", update)
			case update := <-cc.Updates():
				mgr.HandleUpdate("compose", update)
			case <-ctx.Done():
				return
			}
		}
	}()

	hub := api.NewHub()
	sub := mgr.Subscribe()
	go func() {
		for {
			select {
			case msg := <-sub:
				hub.Broadcast(msg)
			case <-ctx.Done():
				return
			}
		}
	}()

	staticFS, err := fs.Sub(frontend.Assets, "dist")
	if err != nil {
		log.Fatalf("failed to load embedded frontend: %v", err)
	}

	handler := api.NewServer(hub, staticFS, dockerCli)
	addr := ":" + cfg.Port
	server := &http.Server{Addr: addr, Handler: handler}

	go func() {
		<-ctx.Done()
		server.Close()
	}()

	log.Printf("docker-flow listening on %s", addr)
	if err := server.ListenAndServe(); err != http.ErrServerClosed {
		log.Fatal(err)
	}
}
