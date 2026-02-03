package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// viewDashboard renders the main dashboard using a split-view layout
func (m Model) viewDashboard() string {
	// Calculate widths - ensure total fits within terminal
	// Account for App padding (4 chars = 2 left + 2 right)
	availableWidth := max(60, m.Width-4)

	// Split: 25% for accounts, 75% for details
	accountsWidth := max(20, availableWidth*25/100)
	detailsWidth := availableWidth - accountsWidth - 1 // -1 for gap

	// Calculate Heights
	// Total chrome height: 1 (App Top) + 5 (Header) + 9 (Logs incl margin) + 5 (Footer) + 1 (App Bottom) = 21
	// We add 4 extra safety buffer lines -> 25
	middleHeight := m.Height - 25
	if middleHeight < 5 {
		middleHeight = 5 // Minimum safety
	}

	// 1. Render Accounts List (Left Pane)
	accountsPane := m.renderAccountsList(accountsWidth, middleHeight)

	// 2. Render Details/Stats (Right Pane)
	detailsPane := m.renderDetailsPane(detailsWidth, middleHeight)

	// Join horizontally with a small gap
	middle := lipgloss.JoinHorizontal(lipgloss.Top, accountsPane, " ", detailsPane)

	// Force middle section to fill available space (but not exceed)
	middleContent := lipgloss.NewStyle().Height(middleHeight).MaxHeight(middleHeight).Render(middle)

	// 3. Render Logs Pane (Bottom)
	logsPane := m.renderLogsPane(8, availableWidth)

	return lipgloss.JoinVertical(lipgloss.Left, middleContent, logsPane)
}

// renderAccountsList renders the interactive list of accounts
func (m Model) renderAccountsList(width, height int) string {
	var rows []string

	// Header
	header := m.Styles.Subtitle.Render("ACCOUNTS")
	rows = append(rows, header)
	rows = append(rows, "") // Spacer

	// List Items
	if len(m.Accounts) == 0 {
		rows = append(rows, m.Styles.Muted.Render("(No accounts)"))
	}

	for i, acc := range m.Accounts {
		cursor := "  "
		style := m.Styles.Muted
		icon := IconDot

		if m.SelectedIdx == i {
			cursor = "→ "
			style = m.Styles.Highlight
		}

		// Status Icon
		var statusStyle lipgloss.Style
		switch acc.State {
		case "provisioned":
			statusStyle = m.Styles.StatusProvisioned
			icon = IconSuccess
		case "running":
			statusStyle = m.Styles.StatusRunning
			icon = IconRunning
		case "waiting":
			statusStyle = m.Styles.StatusWaiting
			icon = IconWaiting
		case "error":
			statusStyle = m.Styles.StatusError
			icon = IconError
		default:
			statusStyle = m.Styles.StatusWaiting
		}

		row := fmt.Sprintf("%s%s %s", cursor, statusStyle.Render(icon), style.Render(acc.Name))
		rows = append(rows, row)
	}

	// Build content
	content := strings.Join(rows, "\n")

	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("240")).
		Padding(0, 1).
		Width(width - 4).
		MaxWidth(width - 4).
		Height(height - 2). // Account for border
		Render(content)
}

// renderDetailsPane renders the selected account details and global stats
func (m Model) renderDetailsPane(width, height int) string {
	// Global Stats at the top
	statsBar := m.renderStatsBarInline()

	// Selected Account Details
	var details string
	if len(m.Accounts) > 0 && m.SelectedIdx < len(m.Accounts) {
		acc := m.Accounts[m.SelectedIdx]

		title := m.Styles.Title.Render(acc.Name)

		grid := []string{
			fmt.Sprintf("%s %s", m.Styles.Label.Render("Region:"), m.Styles.Value.Render(acc.Region)),
			fmt.Sprintf("%s %s", m.Styles.Label.Render("Status:"), m.renderStatusBadge(acc.State)),
			fmt.Sprintf("%s %s", m.Styles.Label.Render("Specs: "), m.Styles.Value.Render(fmt.Sprintf("%.0f OCPU / %.0f GB", acc.OCPUs, acc.MemoryGB))),
			"",
			fmt.Sprintf("%s %d", m.Styles.Label.Render("Errors:"), acc.CapacityHits),
		}

		details = lipgloss.JoinVertical(lipgloss.Left,
			title,
			strings.Join(grid, "\n"),
		)
	} else {
		details = m.Styles.Muted.Render("Select an account to view details")
	}

	// Separator width (account for border/padding)
	sepWidth := max(20, width-6)
	separator := strings.Repeat("─", sepWidth)

	content := lipgloss.JoinVertical(lipgloss.Left,
		statsBar,
		"",
		m.Styles.Muted.Render(separator),
		"",
		details,
	)

	// Fixed width with border
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("240")).
		Padding(0, 1).
		Width(width - 4).
		MaxWidth(width - 4).
		Height(height - 2). // Account for border
		Render(content)
}

