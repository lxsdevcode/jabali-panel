package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
)

// M49 Phase 2 (ADR-0117, GH #170). `jabali docker enable-tenant` turns a host
// into a tenant-docker host: enable daemon-wide userns-remap (so a container-
// root escape lands as the unprivileged dockremap subuid, never host root),
// retrofit existing admin-app data ownership into the remap range, and only on
// full success write the gate flag that lets max_docker_apps>0 take effect.
//
// The dangerous, irreversible-ish step (daemon-wide remap) is wrapped in a
// down -> remap -> chown -> up -> health sequence with the flag written LAST,
// so a host that fails halfway stays "tenant docker off" rather than
// half-migrated. Proven manually on mx 2026-06-21.

const (
	dockerDaemonJSON     = "/etc/docker/daemon.json"
	dockerTenantFlagPath = "/etc/jabali/docker-tenant-enabled"
	dockerSubuidPath     = "/etc/subuid"
	dockerAppsRoot       = "/var/lib/jabali/docker-apps"
	dockremapUser        = "dockremap"
)

func newDockerCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "docker",
		Short: "Manage the docker engine + app-marketplace host (M48/M49)",
	}
	// Single `docker` tree. Previously `newDockerEngineCmd()` registered a
	// SECOND root command with the same Use:"docker", so cobra dispatched only
	// one and the engine subcommands (enable/disable/status) were unreachable
	// (gap-audit 2026-06-22). Fold them in here.
	cmd.AddCommand(
		newDockerEngineActionCmd("enable", "Install docker engine + flip Server Settings toggle"),
		newDockerEngineActionCmd("disable", "Disable the marketplace toggle (keeps docker installed)"),
		newDockerEngineStatusCmd(),
		newDockerEnableTenantCmd(),
	)
	return cmd
}

func newDockerEnableTenantCmd() *cobra.Command {
	var yes bool
	cmd := &cobra.Command{
		Use:   "enable-tenant",
		Short: "Enable userns-remap + tenant docker apps on this host (M49, GH #170)",
		Long: `Enables tenant-level Docker apps on this host:

  1. enable daemon-wide userns-remap in ` + dockerDaemonJSON + `
  2. restart dockerd
  3. chown every installed app's data tree into the dockremap subuid range
  4. bring every app back up and health-check it
  5. write ` + dockerTenantFlagPath + ` (the flag that ungates max_docker_apps)

If any app fails to come back up, the flag is NOT written: the host stays
"tenant docker off". userns-remap is daemon-wide and changes docker's storage
root — existing pulled images become invisible until re-pulled.`,
		RunE: func(_ *cobra.Command, _ []string) error {
			return runDockerEnableTenant(yes)
		},
	}
	cmd.Flags().BoolVar(&yes, "yes", false, "proceed without the interactive confirmation")
	return cmd
}

// mergeUsernsRemap adds "userns-remap":"default" to a daemon.json document if
// absent. Returns the re-serialised JSON and whether a change was made. Pure
// (operates on bytes) so it is unit-tested without touching /etc/docker.
func mergeUsernsRemap(daemonJSON []byte) ([]byte, bool, error) {
	doc := map[string]any{}
	if len(strings.TrimSpace(string(daemonJSON))) > 0 {
		if err := json.Unmarshal(daemonJSON, &doc); err != nil {
			return nil, false, fmt.Errorf("parse daemon.json: %w", err)
		}
	}
	if v, ok := doc["userns-remap"]; ok && v == "default" {
		out, _ := json.MarshalIndent(doc, "", "  ")
		return out, false, nil
	}
	doc["userns-remap"] = "default"
	out, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return nil, false, err
	}
	return append(out, '\n'), true, nil
}

