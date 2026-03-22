// Package main is the entry point for dockgraph, a real-time Docker
// infrastructure visualizer. It connects Docker and Compose collectors
// to a state manager and serves a WebSocket-powered web UI.
package main

import (
	"context"
	"io/fs"
	"log"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"github.com/dockgraph/dockgraph/api"
	"github.com/dockgraph/dockgraph/collector"
	"github.com/dockgraph/dockgraph/frontend"
	"github.com/dockgraph/dockgraph/state"
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

	go pipeUpdates(ctx, mgr, dc, cc)

	hub := api.NewHub()
	go pipeToHub(ctx, mgr.Subscribe(), hub)

	staticFS, err := fs.Sub(frontend.Assets, "dist")
	if err != nil {
		log.Fatalf("failed to load embedded frontend: %v", err)
	}

	handler := api.NewServer(hub, staticFS, dockerCli)
	addr := ":" + cfg.Port
	server := &http.Server{Addr: addr, Handler: handler}

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		server.Shutdown(shutdownCtx)
	}()

	log.Printf("dockgraph listening on %s", addr)
	if err := server.ListenAndServe(); err != http.ErrServerClosed {
		log.Fatal(err)
	}
}

// pipeUpdates routes collector snapshots into the state manager until the context is cancelled.
func pipeUpdates(ctx context.Context, mgr *state.Manager, dc *collector.DockerCollector, cc *collector.ComposeCollector) {
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
}

// pipeToHub forwards merged state messages to the WebSocket hub for broadcast.
func pipeToHub(ctx context.Context, sub <-chan collector.StateMessage, hub *api.Hub) {
	for {
		select {
		case msg := <-sub:
			hub.Broadcast(msg)
		case <-ctx.Done():
			return
		}
	}
}
