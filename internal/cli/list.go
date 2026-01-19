package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/michaeldyrynda/arbor/internal/config"
	"github.com/michaeldyrynda/arbor/internal/git"
	"github.com/spf13/cobra"
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List all worktrees",
	Long: `List all worktrees in the repository with their status.

Shows worktrees with merge status, current worktree indicator,
and main branch highlighting.`,
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

		defaultBranch := cfg.DefaultBranch
		if defaultBranch == "" {
			defaultBranch, _ = git.GetDefaultBranch(barePath)
			if defaultBranch == "" {
				defaultBranch = "main"
			}
		}

		jsonOutput, _ := cmd.Flags().GetBool("json")
		porcelain, _ := cmd.Flags().GetBool("porcelain")
		sortBy, _ := cmd.Flags().GetString("sort-by")
		reverse, _ := cmd.Flags().GetBool("reverse")

		worktrees, err := git.ListWorktreesDetailed(barePath, cwd, defaultBranch)
		if err != nil {
			return fmt.Errorf("listing worktrees: %w", err)
		}

		worktrees = git.SortWorktrees(worktrees, sortBy, reverse)

		if jsonOutput {
			return printJSON(os.Stdout, worktrees)
		}

		if porcelain {
			return printPorcelain(os.Stdout, worktrees)
		}

		return printTable(os.Stdout, worktrees)
	},
}

func printTable(w io.Writer, worktrees []git.Worktree) error {
	if len(worktrees) == 0 {
		fmt.Fprintln(w, "No worktrees found.")
		return nil
	}

	maxWorktreeLen := 8 // "WORKTREE" length
	maxBranchLen := 6   // "BRANCH" length
	maxStatusLen := 6   // "STATUS" length

	for _, wt := range worktrees {
		worktreeName := filepath.Base(wt.Path)
		if len(worktreeName) > maxWorktreeLen {
			maxWorktreeLen = len(worktreeName)
		}
		if len(wt.Branch) > maxBranchLen {
			maxBranchLen = len(wt.Branch)
		}

		statusParts := []string{}
		if wt.IsCurrent {
			statusParts = append(statusParts, "[current]")
		}
		if wt.IsMain {
			statusParts = append(statusParts, "[main]")
		} else if wt.IsMerged {
			statusParts = append(statusParts, "[merged]")
		} else {
			statusParts = append(statusParts, "[not merged]")
		}
		status := strings.Join(statusParts, " ")
		if len(status) > maxStatusLen {
			maxStatusLen = len(status)
		}
	}

	headerFormat := fmt.Sprintf("%%-%ds %%-%ds %%s\n", maxWorktreeLen, maxBranchLen)
	rowFormat := fmt.Sprintf("%%-%ds %%-%ds %%s\n", maxWorktreeLen, maxBranchLen)
	separator := strings.Repeat("-", maxWorktreeLen+maxBranchLen+maxStatusLen+2)

	fmt.Fprintf(w, headerFormat, "WORKTREE", "BRANCH", "STATUS")
	fmt.Fprintln(w, separator)

	for _, wt := range worktrees {
		worktreeName := filepath.Base(wt.Path)

		statusParts := []string{}
		if wt.IsCurrent {
			statusParts = append(statusParts, "[current]")
		}
		if wt.IsMain {
			statusParts = append(statusParts, "[main]")
		} else if wt.IsMerged {
			statusParts = append(statusParts, "[merged]")
		} else {
			statusParts = append(statusParts, "[not merged]")
		}
		status := strings.Join(statusParts, " ")

		fmt.Fprintf(w, rowFormat, worktreeName, wt.Branch, status)
	}

	return nil
}

func printJSON(w io.Writer, worktrees []git.Worktree) error {
	type worktreeJSON struct {
		Path      string `json:"path"`
		Branch    string `json:"branch"`
		IsMain    bool   `json:"isMain"`
		IsCurrent bool   `json:"isCurrent"`
		IsMerged  bool   `json:"isMerged"`
	}

	jsonWorktrees := make([]worktreeJSON, len(worktrees))
	for i, wt := range worktrees {
		jsonWorktrees[i] = worktreeJSON{
			Path:      wt.Path,
			Branch:    wt.Branch,
			IsMain:    wt.IsMain,
			IsCurrent: wt.IsCurrent,
			IsMerged:  wt.IsMerged,
		}
	}

	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	return encoder.Encode(jsonWorktrees)
}

func printPorcelain(w io.Writer, worktrees []git.Worktree) error {
	for _, wt := range worktrees {
		current := ""
		if wt.IsCurrent {
			current = "current"
		}

		main := ""
		if wt.IsMain {
			main = "main"
		}

		merged := ""
		if wt.IsMerged {
			merged = "merged"
		} else {
			merged = "-"
		}

		fmt.Fprintf(w, "%s %s %s %s %s\n", wt.Path, wt.Branch, main, current, merged)
	}

	return nil
}

func init() {
	rootCmd.AddCommand(listCmd)

	listCmd.Flags().Bool("json", false, "Output as JSON array")
	listCmd.Flags().Bool("porcelain", false, "Machine-parseable output")
	listCmd.Flags().String("sort-by", "name", "Sort by: name, branch, created")
	listCmd.Flags().Bool("reverse", false, "Reverse sort order")
}
