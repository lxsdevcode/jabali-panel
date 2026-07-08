// Command jabali-installer is the Bubble Tea front-end to install.sh (M353 /
// GH #353, Option A). It collects a deploy profile + optional-module selection
// in a TUI, then execs install.sh with JABALI_MODULES=<keys> so the proven bash
// engine does the actual install.
//
// Non-interactive parity is mandatory: when JABALI_MODULES is already set, when
// there's no TTY (CI / cloud-init), or when --unattended is passed, the TUI is
// skipped and install.sh runs directly with the inherited environment. This
// keeps every automated deploy working exactly as before.
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
		// current environment (JABALI_MODULES, if any, passes through).
		runInstall(installSh, nil, dryRun)
		return
	}

	m := tui.New()
	p := tea.NewProgram(m)
	final, err := p.Run()
	if err != nil {
		fmt.Fprintf(os.Stderr, "installer TUI error: %v\n", err)
		os.Exit(1)
	}
	fm := final.(tui.Model)
	if fm.Aborted || !fm.Confirmed {
		fmt.Fprintln(os.Stderr, "install cancelled.")
		os.Exit(130)
	}
	keys := fm.SelectedKeys()
	runInstall(installSh, []string{"JABALI_MODULES=" + strings.Join(keys, ",")}, dryRun)
}

// runInstall execs install.sh with the current env plus extraEnv, streaming its
// stdio straight through (Phase T1: inherited stdio; the scrolling progress
// pane is Phase T2).
func runInstall(installSh string, extraEnv []string, dryRun bool) {
	if installSh == "" {
		fmt.Fprintln(os.Stderr, "install.sh not found (pass --install-sh <path> or run from the repo root)")
		os.Exit(1)
	}
	args := []string{installSh}
	if dryRun {
		args = append(args, "--dry-run")
	}
	cmd := exec.Command("bash", args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = append(os.Environ(), extraEnv...)
	if err := cmd.Run(); err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			os.Exit(ee.ExitCode())
		}
		fmt.Fprintf(os.Stderr, "install.sh failed: %v\n", err)
		os.Exit(1)
	}
}

// findInstallSh resolves install.sh from --install-sh, $JABALI_INSTALL_SH, or
// the current directory.
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
	for i, a := range os.Args[1:] {
		if a == f && i+1 < len(os.Args[1:]) {
			return os.Args[1:][i+1]
		}
		if strings.HasPrefix(a, f+"=") {
			return strings.TrimPrefix(a, f+"=")
		}
	}
	return ""
}
