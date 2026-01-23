// Package tui implements the terminal user interface using Bubbletea.
package tui

import "github.com/charmbracelet/lipgloss"

// Color palette
var (
	ColorCyan    = lipgloss.Color("86")
	ColorGreen   = lipgloss.Color("78")
	ColorYellow  = lipgloss.Color("221")
	ColorRed     = lipgloss.Color("196")
	ColorMagenta = lipgloss.Color("213")
	ColorBlue    = lipgloss.Color("111")
	ColorGray    = lipgloss.Color("245")
	ColorDimGray = lipgloss.Color("239")
)

// Tool colors
var ToolColors = map[string]lipgloss.Color{
	"claude":   ColorMagenta,
	"gemini":   ColorBlue,
	"codex":    ColorGreen,
	"opencode": ColorYellow,
	"unknown":  ColorGray,
}

// Status indicator styles
var (
	StatusHealthy = lipgloss.NewStyle().Foreground(ColorGreen)
	StatusStale   = lipgloss.NewStyle().Foreground(ColorYellow)
	StatusDead    = lipgloss.NewStyle().Foreground(ColorRed)
)

// Promise status styles
var (
	PromiseCompleted   = lipgloss.NewStyle().Foreground(ColorGreen)
	PromiseBlocked     = lipgloss.NewStyle().Foreground(ColorRed)
	PromiseNeedsReview = lipgloss.NewStyle().Foreground(ColorYellow)
)

// Common styles
var (
	// Title style
	TitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(ColorCyan)

	// Subtitle/dim text
	SubtitleStyle = lipgloss.NewStyle().
			Foreground(ColorGray)

	// Selected item style
	SelectedStyle = lipgloss.NewStyle().
			Foreground(ColorCyan).
			Bold(true)

	// Dim text style
	DimStyle = lipgloss.NewStyle().
			Foreground(ColorDimGray)

	// Bold text
	BoldStyle = lipgloss.NewStyle().Bold(true)

	// Border box style
	BoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(ColorGray).
			Padding(1, 2)

	// Active section header
	ActiveSectionStyle = lipgloss.NewStyle().
				Foreground(ColorGreen).
				Bold(true)

	// Completed section header
	CompletedSectionStyle = lipgloss.NewStyle().
				Foreground(ColorGray).
				Bold(true)

	// Help key style
	HelpKeyStyle = lipgloss.NewStyle().
			Foreground(ColorCyan)

	// Help text style
	HelpTextStyle = lipgloss.NewStyle().
			Foreground(ColorGray)

	// Error style
	ErrorStyle = lipgloss.NewStyle().
			Foreground(ColorRed)

	// Status message style
	StatusMsgStyle = lipgloss.NewStyle().
			Foreground(ColorCyan)

	// Warning style
	WarningStyle = lipgloss.NewStyle().
			Foreground(ColorYellow)
)

// Health status styles (for stuck/unresponsive detection)
var (
	StatusStuck        = lipgloss.NewStyle().Foreground(lipgloss.Color("#FF6B6B"))
	StatusUnresponsive = lipgloss.NewStyle().Foreground(ColorRed).Bold(true)
)

// Status indicators
const (
	IndicatorHealthy      = "●"
	IndicatorStale        = "◐"
	IndicatorDead         = "○"
	IndicatorStuck        = "◉"
	IndicatorUnresponsive = "✗"
	IndicatorCompleted    = "✓"
	IndicatorBlocked      = "!"
	IndicatorReview       = "?"
	IndicatorSelected     = "❯"
	IndicatorOrchestra    = "🎯"
	IndicatorChild        = "├─"
)

// Progress bar characters
const (
	ProgressFilled = "█"
	ProgressEmpty  = "░"
)

// RenderProgressBar renders a progress bar for the given percentage.
func RenderProgressBar(percent float64, width int) string {
	if width <= 0 {
		width = 20
	}
	filled := int((percent / 100) * float64(width))
	if filled > width {
		filled = width
	}
	if filled < 0 {
		filled = 0
	}
	empty := width - filled

	bar := ""
	for i := 0; i < filled; i++ {
		bar += ProgressFilled
	}
	for i := 0; i < empty; i++ {
		bar += ProgressEmpty
	}
	return bar
}

// GetToolStyle returns the style for a tool name.
func GetToolStyle(tool string) lipgloss.Style {
	color, ok := ToolColors[tool]
	if !ok {
		color = ColorGray
	}
	return lipgloss.NewStyle().Foreground(color)
}
