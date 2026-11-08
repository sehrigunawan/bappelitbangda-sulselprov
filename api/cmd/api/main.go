// Command api runs the Bappelitbangda Sulsel HTTP API and the operator commands (admin, keys).
package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/kindaMid72/bappelitbangda-sulselprov/api/internal/keyring"
	"github.com/kindaMid72/bappelitbangda-sulselprov/api/internal/server"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := run(ctx, os.Args[1:], os.Stdout, os.Getenv); err != nil {
		fmt.Fprintf(os.Stderr, "api: %v\n", err)
		os.Exit(1)
	}
}

// run dispatches on the first argument: no argument serves, "admin" and "keys" are operator
// commands (docs/modules/auth.md, "Configuration and CLI"). Tests call it directly.
func run(ctx context.Context, args []string, stdout io.Writer, getenv func(string) string) error {
	if len(args) == 0 {
		return serve(ctx, stdout, getenv)
	}
	switch args[0] {
	case "admin":
		return adminCommand(ctx, args[1:], stdout, getenv)
	case "keys":
		return keysCommand(ctx, args[1:], stdout, getenv)
	default:
		return fmt.Errorf("unknown command %q (want admin or keys)", args[0])
	}
}

// serve runs the HTTP server until ctx is cancelled.
func serve(ctx context.Context, stdout io.Writer, getenv func(string) string) error {
	logger := slog.New(slog.NewJSONHandler(stdout, nil))
	cfg, err := configFromEnv(getenv)
	if err != nil {
		return err
	}
	dbURL, err := requireEnv(getenv, "DATABASE_URL")
	if err != nil {
		return err
	}
	kekFile, err := requireEnv(getenv, "KEK_FILE")
	if err != nil {
		return err
	}
	kekActive, err := requireEnv(getenv, "KEK_ACTIVE_ID")
	if err != nil {
		return err
	}

	pool, err := openPool(ctx, dbURL)
	if err != nil {
		return err
	}
	defer pool.Close()
	keys, err := keyring.Load(ctx, pool, kekFile, kekActive)
	if err != nil {
		return err
	}

	port := getenv("API_PORT")
	if port == "" {
		port = "8080"
	}
	srv := &http.Server{
		Addr:              ":" + port,
		Handler:           server.NewHandler(logger, server.Deps{Pool: pool, Keyring: keys, Config: cfg, Now: time.Now}),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		logger.Info("listening", "addr", srv.Addr)
		errCh <- srv.ListenAndServe()
	}()

	select {
	case err := <-errCh:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	case <-ctx.Done():
		logger.Info("shutting down")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		return srv.Shutdown(shutdownCtx)
	}
}
