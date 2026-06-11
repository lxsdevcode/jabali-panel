package commands

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"

	"git.linux-hosting.co.il/shukivaknin/jabali2/agentwire"
)

// nginx.tunables.apply — renders the server-wide nginx tunables managed by
// Server Settings → Nginx (M55) into the http{}-scope fragment
//
//	/etc/nginx/conf.d/05-jabali-tunables.conf
//
// and patches the two worker-scope knobs (worker_processes,
// worker_connections) into /etc/nginx/nginx.conf. The 05- prefix loads
// after the 00-jabali-ratelimits.conf zone declarations but before any
// per-vhost include, so these http-scope defaults are in effect when the
// server{} blocks parse (each vhost can still override per-directive).
//
// Atomicity: the fragment is staged to .new, the live files backed up to
// .bak, both swapped in, then a SINGLE `nginx -t` validates the whole
// config. On failure BOTH files roll back from their .bak and the command
// errors — so a bad value (or bad admin custom-snippet) never leaves nginx
// in a state that fails to reload. Same shape as nginx.ratelimits.apply,
// extended to cover nginx.conf.
//
// Inputs are re-validated here (not just at the panel-api edge): the agent
// is the privileged boundary that writes root-owned config, so it must not
// trust the caller's strings. Size/timeout values are regex-checked before
// they reach the file; the free-form CustomHTTP is length-capped and
// NUL-screened but otherwise gated only by nginx -t (matching the documented
// "advanced, can destabilize" contract).

const (
	nginxTunablesFragmentPath = "/etc/nginx/conf.d/05-jabali-tunables.conf"
	nginxMainConfPath         = "/etc/nginx/nginx.conf"
	nginxCustomHTTPMaxLen     = 4000
)

// nginxSizeRe matches an nginx size value: digits with an optional k/m/g
// suffix (case-insensitive). "0" disables the limit (valid for
// client_max_body_size). Empty is rejected by the caller before we get here.
var nginxSizeRe = regexp.MustCompile(`^[0-9]+[kKmMgG]?$`)

// nginxTimeRe matches an nginx time value: digits with an optional unit
// (ms|s|m|h). Bare digits mean seconds to nginx.
var nginxTimeRe = regexp.MustCompile(`^[0-9]+(ms|s|m|h)?$`)

// nginxWorkerProcessesRe matches "auto" or a small positive integer.
var nginxWorkerProcessesRe = regexp.MustCompile(`^(auto|[1-9][0-9]?)$`)

type nginxTunablesApplyParams struct {
	ClientMaxBodySize   string `json:"client_max_body_size"`
	KeepaliveTimeout    string `json:"keepalive_timeout"`
	ServerTokens        bool   `json:"server_tokens"`
	Gzip                bool   `json:"gzip"`
	ClientBodyTimeout   string `json:"client_body_timeout"`
	ClientHeaderTimeout string `json:"client_header_timeout"`
	SendTimeout         string `json:"send_timeout"`
	ProxyConnectTimeout string `json:"proxy_connect_timeout"`
	ProxyReadTimeout    string `json:"proxy_read_timeout"`
	ProxySendTimeout    string `json:"proxy_send_timeout"`
	WorkerProcesses     string `json:"worker_processes"`
	WorkerConnections   uint32 `json:"worker_connections"`
	CustomHTTP          string `json:"custom_http"`
}

type nginxTunablesApplyResponse struct {
	FragmentPath  string `json:"fragment_path"`
	WorkerPatched bool   `json:"worker_patched"`
	NoChange      bool   `json:"no_change,omitempty"`
	Rolled        bool   `json:"rolled_back,omitempty"`
}

