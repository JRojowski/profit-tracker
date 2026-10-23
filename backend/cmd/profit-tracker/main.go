// Command profit-tracker runs the tool: the HTTP API and the embedded frontend on one port.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jrojowski/profit-tracker/backend/internal/httpapi"
	"github.com/jrojowski/profit-tracker/backend/internal/price"
	"github.com/jrojowski/profit-tracker/backend/internal/price/yahoo"
	"github.com/jrojowski/profit-tracker/backend/internal/store"
	"github.com/jrojowski/profit-tracker/backend/internal/web"
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	if err := start(logger); err != nil {
		logger.Error("profit-tracker stopped", "error", err)
		os.Exit(1)
	}
}

// start reads the configuration and runs the server until an interrupt or SIGTERM.
func start(logger *slog.Logger) error {
	cfg, err := parseConfig(os.Args[1:], os.Getenv, os.UserConfigDir)
	if errors.Is(err, flag.ErrHelp) {
		return nil
	}
	if err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	return run(ctx, cfg, logger)
}

// run opens the database, starts the server and stops it when ctx ends.
func run(ctx context.Context, cfg config, logger *slog.Logger) error {
	db, err := store.Open(ctx, cfg.dbPath)
	if err != nil {
		return err
	}
	defer db.Close()

	frontend, err := web.Dist()
	if err != nil {
		return err
	}

	// Bind the port first. Thus a busy port fails at once, and the log tells the truth.
	ln, err := new(net.ListenConfig).Listen(ctx, "tcp", cfg.addr)
	if err != nil {
		return fmt.Errorf("listen on %s: %w", cfg.addr, err)
	}

	// The API and the refresh at start share one price service, so that their refreshes do not overlap.
	prices := price.NewService(db, yahoo.New(), time.Now)
	srv := &http.Server{
		Handler: httpapi.NewHandler(httpapi.Options{
			Store:    db,
			Frontend: frontend,
			Logger:   logger,
			Prices:   prices,
			Now:      time.Now,
		}),
		ReadHeaderTimeout: 10 * time.Second,
	}

	serveErr := make(chan error, 1)
	logger.Info("profit-tracker started", "url", "http://"+ln.Addr().String(), "db", cfg.dbPath)
	go func() { serveErr <- srv.Serve(ln) }()

	// The refresh at start runs in the background. The server answers while it runs.
	refreshCtx, stopRefresh := context.WithCancel(ctx)
	refreshDone := make(chan struct{})
	go func() {
		defer close(refreshDone)
		refreshAtStart(refreshCtx, prices, logger)
	}()
	// The database closes after run returns. Thus run waits for the refresh to end.
	defer func() {
		stopRefresh()
		<-refreshDone
	}()

	select {
	case err := <-serveErr:
		return fmt.Errorf("serve on %s: %w", cfg.addr, err)
	case <-ctx.Done():
	}

	logger.Info("profit-tracker stops")
	shutdownCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("shut down server: %w", err)
	}
	return nil
}

// refreshAtStart gets new prices when a held Instrument has no price fetched today, and logs the result.
func refreshAtStart(ctx context.Context, prices *price.Service, logger *slog.Logger) {
	result, ran, err := prices.RefreshIfStale(ctx)
	if errors.Is(err, context.Canceled) {
		logger.Info("price refresh at start stopped, because the tool stops", "refreshed", result.Refreshed)
		return
	}
	if err != nil {
		logger.Error("price refresh at start failed", "error", err)
		return
	}
	if !ran {
		logger.Info("prices are current, no refresh at start")
		return
	}
	for _, f := range result.Failures {
		logger.Warn("no price for instrument", "ticker", f.Ticker, "error", f.Err)
	}
	logger.Info("price refresh at start done", "refreshed", result.Refreshed, "failures", len(result.Failures))
}
