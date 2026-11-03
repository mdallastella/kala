// Command kala runs the Kala server and its maintenance subcommands.
//
//	kala [serve]                    run the HTTP server (default)
//	kala backup <file>              write a consistent copy of the database
//	kala user reset-password <email>  set a new password read from stdin
//	kala healthcheck                exit non-zero unless the local server answers /healthz
//	kala convert [--vikunja <zip>] [--clockify <csv>] [--tz <zone>]
//	                                write a kala export of Vikunja/Clockify data to stdout
package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"flag"
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
	"kala/internal/importer"
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
	case "healthcheck":
		return healthcheck(ctx, cfg)
	case "convert":
		return convert(args)
	case "help", "-h", "--help":
		fmt.Println("usage: kala [serve | backup <file> | user reset-password <email> | healthcheck |\n" +
			"            convert [--vikunja <zip|json>] [--clockify <csv>] [--tz <zone>] > kala.json]")
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

// convert writes a kala export built from a Vikunja export and/or a Clockify
// CSV to stdout, to be loaded with the regular JSON import.
func convert(args []string) error {
	fs := flag.NewFlagSet("convert", flag.ContinueOnError)
	vikunjaPath := fs.String("vikunja", "", "Vikunja export (.zip or data.json)")
	clockifyPath := fs.String("clockify", "", "Clockify detailed time entry export (.csv)")
	tz := fs.String("tz", "", "timezone of Clockify times and Vikunja due dates (default: local)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *vikunjaPath == "" && *clockifyPath == "" || fs.NArg() > 0 {
		return errors.New("usage: kala convert [--vikunja <zip|json>] [--clockify <csv>] [--tz <zone>] > kala.json")
	}
	loc := time.Local
	if *tz != "" {
		var err error
		if loc, err = time.LoadLocation(*tz); err != nil {
			return fmt.Errorf("--tz: %w", err)
		}
	}
	read := func(path string) ([]byte, error) {
		if path == "" {
			return nil, nil
		}
		return os.ReadFile(path) //nolint:gosec // paths come from the operator running the CLI
	}
	vikunja, err := read(*vikunjaPath)
	if err != nil {
		return err
	}
	clockify, err := read(*clockifyPath)
	if err != nil {
		return err
	}
	ex, st, err := importer.Convert(vikunja, clockify, loc, time.Now())
	if err != nil {
		return err
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(ex); err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "timezone %s: %d tasks, %d time entries; %d Clockify groups matched a Vikunja task, %d became new tasks\n",
		loc, st.Tasks, st.Entries, st.Matched, st.Created)
	return nil
}

// healthcheck probes the server listening on cfg.Addr; it exists because the
// distroless image has no shell or curl for a Docker HEALTHCHECK.
func healthcheck(ctx context.Context, cfg config.Config) error {
	host, port, err := net.SplitHostPort(cfg.Addr)
	if err != nil {
		return fmt.Errorf("parse KALA_ADDR: %w", err)
	}
	if host == "" || host == "0.0.0.0" || host == "::" {
		host = "127.0.0.1"
	}
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://"+net.JoinHostPort(host, port)+"/healthz", nil)
	if err != nil {
		return err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("healthz: %s", resp.Status)
	}
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