// parseDockremapBase reads the base subuid for the dockremap user out of an
// /etc/subuid body (lines "name:base:count"). The chown retrofit shifts app
// data into <base>:<base>. Pure for testing.
func parseDockremapBase(subuidBody string) (int, error) {
	sc := bufio.NewScanner(strings.NewReader(subuidBody))
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.Split(line, ":")
		if len(parts) >= 2 && parts[0] == dockremapUser {
			base, err := strconv.Atoi(parts[1])
			if err != nil {
				return 0, fmt.Errorf("dockremap subuid base %q not an int: %w", parts[1], err)
			}
			return base, nil
		}
	}
	return 0, fmt.Errorf("no %s entry in subuid", dockremapUser)
}

// listInstalledAppDirs returns the per-app data dirs under dockerAppsRoot (each
// holds a compose.yml). Used to down/chown/up every app during the retrofit.
func listInstalledAppDirs(root string) ([]string, error) {
	ents, err := os.ReadDir(root)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var dirs []string
	for _, e := range ents {
		if !e.IsDir() {
			continue
		}
		d := filepath.Join(root, e.Name())
		if _, err := os.Stat(filepath.Join(d, "compose.yml")); err == nil {
			dirs = append(dirs, d)
		}
	}
	return dirs, nil
}

func runDockerEnableTenant(yes bool) error {
	if os.Geteuid() != 0 {
		return fmt.Errorf("must run as root")
	}
	if _, err := os.Stat(dockerTenantFlagPath); err == nil {
		fmt.Println("tenant docker already enabled (" + dockerTenantFlagPath + " exists)")
		return nil
	}
	apps, err := listInstalledAppDirs(dockerAppsRoot)
	if err != nil {
		return fmt.Errorf("scan installed apps: %w", err)
	}
	if !yes {
		fmt.Printf("This enables daemon-wide userns-remap and retrofits %d installed app(s).\n", len(apps))
		fmt.Printf("Existing pulled images become invisible until re-pulled. Re-run with --yes to proceed.\n")
		return nil
	}

	// 1. down every app.
	for _, d := range apps {
		_ = exec.Command("docker", "compose", "-f", filepath.Join(d, "compose.yml"), "down").Run()
	}

	// 2. enable remap in daemon.json + restart dockerd.
	cur, _ := os.ReadFile(dockerDaemonJSON)
	merged, changed, err := mergeUsernsRemap(cur)
	if err != nil {
		return err
	}
	if changed {
		if err := os.WriteFile(dockerDaemonJSON, merged, 0o644); err != nil {
			return fmt.Errorf("write daemon.json: %w", err)
		}
	}
	if out, err := exec.Command("systemctl", "restart", "docker").CombinedOutput(); err != nil {
		return fmt.Errorf("restart docker: %v: %s", err, out)
	}

	// 3. chown app data trees into the dockremap range.
	subuid, _ := os.ReadFile(dockerSubuidPath)
	base, err := parseDockremapBase(string(subuid))
	if err != nil {
		return fmt.Errorf("resolve dockremap base (remap not active?): %w", err)
	}
	for _, d := range apps {
		if out, err := exec.Command("chown", "-R", fmt.Sprintf("%d:%d", base, base), d).CombinedOutput(); err != nil {
			return fmt.Errorf("chown %s into remap range: %v: %s", d, err, out)
		}
	}

	// 4. bring every app back up + health.
	for _, d := range apps {
		if out, err := exec.Command("docker", "compose", "-f", filepath.Join(d, "compose.yml"), "up", "-d").CombinedOutput(); err != nil {
			return fmt.Errorf("app %s failed to come back up post-remap (flag NOT written): %v: %s", filepath.Base(d), err, out)
		}
	}

	// 5. success: write the gate flag.
	if err := os.MkdirAll(filepath.Dir(dockerTenantFlagPath), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(dockerTenantFlagPath, []byte("M49 tenant docker enabled\n"), 0o644); err != nil {
		return fmt.Errorf("write tenant flag: %w", err)
	}
	fmt.Printf("tenant docker enabled: userns-remap active, %d app(s) retrofitted, %s written\n", len(apps), dockerTenantFlagPath)
	return nil
}
