package main

import (
	"context"
	"embed"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"sync/atomic"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"

	"ww-excel/internal/document"
	"ww-excel/internal/model"
	"ww-excel/internal/service"
)

//go:embed all:frontend/dist
var assets embed.FS

func init() {
	// Registered events get strongly typed TS bindings.
	application.RegisterEvent[service.BenchEvent](service.BenchEventName)
	application.RegisterEvent[service.CellsChanged](service.EventCellsChanged)
	application.RegisterEvent[service.LayoutChanged](service.EventLayoutChanged)
	application.RegisterEvent[service.DocState](service.EventSheetsChanged)
	application.RegisterEvent[service.DocState](service.EventDocState)
	application.RegisterEvent[service.DocState](service.EventDocReset)
	application.RegisterEvent[string](service.EventMenuAction)
}

// docApp owns the document windows: one session and window per document.
type docApp struct {
	app    *application.App
	hub    *service.Hub
	docs   *service.DocumentService
	menus  *appMenus
	recent *service.RecentFiles

	mu      sync.Mutex
	windows map[uint]*docWindow
	nextID  int
	// Set once the user agreed to quit with unsaved changes.
	quitApproved atomic.Bool
}

type docWindow struct {
	win      *application.WebviewWindow
	session  *service.Session
	approved atomic.Bool // the user agreed to close it
}

func main() {
	a := &docApp{hub: service.NewHub(), windows: map[uint]*docWindow{}}
	a.recent = service.LoadRecent(service.DefaultRecentPath())
	dialogs := service.WailsDialogs{Window: func() application.Window { return a.app.Window.Current() }}
	sheets := service.NewSheetService(a.hub)
	a.docs = service.NewDocumentService(a.hub, dialogs)
	a.docs.OpenWindow = func(doc *document.Document) { a.openWindow(doc) }
	a.docs.FocusPath = a.focusPath
	a.docs.Opened = a.recent.Add

	a.app = application.New(application.Options{
		Name:        "WW Excel",
		Description: "Spreadsheet app for macOS",
		Services: []application.Service{
			application.NewService(sheets),
			application.NewService(a.docs),
			application.NewService(service.NewBenchService(sheets, a.docs)),
		},
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(assets),
		},
		Mac: application.MacOptions{
			ApplicationShouldTerminateAfterLastWindowClosed: true,
		},
		// ⌘Q does not pass through the windows' closing hooks, so ask
		// about each unsaved document here, then quit.
		ShouldQuit: a.shouldQuit,
	})
	service.ConnectClipboard(sheets, a.app.Clipboard.SetText, a.app.Clipboard.Text)

	a.menus = buildMenu(a)
	a.app.Menu.Set(a.menus.root)
	a.recent.OnChange = func(files []string) { a.menus.setRecent(a, files) }
	a.menus.setRecent(a, a.recent.List())

	first := a.openWindow(initialDocument())

	// Files from the command line or from Finder (double-click, drop on
	// the Dock icon) open like 檔案 > 開啟.
	if len(os.Args) > 1 {
		paths := os.Args[1:]
		a.app.Event.OnApplicationEvent(events.Common.ApplicationStarted, func(*application.ApplicationEvent) {
			go func() {
				for _, p := range paths {
					a.docs.OpenFile(service.WindowContext(first.win), p)
				}
			}()
		})
	}
	a.app.Event.OnApplicationEvent(events.Common.ApplicationOpenedWithFile, func(e *application.ApplicationEvent) {
		if path := e.Context().Filename(); path != "" {
			go a.docs.OpenFile(a.currentContext(), path)
		}
	})

	if err := a.app.Run(); err != nil {
		log.Fatal(err)
	}
}

