package commands

import (
	"os"
	"strings"
	"testing"
	"text/template"
)

func renderPoolTemplate(t *testing.T, spec phpPoolSpecTemplate) string {
	t.Helper()
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
	if err := tmpl.Execute(&b, spec); err != nil {
		t.Fatalf("execute: %v", err)
	}
	return b.String()
}

func dfLine(out string) string {
	for _, ln := range strings.Split(out, "\n") {
		if strings.HasPrefix(strings.TrimSpace(ln), "php_admin_value[disable_functions]") {
			return ln
		}
	}
	return ""
}

func baseSpec() phpPoolSpecTemplate {
	return phpPoolSpecTemplate{
		PoolName: "alice", User: "alice", Group: "alice",
		SocketPath: "/run/php/jabali-alice/fpm.sock",
		PmMode:     "ondemand", PmMaxChildren: 5, ProcessIdleTimeoutSeconds: 10,
	}
}

// The #401 command-exec lockdown is the safe default, rendered from the spec
// (GH #402 made it spec-driven, no longer hard-coded in the .tmpl).
func TestPoolTemplate_DisableFunctions_Default(t *testing.T) {
	spec := baseSpec()
	spec.DisableFunctions = defaultDisableFunctions
	line := dfLine(renderPoolTemplate(t, spec))
	if line == "" {
		t.Fatal("default render missing php_admin_value[disable_functions]")
	}
	for _, fn := range []string{"exec", "shell_exec", "system", "proc_open", "popen", "pcntl_exec"} {
		if !strings.Contains(line, fn) {
			t.Errorf("default disable_functions must include %q (line: %s)", fn, line)
		}
	}
	for _, fn := range []string{"curl_exec", "file_get_contents", "fsockopen"} {
		if strings.Contains(line, fn) {
			t.Errorf("disable_functions must NOT include %q (breaks legit apps)", fn)
		}
	}
}

// GH #402: an admin opt-out empties DisableFunctions -> NO line emitted.
func TestPoolTemplate_DisableFunctions_OptOutEmitsNoLine(t *testing.T) {
	spec := baseSpec()
	spec.DisableFunctions = ""
	if line := dfLine(renderPoolTemplate(t, spec)); line != "" {
		t.Fatalf("opt-out (empty) must emit no disable_functions line, got: %s", line)
	}
}

func TestPoolTemplate_DisableFunctions_CustomValue(t *testing.T) {
	spec := baseSpec()
	spec.DisableFunctions = "exec,shell_exec"
	line := dfLine(renderPoolTemplate(t, spec))
	if !strings.Contains(line, "= exec,shell_exec") {
		t.Fatalf("custom value not rendered verbatim, got: %s", line)
	}
}

// The default const must be the #401 list, and disable_functions must stay in
// forbiddenDirectives so a tenant admin_values override can never shorten it.
func TestDisableFunctions_DefaultAndGuard(t *testing.T) {
	for _, fn := range []string{"exec", "passthru", "shell_exec", "system", "proc_open", "popen", "pcntl_exec", "pcntl_fork", "proc_nice", "dl"} {
		if !strings.Contains(defaultDisableFunctions, fn) {
			t.Errorf("defaultDisableFunctions must include %q", fn)
		}
	}
	if !forbiddenDirectives["disable_functions"] {
		t.Error("disable_functions must stay in forbiddenDirectives (tenant override guard)")
	}
}
