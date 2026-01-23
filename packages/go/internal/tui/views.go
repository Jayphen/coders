package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"

	"github.com/Jayphen/coders/internal/tmux"
	"github.com/Jayphen/coders/internal/types"
)

// renderHeader renders the application header.
func (m Model) renderHeader() string {
	title := TitleStyle.Render("Coders Session Manager")
	version := ""
	if m.version != "" {
		version = " " + SubtitleStyle.Render("v"+m.version)
	}
	subtitle := SubtitleStyle.Render("Manage your AI coding sessions")

	return title + version + "\n" + subtitle
}

// renderConfirmDialog renders the kill confirmation dialog.
func (m Model) renderConfirmDialog() string {
	completedCount := m.countCompleted()
	msg := fmt.Sprintf("Kill all %d completed session(s)? (y/n)", completedCount)

	style := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ColorYellow).
		Padding(1, 2).
		Foreground(ColorYellow)

	return style.Render(msg)
}

// renderSpawnPrompt renders the spawn input dialog.
func (m Model) renderSpawnPrompt() string {
	var b strings.Builder

	style := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ColorCyan).
		Padding(1, 2)

	titleStyle := lipgloss.NewStyle().Foreground(ColorCyan)
	b.WriteString(titleStyle.Render("Spawn a new session"))
	b.WriteString("\n\n")
	b.WriteString(DimStyle.Render("Args: "))
	b.WriteString(m.spawnInput.View())
	b.WriteString("\n")
	b.WriteString(DimStyle.Render("Enter to spawn, Esc to cancel"))

	return style.Render(b.String())
}

// renderSessionList renders the session list.
func (m Model) renderSessionList() string {
	if m.loading && len(m.sessions) == 0 {
		return m.spinner.View() + " Loading sessions..."
	}

	if len(m.sessions) == 0 {
		style := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(ColorGray).
			Padding(1, 2).
			Foreground(ColorGray)
		return style.Render("No active coder sessions")
	}

	var b strings.Builder

	// Column headers
	headers := fmt.Sprintf(" %-3s%-28s%-10s%-20s%-8s",
		"", "SESSION", "TOOL", "TASK/SUMMARY", "STATUS")
	b.WriteString(DimStyle.Bold(true).Render(headers))
	b.WriteString("\n")

	// Split into active and completed
	var active, completed []int
	for i, s := range m.sessions {
		if s.HasPromise {
			completed = append(completed, i)
		} else {
			active = append(active, i)
		}
	}

	// Render active sessions
	if len(active) > 0 {
		b.WriteString(ActiveSectionStyle.Render(fmt.Sprintf("Active (%d)", len(active))))
		b.WriteString("\n")
		for _, i := range active {
			b.WriteString(m.renderSessionRow(i))
			b.WriteString("\n")
		}
	}

	// Render completed sessions
	if len(completed) > 0 {
		if len(active) > 0 {
			b.WriteString("\n")
		}
		b.WriteString(CompletedSectionStyle.Render(fmt.Sprintf("Completed (%d)", len(completed))))
		b.WriteString("\n")
		for _, i := range completed {
			b.WriteString(m.renderSessionRow(i))
			b.WriteString("\n")
		}
	}

	return b.String()
}

