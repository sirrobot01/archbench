package ui

import (
	"image/color"
	"os"
	"strings"

	"charm.land/lipgloss/v2"
	"charm.land/lipgloss/v2/table"
)

var (
	accent      = lipgloss.Color("#2F80ED")
	success     = lipgloss.Color("#2E7D32")
	warning     = lipgloss.Color("#C77800")
	danger      = lipgloss.Color("#D32F2F")
	textMuted   = lipgloss.Color("#6B7280")
	borderMuted = lipgloss.Color("#9CA3AF")

	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(accent)

	subtleStyle = lipgloss.NewStyle().
			Foreground(textMuted)

	headerStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(accent).
			Padding(0, 1)

	cellStyle = lipgloss.NewStyle().
			Padding(0, 1)

	evenRowStyle = cellStyle
	oddRowStyle  = cellStyle.Foreground(textMuted)

	borderStyle = lipgloss.NewStyle().
			Foreground(borderMuted)
)

// Title renders a section title for terminal output.
func Title(s string) string {
	return style(titleStyle).Render(s)
}

// Subtle renders low-emphasis terminal text.
func Subtle(s string) string {
	return style(subtleStyle).Render(s)
}

// Success renders positive status text.
func Success(s string) string {
	return renderColor(success, s)
}

// Warning renders warning status text.
func Warning(s string) string {
	return renderColor(warning, s)
}

// Danger renders failing status text.
func Danger(s string) string {
	return renderColor(danger, s)
}

// Diff renders comparison text with directional color.
func Diff(s string) string {
	switch {
	case strings.Contains(s, "faster"):
		return Success(s)
	case strings.Contains(s, "slower"), strings.Contains(s, "DIVERGES"):
		return Danger(s)
	case s == "(new)":
		return Warning(s)
	default:
		return Subtle(s)
	}
}

// Status renders a test status with color.
func Status(s string) string {
	switch s {
	case "pass":
		return Success(s)
	case "skip":
		return Warning(s)
	case "fail":
		return Danger(s)
	default:
		return Subtle(s)
	}
}

// RenderTable returns a styled terminal table.
func RenderTable(headers []string, rows [][]string, rightAligned map[int]bool) string {
	t := table.New().
		Border(lipgloss.RoundedBorder()).
		BorderStyle(style(borderStyle)).
		BorderRow(false).
		Headers(headers...).
		Rows(rows...).
		StyleFunc(func(row, col int) lipgloss.Style {
			if row == table.HeaderRow {
				return align(headerStyle, col, rightAligned)
			}
			base := evenRowStyle
			if row%2 != 0 {
				base = oddRowStyle
			}
			return align(base, col, rightAligned)
		})

	return t.Render()
}

func align(s lipgloss.Style, col int, rightAligned map[int]bool) lipgloss.Style {
	s = style(s)
	if rightAligned[col] {
		return s.Align(lipgloss.Right)
	}
	return s
}

func renderColor(c color.Color, s string) string {
	return style(lipgloss.NewStyle().Foreground(c)).Render(s)
}

func style(s lipgloss.Style) lipgloss.Style {
	if plain() {
		return stripStyle(s)
	}
	return s
}

func stripStyle(s lipgloss.Style) lipgloss.Style {
	return s.
		UnsetForeground().
		UnsetBackground().
		Bold(false).
		Italic(false).
		Underline(false)
}

func plain() bool {
	return os.Getenv("NO_COLOR") != "" || os.Getenv("TERM") == "dumb"
}
