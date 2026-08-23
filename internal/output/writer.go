package output

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"golang.org/x/term"
)

type Format string

const (
	FormatTable Format = "table"
	FormatJSON  Format = "json"
)

type ColumnStyle int

const (
	ColumnStyleDefault ColumnStyle = iota
	ColumnStyleStatus
)

type Options struct {
	Format  string
	Out     io.Writer
	NoColor bool
}

type Table struct {
	Headers      []string
	Rows         [][]string
	ColumnStyles map[int]ColumnStyle
}

type Writer struct {
	format   Format
	out      io.Writer
	renderer *lipgloss.Renderer
}

func New(options Options) (*Writer, error) {
	if options.Out == nil {
		return nil, fmt.Errorf("output writer is required")
	}

	selected := Format(options.Format)
	switch selected {
	case FormatTable, FormatJSON:
	default:
		return nil, fmt.Errorf("unsupported output format %q; want table or json", options.Format)
	}

	writer := &Writer{
		format: selected,
		out:    options.Out,
	}
	if shouldStyle(options.Out, options.NoColor) {
		writer.renderer = lipgloss.NewRenderer(options.Out)
	}

	return writer, nil
}

func (w *Writer) Write(value any, table Table) error {
	switch w.format {
	case FormatJSON:
		return json.NewEncoder(w.out).Encode(value)
	case FormatTable:
		return w.writeTable(table)
	default:
		return fmt.Errorf("unsupported output format %q", w.format)
	}
}

func (w *Writer) writeTable(table Table) error {
	rows := table.Rows
	if len(table.Headers) > 0 {
		rows = append([][]string{table.Headers}, rows...)
	}

	widths := make([]int, maxColumns(rows))
	for rowIndex, row := range rows {
		for columnIndex, value := range row {
			widths[columnIndex] = max(widths[columnIndex], lipgloss.Width(w.renderCell(table, rowIndex == 0 && len(table.Headers) > 0, columnIndex, value)))
		}
	}

	for rowIndex, row := range rows {
		for columnIndex := range widths {
			value := ""
			if columnIndex < len(row) {
				value = row[columnIndex]
			}

			rendered := w.renderCell(table, rowIndex == 0 && len(table.Headers) > 0, columnIndex, value)
			if _, err := fmt.Fprint(w.out, rendered); err != nil {
				return err
			}
			if columnIndex < len(widths)-1 {
				if _, err := fmt.Fprint(w.out, strings.Repeat(" ", widths[columnIndex]-lipgloss.Width(rendered)+2)); err != nil {
					return err
				}
			}
		}

		if _, err := fmt.Fprintln(w.out); err != nil {
			return err
		}
	}

	return nil
}

func (w *Writer) renderCell(table Table, isHeader bool, column int, value string) string {
	if w.renderer == nil {
		return value
	}

	if isHeader {
		return w.renderer.NewStyle().Bold(true).Foreground(lipgloss.Color("12")).Render(value)
	}

	if table.ColumnStyles[column] != ColumnStyleStatus {
		return value
	}

	style := w.renderer.NewStyle().Bold(true).Padding(0, 1)
	switch strings.ToLower(value) {
	case "running":
		return style.Foreground(lipgloss.Color("10")).Render(value)
	case "pending", "queued":
		return style.Foreground(lipgloss.Color("11")).Render(value)
	case "failed", "error":
		return style.Foreground(lipgloss.Color("9")).Render(value)
	default:
		return style.Foreground(lipgloss.Color("8")).Render(value)
	}
}

func shouldStyle(out io.Writer, noColor bool) bool {
	if noColor || os.Getenv("NO_COLOR") != "" {
		return false
	}

	file, ok := out.(*os.File)
	return ok && term.IsTerminal(int(file.Fd()))
}

func maxColumns(rows [][]string) int {
	var columns int
	for _, row := range rows {
		columns = max(columns, len(row))
	}

	return columns
}
