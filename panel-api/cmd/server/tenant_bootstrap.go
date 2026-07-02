package main

// Tenant bootstrap — auto-creates one non-admin user + their primary
// domain on first boot when the install operator set
// JABALI_BOOTSTRAP_TENANT_* env vars before running install.sh. Issue
// GH#120 asked for "admin + tenant + primary domain ready at install
// end" so single-tenant boxes don't require a separate manual click
// dance after the installer prints its one-time-login URL.
//
// Idempotent: skips when the email already exists, when the env vars
// aren't set, or when the matching domain row already exists. Failures
// log + don't block panel startup — the operator can re-run install.sh
// after fixing whatever upstream issue blocked the create.

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	"git.jabali-panel.com/shukivaknin/jabali2/panel-api/internal/repository"
)

func bootstrapTenantFromEnv(log *slog.Logger) {
	email := strings.TrimSpace(os.Getenv("JABALI_BOOTSTRAP_TENANT_EMAIL"))
	domain := strings.TrimSpace(os.Getenv("JABALI_BOOTSTRAP_TENANT_DOMAIN"))
	password := os.Getenv("JABALI_BOOTSTRAP_TENANT_PASSWORD")

	if email == "" {
		// Operator didn't request tenant bootstrap. Multi-tenant installs
		// follow this path (admin creates tenants via UI).
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// Repo init is cheap (re-uses the same sharedDB the admin bootstrap
	// just used). Skip when DB isn't wired (unit test paths).
	if sharedDB == nil {
		log.Warn("tenant bootstrap: skipping (sharedDB not initialised)")
		return
	}
	userRepo := repository.NewUserRepository(sharedDB)

	// Idempotency check 1: user already exists.
	existing, err := userRepo.FindByEmail(ctx, email)
	if err == nil && existing != nil {
		log.Info("tenant bootstrap: user already exists, skipping",
			"email", email, "user_id", existing.ID)
		// Even if user exists, check domain — operator may have re-run
		// install.sh after fixing a bad domain name.
	} else if err != nil && !errors.Is(err, repository.ErrNotFound) {
		log.Error("tenant bootstrap: user lookup failed", "err", err)
		return
	}

	var userID string
	if existing != nil {
		userID = existing.ID
	} else {
		// Generate a strong random password when the operator didn't pass one.
		// The generated value is logged at WARN so the operator can see it
		// in the install log; production deployments should always pass an
		// explicit JABALI_BOOTSTRAP_TENANT_PASSWORD.
		if password == "" {
			password = generateBootstrapPassword()
			log.Warn("tenant bootstrap: generated random password",
				"email", email, "password", password,
				"hint", "set JABALI_BOOTSTRAP_TENANT_PASSWORD to override")
		}

		first, last := splitNameFromEmail(email)
		u, warn, err := createUserDirect(ctx, cliUserInput{
			Email:     email,
			Password:  password,
			NameFirst: first,
			NameLast:  last,
			IsAdmin:   false,
		})
		if err != nil {
			log.Error("tenant bootstrap: user create failed", "email", email, "err", err)
			return
		}
		userID = u.ID
		if warn != "" {
			log.Warn("tenant bootstrap: user create warning", "warn", warn)
		}
		log.Info("tenant bootstrap: user created", "email", email, "user_id", userID)
	}

	if domain == "" {
		log.Info("tenant bootstrap: no domain requested, done")
		return
	}

	// Idempotency check 2: domain already exists for this user.
	domainRepo := repository.NewDomainRepository(sharedDB)
	if existingDom, _ := domainRepo.FindByName(ctx, domain); existingDom != nil {
		log.Info("tenant bootstrap: domain already exists, skipping",
			"domain", domain, "owner_id", existingDom.UserID)
		return
	}

	d, warnings, err := createDomainDirect(ctx, cliDomainInput{
		Name:   domain,
		UserID: userID,
	})
	if err != nil {
		log.Error("tenant bootstrap: domain create failed",
			"domain", domain, "owner_id", userID, "err", err)
		return
	}
	for _, w := range warnings {
		log.Warn("tenant bootstrap: domain create warning", "warn", w)
	}
	log.Info("tenant bootstrap: domain created",
		"domain", d.Name, "id", d.ID, "owner_id", userID)
}

// generateBootstrapPassword returns a 24-char URL-safe random string.
// Used only when the operator runs install.sh without setting
// JABALI_BOOTSTRAP_TENANT_PASSWORD. The value is logged at WARN so the
// operator can grab it from the install log.
func generateBootstrapPassword() string {
	b := make([]byte, 18) // 18 bytes -> 24 base64url chars without padding
	if _, err := rand.Read(b); err != nil {
		// Crypto-PRNG failure is exceptional. Fall back to a clearly-
		// flagged placeholder so the install doesn't die — operator must
		// rotate immediately.
		return fmt.Sprintf("changeme-%d", time.Now().Unix())
	}
	return base64.RawURLEncoding.EncodeToString(b)
}

// splitNameFromEmail derives a first/last name from the local part of
// the email when the operator didn't pass anything explicit. `alice` →
// ("Alice", "User"). `alice.smith` → ("Alice", "Smith"). Best-effort
// only — the operator can edit it in the panel.
func splitNameFromEmail(email string) (first, last string) {
	at := strings.IndexByte(email, '@')
	local := email
	if at > 0 {
		local = email[:at]
	}
	parts := strings.FieldsFunc(local, func(r rune) bool {
		return r == '.' || r == '_' || r == '-' || r == '+'
	})
	if len(parts) == 0 {
		return "Tenant", "User"
	}
	first = strings.Title(parts[0])
	if len(parts) > 1 {
		last = strings.Title(parts[len(parts)-1])
	} else {
		last = "User"
	}
	return first, last
}
