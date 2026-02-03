package tui

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/yourusername/oci-arm-provisioner/internal/config"
	"github.com/yourusername/oci-arm-provisioner/internal/logger"
	"github.com/yourusername/oci-arm-provisioner/internal/notifier"
)

// View represents different screens in the TUI
type View int

const (
	ViewDashboard View = iota
	ViewLogs
	ViewConfig
	ViewHelp
)

// AccountStatus represents the current state of an account
type AccountStatus struct {
	Name         string
	Region       string
	State        string // "running", "provisioned", "waiting", "error"
	InstanceID   string
	PublicIP     string
	OCPUs        float32
	MemoryGB     float32
	CapacityHits int
	LastError    string
	Provisioned  bool
}

// tickMsg is sent periodically to update the UI
type tickMsg time.Time

// statsUpdateMsg contains updated statistics
type statsUpdateMsg struct {
	Stats notifier.Stats
}

// KeyMap defines the keybindings for the TUI
type KeyMap struct {
	Quit      key.Binding
	Help      key.Binding
	Dashboard key.Binding
	Logs      key.Binding
	Config    key.Binding
	Pause     key.Binding
	Resume    key.Binding
	Up        key.Binding
	Down      key.Binding
	Enter     key.Binding
	Escape    key.Binding
	Tab       key.Binding
}

// DefaultKeyMap returns the default key bindings
func DefaultKeyMap() KeyMap {
	return KeyMap{
		Quit: key.NewBinding(
			key.WithKeys("q", "ctrl+c"),
			key.WithHelp("q", "quit"),
		),
		Help: key.NewBinding(
			key.WithKeys("?"),
			key.WithHelp("?", "help"),
		),
		Dashboard: key.NewBinding(
			key.WithKeys("d", "1"),
			key.WithHelp("d/1", "dashboard"),
		),
		Logs: key.NewBinding(
			key.WithKeys("l", "2"),
			key.WithHelp("l/2", "logs"),
		),
		Config: key.NewBinding(
			key.WithKeys("c", "3"),
			key.WithHelp("c/3", "config"),
		),
		Pause: key.NewBinding(
			key.WithKeys("p"),
			key.WithHelp("p", "pause"),
		),
		Resume: key.NewBinding(
			key.WithKeys("r"),
			key.WithHelp("r", "resume"),
		),
		Up: key.NewBinding(
			key.WithKeys("up", "k"),
			key.WithHelp("↑/k", "up"),
		),
		Down: key.NewBinding(
			key.WithKeys("down", "j"),
			key.WithHelp("↓/j", "down"),
		),
		Enter: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("enter", "select"),
		),
		Escape: key.NewBinding(
			key.WithKeys("esc"),
			key.WithHelp("esc", "back"),
		),
		Tab: key.NewBinding(
			key.WithKeys("tab"),
			key.WithHelp("tab", "next"),
		),
	}
}

// ShortHelp returns the short help bindings
func (k KeyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Help, k.Dashboard, k.Logs, k.Config, k.Pause, k.Quit}
}

// FullHelp returns the full help bindings
func (k KeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Dashboard, k.Logs, k.Config},
		{k.Pause, k.Resume},
		{k.Up, k.Down, k.Enter, k.Escape},
		{k.Help, k.Quit},
	}
}

// Model is the main TUI model
type Model struct {
	// Configuration
	Config  *config.Config
	Tracker *notifier.Tracker
	Runner  *ProvisionerRunner

	// UI State
	CurrentView View
	Width       int
	Height      int
	Ready       bool

	// Dashboard state
	Accounts    []AccountStatus
	SelectedIdx int
	Paused      bool
	StartTime   time.Time

	// Stats
	TotalCycles    int
	CapacityErrors int
	SuccessCount   int

	// Logs
	Logs               []LogEntry
	DashboardLogOffset int

	// Components
	Keys     KeyMap
	Styles   Styles
	Help     help.Model
	Viewport viewport.Model
	Spinner  spinner.Model
	Progress progress.Model

	// Context for cancellation
	ctx    context.Context
	cancel context.CancelFunc
}

