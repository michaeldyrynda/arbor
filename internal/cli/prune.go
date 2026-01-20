package cli

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/michaeldyrynda/arbor/internal/config"
	"github.com/michaeldyrynda/arbor/internal/git"
	"github.com/michaeldyrynda/arbor/internal/presets"
	"github.com/michaeldyrynda/arbor/internal/scaffold"
	"github.com/spf13/cobra"
)

var pruneCmd = &cobra.Command{
	Use:   "prune",
	Short: "Remove merged worktrees",
	Long: `Removes merged worktrees automatically.

Lists all worktrees, identifies merged ones, and provides an
interactive review before removal.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cwd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("getting current directory: %w", err)
		}

		barePath, err := git.FindBarePath(cwd)
		if err != nil {
			return fmt.Errorf("finding bare repository: %w", err)
		}

		projectPath := filepath.Dir(barePath)
		cfg, err := config.LoadProject(projectPath)
		if err != nil {
			return fmt.Errorf("loading project config: %w", err)
		}

		force := mustGetBool(cmd, "force")
		dryRun := mustGetBool(cmd, "dry-run")
		verbose := mustGetBool(cmd, "verbose")

		worktrees, err := git.ListWorktrees(barePath)
		if err != nil {
			return fmt.Errorf("listing worktrees: %w", err)
		}

		defaultBranch := cfg.DefaultBranch
		if defaultBranch == "" {
			defaultBranch, _ = git.GetDefaultBranch(barePath)
			if defaultBranch == "" {
				defaultBranch = config.DefaultBranch
			}
		}

		var removable []git.Worktree
		fmt.Println("Worktree status:")
		fmt.Println(strings.Repeat("-", 60))

		for _, wt := range worktrees {
			if wt.Branch == defaultBranch || wt.Branch == "(bare)" {
				fmt.Printf("  %-30s %s\n", wt.Branch, wt.Path)
				continue
			}

			merged, err := git.IsMerged(barePath, wt.Branch, defaultBranch)
			if err != nil {
				fmt.Printf("  %-30s %s (error checking merge status)\n", wt.Branch, wt.Path)
				continue
			}

			if merged {
				removable = append(removable, wt)
				status := "MERGED"
				fmt.Printf("  %-30s %s [%s]\n", wt.Branch, wt.Path, status)
			} else {
				fmt.Printf("  %-30s %s [not merged]\n", wt.Branch, wt.Path)
			}
		}

		fmt.Println(strings.Repeat("-", 60))

		if len(removable) == 0 {
			fmt.Println("No merged worktrees to remove.")
			return nil
		}

		fmt.Printf("\n%d merged worktree(s) found.\n", len(removable))

		var toRemove []git.Worktree
		if force {
			toRemove = removable
		} else {
			fmt.Println("\nSelect worktrees to remove (comma-separated numbers, 'all', 'none'):")
			for i, wt := range removable {
				fmt.Printf("  %d. %s (%s)\n", i, wt.Branch, wt.Path)
			}
			fmt.Println()

			fmt.Print("Selection: ")
			reader := bufio.NewReader(os.Stdin)
			input, err := reader.ReadString('\n')
			if err != nil {
				return fmt.Errorf("reading input: %w", err)
			}
			input = strings.TrimSpace(input)

			if input == "all" {
				toRemove = removable
			} else if input == "none" {
				fmt.Println("No worktrees removed.")
				return nil
			} else {
				parts := strings.Split(input, ",")
				for _, part := range parts {
					part = strings.TrimSpace(part)
					if idx, err := strconv.Atoi(part); err == nil {
						if idx >= 0 && idx < len(removable) {
							toRemove = append(toRemove, removable[idx])
						}
					}
				}
			}
		}

		if len(toRemove) == 0 {
			fmt.Println("No worktrees selected for removal.")
			return nil
		}

		fmt.Printf("\nRemoving %d worktree(s):\n", len(toRemove))
		for _, wt := range toRemove {
			fmt.Printf("  - %s (%s)\n", wt.Branch, wt.Path)
		}
		fmt.Println()

		presetManager := presets.NewManager()
		scaffoldManager := scaffold.NewScaffoldManager()
		presets.RegisterAllWithScaffold(scaffoldManager)

		for _, wt := range toRemove {
			fmt.Printf("Removing %s...\n", wt.Branch)

			if !dryRun {
				preset := cfg.Preset
				if preset == "" {
					preset = presetManager.Detect(wt.Path)
				}

				if err := scaffoldManager.RunCleanup(wt.Path, wt.Branch, "", preset, cfg, false, verbose); err != nil {
					fmt.Printf("Warning: cleanup steps failed: %v\n", err)
				}

				if err := git.RemoveWorktree(wt.Path, true); err != nil {
					fmt.Printf("Error removing worktree: %v\n", err)
				}
			} else {
				fmt.Printf("[DRY RUN] Would remove %s and run cleanup\n", wt.Branch)
			}
		}

		fmt.Println("\nDone.")
		return nil
	},
}

func init() {
	rootCmd.AddCommand(pruneCmd)

	pruneCmd.Flags().BoolP("force", "f", false, "Skip interactive confirmation")
}
