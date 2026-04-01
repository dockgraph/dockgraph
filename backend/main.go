// Package main is the entry point for dockgraph, a real-time Docker
// infrastructure visualizer. It connects Docker and Compose collectors
// to a state manager and serves a WebSocket-powered web UI.
package main

import (
	"context"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/docker/docker/client"
	"github.com/dockgraph/dockgraph/api"
	"github.com/dockgraph/dockgraph/auth"
	"github.com/dockgraph/dockgraph/collector"
	"github.com/dockgraph/dockgraph/frontend"
	"github.com/dockgraph/dockgraph/state"
)

// Version is set at build time via -ldflags.
var Version = "dev"

// dockerHealth adapts a Docker client to the api.HealthChecker interface.
type dockerHealth struct {
	cli *client.Client
}

func (d *dockerHealth) HealthCheck(ctx context.Context) error {
	_, err := d.cli.Ping(ctx)
	return err
}

func main() {
	if len(os.Args) > 1 && os.Args[1] == "--version" {
		fmt.Println("dockgraph", Version)
		os.Exit(0)
	}

	if len(os.Args) > 1 && (os.Args[1] == "--help" || os.Args[1] == "-h") {
		fmt.Println("Usage: dockgraph [--version | --help | --healthcheck]")
		fmt.Println()
		fmt.Println("Real-time Docker infrastructure topology visualizer.")
		fmt.Println()
		fmt.Println("Environment variables:")
		fmt.Println("  DG_BIND_ADDR       Listen address (default: 0.0.0.0)")
		fmt.Println("  DG_PORT            HTTP port (default: 7800)")
		fmt.Println("  DG_POLL_INTERVAL   Docker API poll interval (default: 30s)")
		fmt.Println("  DG_COMPOSE_PATH    Comma-separated compose file paths (default: auto-detect)")
		fmt.Println("  DG_PASSWORD        Password for UI/WebSocket access (default: disabled)")
		fmt.Println("  DG_STATS_INTERVAL  Stats poll interval (default: 3s)")
		fmt.Println("  DG_STATS_WORKERS   Max concurrent stats calls (default: 50)")
		os.Exit(0)
	}

	cfg := LoadConfig()

	if len(os.Args) > 1 && os.Args[1] == "--healthcheck" {
		httpClient := &http.Client{Timeout: 5 * time.Second}
		resp, err := httpClient.Get("http://localhost:" + cfg.Port + "/healthz")
		if err != nil {
			os.Exit(1)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			os.Exit(1)
		}
		os.Exit(0)
	}

	if cfg.PasswordHash == "" {
		log.Println("WARN  Authentication is disabled. Set DG_PASSWORD to enable protection.")
		log.Println("WARN  Tip: generate a strong password with: openssl rand -base64 24")
	} else {
		log.Println("INFO  Authentication enabled")
	}

	dockerCli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		log.Fatalf("failed to create Docker client: %v", err)
	}
	defer dockerCli.Close()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	mgr := state.NewManager()

	dc := collector.NewDockerCollector(dockerCli, cfg.PollInterval)
	if err := dc.Start(ctx); err != nil {
		log.Fatalf("docker collector start failed: %v", err)
	}
	defer dc.Stop()

	composePaths := cfg.ComposePaths
	if len(composePaths) > 0 {
		log.Printf("using explicit compose paths: %v", composePaths)
	} else {
		detected, detectErr := collector.DetectComposePaths(ctx, dockerCli)
		if detectErr != nil {
			log.Printf("failed to detect compose paths from mounts: %v", detectErr)
		} else if len(detected) > 0 {
			log.Printf("auto-detected compose paths from mounts: %v", detected)
			composePaths = detected
		}
	}

	cc := collector.NewComposeCollector(composePaths)
	if err := cc.Start(ctx); err != nil {
		log.Printf("compose collector start failed (continuing without): %v", err)
	} else {
		defer cc.Stop()
	}

	sc := collector.NewStatsCollector(dockerCli, cfg.StatsInterval, cfg.StatsWorkers)
	sc.Start(ctx)
	defer sc.Stop()

	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("pipeUpdates panic: %v", r)
			}
		}()
		pipeUpdates(ctx, mgr, dc, cc)
	}()

	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("pipeStats panic: %v", r)
			}
		}()
		for {
			select {
			case snap := <-sc.Updates():
				mgr.HandleStats(snap)
			case <-ctx.Done():
				return
			}
		}
	}()

	var authService *auth.Service
	if cfg.PasswordHash != "" {
		signingKey, keyErr := auth.GenerateSigningKey()
		if keyErr != nil {
			log.Fatalf("failed to generate signing key: %v", keyErr)
		}
		authService = &auth.Service{
			PasswordHash: cfg.PasswordHash,
			SigningKey:   signingKey,
			Sessions:     auth.NewSessionManager(),
			Limiter:      auth.NewRateLimiter(5, time.Minute),
			LoginPage:    frontend.LoginHTML,
		}
		go func() {
			ticker := time.NewTicker(5 * time.Minute)
			defer ticker.Stop()
			for {
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
					authService.Limiter.Cleanup()
				}
			}
		}()
	}

	hub := api.NewHub()
	if authService != nil {
		hub.CheckExpired = func(iat, exp time.Time) bool {
			return time.Now().After(exp) || !authService.Sessions.IsValid(iat)
		}
		hub.StartSessionSweep(ctx, 30*time.Second)
	}
	sub, unsub := mgr.Subscribe()
	go func() {
		defer unsub()
		defer func() {
			if r := recover(); r != nil {
				log.Printf("pipeToHub panic: %v", r)
			}
		}()
		pipeToHub(ctx, sub, hub)
	}()

	statsSub, statsUnsub := mgr.Subscribe()
	go func() {
		defer statsUnsub()
		defer func() {
			if r := recover(); r != nil {
				log.Printf("pipeStatsToHub panic: %v", r)
			}
		}()
		for {
			select {
			case msg, ok := <-statsSub:
				if !ok {
					return
				}
				if msg.Type == "stats" {
					hub.BroadcastStats(msg)
				}
			case <-ctx.Done():
				return
			}
		}
	}()

	staticFS, err := fs.Sub(frontend.Assets, "dist")
	if err != nil {
		log.Fatalf("failed to load embedded frontend: %v", err)
	}

	handler := api.NewServer(hub, staticFS, &dockerHealth{cli: dockerCli}, authService, dockerCli)
	addr := cfg.BindAddr + ":" + cfg.Port
	server := &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadTimeout:       15 * time.Second,
		ReadHeaderTimeout: 10 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	go func() {
		<-ctx.Done()
		hub.Shutdown()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdownCtx)
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
			mgr.HandleUpdate("docker", true, update)
		case update := <-cc.Updates():
			mgr.HandleUpdate("compose", false, update)
		case <-ctx.Done():
			return
		}
	}
}

// pipeToHub forwards merged state messages to the WebSocket hub for broadcast.
func pipeToHub(ctx context.Context, sub <-chan collector.StateMessage, hub *api.Hub) {
	for {
		select {
		case msg, ok := <-sub:
			if !ok {
				return
			}
			hub.Broadcast(msg)
		case <-ctx.Done():
			return
		}
	}
}
