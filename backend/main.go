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
	"strings"
	"syscall"
	"time"

	"github.com/docker/docker/api/types/events"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/swarm"
	"github.com/docker/docker/client"
	"github.com/dockgraph/dockgraph/agent"
	"github.com/dockgraph/dockgraph/api"
	"github.com/dockgraph/dockgraph/auth"
	"github.com/dockgraph/dockgraph/collector"
	"github.com/dockgraph/dockgraph/frontend"
	"github.com/dockgraph/dockgraph/state"
)

// Version is set at build time via -ldflags.
var Version = "dev"

// logRecover logs panics from background goroutines without crashing the process.
func logRecover(name string) {
	if r := recover(); r != nil {
		log.Printf("%s panic: %v", name, r)
	}
}

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
		fmt.Println("  DG_COMPOSE_PATH    Comma-separated compose/stack file paths, optionally name=/path (default: auto-detect)")
		fmt.Println("  DG_PASSWORD        Password for UI/WebSocket access (or DG_PASSWORD_FILE; default: disabled)")
		fmt.Println("  DG_STATS_INTERVAL  Stats poll interval (default: 3s)")
		fmt.Println("  DG_STATS_WORKERS   Max concurrent stats calls (default: 50)")
		fmt.Println("  DG_MODE            auto | standalone | swarm | agent (default: auto)")
		fmt.Println("  DG_SWARM_POLL_INTERVAL  Swarm task poll interval (default: 5s)")
		fmt.Println("  DG_AGENT_PORT      Per-node agent HTTP port (default: 7801)")
		fmt.Println("  DG_AGENT_ADDR      DNS name resolving to all agents (default: tasks.agent)")
		fmt.Println("  DG_AGENT_TOKEN     Shared agent secret (or DG_AGENT_TOKEN_FILE; required in agent mode)")
		os.Exit(0)
	}

	// Before LoadConfig: the probe runs every healthcheck interval and must
	// not read secrets or run the Argon2id password hash each time.
	if len(os.Args) > 1 && os.Args[1] == "--healthcheck" {
		httpClient := &http.Client{Timeout: 5 * time.Second}
		resp, err := httpClient.Get("http://localhost:" + healthcheckPort() + "/healthz")
		if err != nil {
			os.Exit(1)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			os.Exit(1)
		}
		os.Exit(0)
	}

	cfg := LoadConfig()

	dockerCli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		log.Fatalf("failed to create Docker client: %v", err)
	}
	defer dockerCli.Close()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	mode, swarmInfo, hostname, err := detectMode(ctx, cfg.Mode, dockerCli.Info, infoRetry)
	if err != nil {
		log.Fatalf("mode detection: %v", err)
	}
	if mode == collector.ModeAgent {
		runAgent(ctx, cfg, dockerCli)
		return
	}
	swarmMode := mode == collector.ModeSwarm
	log.Printf("INFO  Running in %s mode", mode)

	if cfg.PasswordHash == "" {
		log.Println("WARN  Authentication is disabled. Set DG_PASSWORD to enable protection.")
		log.Println("WARN  Tip: generate a strong password with: openssl rand -base64 24")
	} else {
		log.Println("INFO  Authentication enabled")
	}

	mgr := state.NewManager()

	dc := collector.NewDockerCollector(dockerCli, cfg.PollInterval)
	if swarmMode {
		dc.EnableSwarm(cfg.SwarmPollInterval)
	}
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
	if swarmMode {
		cc.EnableSwarm()
	}
	if err := cc.Start(ctx); err != nil {
		log.Printf("compose collector start failed (continuing without): %v", err)
	} else {
		defer cc.Stop()
	}

	sc := collector.NewStatsCollector(dockerCli, cfg.StatsInterval, cfg.StatsWorkers)
	var serverOpts []api.ServerOption
	if swarmMode {
		sc.SetLocalNode(swarmInfo.NodeID, hostname)
		if cfg.AgentToken == "" {
			log.Println("INFO  DG_AGENT_TOKEN not set: agents disabled, stats and container logs limited to this node")
		} else {
			pool := collector.NewAgentPool(collector.AgentPoolConfig{
				Addr:        cfg.AgentAddr,
				Port:        cfg.AgentPort,
				Token:       cfg.AgentToken,
				LocalNodeID: swarmInfo.NodeID,
				Tasks:       dockerCli,
			})
			pool.Start(ctx)
			defer pool.Stop()
			sc.SetRemote(pool)
			serverOpts = append(serverOpts, api.WithAgentProxy(&api.AgentProxy{Locator: pool, Token: cfg.AgentToken}))
		}
	}
	sc.Start(ctx)
	defer sc.Stop()

	statsHistory := collector.NewStatsHistory(24 * time.Hour)
	eventHistory := collector.NewEventHistory(500)

	go func() {
		defer logRecover("pipeUpdates")
		pipeUpdates(ctx, mgr, dc, cc)
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
		defer logRecover("pipeToHub")
		pipeToHub(ctx, sub, hub)
	}()

	go func() {
		defer logRecover("pipeStats")
		pipeStatsWithHistory(ctx, sc, hub, statsHistory)
	}()

	go func() {
		defer logRecover("pipeEvents")
		pipeEvents(ctx, dockerCli, eventHistory, swarmMode)
	}()

	staticFS, err := fs.Sub(frontend.Assets, "dist")
	if err != nil {
		log.Fatalf("failed to load embedded frontend: %v", err)
	}

	systemCache := api.NewCachedSystemData(ctx, dockerCli, 5*time.Minute, 60*time.Second)
	handler := api.NewServer(hub, staticFS, &dockerHealth{cli: dockerCli}, authService, dockerCli, systemCache, statsHistory, eventHistory, mode, serverOpts...)
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