// openWindow shows a document in a new window.
func (a *docApp) openWindow(doc *document.Document) *docWindow {
	a.mu.Lock()
	a.nextID++
	n := a.nextID
	a.mu.Unlock()

	w := &docWindow{}
	w.session = service.NewSession(doc, func(name string, data any) {
		if w.win != nil {
			w.win.EmitEvent(name, data)
		}
	})
	offset := (n - 1) % 8 * 24
	w.win = a.app.Window.NewWithOptions(application.WebviewWindowOptions{
		Name:             fmt.Sprintf("doc-%d", n),
		Title:            windowTitle(w.session.State()),
		Width:            1280,
		Height:           800,
		X:                80 + offset,
		Y:                60 + offset,
		InitialPosition:  application.WindowXY,
		BackgroundColour: application.NewRGB(255, 255, 255),
		URL:              "/",
	})
	id := w.win.ID()
	a.hub.Add(id, w.session)
	a.mu.Lock()
	a.windows[id] = w
	a.mu.Unlock()

	w.session.CloseWindow = w.win.Close
	w.session.OnState = func(st service.DocState) {
		w.win.SetTitle(windowTitle(st))
		if w.win.IsFocused() {
			a.menus.update(st)
		}
	}
	w.win.OnWindowEvent(events.Common.WindowFocus, func(*application.WindowEvent) {
		a.menus.update(w.session.State())
	})

	// Ask about unsaved changes before closing. The hook runs on the UI
	// thread, so the dialog is shown from a goroutine and the close is
	// retried once the user decides.
	w.win.RegisterHook(events.Common.WindowClosing, func(e *application.WindowEvent) {
		if w.approved.Load() || a.quitApproved.Load() || !w.session.Dirty() {
			a.forget(id)
			return
		}
		e.Cancel()
		go func() {
			if a.docs.ConfirmClose(service.WindowContext(w.win)) {
				w.approved.Store(true)
				w.win.Close()
			}
		}()
	})
	return w
}

func (a *docApp) forget(id uint) {
	a.hub.Remove(id)
	a.mu.Lock()
	w := a.windows[id]
	delete(a.windows, id)
	a.mu.Unlock()
	if w != nil {
		w.session.Close()
	}
}

func (a *docApp) shouldQuit() bool {
	if a.quitApproved.Load() {
		return true
	}
	a.mu.Lock()
	var dirty []*docWindow
	for _, w := range a.windows {
		if w.session.Dirty() {
			dirty = append(dirty, w)
		}
	}
	a.mu.Unlock()
	if len(dirty) == 0 {
		return true
	}
	go func() {
		for _, w := range dirty {
			w.win.Focus()
			if !a.docs.ConfirmClose(service.WindowContext(w.win)) {
				return
			}
		}
		a.quitApproved.Store(true)
		a.app.Quit()
	}()
	return false
}

func (a *docApp) focusPath(path string) bool {
	id, ok := a.hub.ByPath(path)
	if !ok {
		return false
	}
	a.mu.Lock()
	w := a.windows[id]
	a.mu.Unlock()
	if w == nil {
		return false
	}
	w.win.Focus()
	return true
}

// currentContext addresses the focused document window (or the newest).
func (a *docApp) currentContext() context.Context {
	if w := a.app.Window.Current(); w != nil {
		return service.WindowContext(w)
	}
	return service.WindowContext(nil)
}

// sendToCurrent forwards a menu action to the focused document window.
func (a *docApp) sendToCurrent(action string) {
	w, ok := a.app.Window.Current().(*application.WebviewWindow)
	if !ok || w == nil {
		a.mu.Lock()
		for _, dw := range a.windows {
			w = dw.win
		}
		a.mu.Unlock()
	}
	if w != nil {
		w.EmitEvent(service.EventMenuAction, action)
	}
}

// initialDocument is empty, except in bench mode or when fake data is
// requested via WW_FAKE_ROWS / WW_FAKE_COLS.
func initialDocument() *document.Document {
	rows, cols := envInt("WW_FAKE_ROWS", 0), envInt("WW_FAKE_COLS", 0)
	if os.Getenv("WW_BENCH") == "1" {
		rows, cols = max(rows, 20_000), max(cols, 5)
	}
	if rows == 0 || cols == 0 {
		return document.New()
	}
	wb := model.NewWorkbook()
	model.FillFake(wb.Sheets[0], rows, cols, 1)
	return document.FromWorkbook(wb, "")
}

