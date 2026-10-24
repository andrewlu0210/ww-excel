package service

import (
	"fmt"
	"os"

	"github.com/wailsapp/wails/v3/pkg/application"

	"ww-excel/internal/i18n"
)

// WailsDialogs shows native macOS dialogs, attached as sheets to the
// window returned by Window.
type WailsDialogs struct {
	Window func() application.Window
}

func (w WailsDialogs) AskSave(docName string) SaveChoice {
	// Button callbacks may run on another thread after Show returns.
	choice := make(chan SaveChoice, 1)
	d := application.Get().Dialog.Question().
		SetTitle(fmt.Sprintf(i18n.T("要儲存對「%s」所做的變更嗎？"), docName)).
		SetMessage(i18n.T("如果不儲存，您的變更將會遺失。"))
	save := d.AddButton(i18n.T("儲存")).OnClick(func() { choice <- ChoiceSave })
	d.AddButton(i18n.T("不儲存")).OnClick(func() { choice <- ChoiceDiscard })
	cancel := d.AddButton(i18n.T("取消")).OnClick(func() { choice <- ChoiceCancel })
	d.SetDefaultButton(save).SetCancelButton(cancel)
	if win := w.window(); win != nil {
		d.AttachToWindow(win)
	}
	d.Show()
	return <-choice
}

func (w WailsDialogs) PickOpen() (string, error) {
	d := application.Get().Dialog.OpenFile().
		CanChooseFiles(true).
		AddFilter(i18n.T("試算表（*.xlsx, *.wwx, *.csv）"), "*.xlsx;*.xlsm;*.wwx;*.csv")
	if win := w.window(); win != nil {
		d.AttachToWindow(win)
	}
	return d.PromptForSingleSelection()
}

func (w WailsDialogs) PickSave(suggestedName string) (string, error) {
	d := application.Get().Dialog.SaveFile().
		SetFilename(suggestedName).
		CanCreateDirectories(true).
		SetMessage(i18n.T("可存成 .xlsx（Excel 活頁簿）、.wwx（原生格式）或 .csv（只保存第一個工作表的值）。"))
	if win := w.window(); win != nil {
		d.AttachToWindow(win)
	}
	return d.PromptForSingleSelection()
}

func (w WailsDialogs) PickImage() (string, error) {
	d := application.Get().Dialog.OpenFile().
		CanChooseFiles(true).
		AddFilter(i18n.T("圖片（*.png, *.jpg, *.gif）"), "*.png;*.jpg;*.jpeg;*.gif")
	if win := w.window(); win != nil {
		d.AttachToWindow(win)
	}
	return d.PromptForSingleSelection()
}

func (w WailsDialogs) PickPDF(suggestedName string) (string, error) {
	d := application.Get().Dialog.SaveFile().
		SetFilename(suggestedName).
		CanCreateDirectories(true).
		SetMessage(i18n.T("匯出為 PDF"))
	if win := w.window(); win != nil {
		d.AttachToWindow(win)
	}
	return d.PromptForSingleSelection()
}

func (w WailsDialogs) Confirm(title, message, ok string) bool {
	// The unattended self-test cannot click; it confirms everything.
	if os.Getenv("WW_SELFTEST") == "1" {
		return true
	}
	answer := make(chan bool, 1)
	d := application.Get().Dialog.Question().SetTitle(i18n.T(title)).SetMessage(i18n.T(message))
	d.AddButton(i18n.T(ok)).OnClick(func() { answer <- true })
	cancel := d.AddButton(i18n.T("取消")).OnClick(func() { answer <- false })
	// Deleting is destructive, so Cancel is the default.
	d.SetDefaultButton(cancel).SetCancelButton(cancel)
	if win := w.window(); win != nil {
		d.AttachToWindow(win)
	}
	d.Show()
	return <-answer
}

func (w WailsDialogs) ShowError(title, message string) {
	w.show(application.Get().Dialog.Error(), title, message)
}

func (w WailsDialogs) ShowWarning(title, message string) {
	w.show(application.Get().Dialog.Warning(), title, message)
}

func (w WailsDialogs) show(d *application.MessageDialog, title, message string) {
	// Messages are often errors built in Go from Chinese source text.
	d.SetTitle(i18n.T(title)).SetMessage(i18n.T(message))
	if win := w.window(); win != nil {
		d.AttachToWindow(win)
	}
	d.Show()
}

func (w WailsDialogs) window() application.Window {
	if w.Window == nil {
		return nil
	}
	return w.Window()
}
