package tui

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/lipgloss"

	"github.com/Jayphen/coders/internal/tmux"
	"github.com/Jayphen/coders/internal/types"
)

// widthCache caches ANSI-aware width calculations to avoid repeated lipgloss.Width calls.
var (
	widthCache   = make(map[string]int)
	widthCacheMu sync.RWMutex
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

	// Column headers (widths match row: 3 + 24 + 10 + 24 + status)
	headers := fmt.Sprintf(" %-3s%-24s%-10s%-24s%s",
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

// renderMainContent renders the split view (session list + detail/preview panel).
func (m Model) renderMainContent(maxHeight int) string {
	list := m.renderSessionList()
	availableWidth := m.contentWidth()

	if availableWidth == 0 {
		if maxHeight > 0 {
			listHeight := maxHeight
			rightHeight := 0
			if listHeight > 2 {
				rightHeight = listHeight / 2
				listHeight = listHeight - rightHeight - 1
			}
			if listHeight < 1 {
				listHeight = 1
			}
			list = truncateLines(list, listHeight, DimStyle.Render("..."))
			if rightHeight <= 0 {
				return list
			}
			right := m.renderRightPanel(0, rightHeight)
			if right == "" {
				return list
			}
			return list + "\n" + right
		}
		right := m.renderRightPanel(0, maxHeight)
		return list + "\n" + right
	}

	const (
		gap      = 2
		minLeft  = 72
		minRight = 32
	)

	if availableWidth < minLeft+minRight+gap {
		if maxHeight > 0 {
			listHeight := maxHeight
			rightHeight := 0
			if listHeight > 2 {
				rightHeight = listHeight / 2
				listHeight = listHeight - rightHeight - 1
			}
			if listHeight < 1 {
				listHeight = 1
			}
			list = truncateLines(list, listHeight, DimStyle.Render("..."))
			if rightHeight <= 0 {
				return list
			}
			right := m.renderRightPanel(availableWidth, rightHeight)
			if right == "" {
				return list
			}
			return list + "\n" + right
		}
		right := m.renderRightPanel(availableWidth, maxHeight)
		return list + "\n" + right
	}

	leftWidth := minLeft
	rightWidth := availableWidth - leftWidth - gap

	if maxHeight > 0 {
		list = truncateLines(list, maxHeight, DimStyle.Render("..."))
	}
	left := lipgloss.NewStyle().Width(leftWidth).Render(list)
	right := m.renderRightPanel(rightWidth, maxHeight)

	return lipgloss.JoinHorizontal(lipgloss.Top, left, strings.Repeat(" ", gap), right)
}

func (m Model) contentWidth() int {
	if m.width <= 0 {
		return 0
	}
	width := m.width - 2
	if width < 0 {
		return 0
	}
	return width
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

	// Select pre-cached name style based on session state
	var nameStyle lipgloss.Style
	if s.IsOrchestrator {
		nameStyle = NameStyleOrchestrator
	} else if s.HasPromise {
		nameStyle = NameStyleCompleted
	} else if isSelected {
		nameStyle = NameStyleSelected
	} else {
		nameStyle = NameStyleDefault
	}

	// Truncate name
	maxNameLen := 20
	if s.IsOrchestrator {
		maxNameLen = 22
	}
	if len(displayName) > maxNameLen {
		displayName = displayName[:maxNameLen-3] + "..."
	}
	namePart := nameStyle.Render(prefix + displayName)

	// Tool - use pre-cached tool style
	var toolPart string
	if s.HasPromise {
		toolPart = GetToolStyleDimmed(s.Tool).Render(s.Tool)
	} else {
		toolPart = GetToolStyle(s.Tool).Render(s.Tool)
	}

	// Task/Summary
	displayText := s.Task
	if s.Promise != nil {
		displayText = s.Promise.Summary
	}
	if displayText == "" {
		displayText = "-"
	}
	if len(displayText) > 20 {
		displayText = displayText[:17] + "..."
	}
	// Use pre-cached task style
	var taskPart string
	if s.HasPromise || displayText == "-" {
		taskPart = TaskStyleDimmed.Render(displayText)
	} else {
		taskPart = TaskStyleDefault.Render(displayText)
	}

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
	} else if s.HealthCheck != nil && (s.HealthCheck.Status == types.HealthStuck || s.HealthCheck.Status == types.HealthUnresponsive) {
		// Show stuck/unresponsive from health check
		switch s.HealthCheck.Status {
		case types.HealthStuck:
			statusPart = StatusStuck.Render(IndicatorStuck)
		case types.HealthUnresponsive:
			statusPart = StatusUnresponsive.Render(IndicatorUnresponsive)
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

	// Format row with fixed widths using lipgloss padding
	// (fmt.Sprintf doesn't work with ANSI-styled strings)
	namePadded := padRight(namePart, 24)
	toolPadded := padRight(toolPart, 10)
	taskPadded := padRight(taskPart, 24)

	return selector + namePadded + toolPadded + taskPadded + statusPart
}

// padRight pads a string to the specified visible width.
// Uses a cache to avoid repeated ANSI-aware width calculations.
func padRight(s string, width int) string {
	// Check cache first
	widthCacheMu.RLock()
	visibleWidth, cached := widthCache[s]
	widthCacheMu.RUnlock()

	if !cached {
		// Calculate and cache the width
		visibleWidth = lipgloss.Width(s)
		widthCacheMu.Lock()
		widthCache[s] = visibleWidth
		widthCacheMu.Unlock()
	}

	if visibleWidth >= width {
		return s
	}
	return s + strings.Repeat(" ", width-visibleWidth)
}

func truncateLines(s string, maxLines int, suffix string) string {
	if maxLines <= 0 {
		return ""
	}
	lines := strings.Split(s, "\n")
	if len(lines) <= maxLines {
		return s
	}
	if suffix != "" {
		if maxLines == 1 {
			return suffix
		}
		lines = lines[:maxLines-1]
		lines = append(lines, suffix)
		return strings.Join(lines, "\n")
	}
	return strings.Join(lines[:maxLines], "\n")
}

func tailLines(s string, maxLines int) string {
	if maxLines <= 0 {
		return ""
	}
	lines := strings.Split(s, "\n")
	if len(lines) <= maxLines {
		return s
	}
	return strings.Join(lines[len(lines)-maxLines:], "\n")
}

// tailLinesFromSlice is an optimized version that operates on pre-split lines.
// This avoids re-splitting the same text on every render.
func tailLinesFromSlice(lines []string, maxLines int) string {
	if maxLines <= 0 {
		return ""
	}
	if len(lines) <= maxLines {
		return strings.Join(lines, "\n")
	}
	return strings.Join(lines[len(lines)-maxLines:], "\n")
}

// renderSessionDetail renders the detail panel for the selected session.
func (m Model) renderSessionDetail(width int) string {
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

	// Health check info (if stuck or unresponsive)
	if s.HealthCheck != nil && (s.HealthCheck.Status == types.HealthStuck || s.HealthCheck.Status == types.HealthUnresponsive) {
		b.WriteString("\n")
		var healthStyle lipgloss.Style
		var healthLabel string
		switch s.HealthCheck.Status {
		case types.HealthStuck:
			healthStyle = StatusStuck
			healthLabel = IndicatorStuck + " Stuck"
		case types.HealthUnresponsive:
			healthStyle = StatusUnresponsive
			healthLabel = IndicatorUnresponsive + " Unresponsive"
		}
		b.WriteString(m.renderDetailRow("Health:", healthStyle.Render(healthLabel)))
		if s.HealthCheck.Message != "" {
			b.WriteString(m.renderDetailRow("", DimStyle.Render(s.HealthCheck.Message)))
		}
	}

	// Wrap in box
	style := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ColorGray).
		Padding(1, 2).
		MarginTop(1)
	if width > 0 {
		style = style.Width(width)
	}

	return style.Render(b.String())
}

// renderRightPanel renders the detail panel and preview panel.
func (m Model) renderRightPanel(width int, maxHeight int) string {
	detail := m.renderSessionDetail(width)
	if maxHeight > 0 {
		detail = truncateLines(detail, maxHeight, "")
	}
	detailHeight := lipgloss.Height(detail)

	previewHeight := 0
	if maxHeight > 0 {
		previewHeight = maxHeight - detailHeight
	}

	preview := m.renderSessionPreview(width, previewHeight)
	if preview == "" {
		return detail
	}
	return lipgloss.JoinVertical(lipgloss.Top, detail, preview)
}

// renderSessionPreview renders a live preview of the selected session output.
func (m Model) renderSessionPreview(width int, maxHeight int) string {
	s := m.selectedSession()
	title := "Preview"
	if s != nil {
		displayName := s.Name
		if s.IsOrchestrator {
			displayName = "orchestrator"
		} else {
			displayName = strings.TrimPrefix(s.Name, tmux.SessionPrefix)
		}
		title = "Preview: " + displayName
	}

	content := m.preview
	if m.previewLoading {
		if strings.TrimSpace(content) == "" {
			content = DimStyle.Render("Loading preview...")
		} else {
			content = content + "\n" + DimStyle.Render("(updating...)")
		}
	} else if m.previewErr != nil {
		content = ErrorStyle.Render("Preview unavailable")
	} else if strings.TrimSpace(content) == "" {
		content = DimStyle.Render("No output yet")
	}

	borderColor := ColorBlue
	headerStyle := BoldStyle
	inputLabelStyle := DimStyle
	if m.previewFocus {
		borderColor = ColorCyan
		headerStyle = lipgloss.NewStyle().Bold(true).Foreground(ColorCyan)
		inputLabelStyle = HelpKeyStyle
	}

	style := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(borderColor).
		Padding(1, 2).
		MarginTop(1)
	if width > 0 {
		style = style.Width(width)
	}

	header := headerStyle.Render(title)
	inputModel := m.previewInput
	if width > 0 {
		const (
			borderWidth     = 2
			paddingWidth    = 4
			minInputWidth   = 10
			inputLabelWidth = 6
		)
		inputWidth := width - borderWidth - paddingWidth - inputLabelWidth
		if inputWidth < minInputWidth {
			inputWidth = minInputWidth
		}
		inputModel.Width = inputWidth
	}
	inputLine := inputLabelStyle.Render("Send: ") + inputModel.View()
	if maxHeight > 0 {
		const (
			previewMarginTop = 1
			previewBorder    = 2
			previewPadding   = 2
			previewHeader    = 1
			previewHeaderGap = 1
			previewInputGap  = 1
			previewInputLine = 1
			minContentLines  = 0
		)
		overhead := previewMarginTop + previewBorder + previewPadding + previewHeader + previewHeaderGap + previewInputGap + previewInputLine
		maxContentLines := maxHeight - overhead
		if maxContentLines < minContentLines {
			return ""
		}
		// Use cached split lines if available and content hasn't been modified
		if content == m.previewSplitText && m.previewSplitLines != nil {
			content = tailLinesFromSlice(m.previewSplitLines, maxContentLines)
		} else {
			content = tailLines(content, maxContentLines)
		}
	}

	return style.Render(header + "\n\n" + content + "\n\n" + inputLine)
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
	var countsBuilder strings.Builder
	countsBuilder.WriteString(DimStyle.Render(fmt.Sprintf("%d active", activeCount)))
	if completedCount > 0 {
		countsBuilder.WriteString(DimStyle.Render(fmt.Sprintf(", %d completed", completedCount)))
	}
	counts := countsBuilder.String()

	// Help text
	help := []string{
		HelpKeyStyle.Render("↑↓/jk") + " nav",
		HelpKeyStyle.Render("tab") + " focus",
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

	// Calculate spacing using visible width, not byte length
	countsWidth := lipgloss.Width(counts)
	spacing := 40 - countsWidth
	if spacing < 2 {
		spacing = 2
	}

	var b strings.Builder

	// Status message (if present)
	if m.statusMessage != "" && !m.confirmKill {
		statusLine := StatusMsgStyle.Render(m.statusMessage)
		b.WriteString(statusLine)
		b.WriteString("\n")
	}

	b.WriteString(counts)
	b.WriteString(strings.Repeat(" ", spacing))
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
