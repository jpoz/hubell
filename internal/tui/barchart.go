package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// BarChartData represents a single bar in the chart.
type BarChartData struct {
	Label string
	Value int
}

// renderBarChart renders a text-based bar chart using block characters.
func renderBarChart(data []BarChartData, maxWidth, maxHeight int, barColor, labelColor, currentWeekColor lipgloss.Color) string {
	if len(data) == 0 {
		return ""
	}

	// Find max value for scaling
	maxVal := 0
	for _, d := range data {
		if d.Value > maxVal {
			maxVal = d.Value
		}
	}

	// Bar dimensions
	barWidth := 3
	gap := 1
	chartBars := len(data)
	// Y-axis label width (e.g. "10 │")
	yAxisWidth := 4

	// Limit available height for bars (leave room for x-axis labels)
	barMaxHeight := max(maxHeight-2, 1)

	// Check we have enough width
	neededWidth := yAxisWidth + chartBars*(barWidth+gap)
	if neededWidth > maxWidth {
		// Reduce bars from the left if needed
		excess := neededWidth - maxWidth
		barsToRemove := (excess + barWidth + gap - 1) / (barWidth + gap)
		if barsToRemove >= chartBars {
			barsToRemove = chartBars - 1
		}
		data = data[barsToRemove:]
		chartBars = len(data)
	}

	barStyle := lipgloss.NewStyle().Foreground(barColor)
	currentStyle := lipgloss.NewStyle().Foreground(currentWeekColor)
	lblStyle := lipgloss.NewStyle().Foreground(labelColor)

	var b strings.Builder

	// Render rows from top to bottom
	for row := barMaxHeight; row >= 1; row-- {
		// Y-axis label: show at top and midpoint
		threshold := float64(row) / float64(barMaxHeight) * float64(maxVal)
		if maxVal > 0 && (row == barMaxHeight || row == 1) {
			val := int(threshold + 0.5)
			if row == 1 {
				val = 0
			}
			b.WriteString(lblStyle.Render(fmt.Sprintf("%2d", val)))
			b.WriteString(lblStyle.Render(" │"))
		} else {
			b.WriteString(lblStyle.Render("   │"))
		}

		for i, d := range data {
			var barHeight int
			if maxVal > 0 {
				barHeight = int(float64(d.Value) / float64(maxVal) * float64(barMaxHeight))
				if d.Value > 0 && barHeight == 0 {
					barHeight = 1
				}
			}

			style := barStyle
			if i == len(data)-1 {
				style = currentStyle
			}

			if row <= barHeight {
				b.WriteString(style.Render(strings.Repeat("█", barWidth)))
			} else if row == barHeight+1 && d.Value > 0 {
				// Show count above bar
				countStr := fmt.Sprintf("%d", d.Value)
				padded := fmt.Sprintf("%-*s", barWidth, countStr)
				b.WriteString(lblStyle.Render(padded))
			} else {
				b.WriteString(strings.Repeat(" ", barWidth))
			}

			if i < len(data)-1 {
				b.WriteString(" ")
			}
		}
		b.WriteString("\n")
	}

	// X-axis line
	b.WriteString(lblStyle.Render("   └"))
	b.WriteString(lblStyle.Render(strings.Repeat("─", chartBars*(barWidth+gap))))
	b.WriteString("\n")

	// X-axis labels (show abbreviated week labels)
	b.WriteString("    ")
	for i, d := range data {
		label := d.Label
		if len(label) > barWidth {
			label = label[len(label)-barWidth:]
		}
		padded := fmt.Sprintf("%-*s", barWidth, label)
		if i == len(data)-1 {
			b.WriteString(currentStyle.Render(padded))
		} else {
			b.WriteString(lblStyle.Render(padded))
		}
		if i < len(data)-1 {
			b.WriteString(" ")
		}
	}

	return b.String()
}
