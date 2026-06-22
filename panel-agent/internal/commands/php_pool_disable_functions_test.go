package commands

import (
	"os"
	"strings"
	"testing"
	"text/template"
)

// The shipped per-user pool template must hard-code the GH #401
// command-exec lockdown as a php_admin_value (so a tenant can't re-enable
// it), and it must be in the agent's forbiddenDirectives set so an admin
// override can't smuggle a weaker value through. This renders the actual
// template that install.sh + `jabali update` ship.
func TestPoolTemplate_DisableFunctionsHardcoded(t *testing.T) {
	const path = "../../../install/php/jabali-php-pool.conf.tmpl"
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read template: %v", err)
	}
	tmpl, err := template.New("pool").Parse(string(data))
	if err != nil {
		t.Fatalf("parse template: %v", err)
	}
	var b strings.Builder
	spec := phpPoolSpecTemplate{
		PoolName: "alice", User: "alice", Group: "alice",
		SocketPath: "/run/php/jabali-alice/fpm.sock",
		PmMode:     "ondemand", PmMaxChildren: 5, ProcessIdleTimeoutSeconds: 10,
	}
	if err := tmpl.Execute(&b, spec); err != nil {
		t.Fatalf("execute: %v", err)
	}
	out := b.String()

	// Inspect only the directive line, not surrounding comments.
	var dfLine string
	for _, ln := range strings.Split(out, "\n") {
		if strings.HasPrefix(strings.TrimSpace(ln), "php_admin_value[disable_functions]") {
			dfLine = ln
			break
		}
	}
	if dfLine == "" {
		t.Fatalf("rendered pool missing php_admin_value[disable_functions]:\n%s", out)
	}
	for _, fn := range []string{"exec", "shell_exec", "system", "proc_open", "popen", "pcntl_exec"} {
		if !strings.Contains(dfLine, fn) {
			t.Errorf("disable_functions must include %q (line: %s)", fn, dfLine)
		}
	}
	// curl/file_get_contents stay enabled (WordPress HTTP API); SSRF is an
	// egress-layer concern. Guard against an over-broad list.
	for _, fn := range []string{"curl_exec", "file_get_contents", "fsockopen"} {
		if strings.Contains(dfLine, fn) {
			t.Errorf("disable_functions must NOT include %q (breaks legit apps)", fn)
		}
	}

	// forbiddenDirectives guards the override path.
	if !forbiddenDirectives["disable_functions"] {
		t.Error("disable_functions must be in forbiddenDirectives")
	}
}
