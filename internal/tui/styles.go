package tui

import (
	"github.com/charmbracelet/lipgloss"
)

// Theme defines the color scheme for the TUI
type Theme struct {
	Primary    lipgloss.Color
	Secondary  lipgloss.Color
	Success    lipgloss.Color
	Warning    lipgloss.Color
	Error      lipgloss.Color
	Info       lipgloss.Color
	Background lipgloss.Color
	Surface    lipgloss.Color
	Text       lipgloss.Color
	TextMuted  lipgloss.Color
	Border     lipgloss.Color
	Accent     lipgloss.Color
	Gradient1  lipgloss.Color
	Gradient2  lipgloss.Color
}

// DefaultTheme returns a vibrant, modern theme optimized for local terminals
var DefaultTheme = Theme{
	Primary:    lipgloss.Color("#7C3AED"), // Violet
	Secondary:  lipgloss.Color("#06B6D4"), // Cyan
	Success:    lipgloss.Color("#10B981"), // Emerald
	Warning:    lipgloss.Color("#F59E0B"), // Amber
	Error:      lipgloss.Color("#EF4444"), // Red
	Info:       lipgloss.Color("#3B82F6"), // Blue
	Background: lipgloss.Color("#0F172A"), // Slate 900
	Surface:    lipgloss.Color("#1E293B"), // Slate 800
	Text:       lipgloss.Color("15"),      // White
	TextMuted:  lipgloss.Color("250"),     // Light Gray
	Border:     lipgloss.Color("240"),     // Gray
	Accent:     lipgloss.Color("13"),      // Magenta
	Gradient1:  lipgloss.Color("#7C3AED"), // Violet
	Gradient2:  lipgloss.Color("#06B6D4"), // Cyan
}

// Styles holds all the styled components
type Styles struct {
	// App-level
	App       lipgloss.Style
	Header    lipgloss.Style
	Footer    lipgloss.Style
	StatusBar lipgloss.Style

	// Cards
	Card        lipgloss.Style
	CardTitle   lipgloss.Style
	CardActive  lipgloss.Style
	CardSuccess lipgloss.Style
	CardWarning lipgloss.Style
	CardError   lipgloss.Style

	// Text
	Title     lipgloss.Style
	Subtitle  lipgloss.Style
	Label     lipgloss.Style
	Value     lipgloss.Style
	Highlight lipgloss.Style
	Muted     lipgloss.Style

	// Status indicators
	StatusRunning     lipgloss.Style
	StatusProvisioned lipgloss.Style
	StatusWaiting     lipgloss.Style
	StatusError       lipgloss.Style

	// Interactive
	Button       lipgloss.Style
	ButtonActive lipgloss.Style
	Input        lipgloss.Style
	InputFocused lipgloss.Style

	// Help
	HelpKey  lipgloss.Style
	HelpDesc lipgloss.Style
}

// NewStyles creates a new Styles instance with the given theme
func NewStyles(theme Theme) Styles {
	return Styles{
		// App-level
		App: lipgloss.NewStyle().
			Foreground(theme.Text).
			Padding(1, 2),

		Header: lipgloss.NewStyle().
			Bold(true).
			Foreground(theme.Text).
			Background(theme.Surface).
			Padding(1, 2).
			MarginBottom(1).
			Border(lipgloss.RoundedBorder(), false, false, true, false).
			BorderForeground(theme.Border),

		Footer: lipgloss.NewStyle().
			Foreground(theme.Text).
			Background(theme.Surface).
			Padding(1, 2).
			MarginTop(1).
			Border(lipgloss.RoundedBorder(), true, false, false, false).
			BorderForeground(theme.Border),

		StatusBar: lipgloss.NewStyle().
			Foreground(theme.Text).
			Background(theme.Surface).
			Padding(1, 2),

		// Cards
		Card: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(theme.Border).
			Padding(1, 2).
			MarginRight(2).
			MarginBottom(1).
			Width(24),

		CardTitle: lipgloss.NewStyle().
			Bold(true).
			Foreground(theme.Accent).
			MarginBottom(1).
			Underline(true),

		CardActive: lipgloss.NewStyle().
			Border(lipgloss.ThickBorder()).
			BorderForeground(theme.Primary).
			Padding(1, 2).
			MarginRight(2).
			MarginBottom(1).
			Width(24),

		CardSuccess: lipgloss.NewStyle().
			Border(lipgloss.DoubleBorder()).
			BorderForeground(theme.Success).
			Padding(1, 2).
			MarginRight(2).
			MarginBottom(1).
			Width(24),

		CardWarning: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(theme.Warning).
			Padding(1, 2).
			MarginRight(2).
			MarginBottom(1).
			Width(24),

		CardError: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(theme.Error).
			Padding(1, 2).
			MarginRight(2).
			MarginBottom(1).
			Width(24),

		// Text
		Title: lipgloss.NewStyle().
			Bold(true).
			Foreground(theme.Primary).
			MarginBottom(1).
			Padding(0, 1),

		Subtitle: lipgloss.NewStyle().
			Foreground(theme.TextMuted).
			Italic(true).
			Padding(0, 1),

		Label: lipgloss.NewStyle().
			Foreground(theme.TextMuted),

		Value: lipgloss.NewStyle().
			Foreground(theme.Text).
			Bold(true),

		Highlight: lipgloss.NewStyle().
			Foreground(theme.Accent).
			Bold(true),

		Muted: lipgloss.NewStyle().
			Foreground(theme.TextMuted),

		// Status indicators
		StatusRunning: lipgloss.NewStyle().
			Foreground(theme.Info).
			Bold(true),

		StatusProvisioned: lipgloss.NewStyle().
			Foreground(theme.Success).
			Bold(true),

		StatusWaiting: lipgloss.NewStyle().
			Foreground(theme.Warning),

		StatusError: lipgloss.NewStyle().
			Foreground(theme.Error),

		// Interactive
		Button: lipgloss.NewStyle().
			Foreground(theme.Text).
			Background(theme.Surface).
			Padding(0, 3).
			MarginRight(1),

		ButtonActive: lipgloss.NewStyle().
			Foreground(theme.Background).
			Background(theme.Primary).
			Padding(0, 3).
			MarginRight(1).
			Bold(true),

		Input: lipgloss.NewStyle().
			Border(lipgloss.NormalBorder()).
			BorderForeground(theme.Border).
			Padding(0, 1),

		InputFocused: lipgloss.NewStyle().
			Border(lipgloss.NormalBorder()).
			BorderForeground(theme.Primary).
			Padding(0, 1),

		// Help
		HelpKey: lipgloss.NewStyle().
			Foreground(theme.Accent).
			Bold(true),

		HelpDesc: lipgloss.NewStyle().
			Foreground(theme.Text),
	}
}

// Gradient creates a simple gradient effect between two colors
func Gradient(text string, from, to lipgloss.Color) string {
	return lipgloss.NewStyle().
		Foreground(from).
		Render(text)
}

// Icon constants for status display
const (
	IconSuccess = "✅"
	IconError   = "❌"
	IconWarning = "⚠️"
	IconInfo    = "ℹ️"
	IconRunning = "🔄"
	IconWaiting = "⏳"
	IconRocket  = "🚀"
	IconCheck   = "✓"
	IconCross   = "✗"
	IconDot     = "●"
	IconArrow   = "→"
	IconServer  = "🖥️"
	IconCloud   = "☁️"
	IconKey     = "🔑"
	IconGear    = "⚙️"
	IconChart   = "📊"
	IconLogs    = "📋"
	IconHelp    = "❓"
)
