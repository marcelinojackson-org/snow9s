package ui

import (
	"testing"

	"github.com/yourusername/snow9s/pkg/models"
)

func TestTableFiltering(t *testing.T) {
	table := NewServicesTable(DefaultStyles())
	services := []models.Service{{Name: "alpha", Namespace: "PUBLIC", Status: "running"}, {Name: "beta", Namespace: "PUBLIC", Status: "stopped"}}
	table.SetServices(services)
	if got := table.GetRowCount(); got != 3 { // header + rows
		t.Fatalf("expected 3 rows got %d", got)
	}
	table.SetFilter("run")
	if got := table.GetRowCount(); got != 2 {
		t.Fatalf("expected filtered rows to leave header+1 got %d", got)
	}
}

func TestStatusColoring(t *testing.T) {
	table := NewServicesTable(DefaultStyles())
	table.SetServices([]models.Service{{Name: "svc", Namespace: "PUBLIC", Status: "running"}})
	cell := table.GetCell(1, 2) // status column first row
	fg, _, _ := cell.Style.Decompose()
	if fg != DefaultStyles().StatusRunning {
		t.Fatalf("status color not applied")
	}
}
