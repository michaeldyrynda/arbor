package ui

import (
	"fmt"
	"os"

	"github.com/charmbracelet/huh/spinner"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/log"
)

var logger *log.Logger

func init() {
	logger = log.New(os.Stderr)
	logger.SetLevel(log.InfoLevel)
}

func PrintSuccess(msg string) {
	logger.Info("✓ " + msg)
}

func PrintWarning(msg string) {
	logger.Warn("⚠ " + msg)
}

func PrintError(msg string) {
	logger.Error("✗ " + msg)
}

func PrintInfo(msg string) {
	logger.Info("ℹ " + msg)
}

func PrintStep(msg string) {
	logger.Info("→ " + msg)
}

func PrintDone(msg string) {
	style := lipgloss.NewStyle().
		Foreground(ColorSuccess).
		Bold(true)
	fmt.Println(style.Render("✓ " + msg))
}

func PrintSuccessPath(msg, path string) {
	style := lipgloss.NewStyle().
		Foreground(ColorSuccess)
	fmt.Println(style.Render("✓ "+msg+": ") + CodeStyle.Render(path))
}

func PrintErrorWithHint(msg, hint string) {
	style := lipgloss.NewStyle().
		Foreground(ColorError)
	fmt.Println(style.Render("✗ " + msg))
	fmt.Println("  " + MutedStyle.Render(hint))
}

func RunWithSpinner(title string, action func() error) error {
	var err error
	sp := spinner.New().
		Title(title).
		Action(func() {
			err = action()
		})
	if runErr := sp.Run(); runErr != nil {
		return runErr
	}
	return err
}