// renderSessionRow renders a single session row.
func (m Model) renderSessionRow(index int) string {
	s := m.sessions[index]
	isSelected := index == m.selectedIndex

	// Selection indicator
	selector := "  "
	if isSelected {
		selector = SelectedStyle.Render(IndicatorSelected + " ")
	}

	// Session name
	displayName := s.Name
	if s.IsOrchestrator {
		displayName = "orchestrator"
	} else {
		displayName = strings.TrimPrefix(s.Name, tmux.SessionPrefix)
	}

	// Prefix for orchestrator or child
	prefix := ""
	if s.IsOrchestrator {
		prefix = IndicatorOrchestra + " "
	} else if s.ParentSessionID != "" {
		prefix = IndicatorChild + " "
	}

	nameStyle := lipgloss.NewStyle()
	if s.IsOrchestrator {
		nameStyle = nameStyle.Foreground(ColorCyan).Bold(true)
	} else if s.HasPromise {
		nameStyle = nameStyle.Foreground(ColorGray)
	} else if isSelected {
		nameStyle = nameStyle.Bold(true)
	}

	// Truncate name
	maxNameLen := 22
	if s.IsOrchestrator {
		maxNameLen = 24
	}
	if len(displayName) > maxNameLen {
		displayName = displayName[:maxNameLen-3] + "..."
	}
	namePart := nameStyle.Render(prefix + displayName)

	// Tool
	toolStyle := GetToolStyle(s.Tool)
	if s.HasPromise {
		toolStyle = toolStyle.Foreground(ColorDimGray)
	}
	toolPart := toolStyle.Render(s.Tool)

	// Task/Summary
	displayText := s.Task
	if s.Promise != nil {
		displayText = s.Promise.Summary
	}
	if displayText == "" {
		displayText = "-"
	}
	if len(displayText) > 18 {
		displayText = displayText[:15] + "..."
	}
	taskStyle := lipgloss.NewStyle()
	if s.HasPromise || displayText == "-" {
		taskStyle = taskStyle.Foreground(ColorDimGray)
	}
	taskPart := taskStyle.Render(displayText)

	// Status indicator
	var statusPart string
	if s.Promise != nil {
		switch s.Promise.Status {
		case types.PromiseCompleted:
			statusPart = PromiseCompleted.Render(IndicatorCompleted)
		case types.PromiseBlocked:
			statusPart = PromiseBlocked.Render(IndicatorBlocked)
		case types.PromiseNeedsReview:
			statusPart = PromiseNeedsReview.Render(IndicatorReview)
		}
	} else {
		switch s.HeartbeatStatus {
		case types.HeartbeatHealthy:
			statusPart = StatusHealthy.Render(IndicatorHealthy)
		case types.HeartbeatStale:
			statusPart = StatusStale.Render(IndicatorStale)
		default:
			statusPart = StatusDead.Render(IndicatorDead)
		}
	}

	// Format row with fixed widths
	return fmt.Sprintf("%s%-28s%-10s%-20s%s",
		selector, namePart, toolPart, taskPart, statusPart)
}

// renderSessionDetail renders the detail panel for the selected session.
func (m Model) renderSessionDetail() string {
	s := m.selectedSession()
	if s == nil {
		return ""
	}

	var b strings.Builder

	// Title
	titleStyle := TitleStyle
	if s.HasPromise {
		titleStyle = titleStyle.Foreground(ColorGray)
	}
	title := ""
	if s.IsOrchestrator {
		title = IndicatorOrchestra + " "
	}
	title += s.Name

	b.WriteString(titleStyle.Render(title))

	// Promise status badge
	if s.Promise != nil {
		var badge string
		switch s.Promise.Status {
		case types.PromiseCompleted:
			badge = PromiseCompleted.Render(" [Completed]")
		case types.PromiseBlocked:
			badge = PromiseBlocked.Render(" [Blocked]")
		case types.PromiseNeedsReview:
			badge = PromiseNeedsReview.Render(" [Needs Review]")
		}
		b.WriteString(badge)
	}
	b.WriteString("\n\n")

	// Promise info
	if s.Promise != nil {
		b.WriteString(m.renderDetailRow("Summary:", PromiseCompleted.Render(s.Promise.Summary)))
		b.WriteString(m.renderDetailRow("Finished:", formatAge(time.UnixMilli(s.Promise.Timestamp))))
		if len(s.Promise.Blockers) > 0 {
			b.WriteString(m.renderDetailRow("Blockers:", PromiseBlocked.Render(strings.Join(s.Promise.Blockers, ", "))))
		}
	}

	// Basic info
	taskDisplay := s.Task
	if taskDisplay == "" {
		taskDisplay = "No task specified"
	} else {
		taskDisplay = strings.ReplaceAll(taskDisplay, "-", " ")
	}
	taskStyle := lipgloss.NewStyle()
	if s.HasPromise {
		taskStyle = DimStyle
	}
	b.WriteString(m.renderDetailRow("Task:", taskStyle.Render(taskDisplay)))
	b.WriteString(m.renderDetailRow("Tool:", GetToolStyle(s.Tool).Render(s.Tool)))
	b.WriteString(m.renderDetailRow("Directory:", DimStyle.Render(s.Cwd)))

	if s.CreatedAt != nil {
		b.WriteString(m.renderDetailRow("Created:", formatAge(*s.CreatedAt)))
	}

	// Usage stats
	if s.Usage != nil {
		b.WriteString("\n")
		b.WriteString(m.renderDetailRow("Usage:", ""))
		if s.Usage.Cost != "" {
			b.WriteString("  " + WarningStyle.Render("Cost: "+s.Usage.Cost) + "\n")
		}
		if s.Usage.Tokens > 0 {
			b.WriteString(fmt.Sprintf("  Tokens: %d\n", s.Usage.Tokens))
		}
		if s.Usage.APICalls > 0 {
			b.WriteString(fmt.Sprintf("  API Calls: %d\n", s.Usage.APICalls))
		}
		if s.Usage.SessionLimitPct > 0 {
			color := ColorGreen
			if s.Usage.SessionLimitPct > 90 {
				color = ColorRed
			}
			style := lipgloss.NewStyle().Foreground(color)
			b.WriteString(fmt.Sprintf("  Session Limit: %s\n", style.Render(fmt.Sprintf("%.0f%%", s.Usage.SessionLimitPct))))
			b.WriteString("  " + style.Render(RenderProgressBar(s.Usage.SessionLimitPct, 20)) + "\n")
		}
	}

	// Parent session
	if s.ParentSessionID != "" {
		b.WriteString(m.renderDetailRow("Parent:", s.ParentSessionID))
	}

	// Wrap in box
	style := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ColorGray).
		Padding(1, 2).
		MarginTop(1)

	return style.Render(b.String())
}

