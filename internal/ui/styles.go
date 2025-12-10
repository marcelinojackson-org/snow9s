package ui

import (
	"strings"

	"github.com/gdamore/tcell/v2"
)

// StyleConfig captures the k9s-inspired palette used throughout the UI.
type StyleConfig struct {
	Background      tcell.Color
	PrimaryText     tcell.Color
	SecondaryText   tcell.Color
	HeaderBg        tcell.Color
	HeaderText      tcell.Color
	SelectionBg     tcell.Color
	SelectionText   tcell.Color
	Border          tcell.Color
	RowAltBg        tcell.Color
	StatusRunning   tcell.Color
	StatusStarting  tcell.Color
	StatusStopped   tcell.Color
	StatusSuspended tcell.Color
}

// DefaultStyles returns the base k9s-like scheme.
func DefaultStyles() StyleConfig {
	return StyleConfig{
		Background:      tcell.ColorBlack,
		PrimaryText:     tcell.ColorWhite,
		SecondaryText:   tcell.NewHexColor(0x808080),
		HeaderBg:        tcell.NewHexColor(0x00FFFF),
		HeaderText:      tcell.ColorBlack,
		SelectionBg:     tcell.ColorWhite,
		SelectionText:   tcell.ColorBlack,
		Border:          tcell.NewHexColor(0x333333),
		RowAltBg:        tcell.NewHexColor(0x111111),
		StatusRunning:   tcell.NewHexColor(0x00FF00),
		StatusStarting:  tcell.NewHexColor(0xFFFF00),
		StatusStopped:   tcell.NewHexColor(0xFF0000),
		StatusSuspended: tcell.NewHexColor(0x666666),
	}
}

// StatusColor picks the right status color using the StyleConfig.
func (s StyleConfig) StatusColor(status string) tcell.Color {
	switch strings.ToLower(status) {
	case "running", "started", "ready":
		return s.StatusRunning
	case "starting", "init", "pending":
		return s.StatusStarting
	case "suspended", "paused":
		return s.StatusSuspended
	case "stopped", "failed", "error", "down":
		return s.StatusStopped
	default:
		return s.SecondaryText
	}
}
