package mailhook

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
)

// hookRequest is the JSON Stalwart POSTs at the DATA stage (ADR-0143).
// Only the fields we need are modelled.
type hookRequest struct {
	Envelope struct {
		From struct {
			Address string `json:"address"`
		} `json:"from"`
	} `json:"envelope"`
	Message struct {
		Headers  [][2]string `json:"headers"`
		Contents string      `json:"contents"`
		Size     int         `json:"size"`
	} `json:"message"`
}

type modification struct {
	Type  string `json:"type"`
	Value string `json:"value"`
}

type hookResponse struct {
	Action        string         `json:"action"`
	Modifications []modification `json:"modifications,omitempty"`
}

// DisclaimerSource resolves the disclaimer for a sender domain.
type DisclaimerSource interface {
	ForDomain(ctx context.Context, domain string) (Disclaimer, bool, error)
}

// Handler is the MTA-hook HTTP handler.
type Handler struct {
	Source   DisclaimerSource
	Token    string // required Bearer token; empty disables the check (loopback dev only)
	MaxBytes int    // skip rewrite if the new body would exceed this (0 = no cap)
	Log      *slog.Logger
}

// accept is the passthrough response: deliver unchanged.
var accept = hookResponse{Action: "accept"}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !h.authOK(r) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	var req hookRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		// Malformed request: fail open so mail still flows.
		h.Log.Warn("mailhook: decode request failed", "err", err)
		writeJSON(w, accept)
		return
	}

	resp := h.process(r.Context(), &req)
	writeJSON(w, resp)
}

func (h *Handler) process(ctx context.Context, req *hookRequest) hookResponse {
	domain := domainOf(req.Envelope.From.Address)
	if domain == "" {
		return accept
	}
	d, ok, err := h.Source.ForDomain(ctx, domain)
	if err != nil {
		// DB error → fail open (deliver without disclaimer, never bounce).
		h.Log.Warn("mailhook: disclaimer lookup failed", "domain", domain, "err", err)
		return accept
	}
	if !ok {
		return accept
	}

	newBody, err := rewriteBody(req.Message.Headers, []byte(req.Message.Contents), d)
	if err != nil {
		h.Log.Warn("mailhook: rewrite failed", "domain", domain, "err", err)
		return accept
	}
	if h.MaxBytes > 0 && len(newBody) > h.MaxBytes {
		h.Log.Warn("mailhook: rewritten body exceeds cap; passthrough", "domain", domain, "size", len(newBody), "cap", h.MaxBytes)
		return accept
	}

	h.Log.Info("mailhook: disclaimer applied", "domain", domain, "in", req.Message.Size, "out", len(newBody))
	return hookResponse{
		Action:        "accept",
		Modifications: []modification{{Type: "replaceContents", Value: string(newBody)}},
	}
}

func (h *Handler) authOK(r *http.Request) bool {
	if h.Token == "" {
		return true
	}
	const pfx = "Bearer "
	got := r.Header.Get("Authorization")
	if !strings.HasPrefix(got, pfx) {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(strings.TrimPrefix(got, pfx)), []byte(h.Token)) == 1
}

// domainOf returns the lowercased domain of an email address, or "".
func domainOf(addr string) string {
	addr = strings.TrimSpace(addr)
	at := strings.LastIndex(addr, "@")
	if at < 0 || at == len(addr)-1 {
		return ""
	}
	return strings.ToLower(addr[at+1:])
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}
