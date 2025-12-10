package ui

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/yourusername/snow9s/internal/config"
	"github.com/yourusername/snow9s/internal/snowflake"
	"github.com/yourusername/snow9s/pkg/models"
)

const appVersion = "0.1.0"

// App wires the widgets, navigation, and data refresh loop.
type App struct {
	app           *tview.Application
	styles        StyleConfig
	header        *Header
	footer        *Footer
	errorView     *tview.TextView
	table         *ServicesTable
	filterField   *tview.InputField
	spcs          *snowflake.SPCS
	cfg           config.Config
	refreshTicker *time.Ticker
	refreshMu     sync.Mutex
	loading       bool
	cancel        context.CancelFunc
	debugView     *tview.TextView
	debugEnabled  bool
	helpVisible   bool
	defaultHints  []string
}

// NewApp constructs the layout with k9s-inspired styling.
func NewApp(cfg config.Config, spcs *snowflake.SPCS, debugEnabled bool) *App {
	styles := DefaultStyles()
	app := tview.NewApplication()
	app.EnableMouse(true)

	header := NewHeader(cfg, appVersion, styles)
	footer := NewFooter(styles)
	footer.SetHints(defaultKeyHints())

	errorView := tview.NewTextView().SetDynamicColors(true)
	errorView.SetBackgroundColor(tcell.ColorRed)
	errorView.SetTextColor(tcell.ColorWhite)
	errorView.SetWrap(false)
	errorView.SetText("")

	table := NewServicesTable(styles)
	table.SetTitle(" Services ").SetTitleAlign(tview.AlignLeft)

	filterField := tview.NewInputField().SetLabel("")
	filterField.SetFieldBackgroundColor(styles.RowAltBg)
	filterField.SetFieldTextColor(styles.PrimaryText)
	filterField.SetLabelColor(styles.PrimaryText)
	filterField.SetBorder(true)
	filterField.SetBorderColor(styles.Border)
	filterField.SetTitle(" / Filter ")
	filterField.SetDoneFunc(func(key tcell.Key) {
		if key == tcell.KeyEsc {
			filterField.SetText("")
		}
	})
	filterField.SetChangedFunc(func(text string) {
		table.SetFilter(text)
		footer.SetStatus(fmt.Sprintf("%s  filter: %s", table.SelectionInfo(), text))
	})
	filterField.SetDisabled(true)

	var debugView *tview.TextView
	if debugEnabled {
		debugView = tview.NewTextView().SetDynamicColors(true)
		debugView.SetBackgroundColor(styles.RowAltBg)
		debugView.SetTextColor(styles.SecondaryText)
		debugView.SetBorder(true)
		debugView.SetTitle(" Debug ")
	}

	return &App{
		app:          app,
		styles:       styles,
		header:       header,
		footer:       footer,
		errorView:    errorView,
		table:        table,
		filterField:  filterField,
		spcs:         spcs,
		cfg:          cfg,
		debugView:    debugView,
		debugEnabled: debugEnabled,
		defaultHints: defaultKeyHints(),
	}
}

// Run boots the TUI, wiring key bindings and refresh loop.
func (a *App) Run(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	a.cancel = cancel
	defer cancel()

	rootFlex := tview.NewFlex().SetDirection(tview.FlexRow)
	rootFlex.SetBackgroundColor(a.styles.Background)
	rootFlex.AddItem(a.header.View(), 1, 0, false)
	rootFlex.AddItem(a.errorView, 1, 0, false)
	if a.debugEnabled {
		body := tview.NewFlex().SetDirection(tview.FlexRow)
		body.AddItem(a.table, 0, 1, true)
		body.AddItem(a.debugView, 5, 0, false)
		rootFlex.AddItem(body, 0, 1, true)
	} else {
		rootFlex.AddItem(a.table, 0, 1, true)
	}
	rootFlex.AddItem(a.filterField, 1, 0, false)
	rootFlex.AddItem(a.footer.View(), 1, 0, false)

	a.app.SetRoot(rootFlex, true)
	a.app.SetFocus(a.table)
	a.bindKeys()

	// handle Ctrl+C
	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		select {
		case <-ctx.Done():
		case <-sigCh:
			cancel()
			a.app.Stop()
		}
	}()

	a.startRefreshLoop(ctx)

	if err := a.app.Run(); err != nil {
		return err
	}
	return nil
}

