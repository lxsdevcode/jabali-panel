package hostedsvc

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/mail"
	"strings"
	"sync"
	"time"
)

// API glues the store, DNS backend, and mailer behind the six v1 endpoints.
// Wire shapes live in wire.go and are pinned by contract fixtures the
// panel-side client (phase 3) tests against.
type API struct {
	Store  *Store
	DNS    DNSBackend
	Mailer Mailer
	Log    *slog.Logger

	// ipMu serialises claim per source IP so two concurrent claims from one
	// NAT can't race the collision-suffix scan into the same label.
	ipMu sync.Mutex
}

func (a *API) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /v1/register", a.register)
	mux.HandleFunc("POST /v1/claim", a.claim)
	mux.HandleFunc("POST /v1/heartbeat", a.heartbeat)
	mux.HandleFunc("POST /v1/acme/present", a.acmePresent)
	mux.HandleFunc("POST /v1/acme/cleanup", a.acmeCleanup)
	mux.HandleFunc("POST /v1/release", a.release)
	return mux
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, status int, code, msg string) {
	writeJSON(w, status, ErrorResponse{Error: code, Message: msg})
}

func decode[T any](w http.ResponseWriter, r *http.Request, into *T) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 16<<10)
	if err := json.NewDecoder(r.Body).Decode(into); err != nil {
		writeErr(w, http.StatusBadRequest, "bad_json", "request body must be a JSON object")
		return false
	}
	return true
}

func normEmail(raw string) (string, bool) {
	addr, err := mail.ParseAddress(strings.TrimSpace(raw))
	if err != nil || addr.Address != strings.TrimSpace(raw) {
		return "", false
	}
	return strings.ToLower(addr.Address), true
}

// register: mail a verification code to the address.
func (a *API) register(w http.ResponseWriter, r *http.Request) {
	var req RegisterRequest
	if !decode(w, r, &req) {
		return
	}
	email, ok := normEmail(req.Email)
	if !ok {
		writeErr(w, http.StatusUnprocessableEntity, "bad_email", "not a valid email address")
		return
	}
	code, err := NewCode()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "internal", "code generation failed")
		return
	}
	if err := a.Store.PutCode(email, code); err != nil {
		if errors.Is(err, ErrRateLimited) {
			writeErr(w, http.StatusTooManyRequests, "rate_limited", "a code was sent recently — wait a minute")
			return
		}
		a.Log.Error("register: store", "err", err)
		writeErr(w, http.StatusInternalServerError, "internal", "try again later")
		return
	}
	if err := a.Mailer.SendCode(email, code); err != nil {
		a.Log.Error("register: send", "email", email, "err", err)
		writeErr(w, http.StatusBadGateway, "send_failed", "could not deliver the code — check the address")
		return
	}
	a.Store.Audit(email, "register", "code sent")
	writeJSON(w, http.StatusOK, RegisterResponse{Ok: true})
}

// claim: verify the code, derive the label from the OBSERVED source IP,
// create DNS, return the label + bearer token.
func (a *API) claim(w http.ResponseWriter, r *http.Request) {
	var req ClaimRequest
	if !decode(w, r, &req) {
		return
	}
	email, ok := normEmail(req.Email)
	if !ok {
		writeErr(w, http.StatusUnprocessableEntity, "bad_email", "not a valid email address")
		return
	}
	ip := ClientIP(r)
	if ip == nil {
		writeErr(w, http.StatusBadRequest, "no_ip", "could not determine source address")
		return
	}
	base, err := LabelFromIP(ip)
	if err != nil {
		writeErr(w, http.StatusUnprocessableEntity, "bad_source", err.Error())
		return
	}
	if err := a.Store.ConsumeCode(email, req.Code); err != nil {
		switch {
		case errors.Is(err, ErrCodeAttempts):
			writeErr(w, http.StatusTooManyRequests, "code_attempts", "too many wrong codes — request a new one")
		case errors.Is(err, ErrCodeInvalid):
			writeErr(w, http.StatusForbidden, "code_invalid", "code wrong or expired")
		default:
			a.Log.Error("claim: consume", "err", err)
			writeErr(w, http.StatusInternalServerError, "internal", "try again later")
		}
		return
	}

	a.ipMu.Lock()
	defer a.ipMu.Unlock()
	label := base
	for n := 1; ; n++ {
		exists, err := a.Store.LabelExists(label)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, "internal", "try again later")
			return
		}
		if !exists {
			break
		}
		if n > 26 {
			writeErr(w, http.StatusConflict, "label_space_exhausted", "too many hosts behind this IP")
			return
		}
		label = CollisionLabel(base, n)
	}

	raw, hash, err := NewToken()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "internal", "token generation failed")
		return
	}
	if err := a.Store.CreateLabel(label, ip.String(), email, hash); err != nil {
		if errors.Is(err, ErrEmailCap) {
			writeErr(w, http.StatusForbidden, "email_cap", "this email already holds the maximum number of hostnames")
			return
		}
		a.Log.Error("claim: create", "err", err)
		writeErr(w, http.StatusInternalServerError, "internal", "try again later")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()
	if err := a.DNS.EnsureA(ctx, label, ip.String()); err != nil {
		a.Log.Error("claim: dns", "label", label, "err", err)
		writeErr(w, http.StatusInternalServerError, "dns_failed", "could not publish the record — try again")
		return
	}
	// Wildcard A so mail.<label>/autoconfig.<label>/previews resolve to the
	// box too — lets the panel's existing HTTP-01 cert flow cover them with
	// no DNS-01 broker in v1. Best-effort: the apex hostname already works
	// if this lags; the reconciler-style heartbeat is not blocked on it.
	if err := a.DNS.EnsureWildcardA(ctx, label, ip.String()); err != nil {
		a.Log.Warn("claim: wildcard dns (non-fatal)", "label", label, "err", err)
	}
	a.Log.Info("label claimed", "label", label, "ip", ip.String())
	writeJSON(w, http.StatusOK, ClaimResponse{Label: label, FQDN: FQDN(label), Token: raw})
}

