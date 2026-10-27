package main

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"

	"ww-excel/internal/document"
	"ww-excel/internal/i18n"
	"ww-excel/internal/model"
	"ww-excel/internal/recovery"
	"ww-excel/internal/service"
	"ww-excel/internal/settings"
	"ww-excel/internal/update"
)

//go:embed all:frontend/dist
var assets embed.FS

// Use PNG for runtime Dock/About images; ICNS is reserved for bundle resources.
//
//go:embed build/appicon.png
var appIcon []byte

// Set at build time (scripts/package.sh) with -ldflags "-X main.version=…".
var (
	version = "1.1.0"
	// updateRepo is the GitHub repository whose releases are checked.
	updateRepo = "andrewlu0210/ww-excel"
)

func init() {
	// Registered events get strongly typed TS bindings.
	application.RegisterEvent[service.BenchEvent](service.BenchEventName)
	application.RegisterEvent[service.CellsChanged](service.EventCellsChanged)
	application.RegisterEvent[service.LayoutChanged](service.EventLayoutChanged)
	application.RegisterEvent[service.DocState](service.EventSheetsChanged)
	application.RegisterEvent[service.DocState](service.EventDocState)
	application.RegisterEvent[service.DocState](service.EventDocReset)
	application.RegisterEvent[string](service.EventMenuAction)
	application.RegisterEvent[service.UISettings](service.EventSettingsChanged)
}

// docApp owns the document windows: one session and window per document.
type docApp struct {
	app      *application.App
	hub      *service.Hub
	docs     *service.DocumentService
	menus    *appMenus
	recent   *service.RecentFiles
	settings *service.SettingsService
	autosave *service.Autosaver

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
	// The unattended modes must not touch the user's preferences, recent
	// files or recovery snapshots.
	settingsPath, recoveryDir, recentPath := settings.DefaultPath(), recovery.DefaultDir(), service.DefaultRecentPath()
	preferred := preferredLanguages()
	if unattended() {
		tmp, _ := os.MkdirTemp("", "ww-excel-test")
		settingsPath, recoveryDir = filepath.Join(tmp, "settings.json"), filepath.Join(tmp, "Recovery")
		recentPath = filepath.Join(tmp, "recent.json")
		// The checks compare Chinese UI text; WW_LANG=en previews the
		// English UI (the checks on text then fail).
		preferred = []string{"zh-Hant-TW"}
		if l := os.Getenv("WW_LANG"); l != "" {
			preferred = []string{l}
		}
	}
	a.recent = service.LoadRecent(recentPath)
	a.settings = service.NewSettingsService(settings.Load(settingsPath), preferred)
	a.autosave = service.NewAutosaver(recovery.New(recoveryDir), a.hub)
	dialogs := service.WailsDialogs{Window: func() application.Window { return a.app.Window.Current() }}
	sheets := service.NewSheetService(a.hub)
	a.docs = service.NewDocumentService(a.hub, dialogs)
	a.docs.OpenWindow = func(doc *document.Document) { a.openWindow(doc) }
	a.docs.FocusPath = a.focusPath
	a.docs.Opened = a.recent.Add
	a.docs.PrintPDF = printPDF
	service.FontSource = fontData

	a.app = application.New(application.Options{
		Name:        "WW Excel",
		Icon:        appIcon,
		Description: "Spreadsheet app for macOS",
		Services: []application.Service{
			application.NewService(sheets),
			application.NewService(a.docs),
			application.NewService(a.settings),
			application.NewService(service.NewBenchService(sheets, a.docs)),
		},
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(assets),
			// Print previews are PDFs rendered on demand; fonts the page
			// cannot use directly are served from CoreText.
			Middleware: application.ChainMiddleware(service.PrintHandler, service.FontHandler),
		},
		Mac: application.MacOptions{
			ApplicationShouldTerminateAfterLastWindowClosed: true,
		},
		// ⌘Q does not pass through the windows' closing hooks, so ask
		// about each unsaved document here, then quit.
		ShouldQuit: a.shouldQuit,
		// A normal quit leaves no recovery snapshots behind.
		OnShutdown: func() {
			for _, s := range a.hub.All() {
				a.autosave.Forget(s)
			}
		},
	})
	service.ConnectClipboard(sheets, a.app.Clipboard.SetText, a.app.Clipboard.Text)

	a.buildMenus()
	a.recent.OnChange = func(files []string) {
		onMain(func() { a.menus.setRecent(a, files) })
	}
	a.settings.Changed = a.settingsChanged

	first := a.openWindow(initialDocument())

	stopAutosave := make(chan struct{})
	defer close(stopAutosave)
	go a.autosave.Run(func() time.Duration {
		return time.Duration(a.settings.Get().AutoRecoverMinutes) * time.Minute
	}, stopAutosave)

	a.app.Event.OnApplicationEvent(events.Common.ApplicationStarted, func(*application.ApplicationEvent) {
		// Finder cannot supply a bundle icon for direct executable launches.
		// Reapply after Cocoa finishes launching so it does not reset the Dock image.
		a.app.SetIcon(appIcon)
		setAppearance(a.settings.Get().Appearance)
		// Offer the snapshots of a session that crashed.
		go service.Recover(a.docs, service.WindowContext(first.win), a.autosave.Store)
		if a.settings.Get().CheckUpdates && !unattended() {
			go a.checkUpdates(false)
		}
	})

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
			a.updateMenus(st)
		}
	}
	w.win.OnWindowEvent(events.Common.WindowFocus, func(*application.WindowEvent) {
		a.updateMenus(w.session.State())
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
		a.autosave.Forget(w.session)
		w.session.Close()
	}
}