func (a *App) startRefreshLoop(ctx context.Context) {
	a.refreshTicker = time.NewTicker(5 * time.Second)
	go func() {
		defer a.refreshTicker.Stop()
		a.fetchServices(ctx)
		for {
			select {
			case <-ctx.Done():
				return
			case <-a.refreshTicker.C:
				a.fetchServices(ctx)
			}
		}
	}()
}

func (a *App) fetchServices(ctx context.Context) {
	timeoutCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	a.setLoading(true)
	services, err := a.spcs.ListServices(timeoutCtx)
	a.setLoading(false)
	if err != nil {
		a.showError(fmt.Sprintf("Error fetching services: %v (Ctrl+r to retry)", err))
		return
	}

	if len(services) == 0 {
		a.showError(fmt.Sprintf("No services found in %s", a.cfg.Schema))
	} else {
		a.showError("")
	}

	a.table.SetServices(services)
	a.updateFooterStatus()
	a.header.Refresh()
}

func (a *App) showError(msg string) {
	a.app.QueueUpdateDraw(func() {
		a.errorView.SetText(msg)
		bg := a.styles.Background
		if msg != "" {
			if strings.HasPrefix(msg, "No services") {
				bg = a.styles.RowAltBg
			} else {
				bg = tcell.ColorRed
			}
		}
		a.errorView.SetBackgroundColor(bg)
	})
}

func (a *App) setLoading(loading bool) {
	a.refreshMu.Lock()
	changed := a.loading != loading
	a.loading = loading
	a.refreshMu.Unlock()
	if !changed {
		return
	}

	if loading {
		go a.spin()
	} else {
		a.app.QueueUpdateDraw(func() {
			a.footer.SetHints(a.defaultHints)
			a.updateFooterStatus()
		})
	}
}

func (a *App) spin() {
	frames := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
	idx := 0
	for {
		a.refreshMu.Lock()
		if !a.loading {
			a.refreshMu.Unlock()
			return
		}
		a.refreshMu.Unlock()

		frame := frames[idx%len(frames)]
		a.app.QueueUpdateDraw(func() {
			a.footer.SetHints([]string{fmt.Sprintf("%s Fetching services...", frame)})
		})
		idx++
		time.Sleep(120 * time.Millisecond)
	}
}

func (a *App) bindKeys() {
	a.app.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if a.handleKey(event) {
			return nil
		}
		return event
	})
}

func (a *App) handleKey(event *tcell.EventKey) bool {
	switch event.Key() {
	case tcell.KeyCtrlC:
		a.app.Stop()
		return true
	case tcell.KeyEsc:
		a.filterField.SetText("")
		a.table.SetFilter("")
		a.updateFooterStatus()
		return true
	case tcell.KeyCtrlR:
		a.fetchServices(context.Background())
		return true
	case tcell.KeyCtrlD:
		a.page(1)
		return true
	case tcell.KeyCtrlU:
		a.page(-1)
		return true
	case tcell.KeyDown:
		a.move(1)
		return true
	case tcell.KeyUp:
		a.move(-1)
		return true
	case tcell.KeyEnter:
		a.showError("Enter: drill-down not implemented yet")
		return true
	case tcell.KeyRune:
		switch event.Rune() {
		case 'q':
			a.app.Stop()
			return true
		case 'j':
			a.move(1)
			return true
		case 'k':
			a.move(-1)
			return true
		case 'g':
			a.selectRow(1)
			return true
		case 'G':
			a.selectRow(a.table.GetRowCount() - 1)
			return true
		case '/':
			a.activateFilter()
			return true
		case '?':
			a.toggleHelp()
			return true
		}
	}
	return false
}

