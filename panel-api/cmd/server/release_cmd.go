package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"
)

// newReleaseCmd groups release-channel management (GH #445).
func newReleaseCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "release",
		Short: "Release-channel management (stable/development)",
	}
	c.AddCommand(newReleasePromoteCmd())
	return c
}

// newReleasePromoteCmd force-moves the movable `stable` tag to a reviewed
// commit and pushes it. Stable-channel hosts pick it up on their next
// `jabali update`. Run where there's push access to origin (maintainer/CI),
// not necessarily on a panel host.
func newReleasePromoteCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "promote [<commit-ish>]",
		Short: "Promote a reviewed build to the stable channel (move the `stable` tag)",
		Long: `Force-moves the ` + "`stable`" + ` git tag to <commit-ish> (default: the
current origin/main HEAD) and pushes it. Stable-channel hosts then update to
that reviewed build on their next ` + "`jabali update`" + `.

Run on a box with push access to origin (a maintainer machine or CI) — the
panel host is a read-only deployment target. The release workflow already
builds a release-<sha> tarball for every commit, so the promoted commit's
tarball is what stable hosts download.`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			repoDir := os.Getenv("JABALI_REPO_DIR")
			if repoDir == "" {
				repoDir = defaultRepoDir
			}
			git := func(args ...string) error {
				full := append([]string{"-C", repoDir}, args...)
				c := exec.Command("git", full...)
				c.Stdout, c.Stderr = os.Stdout, os.Stderr
				return c.Run()
			}
			target := "origin/main"
			if len(args) == 1 {
				target = args[0]
			}
			// Refresh main so the default target is the real current HEAD.
			_ = git("fetch", "origin", "main")
			shaOut, err := exec.Command("git", "-C", repoDir, "rev-parse", "--short", target+"^{commit}").Output()
			if err != nil {
				return fmt.Errorf("resolve %q: %w (is it a valid commit?)", target, err)
			}
			sha := strings.TrimSpace(string(shaOut))
			if err := git("tag", "-f", "stable", target); err != nil {
				return fmt.Errorf("move stable tag: %w", err)
			}
			if err := git("push", "-f", "origin", "refs/tags/stable"); err != nil {
				return fmt.Errorf("push stable tag (need push access to origin): %w", err)
			}
			fmt.Printf("\n✓ Promoted stable -> %s (%s). Stable-channel hosts will update to it on next `jabali update`.\n", sha, target)
			return nil
		},
	}
}
