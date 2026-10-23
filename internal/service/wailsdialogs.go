package service

import "github.com/wailsapp/wails/v3/pkg/application"

// WailsDialogs shows native macOS dialogs, attached as sheets to the
// window returned by Window.
type WailsDialogs struct {
	Window func() application.Window
}

func (w WailsDialogs) AskSave(docName string) SaveChoice {
	// Button callbacks may run on another thread after Show returns.
	choice := make(chan SaveChoice, 1)
	d := application.Get().Dialog.Question().
		SetTitle("要儲存對「" + docName + "」所做的變更嗎？").
		SetMessage("如果不儲存，您的變更將會遺失。")
	save := d.AddButton("儲存").OnClick(func() { choice <- ChoiceSave })
	d.AddButton("不儲存").OnClick(func() { choice <- ChoiceDiscard })
	cancel := d.AddButton("取消").OnClick(func() { choice <- ChoiceCancel })
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
		AddFilter("試算表（*.wwx, *.csv）", "*.wwx;*.csv")
	if win := w.window(); win != nil {
		d.AttachToWindow(win)
	}
	return d.PromptForSingleSelection()
}

func (w WailsDialogs) PickSave(suggestedName string) (string, error) {
	d := application.Get().Dialog.SaveFile().
		SetFilename(suggestedName).
		CanCreateDirectories(true).
		SetMessage("副檔名 .wwx 為原生格式；.csv 只保存第一個工作表的值。")
	if win := w.window(); win != nil {
		d.AttachToWindow(win)
	}
	return d.PromptForSingleSelection()
}

func (w WailsDialogs) ShowError(title, message string) {
	w.show(application.Get().Dialog.Error(), title, message)
}

func (w WailsDialogs) ShowWarning(title, message string) {
	w.show(application.Get().Dialog.Warning(), title, message)
}

func (w WailsDialogs) show(d *application.MessageDialog, title, message string) {
	d.SetTitle(title).SetMessage(message)
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
