package commands

import (
	"os"
	"strings"
	"testing"
)

// Regression for 2026-06-04: the bouncer-mode verb used `systemctl
// reload nginx`, but nginx-lua's `init_by_lua_block` (where the
// crowdsec module reads MODE into runtime.conf) only runs at master
// (re)start. Reload re-forks workers from the existing master, so the
// Lua `runtime.conf["MODE"]` value is preserved — the bouncer keeps
// running in the OLD mode even though the conf file on disk has the
// new MODE=.
//
// We don't have a test harness around exec.CommandContext here, so
// this is a static source-level guard: the handler MUST issue a
// "restart" (not "reload") command for nginx after writing the
// bouncer conf.
func TestBouncerMode_HandlerUsesRestartNotReload(t *testing.T) {
	src, err := os.ReadFile("security_crowdsec_bouncer_mode.go")
	if err != nil {
		t.Fatalf("read source: %v", err)
	}
	body := string(src)

	if strings.Contains(body, `"systemctl", "reload", "nginx"`) {
		t.Error("bouncer-mode handler must NOT use `systemctl reload nginx` — reload does not re-execute init_by_lua_block, so the crowdsec bouncer keeps the old MODE value in runtime.conf. Use restart.")
	}
	if !strings.Contains(body, `"systemctl", "restart", "nginx"`) {
		t.Error("bouncer-mode handler must use `systemctl restart nginx` to force init_by_lua re-evaluation against the updated bouncer conf")
	}
}