// renderDetailRow renders a label: value row in the detail panel.
func (m Model) renderDetailRow(label, value string) string {
	labelStyle := DimStyle.Width(12)
	return labelStyle.Render(label) + value + "\n"
}

// renderStatusBar renders the bottom status bar.
func (m Model) renderStatusBar() string {
	activeCount := 0
	completedCount := 0
	for _, s := range m.sessions {
		if s.HasPromise && !s.IsOrchestrator {
			completedCount++
		} else {
			activeCount++
		}
	}

	// Session counts
	counts := DimStyle.Render(fmt.Sprintf("%d active", activeCount))
	if completedCount > 0 {
		counts += DimStyle.Render(fmt.Sprintf(", %d completed", completedCount))
	}

	// Help text
	help := []string{
		HelpKeyStyle.Render("↑↓/jk") + " nav",
		HelpKeyStyle.Render("a/↵") + " attach",
		HelpKeyStyle.Render("s") + " spawn",
		HelpKeyStyle.Render("K") + " kill",
		HelpKeyStyle.Render("r") + " refresh",
		HelpKeyStyle.Render("q") + " quit",
	}
	helpLine := DimStyle.Render(strings.Join(help, "  "))

	// Second line
	returnHelp := DimStyle.Render("Return to TUI: ") + WarningStyle.Render("Ctrl-b L") + DimStyle.Render(" (last session)")

	completedHelp := ""
	if completedCount > 0 {
		completedHelp = "  " + HelpKeyStyle.Render("R") + " resume  " + HelpKeyStyle.Render("C") + " kill all completed"
	}

	// Separator
	sep := lipgloss.NewStyle().
		Border(lipgloss.NormalBorder(), true, false, false, false).
		BorderForeground(ColorGray).
		PaddingTop(1)

	var b strings.Builder
	b.WriteString(counts)
	b.WriteString(strings.Repeat(" ", 40-len(counts))) // Spacing
	b.WriteString(helpLine)
	b.WriteString("\n")
	b.WriteString(returnHelp)
	b.WriteString(completedHelp)

	return sep.Render(b.String())
}

// formatAge formats a time as a human-readable age string.
func formatAge(t time.Time) string {
	d := time.Since(t)

	if d < time.Minute {
		return fmt.Sprintf("%ds ago", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	}
	if d < 24*time.Hour {
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	}
	return fmt.Sprintf("%dd ago", int(d.Hours()/24))
}
