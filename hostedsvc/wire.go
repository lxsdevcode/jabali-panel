package hostedsvc

// Wire shapes for the v1 API. These are the cross-boundary contract with the
// panel-side client (installer + certbot hook, phase 3) — pinned by the
// fixtures in testdata/, same pattern as agentwire. Change a field here and
// the round-trip tests on BOTH sides must be updated consciously.

type RegisterRequest struct {
	Email string `json:"email"`
}

type RegisterResponse struct {
	Ok bool `json:"ok"`
}

type ClaimRequest struct {
	Email string `json:"email"`
	Code  string `json:"code"`
}

type ClaimResponse struct {
	Label string `json:"label"`
	FQDN  string `json:"fqdn"`
	// Token is the bearer secret for every later call. Shown once; the
	// panel stores it at 0600 under /etc/jabali-panel/.
	Token string `json:"token"`
}

type TokenRequest struct {
	Token string `json:"token"`
}

type AcmePresentRequest struct {
	Token string `json:"token"`
	TXT   string `json:"txt"`
}

type HeartbeatResponse struct {
	Ok bool `json:"ok"`
	// IPMoved tells the box its source address no longer matches its label;
	// it should re-claim from the new address (old label survives 7 days).
	IPMoved bool `json:"ip_moved,omitempty"`
}

type OkResponse struct {
	Ok bool `json:"ok"`
}

type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message"`
}
