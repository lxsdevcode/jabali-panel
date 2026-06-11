package commands

import (
	"errors"
	"strings"
	"testing"
)

func TestComposeFailMessage(t *testing.T) {
	out := "Pulling db ...\n" +
		" ⠿ Container jabali-app-owncloud-db  Error\n" +
		"dependency failed to start: container jabali-app-owncloud-db exited (1)\n"
	msg := composeFailMessage("up -d", out, errors.New("exit status 1"))
	if !strings.Contains(msg, "exit status 1") {
		t.Errorf("missing exit error: %q", msg)
	}
	// The real reason (the tail) must be surfaced, not just the bare exit.
	if !strings.Contains(msg, "dependency failed to start") {
		t.Errorf("compose tail not surfaced: %q", msg)
	}
}

func TestComposeFailMessage_EmptyOutput(t *testing.T) {
	msg := composeFailMessage("up -d", "\n\n   \n", errors.New("exit status 1"))
	if msg != "docker compose up -d failed: exit status 1" {
		t.Errorf("unexpected message for empty output: %q", msg)
	}
}

func TestLastNonEmptyLines(t *testing.T) {
	got := lastNonEmptyLines("a\n\nb\n c \n\n", 2)
	if got != "b\nc" {
		t.Errorf("got %q", got)
	}
}
