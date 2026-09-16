package ui

import "github.com/charmbracelet/lipgloss"

var (
	Cyan      = lipgloss.Color("#00f5d4")
	Gray      = lipgloss.Color("#4a5568")
	LightGray = lipgloss.Color("#a0aec0")
	White     = lipgloss.Color("#ffffff")
	Red       = lipgloss.Color("#ff0054")
	Green     = lipgloss.Color("#7bf1a8")
	Yellow    = lipgloss.Color("#ffee32")
	Blue      = lipgloss.Color("#00bbf9")
	Orange    = lipgloss.Color("#ff9f1c")
	DarkBg    = lipgloss.Color("#1a202c")

	PanelTitleStyle  = lipgloss.NewStyle().Foreground(Cyan).Bold(true)
	ActiveTitleStyle = lipgloss.NewStyle().Foreground(Cyan).Bold(true).Underline(true)
	LabelStyle       = lipgloss.NewStyle().Foreground(Gray).Bold(true)
	ValueStyle       = lipgloss.NewStyle().Foreground(White)
	HighlightStyle   = lipgloss.NewStyle().Foreground(Green).Bold(true)
	FavoriteStyle    = lipgloss.NewStyle().Foreground(Yellow).Bold(true)

	KeyStyle  = lipgloss.NewStyle().Background(LightGray).Foreground(DarkBg).Bold(true).Padding(0, 1)
	DescStyle = lipgloss.NewStyle().Foreground(LightGray).Padding(0, 1)

	BoxStyle = lipgloss.NewStyle().
	Border(lipgloss.NormalBorder()).
	BorderForeground(Gray).
	Padding(0, 1)

	ActiveBoxStyle = BoxStyle.
	BorderForeground(Cyan)

	ErrorBoxStyle = BoxStyle.
	BorderForeground(Red)

	SelectedRowStyle = lipgloss.NewStyle().Background(lipgloss.Color("#2d3748")).Foreground(White).Bold(true)

	HelpOverlayStyle = lipgloss.NewStyle().
	Border(lipgloss.RoundedBorder()).
	BorderForeground(Cyan).
	Background(DarkBg).
	Padding(1, 2)

	HelpTitleStyle   = lipgloss.NewStyle().Foreground(Cyan).Bold(true)
	HelpSectionStyle = lipgloss.NewStyle().Foreground(Yellow).Bold(true)

	SmallScreenStyle = lipgloss.NewStyle().
	Foreground(Yellow).
	Bold(true).
	Align(lipgloss.Center, lipgloss.Center)
)
