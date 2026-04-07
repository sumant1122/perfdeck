package ui

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/sumant1122/perfdeck/internal/config"
	"github.com/sumant1122/perfdeck/internal/monitor"
	"github.com/sumant1122/perfdeck/internal/theme"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type tickMsg time.Time
type spinnerMsg time.Time

type cmdResultMsg struct {
	output string
	err    error
}

type metricsMsg struct {
	metrics monitor.MetricsSample
}

type systemMsg struct {
	info monitor.SystemInfo
}

const (
	spinnerInterval = 200 * time.Millisecond
	fixedRows       = 9
	keyCtrlC        = "ctrl+c"
)

var spinnerFrames = []string{"|", "/", "-", "\\"}

type Model struct {
	tabs       []config.Tab
	active     int
	viewport   viewport.Model
	content    string
	statusLine string
	metrics    monitor.MetricHistory
	system     monitor.SystemInfo
	themeIndex int
	spinnerIdx int
	width      int
	height     int
	styles     theme.Styles
}

func NewModel() Model {
	vp := viewport.New(0, 0)
	vp.SetContent("Loading...")

	_, tabs := config.Load()

	return Model{
		tabs:       tabs,
		active:     0,
		viewport:   vp,
		themeIndex: 0,
		styles:     theme.BuildStyles(0),
	}
}

func (m Model) Init() tea.Cmd {
	interval := m.tabs[m.active].RefreshInterval.Duration
	if m.tabs[m.active].Disabled {
		m.content = m.tabs[m.active].DisabledMsg
		m.viewport.SetContent(m.content)
		return tea.Batch(tick(interval), spinnerTick(), sampleMetricsCmd(), sampleSystemCmd())
	}
	return tea.Batch(runCommandCmd(m.tabs[m.active]), tick(interval), spinnerTick(), sampleMetricsCmd(), sampleSystemCmd())
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	interval := m.tabs[m.active].RefreshInterval.Duration

	switch msg := msg.(type) {
	case tea.KeyMsg:
		if isQuitKey(msg) {
			return m, tea.Quit
		}
		switch msg.String() {
		case keyCtrlC:
			return m, tea.Quit
		case "right", "l", "tab":
			m.active = (m.active + 1) % len(m.tabs)
			return m, m.onTabSelected()
		case "left", "h", "shift+tab":
			m.active--
			if m.active < 0 {
				m.active = len(m.tabs) - 1
			}
			return m, m.onTabSelected()
		case "t":
			m.themeIndex = (m.themeIndex + 1) % len(theme.Themes)
			m.styles = theme.BuildStyles(m.themeIndex)
			return m, nil
		}
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.viewport.Width = clampMin(msg.Width-2, 0)
		m.viewport.Height = clampMin(msg.Height-fixedRows, 0)
		m.viewport.SetContent(m.content)
	case tickMsg:
		if m.tabs[m.active].Disabled {
			return m, tea.Batch(tick(interval), sampleMetricsCmd(), sampleSystemCmd())
		}
		return m, tea.Batch(runCommandCmd(m.tabs[m.active]), tick(interval), sampleMetricsCmd(), sampleSystemCmd())
	case spinnerMsg:
		m.spinnerIdx = (m.spinnerIdx + 1) % len(spinnerFrames)
		return m, spinnerTick()
	case cmdResultMsg:
		m.content = sanitizeOutput(strings.TrimSpace(msg.output))
		if m.content == "" {
			m.content = "(no output)"
		}
		m.viewport.SetContent(m.content)
		if msg.err != nil {
			m.statusLine = fmt.Sprintf("error: %v", msg.err)
		} else {
			m.statusLine = fmt.Sprintf("updated %s (every %s)", time.Now().Format("15:04:05"), interval)
		}
	case metricsMsg:
		m.metrics = monitor.UpdateHistory(m.metrics, msg.metrics)
	case systemMsg:
		m.system = msg.info
	}

	var cmd tea.Cmd
	m.viewport, cmd = m.viewport.Update(msg)
	return m, cmd
}

func (m Model) View() string {
	header := m.renderTabs(m.tabs, m.active, m.width)
	metricsRow := m.renderMetricsRow(m.metrics, m.width)
	systemRow := m.renderSystemRow(m.system, m.width)
	title := m.renderContentTitle(m.tabs[m.active].Title, m.width)
	content := m.styles.ContentBox.Width(m.width).Render(m.viewport.View())
	footer := m.renderFooter(m.statusLine, spinnerFrames[m.spinnerIdx], m.width)

	return lipgloss.JoinVertical(
		lipgloss.Left,
		header,
		metricsRow,
		systemRow,
		title,
		content,
		footer,
	)
}

