// Command kala runs the Kala server and its maintenance subcommands.
//
//	kala [serve]                    run the HTTP server (default)
//	kala backup <file>              write a consistent copy of the database
//	kala user reset-password <email>  set a new password read from stdin
package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"kala/internal/api"
	"kala/internal/auth"
	"kala/internal/config"
	"kala/internal/events"
	"kala/internal/mcp"
	"kala/internal/schedule"
	"kala/internal/service"
	"kala/internal/store"
	"kala/web"
)

func main() {
	log := slog.New(slog.NewTextHandler(os.Stderr, nil))
	if err := run(os.Args[1:], log); err != nil {
		log.Error("fatal", "err", err)
		os.Exit(1)
	}
}

func run(args []string, log *slog.Logger) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	cmd := "serve"
	if len(args) > 0 {
		cmd, args = args[0], args[1:]
	}
	switch cmd {
	case "serve":
		return serve(ctx, cfg, log)
	case "backup":
		if len(args) != 1 {
			return errors.New("usage: kala backup <file>")
		}
		return withDB(ctx, cfg, func(_ *service.Service, backup dbBackup) error { return backup(args[0]) })
	case "user":
		if len(args) != 2 || args[0] != "reset-password" {
			return errors.New("usage: kala user reset-password <email>  (password read from stdin)")
		}
		return withDB(ctx, cfg, func(svc *service.Service, _ dbBackup) error { return resetPassword(ctx, svc, args[1]) })
	case "help", "-h", "--help":
		fmt.Println("usage: kala [serve | backup <file> | user reset-password <email>]")
		return nil
	}
	return fmt.Errorf("unknown command %q", cmd)
}

type dbBackup func(dst string) error

func withDB(ctx context.Context, cfg config.Config, fn func(*service.Service, dbBackup) error) error {
	db, err := store.Open(ctx, cfg.DBPath)
	if err != nil {
		return err
	}
	defer db.Close()
	return fn(service.New(db, nil, nil), func(dst string) error { return store.Backup(ctx, db, dst) })
}

func resetPassword(ctx context.Context, svc *service.Service, email string) error {
	fmt.Fprint(os.Stderr, "New password: ")
	line, err := bufio.NewReader(os.Stdin).ReadString('\n')
	if err != nil && line == "" {
		return err
	}
	if err := svc.ResetPassword(ctx, email, strings.TrimRight(line, "\r\n")); err != nil {
		return err
	}
	fmt.Fprintln(os.Stderr, "Password updated; existing sessions were signed out.")
	return nil
}

func serve(ctx context.Context, cfg config.Config, log *slog.Logger) error {
	db, err := store.Open(ctx, cfg.DBPath)
	if err != nil {
		return err
	}
	defer db.Close()

	secret := cfg.JWTSecret
	if secret == "" {
		if secret, err = store.Secret(ctx, db, "jwt_secret", 32); err != nil {
			return err
		}
	}
	hub := events.NewHub()
	svc := service.New(db, hub, nil)
	sched := schedule.New(svc, hub, log)
	go sched.Run(ctx)

	handler := api.New(api.Options{
		Service: svc,
		Signer:  auth.NewSigner([]byte(secret), api.AccessTTL),
		Hub:     hub,
		Config:  cfg,
		Logger:  log,
		Static:  web.Dist(),
		MCP:     mcp.Handler(svc),
	})
	// Request contexts derive from base, so cancelling it on shutdown ends
	// long-lived SSE streams instead of waiting for them to go idle.
	base, cancelBase := context.WithCancel(context.Background())
	defer cancelBase()
	srv := &http.Server{
		Addr:              cfg.Addr,
		BaseContext:       func(net.Listener) context.Context { return base },
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       60 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       120 * time.Second,
	}
	errc := make(chan error, 1)
	go func() {
		log.Info("listening", "addr", cfg.Addr, "db", cfg.DBPath, "dev", cfg.Dev, "signup", cfg.AllowSignup)
		errc <- srv.ListenAndServe()
	}()
	select {
	case err := <-errc:
		return err
	case <-ctx.Done():
	}
	log.Info("shutting down")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	srv.RegisterOnShutdown(cancelBase)
	return srv.Shutdown(shutdownCtx)
}