// renderStatsBarInline renders the stats in a clean inline format
func (m Model) renderStatsBarInline() string {
	return fmt.Sprintf("%s %s   %s %s   %s %s",
		m.Styles.StatusProvisioned.Render(IconSuccess+" Provisioned:"),
		m.Styles.Value.Render(fmt.Sprintf("%d", m.SuccessCount)),
		m.Styles.StatusWaiting.Render(IconWarning+" Hits:"),
		m.Styles.Value.Render(fmt.Sprintf("%d", m.CapacityErrors)),
		m.Styles.StatusRunning.Render(IconChart+" Cycles:"),
		m.Styles.Value.Render(fmt.Sprintf("%d", m.TotalCycles)),
	)
}

// renderLogsPane renders the logs viewport
func (m Model) renderLogsPane(height int, width int) string {
	// Just use the last N logs manually for now if viewport is complex
	// But we have m.Viewport for ViewLogs.
	// For this mini-view, let's just show standard logs.

	var lines []string
	visibleCount := height - 2
	total := len(m.Logs)

	// Calculate range based on offset (offset 0 = latest)
	end := total - m.DashboardLogOffset
	if end > total {
		end = total
	}
	if end < 0 {
		end = 0
	}

	start := end - visibleCount
	if start < 0 {
		start = 0
	}

	visibleLogs := m.Logs[start:end]
	for _, l := range visibleLogs {
		// Format: Time [Level] Msg
		ts := l.Time.Format("15:04:05")

		var levelStyle lipgloss.Style
		switch l.Level {
		case "info":
			levelStyle = m.Styles.StatusRunning
		case "warn":
			levelStyle = m.Styles.StatusWaiting
		case "error":
			levelStyle = m.Styles.StatusError
		case "success":
			levelStyle = m.Styles.StatusProvisioned
		default:
			levelStyle = m.Styles.Muted
		}

		// Truncate message if it's too long
		// Time (8) + Space + Level (~7) + Space + Account (~12) + Space + Msg
		// Reserved ~ 30 chars. Style padding/border adds more.
		// Content width is width-8.
		// Safe buffer: 45 chars for metadata.
		msgMaxWidth := max(10, width-45)
		msg := l.Message
		if len(msg) > msgMaxWidth {
			msg = msg[:msgMaxWidth] + "..."
		}

		line := fmt.Sprintf("%s %s %s %s",
			m.Styles.Muted.Render(ts),
			levelStyle.Render("["+strings.ToUpper(l.Level)+"]"),
			m.Styles.Highlight.Render("["+l.Account+"]"),
			msg,
		)
		lines = append(lines, line)
	}

	// Fill
	for len(lines) < height-2 {
		lines = append(lines, "")
	}

	return m.Styles.Card.
		Width(width - 8).
		MaxWidth(width - 8).
		Height(height).
		Border(lipgloss.RoundedBorder()).
		BorderTop(true).
		Render(lipgloss.JoinVertical(lipgloss.Left,
			m.Styles.Subtitle.Render("Activity Log"),
			strings.Join(lines, "\n"),
		))
}

func (m Model) renderStatusBadge(state string) string {
	switch state {
	case "provisioned":
		return m.Styles.StatusProvisioned.Render("PROVISIONED")
	case "running":
		return m.Styles.StatusRunning.Render("RUNNING")
	case "waiting":
		return m.Styles.StatusWaiting.Render("WAITING")
	case "error":
		return m.Styles.StatusError.Render("ERROR")
	}
	return m.Styles.Muted.Render(strings.ToUpper(state))
}
