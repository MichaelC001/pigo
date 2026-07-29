package tui

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
)

// This file builds the startup splash shown at the top of the transcript: the
// pigo braille logo painted in a vertical rainbow gradient, with the session's
// basic configuration (model, provider, protocol, thinking effort, directory)
// laid out beside it. It is seeded once by withSession so it scrolls up as the
// conversation grows, like a shell's login banner.

// logoLines is the pigo braille-art logo, one string per row.
var logoLines = []string{
	"⣿⣿⣿⣿⡿⠟⠛⠉⠉⠉⠉⠉⠉⠉⠉⠉⠉⠉⠉⠉⠉⠉⠉⠉⠉⠉⠉⠉⠙⣿",
	"⣿⣿⡿⠋⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⣼",
	"⣿⠋⠀⠀⠀⠀⠀⣀⡀⠀⠀⠀⠀⢠⣤⣤⣤⠀⠀⠀⠀⠀⣤⣤⣤⣤⣤⣤⣾⣿",
	"⣧⠀⠀⠀⠀⣠⣾⣿⡇⠀⠀⠀⠀⣿⣿⣿⣿⠀⠀⠀⠀⠀⣿⣿⣿⣿⣿⣿⣿⣿",
	"⣿⣶⣤⣤⣾⣿⣿⣿⡇⠀⠀⠀⠀⣿⣿⣿⣿⠀⠀⠀⠀⠀⣿⣿⣿⣿⣿⣿⣿⣿",
	"⣿⣿⣿⣿⣿⣿⣿⣿⠀⠀⠀⠀⢀⣿⣿⣿⣿⠀⠀⠀⠀⠀⣿⣿⣿⣿⣿⣿⣿⣿",
	"⣿⣿⣿⣿⣿⣿⣿⡟⠀⠀⠀⠀⢸⣿⣿⣿⣿⠀⠀⠀⠀⠀⣿⣿⣿⣿⣿⣿⣿⣿",
	"⣿⣿⣿⣿⣿⣿⣿⠇⠀⠀⠀⠀⣾⣿⣿⣿⣿⠀⠀⠀⠀⠀⣿⣿⣿⣿⣿⣿⣿⣿",
	"⣿⣿⣿⣿⣿⣿⡟⠀⠀⠀⠀⢠⣿⣿⣿⣿⣿⠀⠀⠀⠀⠀⣿⣿⣿⡿⠛⠛⢿⣿",
	"⣿⣿⣿⣿⣿⡿⠁⠀⠀⠀⠀⣾⣿⣿⣿⣿⣿⠀⠀⠀⠀⠀⣿⣿⡟⠀⠀⠀⠀⢻",
	"⣿⣿⣿⣿⡟⠁⠀⠀⠀⠀⣸⣿⣿⣿⣿⣿⣿⡀⠀⠀⠀⠀⠛⠛⠁⠀⠀⠀⠀⣾",
	"⣿⣿⣿⡏⠀⠀⠀⠀⠀⣴⣿⣿⣿⣿⣿⣿⣿⣧⡀⠀⠀⠀⠀⠀⠀⠀⠀⢀⣼⣿",
	"⣿⣿⣿⣿⣄⣀⣀⣠⣾⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣦⣄⣀⣀⣀⣀⣠⣴⣿⣿⣿",
}

// logoColors is the top-to-bottom rainbow ramp painted across the logo rows
// (ANSI 256-color cube): red → orange → yellow → green → cyan → blue.
var logoColors = []string{
	"196", "202", "208", "214", "220", "190", "118",
	"46", "48", "50", "45", "39", "33",
}

// renderBanner paints the logo gradient and joins it with a config panel showing
// the session basics. It performs no I/O and never panics, so it is safe to build
// eagerly at startup.
func renderBanner(theme Theme, opts Options, cwd string) string {
	var logo strings.Builder
	for i, line := range logoLines {
		if i > 0 {
			logo.WriteByte('\n')
		}
		c := logoColors[i%len(logoColors)]
		logo.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color(c)).Render(line))
	}

	title := lipgloss.NewStyle().Foreground(lipgloss.Color(colorAccent)).Bold(true)
	label := lipgloss.NewStyle().Foreground(lipgloss.Color(colorGray))
	value := lipgloss.NewStyle().Foreground(lipgloss.Color(colorUser)).Bold(true)

	rows := [][2]string{
		{"Model", firstNonEmpty(opts.Model, "—")},
		{"Provider", firstNonEmpty(opts.ProviderName, "—")},
		{"Protocol", firstNonEmpty(opts.Protocol, "—")},
		{"Thinking", firstNonEmpty(string(opts.ThinkingLevel), "off")},
		{"Directory", firstNonEmpty(cwd, "—")},
	}

	var info strings.Builder
	info.WriteString(title.Render("pigo") + "  " + theme.System.Render("终端 AI 编程助手") + "\n\n")
	for i, r := range rows {
		if i > 0 {
			info.WriteByte('\n')
		}
		info.WriteString(label.Render(fmt.Sprintf("%-10s ", r[0])) + value.Render(r[1]))
	}

	return lipgloss.JoinHorizontal(lipgloss.Center, logo.String(), "   ", info.String())
}

// firstNonEmpty returns s when it is non-empty, otherwise the fallback.
func firstNonEmpty(s, fallback string) string {
	if strings.TrimSpace(s) == "" {
		return fallback
	}
	return s
}
