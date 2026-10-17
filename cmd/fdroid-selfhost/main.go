// Command fdroid-selfhost serves a self-hosted F-Droid repository together
// with a small web UI and API for publishing APKs.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"fdroid-selfhost/internal/api"
	"fdroid-selfhost/internal/config"
	"fdroid-selfhost/internal/repo"
	"fdroid-selfhost/web"
)

// version is set at build time with -ldflags "-X main.version=...".
var version = "dev"

func main() {
	if err := run(os.Args[1:]); err != nil {
		slog.Error("fatal", "err", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	cfg, err := config.Load(os.Getenv)
	if err != nil {
		return err
	}
	if len(args) == 1 && args[0] == "healthcheck" {
		return healthcheck(cfg.ListenAddr)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	fingerprint, err := repo.Setup(ctx, cfg, repo.ExecRunner)
	if err != nil {
		return fmt.Errorf("preparing repo: %w", err)
	}
	store, err := repo.NewStore(cfg.DataDir)
	if err != nil {
		return err
	}
	updater := repo.NewUpdater(store, cfg.DataDir, cfg.FdroidBin, cfg.PollInterval, repo.ExecRunner)
	go updater.Run(ctx)

	if cfg.UploadToken == "" {
		slog.Warn("UPLOAD_TOKEN is not set: uploads and deletes are disabled")
	}
	srv := &http.Server{
		Addr:              cfg.ListenAddr,
		Handler:           api.New(cfg, store, updater, fingerprint, version, web.Files).Handler(),
		ReadHeaderTimeout: 10 * time.Second,
	}
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
	}()

	slog.Info("serving", "version", version, "addr", cfg.ListenAddr, "repo", cfg.RepoURL(), "fingerprint", fingerprint)
	if err := srv.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

// healthcheck is used by the container HEALTHCHECK so the image needs no curl.
func healthcheck(listenAddr string) error {
	host, port, err := net.SplitHostPort(listenAddr)
	if err != nil {
		return err
	}
	if host == "" || host == "0.0.0.0" {
		host = "127.0.0.1"
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://"+net.JoinHostPort(host, port)+"/healthz", nil)
	if err != nil {
		return err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("healthz returned %s", resp.Status)
	}
	return nil
}
