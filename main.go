package main

import (
	"embed"
	"log"
	"os"
	"strconv"
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

func main() {
	var app *application.App
	var window *application.WebviewWindow
	// Set once the user agreed to close; quitting then proceeds.
	var closeApproved atomic.Bool

	session := service.NewSession(initialDocument(), func(name string, data any) {
		app.Event.Emit(name, data)
	})
	dialogs := service.WailsDialogs{Window: func() application.Window { return window }}
	sheets := service.NewSheetService(session)
	docs := service.NewDocumentService(session, dialogs)

	app = application.New(application.Options{
		Name:        "WW Excel",
		Description: "Spreadsheet app for macOS",
		Services: []application.Service{
			application.NewService(sheets),
			application.NewService(docs),
			application.NewService(service.NewBenchService(sheets, docs)),
		},
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(assets),
		},
		Mac: application.MacOptions{
			ApplicationShouldTerminateAfterLastWindowClosed: true,
		},
		// ⌘Q does not pass through the window's closing hook, so route
		// it there: cancel this quit and close the window, which asks
		// about unsaved changes and then terminates the app.
		ShouldQuit: func() bool {
			if closeApproved.Load() || !session.Dirty() {
				return true
			}
			go window.Close()
			return false
		},
	})

	menus := buildMenu(app)
	app.Menu.Set(menus.root)

	window = app.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:            windowTitle(session.State()),
		Width:            1280,
		Height:           800,
		BackgroundColour: application.NewRGB(255, 255, 255),
		URL:              "/",
	})

	docs.CloseWindow = window.Close
	session.OnState = func(st service.DocState) {
		window.SetTitle(windowTitle(st))
		menus.update(st)
	}

	// Ask about unsaved changes before closing. The hook runs on the UI
	// thread, so the dialog is shown from a goroutine and the close is
	// retried once the user decides.
	window.RegisterHook(events.Common.WindowClosing, func(e *application.WindowEvent) {
		if closeApproved.Load() || !session.Dirty() {
			return
		}
		e.Cancel()
		go func() {
			if docs.ConfirmClose() {
				closeApproved.Store(true)
				window.Close()
			}
		}()
	})

	if len(os.Args) > 1 {
		path := os.Args[1]
		app.Event.OnApplicationEvent(events.Common.ApplicationStarted, func(*application.ApplicationEvent) {
			go docs.OpenPath(path)
		})
	}

	if err := app.Run(); err != nil {
		log.Fatal(err)
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
}

// buildMenu creates the menu bar. Custom items forward an action to the
// frontend, which commits any in-progress edit before acting; the
// clipboard items use native roles so text fields keep working.
func buildMenu(app *application.App) *appMenus {
	send := func(action string) func(*application.Context) {
		return func(*application.Context) { app.Event.Emit(service.EventMenuAction, action) }
	}

	root := app.NewMenu()
	root.AddRole(application.AppMenu)

	file := root.AddSubmenu("檔案")
	file.Add("新增").SetAccelerator("CmdOrCtrl+N").OnClick(send("new"))
	file.Add("開啟…").SetAccelerator("CmdOrCtrl+O").OnClick(send("open"))
	file.AddSeparator()
	file.Add("關閉視窗").SetAccelerator("CmdOrCtrl+W").OnClick(send("close"))
	file.Add("儲存").SetAccelerator("CmdOrCtrl+S").OnClick(send("save"))
	file.Add("另存新檔…").SetAccelerator("CmdOrCtrl+Shift+S").OnClick(send("saveAs"))

	edit := root.AddSubmenu("編輯")
	m := &appMenus{root: root}
	m.undo = edit.Add("復原").SetAccelerator("CmdOrCtrl+Z").OnClick(send("undo")).SetEnabled(false)
	m.redo = edit.Add("重做").SetAccelerator("CmdOrCtrl+Shift+Z").OnClick(send("redo")).SetEnabled(false)
	edit.AddSeparator()
	edit.AddRole(application.Cut)
	edit.AddRole(application.Copy)
	edit.AddRole(application.Paste)
	edit.Add("全選").SetAccelerator("CmdOrCtrl+A").OnClick(send("selectAll"))

	insert := root.AddSubmenu("插入")
	insert.Add("工作表").SetAccelerator("Shift+F11").OnClick(send("insertSheet"))

	format := root.AddSubmenu("格式")
	format.Add("儲存格格式…").SetAccelerator("CmdOrCtrl+1").OnClick(send("formatCells"))
	format.AddSeparator()
	format.Add("粗體").SetAccelerator("CmdOrCtrl+B").OnClick(send("bold"))
	format.Add("斜體").SetAccelerator("CmdOrCtrl+I").OnClick(send("italic"))
	format.Add("底線").SetAccelerator("CmdOrCtrl+U").OnClick(send("underline"))
	format.AddSeparator()
	format.Add("自動調整欄寬").OnClick(send("autofitCols"))
	format.Add("自動調整列高").OnClick(send("autofitRows"))

	root.AddRole(application.WindowMenu)
	return m
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
