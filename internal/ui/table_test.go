package ui

import "testing"

func TestTableFiltering(t *testing.T) {
	table := NewDataTable(DefaultStyles())
	headers := []string{"NAME", "STATUS"}
	rows := []TableRow{
		{Cells: []string{"alpha", "running"}},
		{Cells: []string{"beta", "stopped"}},
	}
	table.SetData(headers, rows)
	if got := table.GetRowCount(); got != 3 { // header + rows
		t.Fatalf("expected 3 rows got %d", got)
	}
	table.SetFilter("run")
	if got := table.GetRowCount(); got != 2 {
		t.Fatalf("expected filtered rows to leave header+1 got %d", got)
	}
}

func TestStatusColoring(t *testing.T) {
	table := NewDataTable(DefaultStyles())
	headers := []string{"NAME", "STATUS"}
	rows := []TableRow{{Cells: []string{"svc", "running"}}}
	table.SetData(headers, rows)
	table.SetStatusColumn(1)
	cell := table.GetCell(1, 1) // status column first row
	fg, _, _ := cell.Style.Decompose()
	if fg != DefaultStyles().StatusRunning {
		t.Fatalf("status color not applied")
	}
}