// onMain runs a menu change on the main thread. Menu changes must not
// overlap: Menu.Update rebuilds every NSMenuItem, and a label set from
// another goroutine at the same time would write to a released item
// (SIGSEGV). Running them all on the main thread serializes them; on
// the main thread itself fn runs inline.
func onMain(fn func()) { application.InvokeSync(fn) }

// setMenus rebuilds the menu bar in the current language.
func (a *docApp) setMenus() { onMain(a.buildMenus) }

// buildMenus creates the menu bar. Called directly only before the app
// runs, when there is no main-thread loop to dispatch to yet.
func (a *docApp) buildMenus() {
	a.menus = buildMenu(a)
	a.app.Menu.Set(a.menus.root)
	a.menus.setRecent(a, a.recent.List())
}

// updateMenus shows a document's undo/redo state in the menu bar.
func (a *docApp) updateMenus(st service.DocState) {
	onMain(func() { a.menus.update(st) })
}

// settingsChanged applies new preferences to the app and every window.
func (a *docApp) settingsChanged(old, cur service.UISettings) {
	if old.Appearance != cur.Appearance {
		setAppearance(cur.Appearance)
	}
	if old.Lang != cur.Lang {
		a.setMenus()
		a.mu.Lock()
		for _, w := range a.windows {
			st := w.session.State()
			w.win.SetTitle(windowTitle(st))
		}
		a.mu.Unlock()
		if w := a.app.Window.Current(); w != nil {
			if dw := a.windowFor(w); dw != nil {
				a.updateMenus(dw.session.State())
			}
		}
	}
	a.app.Event.Emit(service.EventSettingsChanged, cur)
}

// checkUpdates looks for a newer release. At launch (manual false) it
// stays quiet unless there is one; from the menu it always answers.
func (a *docApp) checkUpdates(manual bool) {
	rel, err := update.Latest(context.Background(), http.DefaultClient, updateRepo)
	if errors.Is(err, update.ErrNoRelease) {
		// Nothing published yet: the running version is the newest.
		rel, err = update.Release{Version: version}, nil
	}
	if err != nil {
		if manual {
			a.app.Dialog.Warning().SetTitle(T("無法檢查更新")).SetMessage(err.Error()).Show()
		}
		return
	}
	if !update.Newer(rel.Version, version) {
		if manual {
			a.app.Dialog.Info().SetTitle(T("已是最新版本")).SetMessage(fmt.Sprintf(T("WW Excel %s 是目前最新的版本。"), version)).Show()
		}
		return
	}
	// Dialogs are shown from this goroutine, like service.WailsDialogs.
	d := a.app.Dialog.Question().
		SetTitle(fmt.Sprintf(T("有新版本：WW Excel %s"), rel.Version)).
		SetMessage(fmt.Sprintf(T("您目前使用的是 %s。要前往下載頁面嗎？"), version))
	dl := d.AddButton(T("下載")).OnClick(func() { _ = a.app.Browser.OpenURL(rel.URL) })
	later := d.AddButton(T("稍後"))
	d.SetDefaultButton(dl).SetCancelButton(later)
	d.Show()
}

// unattended reports the bench and self-test modes, which must not
// touch the user's settings or show unexpected dialogs.
func unattended() bool {
	return os.Getenv("WW_SELFTEST") == "1" || os.Getenv("WW_BENCH") == "1"
}