func (m Model) onTabSelected() tea.Cmd {
	if m.tabs[m.active].Disabled {
		m.content = m.tabs[m.active].DisabledMsg
		m.viewport.SetContent(m.content)
		m.statusLine = "disabled"
		return nil
	}
	m.content = "Loading..."
	m.viewport.SetContent(m.content)
	return runCommandCmd(m.tabs[m.active])
}

func tick(d time.Duration) tea.Cmd {
	return tea.Tick(d, func(t time.Time) tea.Msg { return tickMsg(t) })
}

func spinnerTick() tea.Cmd {
	return tea.Tick(spinnerInterval, func(t time.Time) tea.Msg { return spinnerMsg(t) })
}

func sampleMetricsCmd() tea.Cmd {
	return func() tea.Msg {
		return metricsMsg{metrics: monitor.SampleMetrics()}
	}
}

func sampleSystemCmd() tea.Cmd {
	return func() tea.Msg {
		return systemMsg{info: monitor.SampleSystem()}
	}
}

func runCommandCmd(t config.Tab) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
		defer cancel()

		cmd := exec.CommandContext(ctx, t.Cmd[0], t.Cmd[1:]...)
		var out bytes.Buffer
		cmd.Stdout = &out
		cmd.Stderr = &out

		err := cmd.Run()
		return cmdResultMsg{output: out.String(), err: err}
	}
}

// Rendering helpers

// renderMetricsRow renders the top row of sparklines and current values
func (m Model) renderMetricsRow(history monitor.MetricHistory, width int) string {
	if width <= 0 {
		return ""
	}

	// Helper to render a single metric block with color
	renderBlock := func(label string, valStr string, data []float64, min, max float64, isPercent bool) string {
		// Determine color based on latest value
		var color lipgloss.Style
		if len(data) > 0 {
			last := data[len(data)-1]
			// Normalize value for color mapping
			param := last
			if !isPercent {
				// efficient approximation for load/net: reasonable max
				// load: max 4.0 (green), 8.0 (yellow), >8.0 (red)
				// net: max 1MB/s (green), 10MB/s (yellow), >10MB/s (red)
				// This is heuristic, percent is easier
				if max > 0 {
					param = (last / max) * 100
				}
			}

			if param < 50 {
				color = m.styles.Green
			} else if param < 80 {
				color = m.styles.Yellow
			} else {
				color = m.styles.Red
			}
		} else {
			color = m.styles.Processing
		}

		sl := sparkline(data, min, max)
		// Colorize the sparkline and the value
		return fmt.Sprintf("%s %s %s", label, color.Render(valStr), color.Render(sl))
	}

	var blocks []string

	// CPU
	if len(history.CPU) > 0 {
		val := history.CPU[len(history.CPU)-1]
		blocks = append(blocks, renderBlock("CPU", fmt.Sprintf("%0.0f%%", val), history.CPU, 0, 100, true))
	}

	// MEM
	if len(history.Mem) > 0 {
		val := history.Mem[len(history.Mem)-1]
		blocks = append(blocks, renderBlock("MEM", fmt.Sprintf("%0.0f%%", val), history.Mem, 0, 100, true))
	}

	// LOAD (heuristic color: <1.0 green, <high yellow, >high red)
	if len(history.Load) > 0 {
		val := history.Load[len(history.Load)-1]
		max := maxFloat(history.Load)
		if max < 2.0 {
			max = 2.0
		} // Minimum scale for load

		// Custom logic for load color
		var color lipgloss.Style
		if val < 1.0 {
			color = m.styles.Green
		} else if val < 4.0 {
			color = m.styles.Yellow
		} else {
			color = m.styles.Red
		}

		sl := sparkline(history.Load, 0, max)
		blocks = append(blocks, fmt.Sprintf("LOAD %s %s", color.Render(fmt.Sprintf("%0.2f", val)), color.Render(sl)))
	}

	// NET
	if len(history.Net) > 0 {
		val := history.Net[len(history.Net)-1]
		max := maxFloat(history.Net)
		if max < 1 {
			max = 1
		}
		blocks = append(blocks, renderBlock("NET", monitor.FormatRate(val), history.Net, 0, max, false))
	}

	if len(blocks) == 0 {
		return m.styles.Summary.Width(width).Render("Waiting for metrics...")
	}

	row := strings.Join(blocks, "   ")
	return m.styles.Summary.Width(width).Render(row)
}