// New creates a new TUI model
func New(cfg *config.Config, tracker *notifier.Tracker, runner *ProvisionerRunner) Model {
	ctx, cancel := context.WithCancel(context.Background())

	// Initialize accounts from config
	accounts := make([]AccountStatus, 0)
	for name, acc := range cfg.Accounts {
		if acc.Enabled {
			accounts = append(accounts, AccountStatus{
				Name:     name,
				Region:   acc.Region,
				State:    "waiting",
				OCPUs:    acc.OCPUs,
				MemoryGB: acc.MemoryGB,
			})
		}
	}

	// Initialize spinner
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("#7C3AED"))

	// Initialize progress bar
	prog := progress.New(progress.WithDefaultGradient())

	// Initialize viewport (will be sized in Init)
	vp := viewport.New(80, 20)

	return Model{
		Config:      cfg,
		Tracker:     tracker,
		Runner:      runner,
		CurrentView: ViewDashboard,
		Accounts:    accounts,
		StartTime:   time.Now(),
		Keys:        DefaultKeyMap(),
		Styles:      NewStyles(DefaultTheme),
		Help:        help.New(),
		Viewport:    vp,
		Spinner:     s,
		Progress:    prog,
		Logs:        make([]LogEntry, 0, 1000),
		ctx:         ctx,
		cancel:      cancel,
	}
}

// Init initializes the model
func (m Model) Init() tea.Cmd {
	// Start the provisioner runner
	if m.Runner != nil {
		m.Runner.Start(m.ctx)
	}

	cmds := []tea.Cmd{
		tickCmd(),
		m.Spinner.Tick,
	}

	// Listen for account updates and logs if runner is available
	if m.Runner != nil {
		cmds = append(cmds, accountUpdateCmd(m.Runner.StatusChan()))
		cmds = append(cmds, logUpdateCmd(m.Runner.LogChan()))
	}

	return tea.Batch(cmds...)
}

// tickCmd returns a command that sends a tick every second
func tickCmd() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