func windowTitle(st service.DocState) string {
	if st.Dirty {
		return st.Name + " — 已編輯"
	}
	return st.Name
}

type appMenus struct {
	root       *application.Menu
	undo, redo *application.MenuItem
	recent     *application.Menu
}

// buildMenu creates the menu bar. Custom items forward an action to the
// focused window, which commits any in-progress edit before acting; the
// clipboard items use native roles so text fields keep working (the
// grid handles the resulting copy/cut/paste events).
func buildMenu(a *docApp) *appMenus {
	send := func(action string) func(*application.Context) {
		return func(*application.Context) { a.sendToCurrent(action) }
	}

	root := a.app.NewMenu()
	root.AddRole(application.AppMenu)

	file := root.AddSubmenu("檔案")
	file.Add("新增活頁簿").SetAccelerator("CmdOrCtrl+N").OnClick(send("new"))
	file.Add("開啟…").SetAccelerator("CmdOrCtrl+O").OnClick(send("open"))
	m := &appMenus{root: root}
	m.recent = file.AddSubmenu("最近使用的檔案")
	file.AddSeparator()
	file.Add("關閉視窗").SetAccelerator("CmdOrCtrl+W").OnClick(send("close"))
	file.Add("儲存").SetAccelerator("CmdOrCtrl+S").OnClick(send("save"))
	file.Add("另存新檔…").SetAccelerator("CmdOrCtrl+Shift+S").OnClick(send("saveAs"))

	edit := root.AddSubmenu("編輯")
	m.undo = edit.Add("復原").SetAccelerator("CmdOrCtrl+Z").OnClick(send("undo")).SetEnabled(false)
	m.redo = edit.Add("重做").SetAccelerator("CmdOrCtrl+Shift+Z").OnClick(send("redo")).SetEnabled(false)
	edit.AddSeparator()
	edit.AddRole(application.Cut)
	edit.AddRole(application.Copy)
	edit.AddRole(application.Paste)
	edit.Add("選擇性貼上…").SetAccelerator("CmdOrCtrl+Ctrl+V").OnClick(send("pasteSpecial"))
	edit.Add("全選").SetAccelerator("CmdOrCtrl+A").OnClick(send("selectAll"))
	edit.AddSeparator()
	fill := edit.AddSubmenu("填滿")
	fill.Add("向下").SetAccelerator("CmdOrCtrl+D").OnClick(send("fillDown"))
	fill.Add("向右").SetAccelerator("CmdOrCtrl+R").OnClick(send("fillRight"))
	clear := edit.AddSubmenu("清除")
	clear.Add("全部").OnClick(send("clearAll"))
	clear.Add("格式").OnClick(send("clearFormats"))
	clear.Add("內容").OnClick(send("clearContents"))
	edit.Add("刪除…").SetAccelerator("CmdOrCtrl+-").OnClick(send("deleteCells"))
	edit.AddSeparator()
	edit.Add("尋找…").SetAccelerator("CmdOrCtrl+F").OnClick(send("find"))
	edit.Add("取代…").SetAccelerator("CmdOrCtrl+Shift+H").OnClick(send("replace"))

	view := root.AddSubmenu("檢視")
	view.Add("凍結窗格").OnClick(send("freezePanes"))
	view.Add("凍結頂端列").OnClick(send("freezeTopRow"))
	view.Add("凍結第一欄").OnClick(send("freezeFirstCol"))
	view.Add("取消凍結窗格").OnClick(send("unfreeze"))

	insert := root.AddSubmenu("插入")
	insert.Add("儲存格…").SetAccelerator("CmdOrCtrl+Shift+=").OnClick(send("insertCells"))
	insert.Add("列").OnClick(send("insertRows"))
	insert.Add("欄").OnClick(send("insertCols"))
	insert.Add("工作表").SetAccelerator("Shift+F11").OnClick(send("insertSheet"))
	insert.AddSeparator()
	insert.Add("表格…").SetAccelerator("CmdOrCtrl+T").OnClick(send("insertTable"))
	insert.Add("註解").SetAccelerator("Shift+F2").OnClick(send("insertNote"))

	format := root.AddSubmenu("格式")
	format.Add("儲存格格式…").SetAccelerator("CmdOrCtrl+1").OnClick(send("formatCells"))
	format.AddSeparator()
	format.Add("粗體").SetAccelerator("CmdOrCtrl+B").OnClick(send("bold"))
	format.Add("斜體").SetAccelerator("CmdOrCtrl+I").OnClick(send("italic"))
	format.Add("底線").SetAccelerator("CmdOrCtrl+U").OnClick(send("underline"))
	format.AddSeparator()
	cf := format.AddSubmenu("設定格式化的條件")
	cf.Add("新增規則…").OnClick(send("condFormat"))
	cf.Add("管理規則…").OnClick(send("condFormatManager"))
	cf.AddSeparator()
	cf.Add("清除所選儲存格的規則").OnClick(send("clearCondFormats"))
	cf.Add("清除整張工作表的規則").OnClick(send("clearAllCondFormats"))
	format.AddSeparator()
	rows := format.AddSubmenu("列")
	rows.Add("自動調整列高").OnClick(send("autofitRows"))
	rows.Add("隱藏").SetAccelerator("Ctrl+9").OnClick(send("hideRows"))
	rows.Add("取消隱藏").SetAccelerator("Ctrl+Shift+9").OnClick(send("unhideRows"))
	cols := format.AddSubmenu("欄")
	cols.Add("自動調整欄寬").OnClick(send("autofitCols"))
	cols.Add("隱藏").SetAccelerator("Ctrl+0").OnClick(send("hideCols"))
	cols.Add("取消隱藏").SetAccelerator("Ctrl+Shift+0").OnClick(send("unhideCols"))

	formulas := root.AddSubmenu("公式")
	formulas.Add("定義名稱…").OnClick(send("defineName"))
	formulas.Add("名稱管理員…").SetAccelerator("CmdOrCtrl+F3").OnClick(send("nameManager"))

	data := root.AddSubmenu("資料")
	data.Add("排序…").OnClick(send("sort"))
	data.Add("遞增排序").OnClick(send("sortAsc"))
	data.Add("遞減排序").OnClick(send("sortDesc"))
	data.AddSeparator()
	data.Add("篩選").SetAccelerator("CmdOrCtrl+Shift+F").OnClick(send("filter"))
	data.AddSeparator()
	data.Add("資料驗證…").OnClick(send("dataValidation"))
	data.Add("清除資料驗證").OnClick(send("clearValidation"))

	root.AddRole(application.WindowMenu)
	return m
}