func nginxTunablesApplyHandler(ctx context.Context, params json.RawMessage) (any, error) {
	var p nginxTunablesApplyParams
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, &agentwire.AgentError{
			Code:    agentwire.CodeInvalidArgument,
			Message: fmt.Sprintf("failed to parse params: %v", err),
		}
	}
	if err := validateNginxTunables(&p); err != nil {
		return nil, &agentwire.AgentError{
			Code:    agentwire.CodeInvalidArgument,
			Message: err.Error(),
		}
	}

	fragment := BuildNginxTunablesFragment(&p)

	// Compute desired nginx.conf BEFORE touching disk so we can detect a
	// true no-op (fragment unchanged AND worker lines already correct).
	liveMain, _ := os.ReadFile(nginxMainConfPath)
	newMain, workerChanged := patchNginxWorkerLines(liveMain, p.WorkerProcesses, p.WorkerConnections)

	liveFragment, _ := os.ReadFile(nginxTunablesFragmentPath)
	fragmentChanged := !bytes.Equal(liveFragment, []byte(fragment))

	if !fragmentChanged && !workerChanged {
		return &nginxTunablesApplyResponse{
			FragmentPath: nginxTunablesFragmentPath,
			NoChange:     true,
		}, nil
	}

	// Stage + swap with coordinated rollback. We back up whatever we are
	// about to overwrite, swap the new content in, run a single nginx -t,
	// and on failure restore every file we touched.
	var rollbacks []func()
	restore := func() {
		for i := len(rollbacks) - 1; i >= 0; i-- {
			rollbacks[i]()
		}
	}

	if fragmentChanged {
		if err := swapFile(nginxTunablesFragmentPath, []byte(fragment), &rollbacks); err != nil {
			restore()
			return nil, &agentwire.AgentError{Code: agentwire.CodeInternal, Message: err.Error()}
		}
	}
	if workerChanged {
		if err := swapFile(nginxMainConfPath, newMain, &rollbacks); err != nil {
			restore()
			return nil, &agentwire.AgentError{Code: agentwire.CodeInternal, Message: err.Error()}
		}
	}

	testCmd := exec.CommandContext(ctx, "nginx", "-t")
	var testOut bytes.Buffer
	testCmd.Stdout = &testOut
	testCmd.Stderr = &testOut
	if err := testCmd.Run(); err != nil {
		restore()
		return nil, &agentwire.AgentError{
			Code:    agentwire.CodeInternal,
			Message: fmt.Sprintf("nginx -t failed after tunables update, rolled back: %s", testOut.String()),
		}
	}

	// Validated — reload so the new config takes effect.
	reloadCmd := exec.CommandContext(ctx, "systemctl", "reload", "nginx")
	var reloadOut bytes.Buffer
	reloadCmd.Stdout = &reloadOut
	reloadCmd.Stderr = &reloadOut
	if err := reloadCmd.Run(); err != nil {
		// Config is valid but reload failed — leave the files in place
		// (next reload/restart will pick them up) and surface the error.
		return nil, &agentwire.AgentError{
			Code:    agentwire.CodeInternal,
			Message: fmt.Sprintf("nginx -t passed but reload failed: %s", reloadOut.String()),
		}
	}

	return &nginxTunablesApplyResponse{
		FragmentPath:  nginxTunablesFragmentPath,
		WorkerPatched: workerChanged,
	}, nil
}

// swapFile backs up path → .bak (if it exists), writes content, and pushes a
// rollback closure that restores the previous state. The caller runs the
// rollbacks in reverse on any later failure and removes the .bak files on
// success implicitly (they are left on disk; a subsequent successful run
// overwrites them — harmless, and useful for forensic diff).
func swapFile(path string, content []byte, rollbacks *[]func()) error {
	backup := path + ".jabali-bak"
	orig, statErr := os.ReadFile(path)
	had := statErr == nil
	if had {
		if err := os.WriteFile(backup, orig, 0644); err != nil {
			return fmt.Errorf("backup %s: %w", path, err)
		}
	}
	if err := os.WriteFile(path, content, 0644); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	*rollbacks = append(*rollbacks, func() {
		if had {
			_ = os.WriteFile(path, orig, 0644)
		} else {
			_ = os.Remove(path)
		}
	})
	return nil
}

// patchNginxWorkerLines returns the nginx.conf bytes with worker_processes and
// worker_connections set to the requested values, plus whether anything
// changed. It only REPLACES existing directives — it never inserts new ones,
// because worker_connections must live inside the events{} block and
// worker_processes at main scope; getting that placement wrong on an unusual
// nginx.conf would break the config. If a directive isn't present in the
// expected form, it's left untouched (the value silently no-ops for that
// knob rather than risking a malformed file). Debian/Ubuntu stock nginx.conf
// always ships both, so this covers every supported install.
func patchNginxWorkerLines(conf []byte, workerProcesses string, workerConnections uint32) ([]byte, bool) {
	if len(conf) == 0 {
		return conf, false
	}
	out := string(conf)
	changed := false

	if workerProcesses != "" {
		re := regexp.MustCompile(`(?m)^([ \t]*)worker_processes[ \t]+[^;]+;`)
		if loc := re.FindStringSubmatchIndex(out); loc != nil {
			repl := re.ReplaceAllString(out, fmt.Sprintf("${1}worker_processes %s;", workerProcesses))
			if repl != out {
				out = repl
				changed = true
			}
		}
	}
	if workerConnections > 0 {
		re := regexp.MustCompile(`(?m)^([ \t]*)worker_connections[ \t]+[^;]+;`)
		if loc := re.FindStringSubmatchIndex(out); loc != nil {
			repl := re.ReplaceAllString(out, fmt.Sprintf("${1}worker_connections %d;", workerConnections))
			if repl != out {
				out = repl
				changed = true
			}
		}
	}
	return []byte(out), changed
}