// Update handles messages and updates the model
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		// Handle window resize
		m.Width = msg.Width
		m.Height = msg.Height
		m.Ready = true
		m.Help.Width = msg.Width

		// Resize viewport for logs view
		headerHeight := 4
		footerHeight := 3
		verticalMargins := headerHeight + footerHeight
		m.Viewport.Width = msg.Width
		m.Viewport.Height = msg.Height - verticalMargins
		m.updateViewportContent()

	case tea.MouseMsg:
		// 1. Global Footer Click Handling (works in all views)
		// Approx bottom 6 lines
		if msg.Type == tea.MouseLeft && msg.Y >= m.Height-6 {
			// Check X coordinates
			// Start X ~ 4
			if msg.X >= 4 && msg.X < 14 { // Help
				if m.CurrentView == ViewHelp {
					m.CurrentView = ViewDashboard
				} else {
					m.CurrentView = ViewHelp
				}
			} else if msg.X >= 14 && msg.X < 28 { // Dashboard
				m.CurrentView = ViewDashboard
			} else if msg.X >= 28 && msg.X < 38 { // Logs
				m.CurrentView = ViewLogs
			} else if msg.X >= 38 && msg.X < 50 { // Config
				m.CurrentView = ViewConfig
			} else if msg.X >= 50 && msg.X < 62 { // Pause
				m.Paused = !m.Paused
				if m.Runner != nil {
					m.Runner.SetPaused(m.Paused)
				}
			} else if msg.X >= 62 && msg.X < 72 { // Quit
				m.cancel()
				return m, tea.Quit
			}
			// If clicked in footer, we handled it.
			return m, nil
		}

		// 2. View-Specific Mouse Handling
		if m.CurrentView == ViewDashboard {
			// Calculate Logs Pane Region
			// App Padding (1) + Header (3) + Middle (Variable) + Logs (8) + Footer (3) + App Padding (1)
			// We pin Logs to bottom: [Height-12, Height-4)
			logStart := m.Height - 12
			logEnd := m.Height - 4

			inLogs := msg.Y >= logStart && msg.Y < logEnd

			if msg.Type == tea.MouseWheelUp {
				if inLogs {
					m.DashboardLogOffset++
				} else {
					if m.SelectedIdx > 0 {
						m.SelectedIdx--
					}
				}
			} else if msg.Type == tea.MouseWheelDown {
				if inLogs {
					if m.DashboardLogOffset > 0 {
						m.DashboardLogOffset--
					}
				} else {
					if m.SelectedIdx < len(m.Accounts)-1 {
						m.SelectedIdx++
					}
				}
			}
		}

	case tea.KeyMsg:
		switch {
		case key.Matches(msg, m.Keys.Quit):
			m.cancel()
			return m, tea.Quit

		case key.Matches(msg, m.Keys.Help):
			if m.CurrentView == ViewHelp {
				m.CurrentView = ViewDashboard
			} else {
				m.CurrentView = ViewHelp
			}

		case key.Matches(msg, m.Keys.Dashboard) || msg.String() == "1":
			m.CurrentView = ViewDashboard

		case key.Matches(msg, m.Keys.Logs) || msg.String() == "2":
			m.CurrentView = ViewLogs

		case key.Matches(msg, m.Keys.Config) || msg.String() == "3":
			m.CurrentView = ViewConfig

		case key.Matches(msg, m.Keys.Pause):
			m.Paused = true
			if m.Runner != nil {
				m.Runner.SetPaused(true)
			}

		case key.Matches(msg, m.Keys.Resume):
			m.Paused = false
			if m.Runner != nil {
				m.Runner.SetPaused(false)
			}

		case key.Matches(msg, m.Keys.Up):
			if m.CurrentView == ViewDashboard && m.SelectedIdx > 0 {
				m.SelectedIdx--
			}

		case key.Matches(msg, m.Keys.Down):
			if m.CurrentView == ViewDashboard && m.SelectedIdx < len(m.Accounts)-1 {
				m.SelectedIdx++
			}

		case key.Matches(msg, m.Keys.Escape):
			m.CurrentView = ViewDashboard
		}

	case tickMsg:
		// Update stats from tracker
		if m.Tracker != nil {
			stats := m.Tracker.Snapshot()
			m.TotalCycles = stats.TotalCycles
			m.CapacityErrors = stats.CapacityErrors
			m.SuccessCount = stats.SuccessCount
		}
		return m, tickCmd()

	case statsUpdateMsg:
		// Update statistics
		m.TotalCycles = msg.Stats.TotalCycles
		m.CapacityErrors = msg.Stats.CapacityErrors
		m.SuccessCount = msg.Stats.SuccessCount

	case accountUpdateMsg:
		// Update account status from runner
		for i, acc := range m.Accounts {
			if acc.Name == msg.Name {
				m.Accounts[i] = msg.Status
				break
			}
		}
		// Continue listening for more updates
		if m.Runner != nil {
			return m, accountUpdateCmd(m.Runner.StatusChan())
		}

	case spinner.TickMsg:
		// Update spinner
		m.Spinner, cmd = m.Spinner.Update(msg)
		cmds = append(cmds, cmd)

	case logUpdateMsg:
		// Add new log entry
		m.Logs = append(m.Logs, LogEntry(msg))
		// Keep logs detailed but limited history
		if len(m.Logs) > 1000 {
			m.Logs = m.Logs[len(m.Logs)-1000:]
		}

		// Update viewport content
		m.updateViewportContent()
		// Auto-scroll to bottom if we were at bottom?
		// Viewport.SetContent usually resets to top.
		// We want logs to auto-scroll.
		m.Viewport.GotoBottom()

		// Continue listening for logs
		if m.Runner != nil {
			return m, logUpdateCmd(m.Runner.LogChan())
		}

	}

	// Update viewport if on logs view
	if m.CurrentView == ViewLogs {
		m.Viewport, cmd = m.Viewport.Update(msg)
		cmds = append(cmds, cmd)
	}

	if len(cmds) > 0 {
		return m, tea.Batch(cmds...)
	}
	return m, nil
}

