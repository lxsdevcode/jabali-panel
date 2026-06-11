package commands

import (
	"strings"
	"testing"
)

func TestBuildNginxTunablesFragment(t *testing.T) {
	p := &nginxTunablesApplyParams{
		ClientMaxBodySize:   "50m",
		KeepaliveTimeout:    "65s",
		ServerTokens:        false,
		Gzip:                true,
		ClientBodyTimeout:   "60s",
		ClientHeaderTimeout: "60s",
		SendTimeout:         "60s",
		ProxyConnectTimeout: "300s",
		ProxyReadTimeout:    "300s",
		ProxySendTimeout:    "300s",
		CustomHTTP:          "add_header X-Demo 1;",
	}
	got := BuildNginxTunablesFragment(p)
	for _, want := range []string{
		"client_max_body_size 50m;",
		"keepalive_timeout 65s;",
		"server_tokens off;", // ServerTokens=false -> off (secure default)
		"gzip on;",
		"client_body_timeout 60s;",
		"proxy_read_timeout 300s;",
		"# --- Admin custom directives",
		"add_header X-Demo 1;",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("fragment missing %q\n--- got ---\n%s", want, got)
		}
	}
}

func TestBuildNginxTunablesFragment_ServerTokensOnGzipOff(t *testing.T) {
	got := BuildNginxTunablesFragment(&nginxTunablesApplyParams{ServerTokens: true, Gzip: false})
	if !strings.Contains(got, "server_tokens on;") {
		t.Errorf("expected server_tokens on; got:\n%s", got)
	}
	if !strings.Contains(got, "gzip off;") {
		t.Errorf("expected gzip off; got:\n%s", got)
	}
	// Empty custom snippet must not emit the Advanced header.
	if strings.Contains(got, "Admin custom directives") {
		t.Errorf("empty custom snippet should not emit Advanced header:\n%s", got)
	}
}

func TestValidateNginxTunables(t *testing.T) {
	cases := []struct {
		name    string
		p       nginxTunablesApplyParams
		wantErr bool
	}{
		{"valid", nginxTunablesApplyParams{ClientMaxBodySize: "50m", KeepaliveTimeout: "65s", WorkerProcesses: "auto", WorkerConnections: 1024}, false},
		{"valid bare seconds", nginxTunablesApplyParams{ProxyReadTimeout: "300"}, false},
		{"valid size 0", nginxTunablesApplyParams{ClientMaxBodySize: "0"}, false},
		{"bad size", nginxTunablesApplyParams{ClientMaxBodySize: "50mb"}, true},
		{"bad size injection", nginxTunablesApplyParams{ClientMaxBodySize: "50m; }"}, true},
		{"bad time", nginxTunablesApplyParams{KeepaliveTimeout: "65sec"}, true},
		{"bad worker_processes", nginxTunablesApplyParams{WorkerProcesses: "lots"}, true},
		{"worker_connections too high", nginxTunablesApplyParams{WorkerConnections: 2000000}, true},
		{"custom too long", nginxTunablesApplyParams{CustomHTTP: strings.Repeat("a", 4001)}, true},
		{"custom NUL", nginxTunablesApplyParams{CustomHTTP: "a\x00b"}, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := validateNginxTunables(&c.p)
			if (err != nil) != c.wantErr {
				t.Errorf("validateNginxTunables(%+v) err=%v, wantErr=%v", c.p, err, c.wantErr)
			}
		})
	}
}

func TestPatchNginxWorkerLines(t *testing.T) {
	conf := []byte("user www-data;\nworker_processes auto;\npid /run/nginx.pid;\nevents {\n\tworker_connections 768;\n}\nhttp {\n}\n")

	out, changed := patchNginxWorkerLines(conf, "4", 2048)
	if !changed {
		t.Fatal("expected changed=true")
	}
	s := string(out)
	if !strings.Contains(s, "worker_processes 4;") {
		t.Errorf("worker_processes not patched:\n%s", s)
	}
	if !strings.Contains(s, "\tworker_connections 2048;") {
		t.Errorf("worker_connections not patched (or lost indent):\n%s", s)
	}

	// Idempotent: same values -> no change.
	if _, changed2 := patchNginxWorkerLines(out, "4", 2048); changed2 {
		t.Errorf("expected idempotent no-change on second patch")
	}

	// Missing directive form -> left untouched, no panic.
	if _, c := patchNginxWorkerLines([]byte("http {}\n"), "4", 2048); c {
		t.Errorf("expected no change when directives absent")
	}

	// worker_connections=0 means "don't touch that knob".
	if _, c := patchNginxWorkerLines(conf, "", 0); c {
		t.Errorf("expected no change when both inputs empty/zero")
	}
}