func (a *docApp) windowFor(w application.Window) *docWindow {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.windows[w.ID()]
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
		return st.Name + i18n.T(" — 已編輯")
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
	app := root.AddSubmenu("WW Excel")
	app.Add(T("關於 WW Excel")).OnClick(func(*application.Context) { showAbout() })
	app.AddSeparator()
	app.Add(T("偏好設定…")).SetAccelerator("CmdOrCtrl+,").OnClick(send("preferences"))
	app.AddSeparator()
	app.AddRole(application.ServicesMenu)
	app.AddSeparator()
	app.AddRole(application.Hide)
	app.AddRole(application.HideOthers)
	app.AddRole(application.UnHide)
	app.AddSeparator()
	app.AddRole(application.Quit)

	file := root.AddSubmenu(T("檔案"))
	file.Add(T("新增活頁簿")).SetAccelerator("CmdOrCtrl+N").OnClick(send("new"))
	file.Add(T("開啟…")).SetAccelerator("CmdOrCtrl+O").OnClick(send("open"))
	m := &appMenus{root: root}
	m.recent = file.AddSubmenu(T("最近使用的檔案"))
	file.AddSeparator()
	file.Add(T("關閉視窗")).SetAccelerator("CmdOrCtrl+W").OnClick(send("close"))
	file.Add(T("儲存")).SetAccelerator("CmdOrCtrl+S").OnClick(send("save"))
	file.Add(T("另存新檔…")).SetAccelerator("CmdOrCtrl+Shift+S").OnClick(send("saveAs"))
	file.AddSeparator()
	file.Add(T("版面設定…")).OnClick(send("pageSetup"))
	file.Add(T("列印預覽…")).OnClick(send("printPreview"))
	file.Add(T("列印…")).SetAccelerator("CmdOrCtrl+P").OnClick(send("print"))
	file.Add(T("匯出為 PDF…")).OnClick(send("exportPDF"))

	edit := root.AddSubmenu(T("編輯"))
	m.undo = edit.Add(T("復原")).SetAccelerator("CmdOrCtrl+Z").OnClick(send("undo")).SetEnabled(false)
	m.redo = edit.Add(T("重做")).SetAccelerator("CmdOrCtrl+Shift+Z").OnClick(send("redo")).SetEnabled(false)
	edit.AddSeparator()
	edit.AddRole(application.Cut)
	edit.AddRole(application.Copy)
	edit.AddRole(application.Paste)
	edit.Add(T("選擇性貼上…")).SetAccelerator("CmdOrCtrl+Ctrl+V").OnClick(send("pasteSpecial"))
	edit.Add(T("全選")).SetAccelerator("CmdOrCtrl+A").OnClick(send("selectAll"))
	edit.AddSeparator()
	fill := edit.AddSubmenu(T("填滿"))
	fill.Add(T("向下")).SetAccelerator("CmdOrCtrl+D").OnClick(send("fillDown"))
	fill.Add(T("向右")).SetAccelerator("CmdOrCtrl+R").OnClick(send("fillRight"))
	clear := edit.AddSubmenu(T("清除"))
	clear.Add(T("全部")).OnClick(send("clearAll"))
	clear.Add(T("格式")).OnClick(send("clearFormats"))
	clear.Add(T("內容")).OnClick(send("clearContents"))
	edit.Add(T("刪除…")).SetAccelerator("CmdOrCtrl+-").OnClick(send("deleteCells"))
	edit.AddSeparator()
	edit.Add(T("尋找…")).SetAccelerator("CmdOrCtrl+F").OnClick(send("find"))
	edit.Add(T("取代…")).SetAccelerator("CmdOrCtrl+Shift+H").OnClick(send("replace"))

	view := root.AddSubmenu(T("檢視"))
	view.Add(T("凍結窗格")).OnClick(send("freezePanes"))
	view.Add(T("凍結頂端列")).OnClick(send("freezeTopRow"))
	view.Add(T("凍結第一欄")).OnClick(send("freezeFirstCol"))
	view.Add(T("取消凍結窗格")).OnClick(send("unfreeze"))

	insert := root.AddSubmenu(T("插入"))
	insert.Add(T("儲存格…")).SetAccelerator("CmdOrCtrl+Shift+=").OnClick(send("insertCells"))
	insert.Add(T("列")).OnClick(send("insertRows"))
	insert.Add(T("欄")).OnClick(send("insertCols"))
	insert.Add(T("工作表")).SetAccelerator("Shift+F11").OnClick(send("insertSheet"))
	insert.AddSeparator()
	insert.Add(T("表格…")).SetAccelerator("CmdOrCtrl+T").OnClick(send("insertTable"))
	insert.Add(T("註解")).SetAccelerator("Shift+F2").OnClick(send("insertNote"))
	insert.AddSeparator()
	insert.Add(T("圖表…")).OnClick(send("insertChart"))
	insert.Add(T("圖片…")).OnClick(send("insertPicture"))

	format := root.AddSubmenu(T("格式"))
	format.Add(T("儲存格格式…")).SetAccelerator("CmdOrCtrl+1").OnClick(send("formatCells"))
	format.AddSeparator()
	format.Add(T("粗體")).SetAccelerator("CmdOrCtrl+B").OnClick(send("bold"))
	format.Add(T("斜體")).SetAccelerator("CmdOrCtrl+I").OnClick(send("italic"))
	format.Add(T("底線")).SetAccelerator("CmdOrCtrl+U").OnClick(send("underline"))
	format.AddSeparator()
	cf := format.AddSubmenu(T("設定格式化的條件"))
	cf.Add(T("新增規則…")).OnClick(send("condFormat"))
	cf.Add(T("管理規則…")).OnClick(send("condFormatManager"))
	cf.AddSeparator()
	cf.Add(T("清除所選儲存格的規則")).OnClick(send("clearCondFormats"))
	cf.Add(T("清除整張工作表的規則")).OnClick(send("clearAllCondFormats"))
	format.AddSeparator()
	rows := format.AddSubmenu(T("列"))
	rows.Add(T("自動調整列高")).OnClick(send("autofitRows"))
	rows.Add(T("隱藏")).SetAccelerator("Ctrl+9").OnClick(send("hideRows"))
	rows.Add(T("取消隱藏")).SetAccelerator("Ctrl+Shift+9").OnClick(send("unhideRows"))
	cols := format.AddSubmenu(T("欄"))
	cols.Add(T("自動調整欄寬")).OnClick(send("autofitCols"))
	cols.Add(T("隱藏")).SetAccelerator("Ctrl+0").OnClick(send("hideCols"))
	cols.Add(T("取消隱藏")).SetAccelerator("Ctrl+Shift+0").OnClick(send("unhideCols"))

	formulas := root.AddSubmenu(T("公式"))
	formulas.Add(T("定義名稱…")).OnClick(send("defineName"))
	formulas.Add(T("名稱管理員…")).SetAccelerator("CmdOrCtrl+F3").OnClick(send("nameManager"))

	data := root.AddSubmenu(T("資料"))
	data.Add(T("排序…")).OnClick(send("sort"))
	data.Add(T("遞增排序")).OnClick(send("sortAsc"))
	data.Add(T("遞減排序")).OnClick(send("sortDesc"))
	data.AddSeparator()
	data.Add(T("篩選")).SetAccelerator("CmdOrCtrl+Shift+F").OnClick(send("filter"))
	data.AddSeparator()
	data.Add(T("資料驗證…")).OnClick(send("dataValidation"))
	data.Add(T("清除資料驗證")).OnClick(send("clearValidation"))

	root.AddRole(application.WindowMenu)

	help := root.AddSubmenu(T("說明"))
	help.Add(T("檢查更新…")).OnClick(func(*application.Context) { go a.checkUpdates(true) })
	return m
}

// setRecent rebuilds 最近使用的檔案; it must run on the main thread.
func (m *appMenus) setRecent(a *docApp, files []string) {
	m.recent.Clear()
	if len(files) == 0 {
		m.recent.Add(T("（無）")).SetEnabled(false)
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
		m.recent.Add(T("清除最近使用的檔案")).OnClick(func(*application.Context) { a.recent.Clear() })
	}
	m.root.Update()
}

// update must run on the main thread (docApp.updateMenus). The items
// change in place, so the menu is not rebuilt.
func (m *appMenus) update(st service.DocState) {
	m.undo.SetLabel(withAction("復原", st.UndoLabel)).SetEnabled(st.CanUndo)
	m.redo.SetLabel(withAction("重做", st.RedoLabel)).SetEnabled(st.CanRedo)
}

func withAction(verb, label string) string {
	if label == "" {
		return T(verb)
	}
	return fmt.Sprintf(T(verb+"「%s」"), T(label))
}

// T translates a UI string (menus are rebuilt when the language changes).
func T(s string) string { return i18n.T(s) }

func envInt(key string, def int) int {
	if v, err := strconv.Atoi(os.Getenv(key)); err == nil && v > 0 {
		return v
	}
	return def
}
