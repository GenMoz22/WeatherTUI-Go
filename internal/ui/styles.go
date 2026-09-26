package ui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

var (
	Cyan       = lipgloss.Color("#00f5d4")
	DarkCyan   = lipgloss.Color("#00a896")
	Gray       = lipgloss.Color("#3a3f4d")
	BorderGray = lipgloss.Color("#4a5568")
	LightGray  = lipgloss.Color("#a0aec0")
	White      = lipgloss.Color("#f8f9fa")
	Red        = lipgloss.Color("#ff0054")
	Green      = lipgloss.Color("#7bf1a8")
	Yellow     = lipgloss.Color("#ffee32")
	Blue       = lipgloss.Color("#00bbf9")
	Orange     = lipgloss.Color("#ff9f1c")
	DarkBg     = lipgloss.Color("#181825")
	MutedBg    = lipgloss.Color("#2a2d3d")

	// Text typography styles
	PanelTitleStyle  = lipgloss.NewStyle().Foreground(LightGray).Bold(true)
	ActiveTitleStyle = lipgloss.NewStyle().Foreground(Cyan).Bold(true)
	LabelStyle       = lipgloss.NewStyle().Foreground(LightGray).Bold(true)
	ValueStyle       = lipgloss.NewStyle().Foreground(White)
	HighlightStyle   = lipgloss.NewStyle().Foreground(Green).Bold(true)
	FavoriteStyle    = lipgloss.NewStyle().Foreground(Yellow).Bold(true)

	// Bottom navigation bar keybinding styles
	KeyStyle  = lipgloss.NewStyle().Background(Gray).Foreground(White).Bold(true).Padding(0, 1)
	DescStyle = lipgloss.NewStyle().Foreground(LightGray).Padding(0, 1)

	// Rounded border containers
	BoxStyle = lipgloss.NewStyle().
	Border(lipgloss.RoundedBorder()).
	BorderForeground(BorderGray).
	Padding(0, 1)

	ActiveBoxStyle = BoxStyle.
	BorderForeground(Cyan)

	ErrorBoxStyle = BoxStyle.
	BorderForeground(Red)

	// Selection indicators
	SelectedRowStyle = lipgloss.NewStyle().
	Background(MutedBg).
	Foreground(White).
	Bold(true)

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

// RenderPanelBox builds a panel box embedding the title directly into the top border line.
func RenderPanelBox(title string, content string, width, height int, isActive bool, isError bool) string {
	borderColor := BorderGray
	titleStyle := PanelTitleStyle

	if isActive {
		borderColor = Cyan
		titleStyle = ActiveTitleStyle
	}
	if isError {
		borderColor = Red
	}

	innerWidth := width - 2
	innerHeight := height - 2
	if innerWidth < 1 {
		innerWidth = 1
	}
	if innerHeight < 1 {
		innerHeight = 1
	}

	border := lipgloss.RoundedBorder()

	// Top border calculation with embedded title
	var topBorder strings.Builder
	topBorder.WriteString(border.TopLeft)

	if title != "" {
		renderedTitle := titleStyle.Render(title)
		topBorder.WriteString(renderedTitle)
		titleLen := lipgloss.Width(renderedTitle)
		remainingWidth := innerWidth - titleLen
		if remainingWidth > 0 {
			topBorder.WriteString(strings.Repeat(border.Top, remainingWidth))
		}
	} else {
		topBorder.WriteString(strings.Repeat(border.Top, innerWidth))
	}
	topBorder.WriteString(border.TopRight)

	// Bottom border calculation
	bottomBorder := border.BottomLeft + strings.Repeat(border.Bottom, innerWidth) + border.BottomRight

	// Truncate/Pad content to fit internal dimensions
	lines := strings.Split(content, "\n")
	var formattedLines []string

	borderStyle := lipgloss.NewStyle().Foreground(borderColor)

	for i := 0; i < innerHeight; i++ {
		line := ""
		if i < len(lines) {
			line = lines[i]
		}
		lineWidth := lipgloss.Width(line)
		if lineWidth > innerWidth {
			// Safely truncate string while keeping ANSI sequences valid using Lipgloss truncation
			line = lipgloss.NewStyle().MaxWidth(innerWidth).Render(line)
			lineWidth = lipgloss.Width(line)
		}
		padding := innerWidth - lineWidth
		if padding < 0 {
			padding = 0
		}

		renderedLine := borderStyle.Render(border.Left) + line + strings.Repeat(" ", padding) + borderStyle.Render(border.Right)
		formattedLines = append(formattedLines, renderedLine)
	}

	var box strings.Builder
	box.WriteString(borderStyle.Render(topBorder.String()) + "\n")
	box.WriteString(strings.Join(formattedLines, "\n") + "\n")
	box.WriteString(borderStyle.Render(bottomBorder))

	return box.String()
}