// View renders the TUI
func (m Model) View() string {
	if !m.Ready {
		return "Initializing..."
	}

	var content string
	switch m.CurrentView {
	case ViewDashboard:
		content = m.viewDashboard()
	case ViewLogs:
		content = m.viewLogs()
	case ViewConfig:
		content = m.viewConfig()
	case ViewHelp:
		content = m.viewHelp()
	}

	return m.Styles.App.Width(m.Width - 4).Height(m.Height).Render(
		lipgloss.JoinVertical(
			lipgloss.Left,
			m.renderHeader(),
			content,
			m.renderFooter(),
		),
	)
}

// renderHeader renders the application header
func (m Model) renderHeader() string {
	// Title with gradient effect
	title := m.Styles.Title.Render("🚀 OCI ARM Provisioner")

	// Status indicator
	status := m.Styles.StatusRunning.Render("● Running")
	if m.Paused {
		status = m.Styles.StatusWaiting.Render("● Paused")
	}

	// Uptime
	uptime := time.Since(m.StartTime).Round(time.Second)
	uptimeStr := m.Styles.Label.Render("Uptime: ") + m.Styles.Value.Render(uptime.String())

	// Cycle count
	cycleStr := m.Styles.Label.Render("Cycle: ") + m.Styles.Value.Render(fmt.Sprintf("#%d", m.TotalCycles))

	// Build header
	left := lipgloss.JoinHorizontal(lipgloss.Center, title, "  ", status)
	right := lipgloss.JoinHorizontal(lipgloss.Center, uptimeStr, "  ", cycleStr)

	// Calculate spacing
	gap := strings.Repeat(" ", max(0, m.Width-lipgloss.Width(left)-lipgloss.Width(right)-8))

	return m.Styles.Header.Width(m.Width - 8).Render(
		lipgloss.JoinHorizontal(lipgloss.Center, left, gap, right),
	)
}

// renderFooter renders the application footer with clickable buttons
func (m Model) renderFooter() string {
	// Manual button rendering to match click zones
	// Spacing needs to align with mouse detection in Update
	// Start offset: ~2 (App Pad) + ~2 (Footer Pad) = 4 chars?
	// Actually styles.App has Padding(1, 2). Footer has Padding(1, 2).
	// Total left padding = 4 spaces.

	// Helper to style buttons
	btn := func(key, text string, width int, active bool) string {
		style := m.Styles.Muted
		if active {
			style = m.Styles.Label
		}
		// Pad to width
		label := fmt.Sprintf("%s %s", key, text)
		if len(label) < width {
			label = label + strings.Repeat(" ", width-len(label))
		}
		return style.Render(label)
	}

	// Button Layout (Widths must match Update logic somewhat)
	// Help (10): "? Help   "
	// Dash (14): "d/1 Dash     "
	// Logs (10): "l/2 Logs    "
	// Conf (12): "c/3 Conf    "
	// Pause(12): "p Pause     "
	// Quit (10): "q Quit      "

	content := lipgloss.JoinHorizontal(lipgloss.Left,
		btn("?", "Help", 10, m.CurrentView == ViewHelp),
		btn("d/1", "Dash", 14, m.CurrentView == ViewDashboard),
		btn("l/2", "Logs", 10, m.CurrentView == ViewLogs),
		btn("c/3", "Conf", 12, m.CurrentView == ViewConfig),
		btn("p", "Pause", 12, m.Paused),
		btn("q", "Quit", 10, false),
	)

	return m.Styles.Footer.Width(m.Width - 8).Render(content)
}

// viewLogs renders the log viewer with viewport
func (m Model) viewLogs() string {
	// Return viewport view (content updated in Update)
	return lipgloss.JoinVertical(lipgloss.Left,
		m.Styles.Title.Render("📋 Logs"),
		"",
		m.Viewport.View(),
	)
}

