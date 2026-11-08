// Command api runs the yari.run Connect API server.
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"

	"github.com/synapsbranch-ux/yari.run/internal/api"
	"github.com/synapsbranch-ux/yari.run/internal/authn"
	"github.com/synapsbranch-ux/yari.run/internal/config"
	"github.com/synapsbranch-ux/yari.run/internal/db"
	"github.com/synapsbranch-ux/yari.run/internal/gateway"
	"github.com/synapsbranch-ux/yari.run/internal/provider/catalog"
)

func main() {
	if err := run(); err != nil {
		slog.Error("api stopped", "error", err)
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

	// An insert-only River client: the API enqueues jobs in its own transactions, the worker runs them.
	riverClient, err := river.NewClient[pgx.Tx](riverpgxv5.New(pool), &river.Config{})
	if err != nil {
		return err
	}

	modelsPath := os.Getenv("YARI_LITELLM_CONFIG")
	if modelsPath == "" {
		modelsPath = "agent/litellm/config.yaml"
	}
	srv := api.New(api.Deps{
		Pool:    pool,
		Config:  cfg,
		Authn:   authn.New(pool, authn.Config{Env: cfg.Env, Issuer: cfg.ZitadelIssuer, ClientID: cfg.ZitadelClientID}),
		River:   riverClient,
		Catalog: catalog.MustLoad(),
		Models:  &gateway.ConfigModels{Path: modelsPath},
	})

	httpSrv := &http.Server{Addr: cfg.APIAddr, Handler: srv.Handler(), ReadHeaderTimeout: 10 * time.Second}
	errc := make(chan error, 1)
	go func() {
		slog.Info("api listening", "addr", cfg.APIAddr, "env", cfg.Env)
		errc <- httpSrv.ListenAndServe()
	}()

	select {
	case err := <-errc:
		if !errors.Is(err, http.ErrServerClosed) {
			return err
		}
	case <-ctx.Done():
		slog.Info("shutting down")
	}
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	return httpSrv.Shutdown(shutdownCtx)
}
