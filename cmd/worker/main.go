// Command worker runs the River worker: plan applies and agent sessions.
package main

import (
	"context"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"

	"github.com/synapsbranch-ux/yari.run/agent/fixtures"
	"github.com/synapsbranch-ux/yari.run/agent/prompts"
	"github.com/synapsbranch-ux/yari.run/internal/agent"
	agentfake "github.com/synapsbranch-ux/yari.run/internal/agent/fake"
	"github.com/synapsbranch-ux/yari.run/internal/config"
	"github.com/synapsbranch-ux/yari.run/internal/db"
	"github.com/synapsbranch-ux/yari.run/internal/gateway"
	"github.com/synapsbranch-ux/yari.run/internal/jobs"
	"github.com/synapsbranch-ux/yari.run/internal/provider/all"
	"github.com/synapsbranch-ux/yari.run/internal/provider/catalog"
	sandboxdocker "github.com/synapsbranch-ux/yari.run/internal/sandbox/docker"
)

func main() {
	if err := run(); err != nil {
		slog.Error("worker stopped", "error", err)
		os.Exit(1)
	}
}

func selectRuntime(cfg config.Config) (agent.Runtime, error) {
	switch cfg.AgentRuntime {
	case "fake":
		return &agentfake.Runtime{MCPURL: cfg.MCPWorkerURL, Delay: 400 * time.Millisecond}, nil
	case "openhands":
		return newOpenHandsRuntime(cfg)
	}
	return nil, fmt.Errorf("YARI_AGENT_RUNTIME must be fake or openhands, got %q", cfg.AgentRuntime)
}

func run() error {
	cfg := config.FromEnv()
	if err := cfg.RequireDatabase(); err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool, err := db.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()

	// The scaffold applies plans with in-memory fakes: no provider credentials, no network.
	registry, err := all.Fake(catalog.MustLoad())
	if err != nil {
		return err
	}

	deps := jobs.Deps{Pool: pool, Registry: registry, Leaser: jobs.FakeLeaser{}, Config: cfg}
	var live *jobs.LiveSessions
	if cfg.LiteLLMMasterKey == "" {
		slog.Warn("LITELLM_MASTER_KEY is not set: agent sessions are disabled, plan applies still run")
	} else {
		runtime, err := selectRuntime(cfg)
		if err != nil {
			return err
		}
		sandboxes, err := sandboxdocker.New(ctx, sandboxdocker.Config{Runtime: cfg.SandboxRuntime, RequireRuntime: !cfg.IsDev()})
		if err != nil {
			return err
		}
		sample, err := fs.Sub(fixtures.SampleRepo, "sample-repo")
		if err != nil {
			return err
		}
		live = jobs.NewLiveSessions()
		deps.Sandboxes, deps.Runtime, deps.Live, deps.Prompt, deps.SampleRepo = sandboxes, runtime, live, prompts.System, sample
		deps.Gateway = &gateway.Client{BaseURL: cfg.LiteLLMURL, MasterKey: cfg.LiteLLMMasterKey}
	}

	workers, err := jobs.NewWorkers(deps)
	if err != nil {
		return err
	}
	riverCfg := &river.Config{Queues: map[string]river.QueueConfig{river.QueueDefault: {MaxWorkers: 20}}, Workers: workers}
	if deps.Sandboxes != nil {
		riverCfg.PeriodicJobs = []*river.PeriodicJob{river.NewPeriodicJob(
			river.PeriodicInterval(time.Minute),
			func() (river.JobArgs, *river.InsertOpts) { return jobs.ReapArgs{}, nil },
			&river.PeriodicJobOpts{RunOnStart: true},
		)}
	}
	client, err := river.NewClient[pgx.Tx](riverpgxv5.New(pool), riverCfg)
	if err != nil {
		return err
	}
	if err := client.Start(ctx); err != nil {
		return err
	}
	if live != nil {
		go func() {
			if err := jobs.ListenForCommands(ctx, pool, live); err != nil {
				slog.Error("command listener stopped", "error", err)
			}
		}()
	}
	slog.Info("worker started", "agent_runtime", cfg.AgentRuntime, "agent_sessions", deps.Sandboxes != nil)

	<-ctx.Done()
	slog.Info("worker shutting down")
	stopCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	return client.Stop(stopCtx)
}
