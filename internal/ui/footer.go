package ui

import (
	"strings"

	"github.com/rivo/tview"
)

// Footer renders keybinding hints with a dark background.
type Footer struct {
	view   *tview.TextView
	styles StyleConfig
	hints  string
	status string
}

func NewFooter(styles StyleConfig) *Footer {
	view := tview.NewTextView().SetDynamicColors(true)
	view.SetBackgroundColor(styles.RowAltBg)
	view.SetTextColor(styles.PrimaryText)
	view.SetWrap(false)
	return &Footer{view: view, styles: styles}
}

// View exposes the tview component.
func (f *Footer) View() *tview.TextView {
	return f.view
}

// SetHints rewrites the footer message.
func (f *Footer) SetHints(hints []string) {
	f.hints = strings.Join(hints, "  ")
	f.render()
}

// SetStatus appends additional info (e.g. counts, filter) to the footer.
func (f *Footer) SetStatus(status string) {
	f.status = strings.TrimSpace(status)
	f.render()
}

func (f *Footer) render() {
	text := f.hints
	if f.status != "" {
		text = strings.TrimSpace(text + "  " + f.status)
	}
	f.view.SetText(text)
}