// runAgent serves the per-node agent API (DG_MODE=agent) until ctx is
// cancelled. It exits when no agent token is configured.
func runAgent(ctx context.Context, cfg Config, dockerCli *client.Client) {
	srv, err := agent.New(dockerCli, agent.Config{
		Token:         cfg.AgentToken,
		StatsInterval: cfg.StatsInterval,
		StatsWorkers:  cfg.StatsWorkers,
		Health:        &dockerHealth{cli: dockerCli},
	})
	if err != nil {
		log.Fatalf("agent: %v", err)
	}
	log.Println("INFO  Running in agent mode")

	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	done := make(chan struct{})
	go func() {
		defer close(done)
		defer logRecover("agentStats")
		srv.Run(runCtx)
	}()

	addr := cfg.BindAddr + ":" + cfg.AgentPort
	server := &http.Server{
		Addr:              addr,
		Handler:           srv.Handler(),
		ReadTimeout:       15 * time.Second,
		ReadHeaderTimeout: 10 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       120 * time.Second,
	}
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdownCtx)
	}()

	log.Printf("dockgraph agent listening on %s", addr)
	if err := server.ListenAndServe(); err != http.ErrServerClosed {
		log.Printf("agent server: %v", err)
	}
	cancel()
	<-done
}

// pipeStatsWithHistory forwards stats snapshots to the WebSocket hub and records
// them in the history ring buffer for the dashboard charts.
func pipeStatsWithHistory(ctx context.Context, sc *collector.StatsCollector, hub *api.Hub, history *collector.StatsHistory) {
	downsampleTicker := time.NewTicker(time.Minute)
	defer downsampleTicker.Stop()

	for {
		select {
		case snap := <-sc.Updates():
			hub.BroadcastStats(snap)
			history.Record(time.Now(), snap)
		case <-downsampleTicker.C:
			history.Evict()
			history.Downsample(30 * time.Second)
		case <-ctx.Done():
			return
		}
	}
}

// pipeEvents subscribes to Docker events and records them in the event history
// buffer. Swarm mode also records service and node events.
func pipeEvents(ctx context.Context, cli *client.Client, history *collector.EventHistory, swarmMode bool) {
	filter := filters.NewArgs()
	filter.Add("type", string(events.ContainerEventType))
	filter.Add("type", string(events.NetworkEventType))
	filter.Add("type", string(events.VolumeEventType))
	if swarmMode {
		filter.Add("type", string(events.ServiceEventType))
		filter.Add("type", string(events.NodeEventType))
	}

	var selfServices *collector.SelfServices
	if swarmMode {
		selfServices = collector.NewSelfServices(func(ctx context.Context, id string) (swarm.Service, error) {
			svc, _, err := cli.ServiceInspectWithRaw(ctx, id, swarm.ServiceInspectOptions{})
			return svc, err
		})
	}

	msgCh, errCh := cli.Events(ctx, events.ListOptions{Filters: filter})
	for {
		select {
		case msg := <-msgCh:
			action := string(msg.Action)
			// Skip exec and health_status events — healthcheck noise.
			if strings.HasPrefix(action, "exec_") || strings.HasPrefix(action, "health_status") {
				continue
			}
			// Skip events from dockgraph's own container.
			if msg.Actor.Attributes[collector.SelfExcludeLabel] == "true" {
				continue
			}
			// Skip events from dockgraph's own swarm service and its tasks.
			if selfServices != nil && selfServices.IsSelfEvent(ctx, msg) {
				continue
			}
			name := msg.Actor.Attributes["name"]
			if name == "" {
				name = msg.Actor.ID
			}
			ts := time.Unix(0, msg.TimeNano)
			if msg.TimeNano == 0 {
				ts = time.Unix(msg.Time, 0)
			}
			history.Add(collector.DockerEvent{
				Timestamp:  ts,
				Action:     action,
				Type:       string(msg.Type),
				Name:       name,
				Attributes: msg.Actor.Attributes,
			})
		case err := <-errCh:
			if err == nil || ctx.Err() != nil {
				return
			}
			log.Printf("event history: stream error: %v, reconnecting...", err)
			time.Sleep(2 * time.Second)
			msgCh, errCh = cli.Events(ctx, events.ListOptions{Filters: filter})
		case <-ctx.Done():
			return
		}
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
