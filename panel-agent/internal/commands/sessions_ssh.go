package commands

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"syscall"
)

// sessions_ssh.go — GH #338 (SSH channel). Enumerate live SSH sessions (peer IP
// + the sshd process) for the admin Active Sessions view, and revoke one by
// terminating its sshd process. SECURITY: revoke ONLY ever signals a process
// whose comm is sshd/sshd-session — never an arbitrary PID.

var sshPeerRe = regexp.MustCompile(`([0-9a-fA-F:.]+):\d+\s+users:\(\("(sshd[a-z-]*)",pid=(\d+)`)

type sshSession struct {
	ID       string `json:"id"`   // "ssh:<pid>"
	User     string `json:"user"`
	RemoteIP string `json:"remote_ip"`
	Since    string `json:"since"`
	PID      int    `json:"pid"`
}

// sessionsSSHListHandler lists established inbound SSH connections.
func sessionsSSHListHandler(ctx context.Context, _ json.RawMessage) (any, error) {
	// `ss` established connections on the local ssh port with the owning process.
	out, err := exec.CommandContext(ctx, "ss", "-tnHp", "state", "established", "( sport = :22 )").Output()
	if err != nil {
		// No ss / no sessions — return empty, not an error.
		return map[string]any{"sessions": []sshSession{}}, nil
	}
	seen := map[int]bool{}
	sessions := []sshSession{}
	for _, line := range strings.Split(string(out), "\n") {
		m := sshPeerRe.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		pid, perr := strconv.Atoi(m[3])
		if perr != nil || seen[pid] || !isSSHDProcess(pid) {
			continue
		}
		seen[pid] = true
		user, since := procUserAndStart(ctx, pid)
		sessions = append(sessions, sshSession{
			ID: "ssh:" + m[3], User: user, RemoteIP: m[1], Since: since, PID: pid,
		})
	}
	return map[string]any{"sessions": sessions}, nil
}

type sessionsSSHRevokeParams struct {
	PID int `json:"pid"`
}

// sessionsSSHRevokeHandler terminates one SSH session's sshd process. Refuses
// to signal any PID whose comm is not an sshd process.
func sessionsSSHRevokeHandler(ctx context.Context, raw json.RawMessage) (any, error) {
	var p sessionsSSHRevokeParams
	if err := json.Unmarshal(raw, &p); err != nil {
		return nil, csInvalidArg("bad params")
	}
	if p.PID < 2 {
		return nil, csInvalidArg("invalid pid")
	}
	if !isSSHDProcess(p.PID) {
		// Never kill a non-sshd PID (the PID may have been recycled).
		return nil, csInvalidArg("pid is not an sshd session")
	}
	if err := syscall.Kill(p.PID, syscall.SIGHUP); err != nil {
		return nil, csInternal("terminate ssh session", err)
	}
	return map[string]any{"ok": true}, nil
}

// isSSHDProcess reports whether pid's comm is an sshd process (sshd, sshd-session).
func isSSHDProcess(pid int) bool {
	b, err := os.ReadFile("/proc/" + strconv.Itoa(pid) + "/comm")
	if err != nil {
		return false
	}
	comm := strings.TrimSpace(string(b))
	return comm == "sshd" || comm == "sshd-session" || strings.HasPrefix(comm, "sshd")
}

// procUserAndStart returns the process owner + its start time (best-effort).
func procUserAndStart(ctx context.Context, pid int) (user, since string) {
	if out, err := exec.CommandContext(ctx, "ps", "-o", "user=", "-p", strconv.Itoa(pid)).Output(); err == nil {
		user = strings.TrimSpace(string(out))
	}
	if out, err := exec.CommandContext(ctx, "ps", "-o", "lstart=", "-p", strconv.Itoa(pid)).Output(); err == nil {
		since = strings.TrimSpace(string(out))
	}
	return user, since
}

func init() {
	Default.Register("sessions.ssh.list", sessionsSSHListHandler)
	Default.Register("sessions.ssh.revoke", sessionsSSHRevokeHandler)
}