// BuildNginxTunablesFragment renders the http{}-scope fragment. Pure +
// deterministic so the idempotent read-compare in the handler works.
func BuildNginxTunablesFragment(p *nginxTunablesApplyParams) string {
	var b strings.Builder
	b.WriteString("# Auto-generated by jabali — do not edit.\n")
	b.WriteString("# Server-wide nginx tunables (Server Settings → Nginx, M55).\n")
	b.WriteString("# http{}-scope defaults; individual vhosts may override per-directive.\n\n")

	if p.ClientMaxBodySize != "" {
		fmt.Fprintf(&b, "client_max_body_size %s;\n", p.ClientMaxBodySize)
	}
	if p.KeepaliveTimeout != "" {
		fmt.Fprintf(&b, "keepalive_timeout %s;\n", p.KeepaliveTimeout)
	}
	if p.ServerTokens {
		b.WriteString("server_tokens on;\n")
	} else {
		b.WriteString("server_tokens off;\n")
	}
	if p.Gzip {
		b.WriteString("gzip on;\n")
	} else {
		b.WriteString("gzip off;\n")
	}
	if p.ClientBodyTimeout != "" {
		fmt.Fprintf(&b, "client_body_timeout %s;\n", p.ClientBodyTimeout)
	}
	if p.ClientHeaderTimeout != "" {
		fmt.Fprintf(&b, "client_header_timeout %s;\n", p.ClientHeaderTimeout)
	}
	if p.SendTimeout != "" {
		fmt.Fprintf(&b, "send_timeout %s;\n", p.SendTimeout)
	}
	if p.ProxyConnectTimeout != "" {
		fmt.Fprintf(&b, "proxy_connect_timeout %s;\n", p.ProxyConnectTimeout)
	}
	if p.ProxyReadTimeout != "" {
		fmt.Fprintf(&b, "proxy_read_timeout %s;\n", p.ProxyReadTimeout)
	}
	if p.ProxySendTimeout != "" {
		fmt.Fprintf(&b, "proxy_send_timeout %s;\n", p.ProxySendTimeout)
	}

	custom := strings.TrimRight(p.CustomHTTP, " \t\r\n")
	if custom != "" {
		b.WriteString("\n# --- Admin custom directives (Server Settings → Nginx → Advanced) ---\n")
		b.WriteString(custom)
		b.WriteString("\n")
	}
	return b.String()
}

func validateNginxTunables(p *nginxTunablesApplyParams) error {
	sizes := map[string]string{"client_max_body_size": p.ClientMaxBodySize}
	for name, v := range sizes {
		if v != "" && !nginxSizeRe.MatchString(v) {
			return fmt.Errorf("%s: invalid nginx size %q", name, v)
		}
	}
	times := map[string]string{
		"keepalive_timeout":     p.KeepaliveTimeout,
		"client_body_timeout":   p.ClientBodyTimeout,
		"client_header_timeout": p.ClientHeaderTimeout,
		"send_timeout":          p.SendTimeout,
		"proxy_connect_timeout": p.ProxyConnectTimeout,
		"proxy_read_timeout":    p.ProxyReadTimeout,
		"proxy_send_timeout":    p.ProxySendTimeout,
	}
	for name, v := range times {
		if v != "" && !nginxTimeRe.MatchString(v) {
			return fmt.Errorf("%s: invalid nginx time %q", name, v)
		}
	}
	if p.WorkerProcesses != "" && !nginxWorkerProcessesRe.MatchString(p.WorkerProcesses) {
		return fmt.Errorf("worker_processes: must be 'auto' or 1-99")
	}
	if p.WorkerConnections > 1048576 {
		return fmt.Errorf("worker_connections: must be <= 1048576")
	}
	if len(p.CustomHTTP) > nginxCustomHTTPMaxLen {
		return fmt.Errorf("custom_http: must be <= %d chars", nginxCustomHTTPMaxLen)
	}
	if strings.ContainsRune(p.CustomHTTP, 0) {
		return fmt.Errorf("custom_http: must not contain NUL bytes")
	}
	return nil
}

func init() {
	Default.Register("nginx.tunables.apply", nginxTunablesApplyHandler)
}
