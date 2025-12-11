package ui

import (
	"fmt"
	"strings"
	"sync"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

type TableRow struct {
	Cells []string
}

// DataTable extends tview.Table with k9s-like styling and filtering.
type DataTable struct {
	*tview.Table
	styles       StyleConfig
	headers      []string
	rows         []TableRow
	filtered     []TableRow
	filter       string
	statusColumn int
	mu           sync.Mutex
}

// NewDataTable wires defaults that mirror k9s tables.
func NewDataTable(styles StyleConfig) *DataTable {
	table := tview.NewTable()
	table.SetBorders(false)
	table.SetBorder(false)
	table.SetBorderPadding(0, 0, 0, 0)
	table.SetFixed(1, 0)
	table.SetSelectable(true, false)
	table.SetBackgroundColor(styles.Background)
	table.SetBorderColor(styles.Border)
	table.SetSelectedStyle(tcell.StyleDefault.Foreground(styles.SelectionText).Background(styles.SelectionBg).Bold(true))

	return &DataTable{Table: table, styles: styles, statusColumn: -1}
}

// SetStatusColumn configures which column is treated as a status column.
func (t *DataTable) SetStatusColumn(idx int) {
	t.mu.Lock()
	t.statusColumn = idx
	t.mu.Unlock()
	t.render()
}

// SetData refreshes the source data and re-renders.
func (t *DataTable) SetData(headers []string, rows []TableRow) {
	t.mu.Lock()
	t.headers = append([]string(nil), headers...)
	t.rows = append([]TableRow(nil), rows...)
	t.mu.Unlock()
	t.applyFilter()
}

// SetFilter updates the current filter and rerenders.
func (t *DataTable) SetFilter(filter string) {
	t.mu.Lock()
	t.filter = filter
	t.mu.Unlock()
	t.applyFilter()
}

// SelectionInfo returns the formatted selected/total count.
func (t *DataTable) SelectionInfo() string {
	t.mu.Lock()
	defer t.mu.Unlock()
	total := len(t.filtered)
	row, _ := t.GetSelection()
	if total == 0 {
		return "0/0"
	}
	if row <= 0 {
		row = 1
	}
	return fmt.Sprintf("%d/%d", row, total)
}

// SelectedRow returns the currently selected row.
func (t *DataTable) SelectedRow() (TableRow, bool) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if len(t.filtered) == 0 {
		return TableRow{}, false
	}
	row, _ := t.GetSelection()
	if row <= 0 {
		row = 1
	}
	index := row - 1
	if index < 0 || index >= len(t.filtered) {
		return TableRow{}, false
	}
	return t.filtered[index], true
}

// Headers returns the current table headers.
func (t *DataTable) Headers() []string {
	t.mu.Lock()
	defer t.mu.Unlock()
	return append([]string(nil), t.headers...)
}

func (t *DataTable) applyFilter() {
	t.mu.Lock()
	filter := strings.ToLower(strings.TrimSpace(t.filter))
	rows := append([]TableRow(nil), t.rows...)
	t.mu.Unlock()

	filtered := make([]TableRow, 0, len(rows))
	for _, row := range rows {
		if filter == "" {
			filtered = append(filtered, row)
			continue
		}
		joined := strings.ToLower(strings.Join(row.Cells, " "))
		if strings.Contains(joined, filter) {
			filtered = append(filtered, row)
		}
	}

	t.mu.Lock()
	t.filtered = filtered
	t.mu.Unlock()
	t.render()
}

func (t *DataTable) render() {
	t.Clear()

	t.mu.Lock()
	headers := append([]string(nil), t.headers...)
	rows := append([]TableRow(nil), t.filtered...)
	statusCol := t.statusColumn
	t.mu.Unlock()

	// Header row
	for c, h := range headers {
		cell := tview.NewTableCell(fmt.Sprintf(" %s ", h)).
			SetTextColor(t.styles.PrimaryText).
			SetBackgroundColor(t.styles.Background).
			SetAlign(tview.AlignLeft).
			SetExpansion(1).
			SetSelectable(false)
		t.SetCell(0, c, cell)
	}

	// Rows
	for r, row := range rows {
		rowIdx := r + 1 // account for header
		bg := t.styles.Background
		if r%2 == 1 {
			bg = t.styles.RowAltBg
		}
		for c, v := range row.Cells {
			cell := tview.NewTableCell(fmt.Sprintf(" %s ", v)).
				SetTextColor(t.cellColor(c, v, statusCol)).
				SetBackgroundColor(bg).
				SetAlign(tview.AlignLeft).
				SetExpansion(1)
			t.SetCell(rowIdx, c, cell)
		}
	}

	if len(rows) > 0 {
		t.Select(1, 0)
	}
}

func (t *DataTable) cellColor(col int, value string, statusCol int) tcell.Color {
	if col == statusCol {
		return t.styles.StatusColor(value)
	}
	return t.styles.PrimaryText
}