// byToken resolves the caller's label from the bearer token — the ONLY
// identity mechanism. No handler accepts a label parameter, which makes
// "a token may only act on its own label" true by construction.
func (a *API) byToken(w http.ResponseWriter, r *http.Request, token string) *Label {
	if token == "" {
		writeErr(w, http.StatusUnauthorized, "no_token", "token required")
		return nil
	}
	l, err := a.Store.LabelByToken(token)
	switch {
	case errors.Is(err, ErrNotFound):
		writeErr(w, http.StatusUnauthorized, "bad_token", "unknown token")
		return nil
	case errors.Is(err, ErrLabelRevoked):
		writeErr(w, http.StatusForbidden, "revoked", "this hostname was revoked")
		return nil
	case err != nil:
		writeErr(w, http.StatusInternalServerError, "internal", "try again later")
		return nil
	}
	return l
}

func (a *API) heartbeat(w http.ResponseWriter, r *http.Request) {
	var req TokenRequest
	if !decode(w, r, &req) {
		return
	}
	l := a.byToken(w, r, req.Token)
	if l == nil {
		return
	}
	_ = a.Store.TouchLabel(l.Label)
	resp := HeartbeatResponse{Ok: true}
	if ip := ClientIP(r); ip != nil {
		if cur, err := LabelFromIP(ip); err == nil && cur != l.IPv4Label() {
			// The box moved. It should re-claim from the new address; the old
			// label lives for reclaimAfterMove so certs don't die mid-move.
			resp.IPMoved = true
		}
	}
	writeJSON(w, http.StatusOK, resp)
}

func (a *API) acmePresent(w http.ResponseWriter, r *http.Request) {
	var req AcmePresentRequest
	if !decode(w, r, &req) {
		return
	}
	l := a.byToken(w, r, req.Token)
	if l == nil {
		return
	}
	if len(req.TXT) == 0 || len(req.TXT) > 255 {
		writeErr(w, http.StatusUnprocessableEntity, "bad_txt", "txt must be 1-255 bytes")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()
	if err := a.DNS.SetChallenge(ctx, l.Label, req.TXT); err != nil {
		a.Log.Error("acme present: dns", "label", l.Label, "err", err)
		writeErr(w, http.StatusInternalServerError, "dns_failed", "could not publish the challenge")
		return
	}
	writeJSON(w, http.StatusOK, OkResponse{Ok: true})
}

func (a *API) acmeCleanup(w http.ResponseWriter, r *http.Request) {
	var req TokenRequest
	if !decode(w, r, &req) {
		return
	}
	l := a.byToken(w, r, req.Token)
	if l == nil {
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()
	if err := a.DNS.ClearChallenge(ctx, l.Label); err != nil {
		a.Log.Error("acme cleanup: dns", "label", l.Label, "err", err)
	}
	writeJSON(w, http.StatusOK, OkResponse{Ok: true})
}

func (a *API) release(w http.ResponseWriter, r *http.Request) {
	var req TokenRequest
	if !decode(w, r, &req) {
		return
	}
	l := a.byToken(w, r, req.Token)
	if l == nil {
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()
	if err := a.DNS.RemoveLabel(ctx, l.Label); err != nil {
		a.Log.Error("release: dns", "label", l.Label, "err", err)
	}
	if err := a.Store.RevokeLabel(l.Label, l.Email); err != nil {
		writeErr(w, http.StatusInternalServerError, "internal", "try again later")
		return
	}
	writeJSON(w, http.StatusOK, OkResponse{Ok: true})
}

// IPv4Label is the dash form of the label's bound address (collision
// suffixes excluded), used to detect IP moves in heartbeat.
func (l *Label) IPv4Label() string {
	return strings.ReplaceAll(l.IPv4, ".", "-")
}