// setRecent rebuilds 最近使用的檔案.
func (m *appMenus) setRecent(a *docApp, files []string) {
	m.recent.Clear()
	if len(files) == 0 {
		m.recent.Add("（無）").SetEnabled(false)
	}
	for _, path := range files {
		path := path
		m.recent.Add(filepath.Base(path)).OnClick(func(*application.Context) {
			if _, err := os.Stat(path); err != nil {
				a.recent.Remove(path)
			}
			go a.docs.OpenFile(a.currentContext(), path)
		})
	}
	if len(files) > 0 {
		m.recent.AddSeparator()
		m.recent.Add("清除最近使用的檔案").OnClick(func(*application.Context) { a.recent.Clear() })
	}
	m.root.Update()
}

func (m *appMenus) update(st service.DocState) {
	m.undo.SetLabel(withAction("復原", st.UndoLabel)).SetEnabled(st.CanUndo)
	m.redo.SetLabel(withAction("重做", st.RedoLabel)).SetEnabled(st.CanRedo)
	m.root.Update()
}

func withAction(verb, label string) string {
	if label == "" {
		return verb
	}
	return verb + "「" + label + "」"
}

func envInt(key string, def int) int {
	if v, err := strconv.Atoi(os.Getenv(key)); err == nil && v > 0 {
		return v
	}
	return def
}
