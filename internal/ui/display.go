package ui

import (
	"fmt"
	"time"

	ui "github.com/gizak/termui/v3"
	"github.com/gizak/termui/v3/widgets"
	tb "github.com/nsf/termbox-go"

	"snmp-monitor/internal/monitor"
	"snmp-monitor/internal/snmp"
)

type Display struct {
	table  *widgets.Table
	header *widgets.Paragraph
	help   *widgets.Paragraph
}

func NewDisplay(noColor bool) *Display {
	return &Display{}
}

// Init initializes termui
func (d *Display) Init() error {
	if err := ui.Init(); err != nil {
		return err
	}

	tb.SetInputMode(tb.InputEsc)

	d.header = widgets.NewParagraph()
	d.header.Title = " SNMP Interface Monitor "
	d.header.BorderStyle.Fg = ui.ColorCyan
	d.header.TitleStyle.Fg = ui.ColorWhite
	d.header.TitleStyle.Modifier = ui.ModifierBold

	d.table = widgets.NewTable()
	d.table.Title = " Interfaces "
	d.table.BorderStyle.Fg = ui.ColorCyan
	d.table.RowSeparator = false
	d.table.TextAlignment = ui.AlignLeft
	d.table.RowStyles[0] = ui.NewStyle(ui.ColorWhite, ui.ColorClear, ui.ModifierBold)

	d.help = widgets.NewParagraph()
	d.help.Text = " Press q or Ctrl+C to exit"
	d.help.Border = false
	d.help.TextStyle.Fg = ui.ColorWhite

	d.resize()
	return nil
}

func (d *Display) Close() {
	ui.Close()
}

// Clear is a no-op for termui (kept for interface compatibility)
func (d *Display) Clear() {}

func (d *Display) resize() {
	termWidth, termHeight := ui.TerminalDimensions()

	d.header.SetRect(0, 0, termWidth, 3)
	d.table.SetRect(0, 3, termWidth, termHeight-2)
	d.help.SetRect(0, termHeight-2, termWidth, termHeight)
}

// Render draws the interface metrics
func (d *Display) Render(metrics []monitor.InterfaceMetrics, host string, interval time.Duration) {
	d.header.Text = fmt.Sprintf(" Host: %s | Interval: %s | Updated: %s",
		host, interval.String(), time.Now().Format("15:04:05"))

	// Build table rows
	rows := [][]string{
		{"#", "Interface", "Status", "In Rate", "Out Rate", "In Total", "Out Total"},
	}

	for _, m := range metrics {
		rows = append(rows, []string{
			fmt.Sprintf("%d", m.Index),
			truncate(m.Name, 22),
			m.Status.String(),
			formatRate(m.InRate),
			formatRate(m.OutRate),
			formatBytes(m.InTotal),
			formatBytes(m.OutTotal),
		})
	}

	d.table.Rows = rows

	// Color rows based on status
	for i, m := range metrics {
		rowIndex := i + 1 // skip header row
		switch m.Status {
		case snmp.StatusUp:
			if m.InRate > 0 || m.OutRate > 0 {
				d.table.RowStyles[rowIndex] = ui.NewStyle(ui.ColorGreen)
			} else {
				d.table.RowStyles[rowIndex] = ui.NewStyle(ui.ColorWhite)
			}
		case snmp.StatusDown:
			d.table.RowStyles[rowIndex] = ui.NewStyle(ui.ColorRed)
		default:
			d.table.RowStyles[rowIndex] = ui.NewStyle(ui.ColorYellow)
		}
	}

	ui.Render(d.header, d.table, d.help)
}

// RenderError displays an error
func (d *Display) RenderError(err error) {
	p := widgets.NewParagraph()
	p.Title = " Error "
	p.Text = err.Error()
	p.BorderStyle.Fg = ui.ColorRed
	p.TextStyle.Fg = ui.ColorRed
	p.SetRect(0, 0, 50, 5)
	ui.Render(p)
}

// PollEvents returns the termui event channel
func (d *Display) PollEvents() <-chan ui.Event {
	return ui.PollEvents()
}

// HandleResize handles terminal resize
func (d *Display) HandleResize() {
	d.resize()
	ui.Clear()
}

func formatRate(bytesPerSec float64) string {
	switch {
	case bytesPerSec >= 1024*1024*1024:
		return fmt.Sprintf("%.2f GB/s", bytesPerSec/(1024*1024*1024))
	case bytesPerSec >= 1024*1024:
		return fmt.Sprintf("%.2f MB/s", bytesPerSec/(1024*1024))
	case bytesPerSec >= 1024:
		return fmt.Sprintf("%.2f KB/s", bytesPerSec/1024)
	case bytesPerSec > 0:
		return fmt.Sprintf("%.0f B/s", bytesPerSec)
	default:
		return "0 B/s"
	}
}

func formatBytes(bytes uint64) string {
	switch {
	case bytes >= 1024*1024*1024*1024:
		return fmt.Sprintf("%.1f TB", float64(bytes)/(1024*1024*1024*1024))
	case bytes >= 1024*1024*1024:
		return fmt.Sprintf("%.1f GB", float64(bytes)/(1024*1024*1024))
	case bytes >= 1024*1024:
		return fmt.Sprintf("%.1f MB", float64(bytes)/(1024*1024))
	case bytes >= 1024:
		return fmt.Sprintf("%.1f KB", float64(bytes)/1024)
	default:
		return fmt.Sprintf("%d B", bytes)
	}
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return s[:maxLen]
	}
	return s[:maxLen-3] + "..."
}
