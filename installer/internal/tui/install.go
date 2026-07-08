package tui

import (
	"bufio"
	"io"
	"os"
	"os/exec"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// Streaming messages from the install goroutine.
type logLineMsg string
type installDoneMsg struct{ err error }

// startInstall spawns install.sh (with the selected JABALI_MODULES) and streams
// its combined stdout+stderr line-by-line onto `events`, then a final
// installDoneMsg. Returned as a tea.Cmd so it runs off the UI goroutine.
func startInstall(installSh string, extraEnv []string, dryRun bool, events chan tea.Msg) tea.Cmd {
	return func() tea.Msg {
		go func() {
			args := []string{installSh}
			if dryRun {
				args = append(args, "--dry-run")
			}
			cmd := exec.Command("bash", args...)
			cmd.Env = append(os.Environ(), extraEnv...)
			stdout, _ := cmd.StdoutPipe()
			stderr, _ := cmd.StderrPipe()
			if err := cmd.Start(); err != nil {
				events <- installDoneMsg{err: err}
				return
			}
			var wgReaders int
			done := make(chan struct{})
			pump := func(r io.Reader) {
				sc := bufio.NewScanner(r)
				sc.Buffer(make([]byte, 0, 64*1024), 1<<20)
				for sc.Scan() {
					events <- logLineMsg(sc.Text())
				}
				done <- struct{}{}
			}
			wgReaders = 2
			go pump(stdout)
			go pump(stderr)
			for i := 0; i < wgReaders; i++ {
				<-done
			}
			events <- installDoneMsg{err: cmd.Wait()}
		}()
		// Immediately hand back the first waiter so Update starts draining.
		return waitForEvent(events)()
	}
}

// waitForEvent blocks on the events channel and returns the next message; the
// model re-arms it after every non-terminal event so streaming continues.
func waitForEvent(events chan tea.Msg) tea.Cmd {
	return func() tea.Msg { return <-events }
}

// phaseFromLine extracts a human phase label from install.sh's own markers so
// the header can show "current step" without us re-listing every step. install.sh
// logs `[i] <msg>` (start) and `[✓] <msg>` (ok); we surface the latest `[i]`.
func phaseFromLine(line string) (string, bool) {
	s := stripANSI(line)
	if i := strings.Index(s, "[i] "); i >= 0 {
		return strings.TrimSpace(s[i+4:]), true
	}
	return "", false
}

// stripANSI removes the SGR color escapes install.sh emits, so the parsed phase
// + the log pane render cleanly inside the box.
func stripANSI(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] == 0x1b {
			for i < len(s) && s[i] != 'm' {
				i++
			}
			continue
		}
		b.WriteByte(s[i])
	}
	return b.String()
}
