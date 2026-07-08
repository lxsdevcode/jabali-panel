// Command jabali-installer is the Bubble Tea front-end to install.sh (M353 /
// GH #353, Option A). Interactively it collects a deploy profile + module
// selection, then runs install.sh with JABALI_MODULES=<keys> and streams the
// output into a progress pane. install.sh stays the engine.
//
// Non-interactive parity is mandatory: when JABALI_MODULES is already set, when
// there's no TTY (CI / cloud-init), or when --unattended is passed, the TUI is
// skipped and install.sh runs directly with the inherited environment.
package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"golang.org/x/term"

	"git.jabali-panel.com/shukivaknin/jabali2/installer/internal/tui"
)

func main() {
	installSh := findInstallSh()
	dryRun := hasFlag("--dry-run")
	unattended := hasFlag("--unattended") || hasFlag("-y")
	_, modulesPreset := os.LookupEnv("JABALI_MODULES")
	interactive := term.IsTerminal(int(os.Stdin.Fd())) && !unattended && !modulesPreset

	if !interactive {
		// Headless / preset / unattended: run install.sh directly with the
		// current environment (JABALI_MODULES passes through), inherited stdio.
		runDirect(installSh, dryRun)
		return
	}

	if installSh == "" {
		fmt.Fprintln(os.Stderr, "install.sh not found (pass --install-sh <path> or run from the repo root)")
		os.Exit(1)
	}
	final, err := tea.NewProgram(tui.New(installSh, dryRun)).Run()
	if err != nil {
		fmt.Fprintf(os.Stderr, "installer TUI error: %v\n", err)
		os.Exit(1)
	}
	fm := final.(tui.Model)
	if fm.Aborted {
		fmt.Fprintln(os.Stderr, "install cancelled.")
	}
	os.Exit(fm.ExitCode)
}

// runDirect execs install.sh with inherited stdio (non-interactive path).
func runDirect(installSh string, dryRun bool) {
	if installSh == "" {
		fmt.Fprintln(os.Stderr, "install.sh not found (pass --install-sh <path> or run from the repo root)")
		os.Exit(1)
	}
	args := []string{installSh}
	if dryRun {
		args = append(args, "--dry-run")
	}
	cmd := exec.Command("bash", args...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	cmd.Env = os.Environ()
	if err := cmd.Run(); err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			os.Exit(ee.ExitCode())
		}
		fmt.Fprintf(os.Stderr, "install.sh failed: %v\n", err)
		os.Exit(1)
	}
}

func findInstallSh() string {
	if v := flagValue("--install-sh"); v != "" {
		return v
	}
	if v := os.Getenv("JABALI_INSTALL_SH"); v != "" {
		return v
	}
	if _, err := os.Stat("install.sh"); err == nil {
		return "install.sh"
	}
	return ""
}

func hasFlag(f string) bool {
	for _, a := range os.Args[1:] {
		if a == f {
			return true
		}
	}
	return false
}

func flagValue(f string) string {
	rest := os.Args[1:]
	for i, a := range rest {
		if a == f && i+1 < len(rest) {
			return rest[i+1]
		}
		if strings.HasPrefix(a, f+"=") {
			return strings.TrimPrefix(a, f+"=")
		}
	}
	return ""
}