func (a *App) move(delta int) {
	row, col := a.table.GetSelection()
	newRow := row + delta
	if newRow < 1 {
		newRow = 1
	}
	if newRow >= a.table.GetRowCount() {
		newRow = a.table.GetRowCount() - 1
	}
	a.table.Select(newRow, col)
	a.updateFooterStatus()
}

func (a *App) page(direction int) {
	total := a.table.GetRowCount()
	if total <= 1 {
		return
	}
	row, col := a.table.GetSelection()
	pageSize := total / 2
	newRow := row + (pageSize * direction)
	if newRow < 1 {
		newRow = 1
	}
	if newRow >= total {
		newRow = total - 1
	}
	a.table.Select(newRow, col)
	a.updateFooterStatus()
}

func (a *App) selectRow(row int) {
	if row < 1 {
		row = 1
	}
	if row >= a.table.GetRowCount() {
		row = a.table.GetRowCount() - 1
	}
	a.table.Select(row, 0)
	a.updateFooterStatus()
}

func (a *App) activateFilter() {
	a.filterField.SetDisabled(false)
	a.filterField.SetLabel("> ")
	a.app.SetFocus(a.filterField)
	a.footer.SetHints([]string{"esc Clear", "enter Done"})
	a.filterField.SetDoneFunc(func(key tcell.Key) {
		if key == tcell.KeyEsc {
			a.filterField.SetText("")
			a.table.SetFilter("")
		}
		if key == tcell.KeyEnter || key == tcell.KeyEsc {
			a.filterField.SetDisabled(true)
			a.filterField.SetLabel("")
			a.app.SetFocus(a.table)
			a.footer.SetHints(a.defaultHints)
			a.updateFooterStatus()
		}
	})
}

func (a *App) toggleHelp() {
	if a.helpVisible {
		a.showError("")
		a.helpVisible = false
		return
	}
	a.helpVisible = true
	help := "j/k/↓/↑ move  g/G top/bottom  / filter  esc clear  ctrl+r refresh  q quit"
	a.showError(help)
}

func (a *App) updateFooterStatus() {
	filterText := a.filterField.GetText()
	parts := []string{a.table.SelectionInfo()}
	if strings.TrimSpace(filterText) != "" {
		parts = append(parts, fmt.Sprintf("filter: %s", filterText))
	}
	a.footer.SetStatus(strings.Join(parts, "  "))
}

// DebugWriter streams logs into the debug pane when enabled.
func (a *App) DebugWriter() io.Writer {
	if a.debugView == nil {
		return nil
	}
	return &textViewWriter{app: a.app, view: a.debugView}
}

// PrintTable renders a k9s-like table to stdout for the CLI list command.
func PrintTable(services []models.Service) {
	fmt.Println("┌──────────────────────────────────────────────┐")
	fmt.Printf("│ %-12s %-16s %-10s %-12s %-6s │\n", "NAMESPACE", "NAME", "STATUS", "POOL", "AGE")
	fmt.Println("├──────────────────────────────────────────────┤")
	for _, s := range services {
		status := strings.ToUpper(s.Status)
		age := s.Age
		if age == "" && !s.CreatedAt.IsZero() {
			age = models.HumanizeAge(s.CreatedAt)
		}
		fmt.Printf("│ %-12s %-16s %-10s %-12s %-6s │\n", s.Namespace, s.Name, status, s.ComputePool, age)
	}
	fmt.Println("└──────────────────────────────────────────────┘")
}

func defaultKeyHints() []string {
	return []string{"j/k/↓/↑ Move", "g/G Top/Bottom", "ctrl+d/ctrl+u Page", "/ Filter", "ctrl+r Refresh", "q Quit"}
}

type textViewWriter struct {
	app  *tview.Application
	view *tview.TextView
}

func (w *textViewWriter) Write(p []byte) (int, error) {
	msg := string(p)
	w.app.QueueUpdateDraw(func() {
		fmt.Fprint(w.view, msg)
	})
	return len(p), nil
}
