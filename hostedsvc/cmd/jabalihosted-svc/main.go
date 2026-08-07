// Command jabalihosted-svc runs the jabalihosted.com free-hostname service
// (JAB-213): registration + email verification, IP-derived labels, and the
// ACME DNS-01 broker, backed by SQLite + PowerDNS on the same box.
//
// Configuration comes from the environment (systemd EnvironmentFile,
// /etc/jabalihosted/svc.env, 0600 root:root):
//
//	LISTEN         loopback bind, default 127.0.0.1:8470 (nginx terminates TLS)
//	DB_PATH        default /var/lib/jabalihosted/svc.db
//	PDNS_URL       default http://127.0.0.1:8081
//	PDNS_API_KEY   required
//	SMTP_ADDR      submission relay, e.g. mail.reeva.me:587
//	SMTP_HOST      TLS certificate name of the relay
//	SMTP_FROM      authenticated mailbox (MAIL FROM == auth identity — 501 otherwise)
//	SMTP_PASSWORD  relay password
//
// Subcommands:
//
//	serve                     run the API (default)
//	revoke <label>            abuse desk: revoke a label + delete its DNS
package main

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"time"

	_ "modernc.org/sqlite"

	"git.jabali-panel.com/shukivaknin/jabali2/hostedsvc"
)

func env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func main() {
	log := slog.New(slog.NewTextHandler(os.Stderr, nil))
	if err := run(log); err != nil {
		log.Error("jabalihosted-svc: fatal", "err", err)
		os.Exit(1)
	}
}

func run(log *slog.Logger) error {
	dbPath := env("DB_PATH", "/var/lib/jabalihosted/svc.db")
	db, err := sql.Open("sqlite", dbPath+"?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)")
	if err != nil {
		return fmt.Errorf("open db: %w", err)
	}
	defer db.Close()
	store, err := hostedsvc.NewStore(db)
	if err != nil {
		return err
	}

	apiKey := os.Getenv("PDNS_API_KEY")
	if apiKey == "" {
		return fmt.Errorf("PDNS_API_KEY not configured")
	}
	dns := hostedsvc.NewPDNS(env("PDNS_URL", "http://127.0.0.1:8081"), apiKey)

	if len(os.Args) > 1 && os.Args[1] == "revoke" {
		if len(os.Args) != 3 {
			return fmt.Errorf("usage: jabalihosted-svc revoke <label>")
		}
		label := os.Args[2]
		if !hostedsvc.ValidLabel(label) {
			return fmt.Errorf("not a service label: %q", label)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		if err := dns.RemoveLabel(ctx, label); err != nil {
			return fmt.Errorf("dns remove: %w", err)
		}
		if err := store.RevokeLabel(label, "admin"); err != nil {
			return err
		}
		fmt.Printf("revoked %s\n", hostedsvc.FQDN(label))
		return nil
	}

	for _, k := range []string{"SMTP_ADDR", "SMTP_HOST", "SMTP_FROM", "SMTP_PASSWORD"} {
		if os.Getenv(k) == "" {
			return fmt.Errorf("%s not configured", k)
		}
	}
	api := &hostedsvc.API{
		Store: store,
		DNS:   dns,
		Mailer: &hostedsvc.SMTPMailer{
			Addr:     os.Getenv("SMTP_ADDR"),
			Host:     os.Getenv("SMTP_HOST"),
			From:     os.Getenv("SMTP_FROM"),
			Password: os.Getenv("SMTP_PASSWORD"),
		},
		Log: log,
	}

	srv := &http.Server{
		Addr:              env("LISTEN", "127.0.0.1:8470"),
		Handler:           api.Routes(),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
	}
	log.Info("jabalihosted-svc serving", "addr", srv.Addr, "db", dbPath)
	return srv.ListenAndServe()
}
