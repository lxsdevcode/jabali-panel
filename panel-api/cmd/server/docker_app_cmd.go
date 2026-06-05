package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"

	"git.linux-hosting.co.il/shukivaknin/jabali2/panel-api/internal/dockerapp"
	"git.linux-hosting.co.il/shukivaknin/jabali2/panel-api/internal/repository"
)

func dockerAppRepoFromDB() repository.DockerAppRepository {
	return repository.NewDockerAppRepository(sharedDB)
}

func loadDockerCatalogForCLI() (*dockerapp.Catalog, error) {
	cat, errs := dockerapp.LoadDir("/usr/local/share/jabali/docker-apps")
	if cat.Len() == 0 {
		if dev, _ := dockerapp.LoadDir("install/docker-apps"); dev.Len() > 0 {
			cat = dev
		}
	}
	if cat.Len() == 0 {
		if len(errs) > 0 {
			return nil, fmt.Errorf("catalog empty (first error: %s)", errs[0].Error())
		}
		return nil, errors.New("catalog empty: /usr/local/share/jabali/docker-apps unreadable")
	}
	return cat, nil
}

func newDockerAppCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "docker-app",
		Short: "Manage M48 docker-app catalog installs (admin-only)",
	}
	cmd.AddCommand(
		newDockerAppCatalogCmd(),
		newDockerAppListCmd(),
		newDockerAppStatusCmd(),
		newDockerAppLifecycleCmd("start", "Start a stopped install"),
		newDockerAppLifecycleCmd("stop", "Stop a running install"),
		newDockerAppLifecycleCmd("restart", "Restart an install"),
		newDockerAppLifecycleCmd("rebuild", "Force-recreate (docker compose up --force-recreate)"),
		newDockerAppDeleteCmd(),
		newDockerAppLogsCmd(),
		newDockerAppUpdateCmd(),
		newDockerAppBackupsCmd(),
	)
	return cmd
}

func newDockerAppCatalogCmd() *cobra.Command {
	var jsonOut bool
	cmd := &cobra.Command{
		Use:   "catalog",
		Short: "List entries in the installed catalog",
		RunE: func(cmd *cobra.Command, args []string) error {
			cat, err := loadDockerCatalogForCLI()
			if err != nil {
				return err
			}
			entries := cat.All()
			if jsonOut {
				return json.NewEncoder(os.Stdout).Encode(entries)
			}
			tw := tabwriter.NewWriter(os.Stdout, 0, 2, 2, ' ', 0)
			fmt.Fprintln(tw, "SLUG\tNAME\tVERSION\tIMAGE")
			for _, e := range entries {
				fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", e.Slug, e.Name, e.Version, e.ImageChannel)
			}
			return tw.Flush()
		},
	}
	cmd.Flags().BoolVar(&jsonOut, "json", false, "emit JSON instead of a table")
	return cmd
}

func newDockerAppListCmd() *cobra.Command {
	var jsonOut bool
	cmd := &cobra.Command{
		Use:     "list",
		Short:   "List installed docker apps",
		PreRunE: requireDB,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := context.WithTimeout(cmd.Context(), 10*time.Second)
			defer cancel()
			repo := dockerAppRepoFromDB()
			apps, err := repo.ListAll(ctx)
			if err != nil {
				return err
			}
			if jsonOut {
				return json.NewEncoder(os.Stdout).Encode(apps)
			}
			tw := tabwriter.NewWriter(os.Stdout, 0, 2, 2, ' ', 0)
			fmt.Fprintln(tw, "ID\tSLUG\tNAME\tSTATUS\tUPDATE_MODE\tLAST_ERROR")
			for _, a := range apps {
				le := ""
				if a.LastError != nil {
					le = *a.LastError
				}
				fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\n",
					a.ID, a.Slug, a.Name, a.Status, a.UpdateMode, firstLine(le))
			}
			return tw.Flush()
		},
	}
	cmd.Flags().BoolVar(&jsonOut, "json", false, "emit JSON instead of a table")
	return cmd
}

func newDockerAppStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "status <id>",
		Short:   "Show full status of an installed app (DB row + agent status)",
		Args:    cobra.ExactArgs(1),
		PreRunE: requireDBAndAgent,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := context.WithTimeout(cmd.Context(), 30*time.Second)
			defer cancel()
			repo := dockerAppRepoFromDB()
			app, err := repo.FindByID(ctx, args[0])
			if err != nil {
				return err
			}
			ports, _ := repo.ListPortsForApp(ctx, app.ID)
			out := map[string]any{
				"app":   app,
				"ports": ports,
			}
			if sharedAgent != nil {
				raw, agerr := sharedAgent.Call(ctx, "docker_app.status", map[string]any{"slug": app.Slug})
				if agerr != nil {
					out["agent_error"] = agerr.Error()
				} else {
					var agentStatus json.RawMessage = raw
					out["agent"] = agentStatus
				}
			}
			return json.NewEncoder(os.Stdout).Encode(out)
		},
	}
}

func newDockerAppLifecycleCmd(verb, short string) *cobra.Command {
	return &cobra.Command{
		Use:     verb + " <id>",
		Short:   short,
		Args:    cobra.ExactArgs(1),
		PreRunE: requireDBAndAgent,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := context.WithTimeout(cmd.Context(), 90*time.Second)
			defer cancel()
			repo := dockerAppRepoFromDB()
			app, err := repo.FindByID(ctx, args[0])
			if err != nil {
				return err
			}
			raw, err := sharedAgent.Call(ctx, "docker_app."+verb, map[string]any{"slug": app.Slug})
			if err != nil {
				return err
			}
			fmt.Printf("ok: %s -> %s\n", verb, app.Slug)
			os.Stdout.Write(raw)
			os.Stdout.Write([]byte{'\n'})
			return nil
		},
	}
}

func newDockerAppDeleteCmd() *cobra.Command {
	var keepVolumes bool
	cmd := &cobra.Command{
		Use:     "delete <id>",
		Short:   "Uninstall a docker app (stops the stack, removes its row)",
		Args:    cobra.ExactArgs(1),
		PreRunE: requireDBAndAgent,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := context.WithTimeout(cmd.Context(), 2*time.Minute)
			defer cancel()
			repo := dockerAppRepoFromDB()
			app, err := repo.FindByID(ctx, args[0])
			if err != nil {
				return err
			}
			if _, err := sharedAgent.Call(ctx, "docker_app.delete", map[string]any{
				"slug":          app.Slug,
				"purge_volumes": !keepVolumes,
			}); err != nil {
				return err
			}
			// Cleanup managed domain row + ports + the docker_apps row.
			domRepo := repository.NewDomainRepository(sharedDB)
			domList, _, _ := domRepo.List(ctx, repository.ListOptions{})
			for _, d := range domList {
				if d.DockerAppID != nil && *d.DockerAppID == app.ID {
					_ = domRepo.Delete(ctx, d.ID)
				}
			}
			ports, _ := repo.ListPortsForApp(ctx, app.ID)
			for _, p := range ports {
				_ = repo.DeletePort(ctx, p.ID)
			}
			if err := repo.Delete(ctx, app.ID); err != nil {
				return err
			}
			fmt.Println("ok: deleted", app.Slug, "("+app.ID+")")
			return nil
		},
	}
	cmd.Flags().BoolVar(&keepVolumes, "keep-volumes", false, "keep /var/lib/jabali/docker-apps/<slug> data on disk")
	return cmd
}

func newDockerAppLogsCmd() *cobra.Command {
	var lines int
	var service string
	cmd := &cobra.Command{
		Use:     "logs <id>",
		Short:   "Tail container logs",
		Args:    cobra.ExactArgs(1),
		PreRunE: requireDBAndAgent,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := context.WithTimeout(cmd.Context(), 30*time.Second)
			defer cancel()
			repo := dockerAppRepoFromDB()
			app, err := repo.FindByID(ctx, args[0])
			if err != nil {
				return err
			}
			params := map[string]any{"slug": app.Slug}
			if lines > 0 {
				params["lines"] = lines
			}
			if service != "" {
				params["service"] = service
			}
			raw, err := sharedAgent.Call(ctx, "docker_app.logs", params)
			if err != nil {
				return err
			}
			os.Stdout.Write(raw)
			os.Stdout.Write([]byte{'\n'})
			return nil
		},
	}
	cmd.Flags().IntVar(&lines, "lines", 200, "lines to tail")
	cmd.Flags().StringVar(&service, "service", "", "compose service name (default: first service)")
	return cmd
}

func newDockerAppUpdateCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "update <id>",
		Short:   "Pull the latest image and re-create the stack",
		Args:    cobra.ExactArgs(1),
		PreRunE: requireDBAndAgent,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := context.WithTimeout(cmd.Context(), 5*time.Minute)
			defer cancel()
			repo := dockerAppRepoFromDB()
			app, err := repo.FindByID(ctx, args[0])
			if err != nil {
				return err
			}
			raw, err := sharedAgent.Call(ctx, "docker_app.update", map[string]any{"slug": app.Slug})
			if err != nil {
				return err
			}
			os.Stdout.Write(raw)
			os.Stdout.Write([]byte{'\n'})
			return nil
		},
	}
}

func newDockerAppBackupsCmd() *cobra.Command {
	var jsonOut bool
	cmd := &cobra.Command{
		Use:     "backups <id>",
		Short:   "List restic backups taken for this install",
		Args:    cobra.ExactArgs(1),
		PreRunE: requireDB,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := context.WithTimeout(cmd.Context(), 10*time.Second)
			defer cancel()
			repo := dockerAppRepoFromDB()
			rows, err := repo.ListBackupsForApp(ctx, args[0])
			if err != nil {
				return err
			}
			if jsonOut {
				return json.NewEncoder(os.Stdout).Encode(rows)
			}
			tw := tabwriter.NewWriter(os.Stdout, 0, 2, 2, ' ', 0)
			fmt.Fprintln(tw, "ID\tRESTIC_ID\tSIZE_BYTES\tREASON\tCREATED")
			for _, r := range rows {
				fmt.Fprintf(tw, "%s\t%s\t%d\t%s\t%s\n",
					r.ID, r.ResticID, r.SizeBytes, r.Reason, r.CreatedAt.Format(time.RFC3339))
			}
			return tw.Flush()
		},
	}
	cmd.Flags().BoolVar(&jsonOut, "json", false, "emit JSON instead of a table")
	return cmd
}

// ---- jabali docker (engine toggle, mirrors `jabali db postgres enable`) -----

func newDockerEngineCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "docker",
		Short: "Manage the docker engine (M48 opt-in)",
	}
	cmd.AddCommand(
		newDockerEngineActionCmd("enable", "Install docker engine + flip Server Settings toggle"),
		newDockerEngineActionCmd("disable", "Disable the marketplace toggle (keeps docker installed)"),
		newDockerEngineStatusCmd(),
	)
	return cmd
}

func newDockerEngineActionCmd(action, short string) *cobra.Command {
	return &cobra.Command{
		Use:     action,
		Short:   short,
		PreRunE: requireDBAndAgent,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := context.WithTimeout(cmd.Context(), 5*time.Minute)
			defer cancel()
			verb := "docker.install"
			if action == "disable" {
				verb = "docker.disable"
			}
			raw, err := sharedAgent.Call(ctx, verb, map[string]any{})
			if err != nil {
				return err
			}
			os.Stdout.Write(raw)
			os.Stdout.Write([]byte{'\n'})
			return nil
		},
	}
}

func newDockerEngineStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "status",
		Short:   "Show docker engine status (active, marketplace toggle state)",
		PreRunE: requireDBAndAgent,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := context.WithTimeout(cmd.Context(), 30*time.Second)
			defer cancel()
			raw, err := sharedAgent.Call(ctx, "docker.status", map[string]any{})
			if err != nil {
				return err
			}
			os.Stdout.Write(raw)
			os.Stdout.Write([]byte{'\n'})
			return nil
		},
	}
}

func firstLine(s string) string {
	for i, r := range s {
		if r == '\n' {
			return s[:i]
		}
	}
	return s
}
