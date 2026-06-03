package ui

import (
	"context"
	"io"
	"os"

	huhspinner "charm.land/huh/v2/spinner"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/term"
)

// Spinner runs action with an animated spinner when output is an interactive
// terminal. In redirected or CI output, action runs without redraw noise.
func Spinner(ctx context.Context, output io.Writer, title string, action func(context.Context) error) error {
	if !interactive(output) {
		return action(ctx)
	}

	return huhspinner.New().
		Title(title + " ").
		Type(huhspinner.Dots).
		Context(ctx).
		WithOutput(output).
		WithInput(os.Stdin).
		WithTheme(huhspinner.ThemeFunc(func(bool) *huhspinner.Styles {
			return &huhspinner.Styles{
				Spinner: lipgloss.NewStyle().Foreground(accent),
				Title:   lipgloss.NewStyle().Foreground(textMuted),
			}
		})).
		ActionWithErr(action).
		Run()
}

func interactive(w io.Writer) bool {
	if os.Getenv("TERM") == "dumb" {
		return false
	}
	f, ok := w.(*os.File)
	return ok && term.IsTerminal(f.Fd())
}
