package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"runtime/debug"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"git.linux-hosting.co.il/shukivaknin/jabali2/panel-api/internal/api"
)

// newVersionCmd implements `jabali version`. Replaces cobra's bare
// `--version` (which only prints api.Version) with a full report:
// version, commit, build time, Go runtime, OS/arch, binary path, and
// — when accessible — the on-host repo HEAD + last-built-sha file
// (the two artefacts `jabali update` reconciles).
//
// Output formats:
//
//	jabali version            human-readable key-value block
//	jabali version --json     compact JSON for scripts
//
// Build-info source: link-time ldflags. Install.sh + `jabali update`
// pass:
//
//	-X .../api.Version=<short-sha>
//	-X .../api.Commit=<full-sha>
//	-X .../api.BuildTime=<rfc3339>
//
// On a binary built without those (`go build` from a dev checkout),
// the fields fall back to "dev" / "unknown" and we fill what we can
// from `debug.ReadBuildInfo()` (Go's runtime build metadata) so the
// caller still sees enough to file a bug.
func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print Jabali version, commit, and runtime info",
		Long: `Show full version + build info for this jabali-panel binary.

Reports:
  version          link-time -X api.Version (short SHA or tag)
  commit           link-time -X api.Commit (full SHA)
  build_time       link-time -X api.BuildTime (RFC3339)
  go_version       compile-time Go toolchain (runtime.Version)
  os / arch        compile-time GOOS / GOARCH
  binary           os.Executable() of this process
  vcs.revision     fallback to debug.ReadBuildInfo when ldflags absent
  repo_head        git rev-parse HEAD in /opt/jabali-panel (when readable)
  last_built_sha   /var/lib/jabali-panel/last-built-sha (set by jabali update)

Use --json for machine-readable output.`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			info := collectVersionInfo()

			if jsonOutput {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(info)
			}

			// Deterministic order so the output reads top-down from
			// most-important to forensic detail.
			order := []string{
				"version", "commit", "build_time",
				"go_version", "os", "arch",
				"binary", "vcs_revision", "vcs_time", "vcs_modified",
				"repo_head", "last_built_sha",
			}
			seen := map[string]bool{}
			for _, k := range order {
				v, ok := info[k]
				if !ok || v == "" {
					continue
				}
				fmt.Fprintf(cmd.OutOrStdout(), "%-15s %s\n", k+":", v)
				seen[k] = true
			}
			// Surface any unexpected keys at the bottom so future
			// additions show up without code change.
			extras := make([]string, 0)
			for k := range info {
				if !seen[k] {
					extras = append(extras, k)
				}
			}
			sort.Strings(extras)
			for _, k := range extras {
				fmt.Fprintf(cmd.OutOrStdout(), "%-15s %s\n", k+":", info[k])
			}
			return nil
		},
	}
}

// collectVersionInfo gathers every field, omitting entries that come
// back empty so the printer can skip them without nil checks.
func collectVersionInfo() map[string]string {
	out := map[string]string{
		"version":    api.Version,
		"commit":     api.Commit,
		"build_time": api.BuildTime,
		"go_version": runtime.Version(),
		"os":         runtime.GOOS,
		"arch":       runtime.GOARCH,
	}
	if exe, err := os.Executable(); err == nil {
		out["binary"] = exe
	}
	// debug.ReadBuildInfo carries Go-toolchain VCS metadata when the
	// build was a `go build` from a clean checkout (Go 1.18+). Useful
	// as a fallback when ldflags weren't passed (untagged dev build).
	if bi, ok := debug.ReadBuildInfo(); ok {
		for _, s := range bi.Settings {
			switch s.Key {
			case "vcs.revision":
				out["vcs_revision"] = s.Value
			case "vcs.time":
				out["vcs_time"] = s.Value
			case "vcs.modified":
				out["vcs_modified"] = s.Value
			}
		}
	}
	// Best-effort: HEAD of the deployed repo. Missing repo / not a
	// git checkout / git not in PATH all return silently.
	if sha := readDeployedRepoHead(); sha != "" {
		out["repo_head"] = sha
	}
	if b, err := os.ReadFile(lastBuiltSHAPath); err == nil {
		if s := strings.TrimSpace(string(b)); s != "" {
			out["last_built_sha"] = s
		}
	}
	return out
}

// readDeployedRepoHead asks the on-host /opt/jabali-panel checkout
// what HEAD points at. Runs git as `sudo -u <service-user>` because
// the repo is owned by the jabali service account (git 2.35+ refuses
// to run on a uid-mismatched repo). Returns "" on any failure —
// jabali version is a diagnostic, never a hard error.
func readDeployedRepoHead() string {
	repo := os.Getenv("JABALI_REPO_DIR")
	if repo == "" {
		repo = defaultRepoDir
	}
	if _, err := os.Stat(repo + "/.git"); err != nil {
		return ""
	}
	serviceUser := os.Getenv("JABALI_SERVICE_USER")
	if serviceUser == "" {
		serviceUser = "jabali"
	}
	out, err := exec.Command("sudo", "-u", serviceUser,
		"git", "-C", repo, "rev-parse", "HEAD").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}
