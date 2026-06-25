package main

import (
	"errors"
	"strings"
	"testing"
)

func TestClassifyCanaryOutput(t *testing.T) {
	runErr := errors.New("exit status 125")
	t.Run("success proceeds", func(t *testing.T) {
		if classifyCanaryOutput("Hello from Docker!", nil) != nil {
			t.Fatal("nil runErr must proceed")
		}
	})
	t.Run("image fetch failure is inconclusive (proceed)", func(t *testing.T) {
		for _, o := range []string{
			"docker: Error response from daemon: manifest for hello-world not found",
			"pull access denied for hello-world",
			"dial tcp: lookup registry-1.docker.io: no such host",
		} {
			if err := classifyCanaryOutput(o, runErr); err != nil {
				t.Errorf("inconclusive output should proceed, got %v (for %q)", err, o)
			}
		}
	})
	t.Run("LXC runc/AppArmor sysctl failure blocks with #446 hint", func(t *testing.T) {
		o := "OCI runtime create failed: runc create failed: open sysctl net.ipv4.ip_unprivileged_port_start file: reopen fd 8: permission denied"
		err := classifyCanaryOutput(o, runErr)
		if err == nil || !strings.Contains(err.Error(), "#446") {
			t.Fatalf("want #446 diagnostic, got %v", err)
		}
	})
	t.Run("other start failure blocks generically", func(t *testing.T) {
		err := classifyCanaryOutput("some unexpected container init failure", runErr)
		if err == nil || strings.Contains(err.Error(), "#446") {
			t.Fatalf("want generic block, got %v", err)
		}
	})
}

func TestLastChars(t *testing.T) {
	if lastChars("abc", 10) != "abc" {
		t.Error("short string unchanged")
	}
	if got := lastChars("abcdefgh", 3); got != "…fgh" {
		t.Errorf("got %q", got)
	}
}
