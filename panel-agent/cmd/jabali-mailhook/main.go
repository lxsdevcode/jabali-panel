// Command jabali-mailhook is a loopback-only HTTP service that Stalwart calls
// as an MTA hook at the SMTP DATA stage. It appends the per-domain disclaimer
// to outgoing mail, rewriting the MIME body losslessly (GH #233, ADR-0143).
//
// It is the one TCP-loopback exception to M25 (ADR-0050): bound to 127.0.0.1,
// Bearer-token authenticated, single POST endpoint, read-only DB access.
package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	_ "github.com/go-sql-driver/mysql"

	"git.linux-hosting.co.il/shukivaknin/jabali2/panel-agent/internal/mailhook"
)

func main() {
	if err := run(); err != nil {
		slog.Error("jabali-mailhook: fatal", "err", err)
		os.Exit(1)
	}
}

func run() error {
	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))

	addr := envOr("MAILHOOK_ADDR", "127.0.0.1:8462")
	if err := assertLoopback(addr); err != nil {
		return err
	}

	token, err := readSecretFile(envOr("MAILHOOK_TOKEN_PATH", "/etc/jabali-panel/mailhook.token"))
	if err != nil {
		return fmt.Errorf("read hook token: %w", err)
	}
	if token == "" {
		return errors.New("hook token is empty; refusing to start without Bearer auth")
	}

	dsn, err := buildDSN()
	if err != nil {
		return err
	}
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return fmt.Errorf("open db: %w", err)
	}
	defer db.Close()
	db.SetMaxOpenConns(4)
	db.SetConnMaxIdleTime(5 * time.Minute)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		return fmt.Errorf("ping db: %w", err)
	}

	h := &mailhook.Handler{
		Source:   &mailhook.DBSource{DB: db},
		Token:    token,
		MaxBytes: 48 * 1024 * 1024,
		Log:      log,
	}
	mux := http.NewServeMux()
	mux.Handle("/", h)

	srv := &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       60 * time.Second,
		WriteTimeout:      60 * time.Second,
	}

	go func() {
		stop := make(chan os.Signal, 1)
		signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
		<-stop
		sctx, scancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer scancel()
		_ = srv.Shutdown(sctx)
	}()

	log.Info("jabali-mailhook listening", "addr", addr)
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

// assertLoopback refuses to bind anything other than a loopback address — the
// M25 exception is narrow and must never accidentally listen on a public IP.
func assertLoopback(addr string) error {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return fmt.Errorf("invalid MAILHOOK_ADDR %q: %w", addr, err)
	}
	ip := net.ParseIP(host)
	if ip == nil || !ip.IsLoopback() {
		return fmt.Errorf("MAILHOOK_ADDR %q must be a loopback address", addr)
	}
	return nil
}

// buildDSN assembles the read-only MySQL DSN from the same credentials Stalwart
// uses for its SQL directory (jabali-stalwart-ro). Mirrors the Stalwart config.
func buildDSN() (string, error) {
	user := envOr("MAILHOOK_DB_USER", "jabali-stalwart-ro")
	host := envOr("MAILHOOK_DB_ADDR", "127.0.0.1:3306")
	name := envOr("MAILHOOK_DB_NAME", "jabali_panel")
	pass, err := readSecretFile(envOr("MAILHOOK_DB_PASSWORD_PATH", "/etc/jabali-panel/stalwart-mariadb.password"))
	if err != nil {
		return "", fmt.Errorf("read db password: %w", err)
	}
	// user:pass@tcp(host)/db?…  — password URL-escaped to survive special chars.
	return fmt.Sprintf("%s:%s@tcp(%s)/%s?parseTime=true&charset=utf8mb4&loc=UTC&timeout=10s",
		user, url.QueryEscape(pass), host, name), nil
}

func envOr(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func readSecretFile(path string) (string, error) {
	b, err := os.ReadFile(path) //nolint:gosec // operator-owned secret path
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(b)), nil
}
