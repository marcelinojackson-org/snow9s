package ui

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/marcelinojackson-org/snow9s/internal/config"
	"github.com/marcelinojackson-org/snow9s/internal/snowflake"
	"github.com/marcelinojackson-org/snow9s/pkg/models"
)

const appVersion = "0.1.0"

type viewKind string

const (
	viewServices  viewKind = "Services"
	viewPools     viewKind = "Pools"
	viewRepos     viewKind = "Repos"
	viewInstances viewKind = "Instances"
)

type inputMode int

const (
	inputNone inputMode = iota
	inputFilter
	inputCommand
)

type viewData struct {
	headers      []string
	rows         []TableRow
	statusColumn int
	warning      string
}

// App wires the widgets, navigation, and data refresh loop.
type App struct {
	app           *tview.Application
	styles        StyleConfig
	header        *Header
	footer        *Footer
	errorView     *tview.TextView
	table         *DataTable
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
	pages         *tview.Pages
	bottomPages   *tview.Pages
	detailView    *tview.TextView
	detailVisible bool
	view          viewKind
	activeService string
	inputMode     inputMode
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

	table := NewDataTable(styles)
	table.SetTitle(" Services ").SetTitleAlign(tview.AlignLeft)

	filterField := tview.NewInputField().SetLabel("")
	filterField.SetFieldBackgroundColor(styles.RowAltBg)
	filterField.SetFieldTextColor(styles.PrimaryText)
	filterField.SetLabelColor(styles.PrimaryText)
	filterField.SetBorder(false)
	filterField.SetDisabled(true)

	var debugView *tview.TextView
	if debugEnabled {
		debugView = tview.NewTextView().SetDynamicColors(true)
		debugView.SetBackgroundColor(styles.RowAltBg)
		debugView.SetTextColor(styles.SecondaryText)
		debugView.SetBorder(true)
		debugView.SetTitle(" Debug ")
	}

	appState := &App{
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
		view:         viewServices,
	}

	filterField.SetChangedFunc(func(text string) {
		if appState.inputMode != inputFilter {
			appState.footer.SetStatus(fmt.Sprintf("%s  cmd: %s", appState.table.SelectionInfo(), text))
			return
		}
		appState.table.SetFilter(text)
		appState.footer.SetStatus(fmt.Sprintf("%s  filter: %s", appState.table.SelectionInfo(), text))
	})
	filterField.SetDoneFunc(func(key tcell.Key) {
		appState.completeInput(key)
	})

	return appState
}

// Run boots the TUI, wiring key bindings and refresh loop.
func (a *App) Run(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	a.cancel = cancel
	defer cancel()

	a.detailView = tview.NewTextView().SetDynamicColors(true)
	a.detailView.SetBackgroundColor(a.styles.Background)
	a.detailView.SetTextColor(a.styles.PrimaryText)
	a.detailView.SetBorder(true)
	a.detailView.SetBorderColor(a.styles.Border)
	a.detailView.SetTitle(" Details (Esc to close) ")

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
	a.bottomPages = tview.NewPages()
	a.bottomPages.AddPage("footer", a.footer.View(), true, true)
	a.bottomPages.AddPage("input", a.filterField, true, false)
	rootFlex.AddItem(a.bottomPages, 1, 0, false)

	a.pages = tview.NewPages()
	a.pages.AddPage("main", rootFlex, true, true)
	a.pages.AddPage("detail", a.detailView, true, false)
	a.app.SetRoot(a.pages, true)
	a.app.SetFocus(a.table)
	a.bindKeys()
	a.setView(viewServices)

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
		a.fetchCurrentView(ctx)
		for {
			select {
			case <-ctx.Done():
				return
			case <-a.refreshTicker.C:
				a.fetchCurrentView(ctx)
			}
		}
	}()
}

func (a *App) fetchCurrentView(ctx context.Context) {
	if a.inputMode != inputNone || a.detailVisible {
		return
	}
	a.refreshMu.Lock()
	if a.loading {
		a.refreshMu.Unlock()
		return
	}
	a.refreshMu.Unlock()

	go func() {
		timeoutCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()

		a.setLoading(true)
		data, err := a.loadViewData(timeoutCtx)
		a.setLoading(false)

		a.app.QueueUpdateDraw(func() {
			if err != nil {
				a.setError(fmt.Sprintf("Error fetching %s: %v (Ctrl+r to retry)", strings.ToLower(string(a.view)), err))
			} else if data.warning != "" {
				a.setError(data.warning)
			} else {
				a.setError("")
			}
			if err == nil {
				a.table.SetStatusColumn(data.statusColumn)
				a.table.SetData(data.headers, data.rows)
			}
			a.updateFooterStatus()
			a.header.Refresh()
			if a.inputMode == inputNone && !a.detailVisible {
				a.app.SetFocus(a.table)
			}
		})
	}()
}

