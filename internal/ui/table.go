package ui

import (
	"fmt"
	"sync"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/yourusername/snow9s/pkg/models"
)

// ServicesTable extends tview.Table with k9s-like styling and filtering.
type ServicesTable struct {
	*tview.Table
	styles   StyleConfig
	services []models.Service
	filtered []models.Service
	filter   string
	mu       sync.Mutex
}

// NewServicesTable wires defaults that mirror k9s tables.
func NewServicesTable(styles StyleConfig) *ServicesTable {
	table := tview.NewTable()
	table.SetBorders(true)
	table.SetBorder(true)
	table.SetBorderPadding(0, 0, 0, 0)
	table.SetFixed(1, 0)
	table.SetSelectable(true, false)
	table.SetBackgroundColor(styles.Background)
	table.SetBorderColor(styles.Border)
	table.SetSelectedStyle(tcell.StyleDefault.Foreground(styles.SelectionText).Background(styles.SelectionBg).Bold(true))

	return &ServicesTable{Table: table, styles: styles}
}

// SetServices refreshes the source data and re-renders.
func (t *ServicesTable) SetServices(services []models.Service) {
	t.mu.Lock()
	t.services = services
	t.mu.Unlock()
	t.applyFilter()
}

// SetFilter updates the current filter and rerenders.
func (t *ServicesTable) SetFilter(filter string) {
	t.mu.Lock()
	t.filter = filter
	t.mu.Unlock()
	t.applyFilter()
}

// SelectionInfo returns the formatted selected/total count.
func (t *ServicesTable) SelectionInfo() string {
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

func (t *ServicesTable) applyFilter() {
	t.mu.Lock()
	filter := t.filter
	services := append([]models.Service(nil), t.services...)
	t.mu.Unlock()

	filtered := make([]models.Service, 0, len(services))
	for _, s := range services {
		if s.MatchesFilter(filter) {
			filtered = append(filtered, s)
		}
	}

	t.mu.Lock()
	t.filtered = filtered
	t.mu.Unlock()
	t.render()
}

func (t *ServicesTable) render() {
	headers := []string{"NAMESPACE", "NAME", "STATUS", "COMPUTE POOL", "AGE"}

	t.Clear()
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
	t.mu.Lock()
	rows := append([]models.Service(nil), t.filtered...)
	t.mu.Unlock()

	for r, svc := range rows {
		rowIdx := r + 1 // account for header
		bg := t.styles.Background
		if r%2 == 1 {
			bg = t.styles.RowAltBg
		}

		values := []string{svc.Namespace, svc.Name, svc.Status, svc.ComputePool, svc.Age}
		for c, v := range values {
			cell := tview.NewTableCell(fmt.Sprintf(" %-15s", v)).
				SetTextColor(t.cellColor(c, v, svc)).
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

func (t *ServicesTable) cellColor(col int, value string, svc models.Service) tcell.Color {
	if col == 2 { // status column
		return t.styles.StatusColor(value)
	}
	return t.styles.PrimaryText
}
