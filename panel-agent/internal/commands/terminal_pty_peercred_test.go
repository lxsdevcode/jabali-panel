package commands

import (
	"net"
	"os"
	"path/filepath"
	"testing"
)

// TestTerminalPeerCred covers the PTY broker's SO_PEERCRED gate (Gitea #469):
// over a real unix socket the peer's uid/gid are read (ok=true) and match the
// running process, and a non-unix conn fails closed (ok=false) so the caller
// refuses the connection.
func TestTerminalPeerCred(t *testing.T) {
	sock := filepath.Join(t.TempDir(), "t.sock")
	ln, err := net.Listen("unix", sock)
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close()

	accepted := make(chan net.Conn, 1)
	go func() {
		c, aerr := ln.Accept()
		if aerr != nil {
			accepted <- nil
			return
		}
		accepted <- c
	}()

	client, err := net.Dial("unix", sock)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer client.Close()

	srv := <-accepted
	if srv == nil {
		t.Fatal("accept failed")
	}
	defer srv.Close()

	uid, gid, ok := terminalPeerCred(srv)
	if !ok {
		t.Fatal("expected to read peer credentials over a unix socket")
	}
	if uid != os.Getuid() {
		t.Errorf("peer uid = %d, want %d", uid, os.Getuid())
	}
	if gid != os.Getgid() {
		t.Errorf("peer gid = %d, want %d", gid, os.Getgid())
	}

	// Non-unix conn must fail closed.
	a, b := net.Pipe()
	defer a.Close()
	defer b.Close()
	if _, _, ok := terminalPeerCred(a); ok {
		t.Error("non-unix conn must fail closed (ok=false)")
	}
}
