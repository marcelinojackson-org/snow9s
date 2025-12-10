package ui

import (
	"fmt"
	"time"

	"github.com/rivo/tview"
	"github.com/yourusername/snow9s/internal/config"
)

// Header renders the cyan banner with context details.
type Header struct {
	view    *tview.TextView
	cfg     config.Config
	styles  StyleConfig
	version string
}

// NewHeader builds the banner widget.
func NewHeader(cfg config.Config, version string, styles StyleConfig) *Header {
	view := tview.NewTextView().SetDynamicColors(true)
	view.SetBackgroundColor(styles.HeaderBg)
	view.SetTextColor(styles.HeaderText)
	view.SetRegions(false)
	view.SetWrap(false)

	h := &Header{view: view, cfg: cfg, styles: styles, version: version}
	h.Refresh()
	return h
}

// View exposes the underlying TextView for layout composition.
func (h *Header) View() *tview.TextView {
	return h.view
}

// Refresh updates the timestamp and renders the contextual text.
func (h *Header) Refresh() {
	h.view.SetText(h.render())
}

func (h *Header) render() string {
	left := fmt.Sprintf(" snow9s v%s ", h.version)
	ctx := fmt.Sprintf(" Context: %s.snowflake | User: %s ", h.cfg.Account, h.cfg.User)
	right := fmt.Sprintf(" %s ", time.Now().Format("15:04:05"))
	return fmt.Sprintf("%s┃%s┃%s", left, ctx, right)
}