func (m Model) renderTabs(tabs []config.Tab, active, width int) string {
	if width <= 0 {
		return ""
	}
	rendered := make([]string, 0, len(tabs))
	renderedWidths := make([]int, 0, len(tabs))
	for i, t := range tabs {
		var cell string
		if i == active {
			cell = m.styles.ActiveTab.Render(" " + t.Title + " ")
		} else if t.Disabled {
			cell = m.styles.DisabledTab.Render(" " + t.Title + " ")
		} else {
			cell = m.styles.InactiveTab.Render(" " + t.Title + " ")
		}
		rendered = append(rendered, cell)
		renderedWidths = append(renderedWidths, lipgloss.Width(cell))
	}

	total := 0
	for _, w := range renderedWidths {
		total += w
	}
	if total <= width {
		row := lipgloss.JoinHorizontal(lipgloss.Top, rendered...)
		return m.styles.Header.Width(width).Render(row)
	}

	left := active
	right := active
	used := renderedWidths[active]
	for {
		grew := false
		if left > 0 && used+renderedWidths[left-1] <= width {
			left--
			used += renderedWidths[left]
			grew = true
		}
		if right < len(tabs)-1 && used+renderedWidths[right+1] <= width {
			right++
			used += renderedWidths[right]
			grew = true
		}
		if !grew {
			break
		}
	}

	leftOverflow := left > 0
	rightOverflow := right < len(tabs)-1
	overflowWidth := 0
	if leftOverflow {
		overflowWidth += lipgloss.Width(m.styles.Overflow.Render(" … "))
	}
	if rightOverflow {
		overflowWidth += lipgloss.Width(m.styles.Overflow.Render(" … "))
	}

	for used+overflowWidth > width && (left < active || right > active) {
		if right > active && used+overflowWidth-renderedWidths[right] >= 0 {
			used -= renderedWidths[right]
			right--
		} else if left < active && used+overflowWidth-renderedWidths[left] >= 0 {
			used -= renderedWidths[left]
			left++
		} else {
			break
		}
		leftOverflow = left > 0
		rightOverflow = right < len(tabs)-1
		overflowWidth = 0
		if leftOverflow {
			overflowWidth += lipgloss.Width(m.styles.Overflow.Render(" … "))
		}
		if rightOverflow {
			overflowWidth += lipgloss.Width(m.styles.Overflow.Render(" … "))
		}
	}

	parts := make([]string, 0, (right-left)+3)
	if leftOverflow {
		parts = append(parts, m.styles.Overflow.Render(" … "))
	}
	for i := left; i <= right; i++ {
		parts = append(parts, rendered[i])
	}
	if rightOverflow {
		parts = append(parts, m.styles.Overflow.Render(" … "))
	}
	row := lipgloss.JoinHorizontal(lipgloss.Top, parts...)
	return m.styles.Header.Width(width).Render(row)
}

func (m Model) renderSystemRow(info monitor.SystemInfo, width int) string {
	if width <= 0 {
		return ""
	}

	var parts []string
	if info.Disk != "" {
		parts = append(parts, info.Disk)
	}
	if info.Net != "" {
		parts = append(parts, info.Net)
	}
	if info.Uptime != "" {
		parts = append(parts, info.Uptime)
	}

	if len(parts) == 0 {
		return ""
	}

	row := strings.Join(parts, "   ")
	return m.styles.Info.Width(width).Render(row)
}

func (m Model) renderContentTitle(title string, width int) string {
	if width <= 0 {
		return ""
	}
	label := fmt.Sprintf(" %s ", title)
	return m.styles.Summary.Width(width).Render(label)
}

func (m Model) renderFooter(status, spinner string, width int) string {
	help := "q:quit  tab/shift+tab:next/prev  up/down/pgup/pgdn:scroll  t:theme"
	if status != "" {
		help = spinner + "  " + status + "  |  " + help
	} else if spinner != "" {
		help = spinner + "  " + help
	}
	return m.styles.Footer.Width(width).Render(help)
}

func sparkline(values []float64, min, max float64) string {
	if len(values) == 0 {
		return ""
	}
	if max <= min {
		max = min + 1
	}
	levels := []rune(" .:-=+*#%@")
	var b strings.Builder
	for _, v := range values {
		if v < min {
			v = min
		}
		if v > max {
			v = max
		}
		n := int(((v - min) / (max - min)) * float64(len(levels)-1))
		if n < 0 {
			n = 0
		}
		if n >= len(levels) {
			n = len(levels) - 1
		}
		b.WriteRune(levels[n])
	}
	return b.String()
}

func isQuitKey(msg tea.KeyMsg) bool {
	if msg.Type == tea.KeyEsc {
		return true
	}
	if msg.Type == tea.KeyCtrlC {
		return true
	}
	if msg.Type == tea.KeyRunes && len(msg.Runes) == 1 {
		switch msg.Runes[0] {
		case 'q', 'Q':
			return true
		}
	}
	switch msg.String() {
	case "q", "Q", "esc", "ctrl+c":
		return true
	}
	return false
}

func maxFloat(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	out := values[0]
	for _, v := range values[1:] {
		if v > out {
			out = v
		}
	}
	return out
}

func clampMin(value, min int) int {
	if value < min {
		return min
	}
	return value
}
