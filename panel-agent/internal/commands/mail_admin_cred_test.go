package commands

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func withAdminCredFixtures(t *testing.T, envBody string) (tokenPath, envPath string) {
	t.Helper()
	dir := t.TempDir()
	tokenPath = filepath.Join(dir, "stalwart-admin.token")
	envPath = filepath.Join(dir, "stalwart.env")
	if err := os.WriteFile(tokenPath, []byte("OLDTOKEN==\n"), 0o640); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(envPath, []byte(envBody), 0o640); err != nil {
		t.Fatal(err)
	}
	oldTok, oldEnv := stalwartAdminTokenFile, stalwartEnvFile
	stalwartAdminTokenFile, stalwartEnvFile = tokenPath, envPath
	t.Cleanup(func() { stalwartAdminTokenFile, stalwartEnvFile = oldTok, oldEnv })
	return tokenPath, envPath
}

func stubRestarts(t *testing.T) *[]string {
	t.Helper()
	var calls []string
	oldR, oldS, oldV := restartServiceFunc, scheduleRestartFunc, verifyStalwartAuthFunc
	restartServiceFunc = func(_ context.Context, unit string) error { calls = append(calls, "restart:"+unit); return nil }
	scheduleRestartFunc = func(_ context.Context, unit string) error { calls = append(calls, "schedule:"+unit); return nil }
	verifyStalwartAuthFunc = func(_ context.Context, _, _ string) bool { return true }
	t.Cleanup(func() { restartServiceFunc, scheduleRestartFunc, verifyStalwartAuthFunc = oldR, oldS, oldV })
	return &calls
}

func TestMailAdminCred_Reveal(t *testing.T) {
	withAdminCredFixtures(t, "STALWART_RECOVERY_ADMIN=admin:OLDTOKEN==\n")
	stubRestarts(t)
	raw, err := mailAdminCredHandler(context.Background(), json.RawMessage(`{"action":"reveal"}`))
	if err != nil {
		t.Fatalf("reveal: %v", err)
	}
	r := raw.(adminCredResponse)
	if r.User != "admin" || r.Password != "OLDTOKEN==" || r.Rotated {
		t.Fatalf("unexpected reveal: %+v", r)
	}
}

func TestMailAdminCred_Rotate(t *testing.T) {
	tokenPath, envPath := withAdminCredFixtures(t, "# header\nSTALWART_RECOVERY_ADMIN=admin:OLDTOKEN==\nOTHER=keep\n")
	calls := stubRestarts(t)

	raw, err := mailAdminCredHandler(context.Background(), json.RawMessage(`{"action":"rotate"}`))
	if err != nil {
		t.Fatalf("rotate: %v", err)
	}
	r := raw.(adminCredResponse)
	if r.User != "admin" || !r.Rotated || r.Password == "" || r.Password == "OLDTOKEN==" {
		t.Fatalf("rotate did not mint a new token: %+v", r)
	}

	// token file updated to the new value
	gotTok, _ := os.ReadFile(tokenPath)
	if strings.TrimSpace(string(gotTok)) != r.Password {
		t.Fatalf("token file %q != response %q", strings.TrimSpace(string(gotTok)), r.Password)
	}
	// env line rewritten, other lines preserved
	gotEnv, _ := os.ReadFile(envPath)
	wantLine := "STALWART_RECOVERY_ADMIN=admin:" + r.Password
	if !strings.Contains(string(gotEnv), wantLine) {
		t.Fatalf("env missing %q:\n%s", wantLine, gotEnv)
	}
	for _, keep := range []string{"# header", "OTHER=keep"} {
		if !strings.Contains(string(gotEnv), keep) {
			t.Fatalf("env dropped %q:\n%s", keep, gotEnv)
		}
	}
	// stalwart restarted synchronously; panel-api scheduled (delayed)
	joined := strings.Join(*calls, ",")
	if !strings.Contains(joined, "restart:jabali-stalwart") || !strings.Contains(joined, "schedule:jabali-panel") {
		t.Fatalf("restart calls wrong: %v", *calls)
	}
}

func TestMailAdminCred_RotateRollbackOnVerifyFail(t *testing.T) {
	envBody := "STALWART_RECOVERY_ADMIN=admin:OLDTOKEN==\n"
	tokenPath, envPath := withAdminCredFixtures(t, envBody)
	calls := stubRestarts(t)
	// verify fails -> handler must revert env + leave token file untouched.
	verifyStalwartAuthFunc = func(_ context.Context, _, _ string) bool { return false }

	if _, err := mailAdminCredHandler(context.Background(), json.RawMessage(`{"action":"rotate"}`)); err == nil {
		t.Fatal("expected error when rotated password fails to authenticate")
	}
	// token file (panel-agent's per-call source) must still hold the OLD token
	gotTok, _ := os.ReadFile(tokenPath)
	if strings.TrimSpace(string(gotTok)) != "OLDTOKEN==" {
		t.Fatalf("token file mutated despite verify failure: %q", strings.TrimSpace(string(gotTok)))
	}
	// env must be restored to the original
	gotEnv, _ := os.ReadFile(envPath)
	if string(gotEnv) != envBody {
		t.Fatalf("env not rolled back:\n%s", gotEnv)
	}
	// stalwart restarted twice (apply + rollback); panel-api never scheduled
	joined := strings.Join(*calls, ",")
	if strings.Count(joined, "restart:jabali-stalwart") != 2 || strings.Contains(joined, "schedule:jabali-panel") {
		t.Fatalf("unexpected restart sequence: %v", *calls)
	}
}

func TestMailAdminCred_BadAction(t *testing.T) {
	withAdminCredFixtures(t, "STALWART_RECOVERY_ADMIN=admin:OLDTOKEN==\n")
	stubRestarts(t)
	if _, err := mailAdminCredHandler(context.Background(), json.RawMessage(`{"action":"nope"}`)); err == nil {
		t.Fatal("expected error for unknown action")
	}
}
