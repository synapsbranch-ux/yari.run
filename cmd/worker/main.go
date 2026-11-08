// Command worker runs the River worker: plan applies and agent sessions.
package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"

	"github.com/synapsbranch-ux/yari.run/internal/config"
	"github.com/synapsbranch-ux/yari.run/internal/db"
	"github.com/synapsbranch-ux/yari.run/internal/jobs"
	"github.com/synapsbranch-ux/yari.run/internal/provider/all"
	"github.com/synapsbranch-ux/yari.run/internal/provider/catalog"
)

func main() {
	if err := run(); err != nil {
		slog.Error("worker stopped", "error", err)
		os.Exit(1)
	}
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
	workers, err := jobs.NewWorkers(jobs.Deps{Pool: pool, Registry: registry, Leaser: jobs.FakeLeaser{}})
	if err != nil {
		return err
	}
	client, err := river.NewClient[pgx.Tx](riverpgxv5.New(pool), &river.Config{
		Queues:  map[string]river.QueueConfig{river.QueueDefault: {MaxWorkers: 20}},
		Workers: workers,
	})
	if err != nil {
		return err
	}
	if err := client.Start(ctx); err != nil {
		return err
	}
	slog.Info("worker started", "agent_runtime", cfg.AgentRuntime)

	<-ctx.Done()
	slog.Info("worker shutting down")
	stopCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	return client.Stop(stopCtx)
}