func (a *App) showError(msg string) {
	a.app.QueueUpdateDraw(func() {
		a.setError(msg)
	})
}

func (a *App) setError(msg string) {
	a.errorView.SetText(msg)
	bg := a.styles.Background
	if msg != "" {
		if strings.HasPrefix(msg, "No items") || strings.HasPrefix(msg, "No services") {
			bg = a.styles.RowAltBg
		} else {
			bg = tcell.ColorRed
		}
	}
	a.errorView.SetBackgroundColor(bg)
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
			a.footer.SetHints([]string{fmt.Sprintf("%s Fetching %s...", frame, strings.ToLower(string(a.view)))})
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
	if a.detailVisible {
		if event.Key() == tcell.KeyEsc {
			a.closeDetail()
			return true
		}
		return true
	}
	if a.inputMode != inputNone && a.app.GetFocus() == a.filterField {
		if event.Key() == tcell.KeyEsc {
			a.completeInput(tcell.KeyEsc)
			return true
		}
		return false
	}
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
		a.fetchCurrentView(context.Background())
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
		a.openDetail()
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
			a.activateInput(inputFilter, "/ ")
			return true
		case ':':
			a.activateInput(inputCommand, ": ")
			return true
		case 's':
			a.setView(viewServices)
			return true
		case 'p':
			a.setView(viewPools)
			return true
		case 'r':
			a.setView(viewRepos)
			return true
		case 'i':
			if a.view == viewServices {
				a.openInstancesView()
			}
			return true
		case 'b':
			if a.view == viewInstances {
				a.setView(viewServices)
			}
			return true
		case 'n':
			a.activateInput(inputCommand, ":ns ")
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

func (a *App) activateInput(mode inputMode, label string) {
	a.inputMode = mode
	a.filterField.SetDisabled(false)
	a.filterField.SetLabel(label)
	a.filterField.SetText("")
	a.app.SetFocus(a.filterField)
	if a.bottomPages != nil {
		a.bottomPages.SwitchToPage("input")
	}
	if mode == inputCommand {
		a.footer.SetHints([]string{"enter Run", "esc Cancel"})
		return
	}
	a.footer.SetHints([]string{"esc Clear", "enter Done"})
}

func (a *App) completeInput(key tcell.Key) {
	text := strings.TrimSpace(a.filterField.GetText())
	switch a.inputMode {
	case inputFilter:
		if key == tcell.KeyEsc {
			a.filterField.SetText("")
			a.table.SetFilter("")
		}
		if key == tcell.KeyEnter || key == tcell.KeyEsc {
			a.filterField.SetDisabled(true)
			a.filterField.SetLabel("")
			a.inputMode = inputNone
			a.app.SetFocus(a.table)
			a.footer.SetHints(a.defaultHints)
			if a.bottomPages != nil {
				a.bottomPages.SwitchToPage("footer")
			}
			a.updateFooterStatus()
		}
	case inputCommand:
		if key == tcell.KeyEnter {
			a.runCommand(text)
		}
		if key == tcell.KeyEnter || key == tcell.KeyEsc {
			a.filterField.SetText("")
			a.filterField.SetDisabled(true)
			a.filterField.SetLabel("")
			a.inputMode = inputNone
			a.app.SetFocus(a.table)
			a.footer.SetHints(a.defaultHints)
			if a.bottomPages != nil {
				a.bottomPages.SwitchToPage("footer")
			}
			a.updateFooterStatus()
		}
	default:
	}
}

func (a *App) runCommand(cmd string) {
	if cmd == "" {
		return
	}
	fields := strings.Fields(cmd)
	if len(fields) == 0 {
		return
	}
	switch strings.ToLower(fields[0]) {
	case "svc", "service", "services":
		a.setView(viewServices)
	case "pool", "pools", "cp":
		a.setView(viewPools)
	case "repo", "repos", "image", "images":
		a.setView(viewRepos)
	case "inst", "instances":
		a.openInstancesView()
	case "ns", "namespace", "schema":
		if len(fields) < 2 {
			a.showError("Usage: :ns <schema>")
			return
		}
		a.setSchema(fields[1])
	case "help", "?":
		a.toggleHelp()
	default:
		a.showError(fmt.Sprintf("Unknown command: %s", fields[0]))
	}
}

func (a *App) setView(view viewKind) {
	if view == viewInstances && a.activeService == "" {
		a.showError("Select a service first to view instances")
		return
	}
	a.view = view
	a.header.SetView(string(view))
	title := fmt.Sprintf(" %s ", view)
	if view == viewInstances && a.activeService != "" {
		title = fmt.Sprintf(" %s (%s) ", view, a.activeService)
	}
	a.table.SetTitle(title).SetTitleAlign(tview.AlignLeft)
	a.table.SetFilter("")
	a.filterField.SetText("")
	go a.fetchCurrentView(context.Background())
}

func (a *App) setSchema(schema string) {
	if strings.TrimSpace(schema) == "" {
		return
	}
	a.cfg.Schema = schema
	a.spcs.SetSchema(schema)
	a.header.Refresh()
	a.fetchCurrentView(context.Background())
}

func (a *App) openInstancesView() {
	if a.view != viewServices {
		a.showError("Instances view requires Services selection")
		return
	}
	row, ok := a.table.SelectedRow()
	if !ok || len(row.Cells) < 2 {
		a.showError("Select a service first to view instances")
		return
	}
	a.activeService = row.Cells[1]
	a.setView(viewInstances)
}

func (a *App) openDetail() {
	row, ok := a.table.SelectedRow()
	if !ok {
		return
	}
	content := a.buildDetail(row)
	a.detailView.SetText(content)
	a.detailVisible = true
	a.pages.ShowPage("detail")
	a.app.SetFocus(a.detailView)
}

func (a *App) closeDetail() {
	a.detailVisible = false
	a.pages.HidePage("detail")
	a.app.SetFocus(a.table)
}

func (a *App) buildDetail(row TableRow) string {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	switch a.view {
	case viewServices:
		name := ""
		if len(row.Cells) > 1 {
			name = row.Cells[1]
		}
		if name == "" {
			return "No service selected."
		}
		descr, err := a.spcs.DescribeService(ctx, name)
		if err != nil {
			return fmt.Sprintf("Describe service failed: %v", err)
		}
		instances, instErr := a.spcs.ListServiceInstances(ctx, name)
		var b strings.Builder
		b.WriteString(fmt.Sprintf("Service: %s\n\n", name))
		b.WriteString(formatKeyValues(descr))
		b.WriteString("\n\nInstances:\n")
		if instErr != nil {
			b.WriteString(fmt.Sprintf("  Error: %v\n", instErr))
			return b.String()
		}
		if len(instances) == 0 {
			b.WriteString("  (none)\n")
			return b.String()
		}
		for _, inst := range instances {
			b.WriteString(fmt.Sprintf("  %s  %s  %s  %s\n", inst.Name, strings.ToUpper(inst.Status), inst.Node, inst.Age))
		}
		return b.String()
	case viewPools:
		return formatKeyValues(mapFromRow(row, a.table.Headers()))
	case viewRepos:
		return formatKeyValues(mapFromRow(row, a.table.Headers()))
	case viewInstances:
		return formatKeyValues(mapFromRow(row, a.table.Headers()))
	default:
		return "No details available."
	}
}

func (a *App) loadViewData(ctx context.Context) (viewData, error) {
	switch a.view {
	case viewServices:
		services, err := a.spcs.ListServices(ctx)
		if err != nil {
			return viewData{}, err
		}
		headers := []string{"NAMESPACE", "NAME", "STATUS", "POOL", "AGE"}
		rows := make([]TableRow, 0, len(services))
		for _, s := range services {
			age := s.Age
			if age == "" && !s.CreatedAt.IsZero() {
				age = models.HumanizeAge(s.CreatedAt)
			}
			rows = append(rows, TableRow{Cells: []string{s.Namespace, s.Name, strings.ToUpper(s.Status), s.ComputePool, age}})
		}
		if len(rows) == 0 {
			return viewData{headers: headers, rows: rows, statusColumn: 2, warning: fmt.Sprintf("No items found in %s", a.cfg.Schema)}, nil
		}
		return viewData{headers: headers, rows: rows, statusColumn: 2}, nil
	case viewPools:
		pools, err := a.spcs.ListComputePools(ctx)
		if err != nil {
			return viewData{}, err
		}
		headers := []string{"NAME", "STATE", "MIN", "MAX", "FAMILY", "AGE"}
		rows := make([]TableRow, 0, len(pools))
		for _, p := range pools {
			age := p.Age
			if age == "" && !p.CreatedAt.IsZero() {
				age = models.HumanizeAge(p.CreatedAt)
			}
			rows = append(rows, TableRow{Cells: []string{p.Name, strings.ToUpper(p.State), p.MinNodes, p.MaxNodes, p.InstanceFamily, age}})
		}
		if len(rows) == 0 {
			return viewData{headers: headers, rows: rows, statusColumn: 1, warning: "No items found in compute pools"}, nil
		}
		return viewData{headers: headers, rows: rows, statusColumn: 1}, nil
	case viewRepos:
		repos, err := a.spcs.ListImageRepositories(ctx)
		if err != nil {
			return viewData{}, err
		}
		headers := []string{"NAME", "REPO_URL", "OWNER", "AGE"}
		rows := make([]TableRow, 0, len(repos))
		for _, r := range repos {
			age := r.Age
			if age == "" && !r.CreatedAt.IsZero() {
				age = models.HumanizeAge(r.CreatedAt)
			}
			rows = append(rows, TableRow{Cells: []string{r.Name, r.RepositoryURL, r.Owner, age}})
		}
		if len(rows) == 0 {
			return viewData{headers: headers, rows: rows, statusColumn: -1, warning: fmt.Sprintf("No items found in %s.%s", a.cfg.Database, a.cfg.Schema)}, nil
		}
		return viewData{headers: headers, rows: rows, statusColumn: -1}, nil
	case viewInstances:
		if a.activeService == "" {
			headers := []string{"INSTANCE", "STATUS", "NODE", "AGE"}
			return viewData{headers: headers, rows: nil, statusColumn: -1, warning: "Select a service to view instances"}, nil
		}
		instances, err := a.spcs.ListServiceInstances(ctx, a.activeService)
		if err != nil {
			return viewData{}, err
		}
		headers := []string{"INSTANCE", "STATUS", "NODE", "AGE"}
		rows := make([]TableRow, 0, len(instances))
		for _, inst := range instances {
			age := inst.Age
			if age == "" && !inst.CreatedAt.IsZero() {
				age = models.HumanizeAge(inst.CreatedAt)
			}
			rows = append(rows, TableRow{Cells: []string{inst.Name, strings.ToUpper(inst.Status), inst.Node, age}})
		}
		if len(rows) == 0 {
			return viewData{headers: headers, rows: rows, statusColumn: 1, warning: fmt.Sprintf("No instances found for %s", a.activeService)}, nil
		}
		return viewData{headers: headers, rows: rows, statusColumn: 1}, nil
	default:
		return viewData{}, nil
	}
}

func mapFromRow(row TableRow, headers []string) map[string]string {
	out := make(map[string]string, len(headers))
	for i, h := range headers {
		if i < len(row.Cells) {
			out[h] = row.Cells[i]
		}
	}
	return out
}

func formatKeyValues(values map[string]string) string {
	if len(values) == 0 {
		return "(no details)"
	}
	keys := make([]string, 0, len(values))
	for k := range values {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var b strings.Builder
	for _, k := range keys {
		v := values[k]
		if strings.TrimSpace(v) == "" {
			continue
		}
		b.WriteString(fmt.Sprintf("%s: %s\n", k, v))
	}
	return b.String()
}

func (a *App) toggleHelp() {
	if a.helpVisible {
		a.showError("")
		a.helpVisible = false
		return
	}
	a.helpVisible = true
	help := "j/k/↓/↑ move  g/G top/bottom  / filter  : cmd  s/p/r views  i instances  b back  enter details  esc clear  ctrl+r refresh  q quit"
	a.showError(help)
}

func (a *App) updateFooterStatus() {
	filterText := a.filterField.GetText()
	parts := []string{a.table.SelectionInfo()}
	if a.inputMode == inputFilter && strings.TrimSpace(filterText) != "" {
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
	headers := []string{"NAMESPACE", "NAME", "STATUS", "POOL", "AGE"}
	rows := make([][]string, 0, len(services))
	for _, s := range services {
		status := strings.ToUpper(s.Status)
		age := s.Age
		if age == "" && !s.CreatedAt.IsZero() {
			age = models.HumanizeAge(s.CreatedAt)
		}
		rows = append(rows, []string{s.Namespace, s.Name, status, s.ComputePool, age})
	}

	widths := make([]int, len(headers))
	for i, h := range headers {
		widths[i] = len(h)
	}
	for _, row := range rows {
		for i, v := range row {
			if l := len(v); l > widths[i] {
				widths[i] = l
			}
		}
	}

	drawLine := func(left, mid, right string) {
		fmt.Print(left)
		for i, w := range widths {
			fmt.Print(strings.Repeat("─", w+2))
			if i < len(widths)-1 {
				fmt.Print(mid)
			}
		}
		fmt.Println(right)
	}

	drawLine("┌", "┬", "┐")
	fmt.Print("│")
	for i, h := range headers {
		fmt.Printf(" %-*s ", widths[i], h)
		fmt.Print("│")
	}
	fmt.Println()
	drawLine("├", "┼", "┤")
	for _, row := range rows {
		fmt.Print("│")
		for i, v := range row {
			fmt.Printf(" %-*s ", widths[i], v)
			fmt.Print("│")
		}
		fmt.Println()
	}
	drawLine("└", "┴", "┘")
}

func defaultKeyHints() []string {
	return []string{"j/k/↓/↑ Move", "g/G Top/Bottom", "ctrl+d/ctrl+u Page", "s/p/r Views", "i Instances", "b Back", "enter Details", "/ Filter", ": Cmd", "ctrl+r Refresh", "q Quit"}
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