// updateViewportContent regenerates the logs string for the viewport
func (m *Model) updateViewportContent() {
	var content strings.Builder

	if len(m.Logs) == 0 {
		content.WriteString(m.Styles.Muted.Render("No logs yet..."))
	} else {
		for _, log := range m.Logs {
			timeStr := log.Time.Format("15:04:05")
			var levelStyle lipgloss.Style
			switch log.Level {
			case "success":
				levelStyle = m.Styles.StatusProvisioned
			case "error":
				levelStyle = m.Styles.StatusError
			case "warn":
				levelStyle = m.Styles.StatusWaiting
			default:
				levelStyle = m.Styles.Label
			}

			logLine := fmt.Sprintf("[%s] %s [%s] %s",
				timeStr,
				levelStyle.Render(log.Level),
				m.Styles.Highlight.Render(log.Account),
				log.Message,
			)
			content.WriteString(logLine + "\n")
		}
	}

	m.Viewport.SetContent(content.String())
}

// viewConfig renders the config editor (placeholder)
func (m Model) viewConfig() string {
	content := m.Styles.Title.Render("⚙️ Configuration") + "\n\n" +
		m.Styles.Muted.Render("Config editor coming soon...")

	// Force content to fill available vertical space to push Footer to bottom
	// Total chrome ~ 14 lines (Header 5 + Footer 5 + Padding/Margins 4)
	height := m.Height - 14
	if height < 0 {
		height = 0
	}

	return lipgloss.NewStyle().Height(height).Render(content)
}

// viewHelp renders the help screen
func (m Model) viewHelp() string {
	var content strings.Builder
	content.WriteString(m.Styles.Title.Render("❓ Help"))
	content.WriteString("\n\n")

	// Keybindings
	bindings := []struct{ key, desc string }{
		{"d / 1", "Dashboard view"},
		{"l / 2", "Log viewer"},
		{"c / 3", "Configuration"},
		{"p", "Pause provisioning"},
		{"r", "Resume provisioning"},
		{"↑/k", "Navigate up"},
		{"↓/j", "Navigate down"},
		{"?", "Toggle help"},
		{"q", "Quit"},
	}

	for _, b := range bindings {
		keyStyle := m.Styles.HelpKey.Render(fmt.Sprintf("%-8s", b.key))
		descStyle := m.Styles.HelpDesc.Render(b.desc)
		content.WriteString(keyStyle + " " + descStyle + "\n")
	}

	// Force content to fill available vertical space
	height := m.Height - 14
	if height < 0 {
		height = 0
	}

	return lipgloss.NewStyle().Height(height).Render(content.String())
}

// max returns the maximum of two integers
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// UpdateAccountStatus updates the status of an account
func (m *Model) UpdateAccountStatus(name string, status AccountStatus) {
	for i, acc := range m.Accounts {
		if acc.Name == name {
			m.Accounts[i] = status
			return
		}
	}
}

// Run starts the TUI application with full provisioner integration
func Run(cfg *config.Config, tracker *notifier.Tracker, l *logger.Logger) error {
	// 1. Silence console output to prevent TUI corruption
	// We'll restore it when TUI exits (though usually program exits then)
	l.SetConsoleOutput(io.Discard)

	// Create the provisioner runner
	runner := NewProvisionerRunner(cfg, l, tracker)

	// 2. Hook logger to TUI log channel
	// This captures logs from the provisioner (which uses l) and sends them to the TUI
	l.AddHook(func(level, account, msg string) {
		entry := LogEntry{
			Time:    time.Now(),
			Level:   strings.ToLower(level),
			Account: account,
			Message: msg,
		}
		// Non-blocking send to avoid blocking the provisioner
		select {
		case runner.logChan <- entry:
		default:
			// Drop log if channel full to prevent blocking
		}
	})

	// Create TUI model with runner
	model := New(cfg, tracker, runner)

	// Create and run the program
	p := tea.NewProgram(model, tea.WithAltScreen(), tea.WithMouseCellMotion())
	_, err := p.Run()

	// Stop the runner when TUI exits
	runner.Stop()

	return err
}
