package service

import (
	"context"
	"errors"
	"html"
	"strings"
	"sync"

	"ww-excel/internal/command"
	"ww-excel/internal/document"
	"ww-excel/internal/model"
)

// clipState is the internal clipboard. The system clipboard holds its
// text; a paste whose text still matches uses the clip, so formulas and
// formats survive copying within (and between) our windows.
type clipState struct {
	mu   sync.Mutex
	clip *document.Clip
	text string
	// Version of the source document at copy time: a cut is cancelled
	// by later edits, as in Excel.
	version uint64

	// System clipboard access, set by main (nil in tests).
	SetText func(string) bool
	Text    func() (string, bool)
}

// ConnectClipboard connects the system clipboard (a function, not a
// method, so it is not exposed as a binding).
func ConnectClipboard(s *SheetService, set func(string) bool, get func() (string, bool)) {
	s.clip.SetText, s.clip.Text = set, get
}

// CopyResult is what ⌘C put on the clipboard.
type CopyResult struct {
	Text string `json:"text"`
	HTML string `json:"html"`
}

// Copy captures a range for pasting and puts its text (tab-separated,
// as Excel does) and an HTML table on the system clipboard.
func (s *SheetService) Copy(ctx context.Context, sheet int, rg RangeDTO, cut bool) (CopyResult, error) {
	var res CopyResult
	var clip *document.Clip
	var version uint64
	var err error
	s.at(ctx).read(func(d *document.Document) {
		if clip, err = d.Copy(sheet, rg.model(), cut); err != nil {
			return
		}
		version = d.Version()
		res.Text, res.HTML = clipText(clip)
	})
	if err != nil {
		return res, err
	}
	s.clip.mu.Lock()
	s.clip.clip, s.clip.text, s.clip.version = clip, normalizeEOL(res.Text), version
	set := s.clip.SetText
	s.clip.mu.Unlock()
	if set != nil {
		set(res.Text)
	}
	return res, nil
}

// ClearClip ends a pending cut (Esc).
func (s *SheetService) ClearClip(ctx context.Context) {
	s.clip.mu.Lock()
	defer s.clip.mu.Unlock()
	if s.clip.clip != nil && s.clip.clip.Cut {
		s.clip.clip = nil
	}
}

// HasClip reports whether 選擇性貼上 has something to paste.
func (s *SheetService) HasClip(ctx context.Context) bool {
	s.clip.mu.Lock()
	defer s.clip.mu.Unlock()
	return s.clip.clip != nil
}

func normalizeEOL(s string) string {
	return strings.TrimRight(strings.ReplaceAll(s, "\r\n", "\n"), "\n")
}

// clipText renders a clip as TSV and as an HTML table with the cells'
// basic formatting, for other apps.
func clipText(clip *document.Clip) (string, string) {
	rows, cols := clip.Rows(), clip.Cols()
	type cellText struct {
		v  string
		st model.Style
	}
	grid := make(map[[2]int]cellText, len(clip.Cells))
	for _, cc := range clip.Cells {
		v, _ := display(cc.Value, cc.Style)
		grid[[2]int{cc.Row, cc.Col}] = cellText{v, cc.Style}
	}
	var tsv, h strings.Builder
	h.WriteString(`<table style="border-collapse:collapse">`)
	for r := 0; r < rows; r++ {
		h.WriteString("<tr>")
		for c := 0; c < cols; c++ {
			ct := grid[[2]int{r, c}]
			if c > 0 {
				tsv.WriteByte('\t')
			}
			tsv.WriteString(quoteTSV(ct.v))
			h.WriteString(`<td style="` + htmlStyle(ct.st) + `">`)
			h.WriteString(strings.ReplaceAll(html.EscapeString(ct.v), "\n", "<br>"))
			h.WriteString("</td>")
		}
		h.WriteString("</tr>")
		tsv.WriteString("\n")
	}
	h.WriteString("</table>")
	return tsv.String(), h.String()
}

func quoteTSV(v string) string {
	if strings.ContainsAny(v, "\t\n\r\"") {
		return `"` + strings.ReplaceAll(v, `"`, `""`) + `"`
	}
	return v
}

func htmlStyle(st model.Style) string {
	var b strings.Builder
	if st.Font.Bold {
		b.WriteString("font-weight:bold;")
	}
	if st.Font.Italic {
		b.WriteString("font-style:italic;")
	}
	if st.Font.Underline {
		b.WriteString("text-decoration:underline;")
	}
	if st.Font.Color != "" {
		b.WriteString("color:" + st.Font.Color + ";")
	}
	if st.Fill != "" {
		b.WriteString("background:" + st.Fill + ";")
	}
	if st.Font.Name != "" {
		b.WriteString("font-family:'" + strings.ReplaceAll(st.Font.Name, "'", "") + "';")
	}
	if st.Align.H != "" {
		b.WriteString("text-align:" + st.Align.H + ";")
	}
	return b.String()
}

// PasteResult is the area a paste wrote, selected afterwards.
type PasteResult struct {
	Area RangeDTO `json:"area"`
	// Moved is set after pasting cut cells (the copy marquee ends).
	Moved bool `json:"moved"`
}

// Paste pastes at the selection. text is the clipboard text from a paste
// event, or nil to read the system clipboard. Text matching our last
// copy pastes that clip (formulas, formats); other text is entered as
// if typed.
func (s *SheetService) Paste(ctx context.Context, sheet int, sel RangeDTO, text *string) (PasteResult, error) {
	var t string
	if text != nil {
		t = *text
	} else if s.clip.Text != nil {
		t, _ = s.clip.Text()
	}
	s.clip.mu.Lock()
	clip, clipText, version := s.clip.clip, s.clip.text, s.clip.version
	s.clip.mu.Unlock()

	sess := s.at(ctx)
	internal := clip != nil && (normalizeEOL(t) == clipText || t == "" && text == nil && s.clip.Text == nil)
	if internal && clip.Cut {
		var v uint64
		sess.read(func(d *document.Document) {
			if clip.Doc == d {
				v = d.Version()
			} else {
				v = version
			}
		})
		internal = v == version
	}
	if !internal && strings.TrimSpace(t) == "" {
		return PasteResult{}, errors.New("剪貼簿沒有可以貼上的內容")
	}

	var res PasteResult
	var err error
	sess.change(false, func(d *document.Document) (document.Change, bool) {
		if sheetAt(d, sheet) == nil {
			return document.Change{}, false
		}
		var cmd command.Command
		var area model.Range
		if internal {
			cmd, area, err = d.PasteCommand(clip, sheet, sel.model(), document.PasteOptions{})
		} else {
			cmd, area, err = d.PasteTextCommand(sheet, t, sel.model())
		}
		if err != nil {
			return document.Change{}, false
		}
		res.Area = rangeDTO(area)
		return d.Exec(cmd)
	})
	if err == nil && internal && clip.Cut {
		res.Moved = true
		s.clip.mu.Lock()
		if s.clip.clip == clip {
			s.clip.clip = nil
		}
		s.clip.mu.Unlock()
	}
	return res, err
}

// PasteSpecial pastes the internal clip: mode is "all", "values",
// "formats" or "formulas".
func (s *SheetService) PasteSpecial(ctx context.Context, sheet int, sel RangeDTO, mode string, transpose, skipBlanks bool) (RangeDTO, error) {
	s.clip.mu.Lock()
	clip := s.clip.clip
	s.clip.mu.Unlock()
	if clip == nil {
		return RangeDTO{}, errors.New("請先複製儲存格")
	}
	opt := document.PasteOptions{Transpose: transpose, SkipBlanks: skipBlanks}
	switch mode {
	case "values":
		opt.Mode = document.PasteValues
	case "formats":
		opt.Mode = document.PasteFormats
	case "formulas":
		opt.Mode = document.PasteFormulas
	}
	var area model.Range
	err := s.exec(ctx, sheet, func(d *document.Document) (command.Command, error) {
		cmd, a, err := d.PasteCommand(clip, sheet, sel.model(), opt)
		area = a
		return cmd, err
	})
	return rangeDTO(area), err
}

func rangeDTO(r model.Range) RangeDTO { return RangeDTO{R0: r.R0, C0: r.C0, R1: r.R1, C1: r.C1} }

// exec runs a command built for a sheet; build errors are returned.
func (s *SheetService) exec(ctx context.Context, sheet int, build func(d *document.Document) (command.Command, error)) error {
	var err error
	s.at(ctx).change(false, func(d *document.Document) (document.Change, bool) {
		if sheetAt(d, sheet) == nil {
			err = errors.New("無效的工作表")
			return document.Change{}, false
		}
		var cmd command.Command
		if cmd, err = build(d); err != nil || cmd == nil {
			return document.Change{}, false
		}
		return d.Exec(cmd)
	})
	return err
}

// Shift inserts (count > 0) or deletes (count < 0) rows or columns.
func (s *SheetService) Shift(ctx context.Context, sheet int, rows bool, at, count int) error {
	return s.exec(ctx, sheet, func(d *document.Document) (command.Command, error) {
		return d.ShiftCommand(sheet, rows, at, count)
	})
}

// FillSeries extends src over dst (fill handle); copyOnly repeats the
// cells instead of continuing a series (⌘D, ⌘R, ⌥-drag).
func (s *SheetService) FillSeries(ctx context.Context, sheet int, src, dst RangeDTO, copyOnly bool) error {
	return s.exec(ctx, sheet, func(d *document.Document) (command.Command, error) {
		return d.FillCommand(sheet, src.model(), dst.model(), copyOnly)
	})
}

// Hide hides or shows rows (rows = true) or columns lo..hi.
func (s *SheetService) Hide(ctx context.Context, sheet int, rows bool, lo, hi int, hidden bool) error {
	return s.exec(ctx, sheet, func(d *document.Document) (command.Command, error) {
		return d.HideCommand(sheet, rows, lo, hi, hidden)
	})
}

// Freeze keeps the first rows and cols on screen (0, 0 unfreezes).
func (s *SheetService) Freeze(ctx context.Context, sheet, rows, cols int) error {
	return s.exec(ctx, sheet, func(d *document.Document) (command.Command, error) {
		return d.FreezeCommand(sheet, rows, cols)
	})
}

// CurrentRegion is the data block around a cell (⌘A, sort, filter).
func (s *SheetService) CurrentRegion(ctx context.Context, sheet, row, col int) RangeDTO {
	var out RangeDTO
	s.at(ctx).read(func(d *document.Document) { out = rangeDTO(d.CurrentRegion(sheet, row, col)) })
	return out
}

// ---- Find & replace ----

type FindOptionsDTO struct {
	Text       string `json:"text"`
	MatchCase  bool   `json:"matchCase"`
	WholeCell  bool   `json:"wholeCell"`
	InFormulas bool   `json:"inFormulas"`
	AllSheets  bool   `json:"allSheets"`
}

func (o FindOptionsDTO) model() document.FindOptions {
	return document.FindOptions{Text: o.Text, MatchCase: o.MatchCase, WholeCell: o.WholeCell, InFormulas: o.InFormulas, AllSheets: o.AllSheets}
}

type HitDTO struct {
	Sheet int    `json:"sheet"`
	Row   int    `json:"row"`
	Col   int    `json:"col"`
	V     string `json:"v"`
}

func shownText(wb *model.Workbook) document.DisplayFunc {
	return func(sh *model.Sheet, row, col int, c model.Cell) string {
		v, _, _, _, _ := cellView(wb, sh, row, col, c)
		return v
	}
}

// FindNext returns the next match after (row, col), or nil.
func (s *SheetService) FindNext(ctx context.Context, sheet, row, col int, opt FindOptionsDTO, forward bool) *HitDTO {
	var out *HitDTO
	s.at(ctx).read(func(d *document.Document) {
		if h, ok := d.FindNext(sheet, row, col, opt.model(), shownText(d.Workbook()), forward); ok {
			out = &HitDTO{Sheet: h.Sheet, Row: h.Row, Col: h.Col}
		}
	})
	return out
}

// FindAll lists up to 1,000 matches with their text (找全部).
func (s *SheetService) FindAll(ctx context.Context, sheet int, opt FindOptionsDTO) []HitDTO {
	out := []HitDTO{}
	s.at(ctx).read(func(d *document.Document) {
		wb := d.Workbook()
		show := shownText(wb)
		for _, h := range d.FindAll(sheet, opt.model(), show, 1000) {
			sh := wb.Sheets[h.Sheet]
			out = append(out, HitDTO{Sheet: h.Sheet, Row: h.Row, Col: h.Col, V: show(sh, h.Row, h.Col, sh.Get(h.Row, h.Col))})
		}
	})
	return out
}

// Replace replaces matches: every match when all, else only the cell at
// (sheet, row, col) if it matches. Returns how many cells changed.
func (s *SheetService) Replace(ctx context.Context, sheet int, opt FindOptionsDTO, with string, all bool, row, col int) (int, error) {
	n := 0
	err := s.exec(ctx, sheet, func(d *document.Document) (command.Command, error) {
		o := opt.model()
		hits := d.FindAll(sheet, o, shownText(d.Workbook()), 0)
		if !all {
			var one []document.Hit
			for _, h := range hits {
				if h.Sheet == sheet && h.Row == row && h.Col == col {
					one = append(one, h)
				}
			}
			hits = one
		}
		cmd, count, err := d.ReplaceCommand(hits, o, with)
		n = count
		return cmd, err
	})
	return n, err
}

// ---- Sort & filter ----

type SortInfo struct {
	Range  RangeDTO `json:"range"`
	Header bool     `json:"header"`
	OK     bool     `json:"ok"`
}

// SortInfo returns the range a sort of the selection applies to.
func (s *SheetService) SortInfo(ctx context.Context, sheet int, sel RangeDTO) SortInfo {
	var out SortInfo
	s.at(ctx).read(func(d *document.Document) {
		rg, header, ok := d.SortRange(sheet, sel.model())
		out = SortInfo{Range: rangeDTO(rg), Header: header, OK: ok}
	})
	return out
}

type SortKeyDTO struct {
	Col  int  `json:"col"`
	Desc bool `json:"desc"`
}

// Sort sorts the rows of rg by keys.
func (s *SheetService) Sort(ctx context.Context, sheet int, rg RangeDTO, keys []SortKeyDTO, header bool) error {
	return s.exec(ctx, sheet, func(d *document.Document) (command.Command, error) {
		ks := make([]document.SortKey, len(keys))
		for i, k := range keys {
			ks[i] = document.SortKey{Col: k.Col, Desc: k.Desc}
		}
		return d.SortCommand(sheet, rg.model(), ks, header)
	})
}

// ToggleFilter turns the AutoFilter on (around the selection) or off.
func (s *SheetService) ToggleFilter(ctx context.Context, sheet int, sel RangeDTO) error {
	return s.exec(ctx, sheet, func(d *document.Document) (command.Command, error) {
		return d.ToggleFilterCommand(sheet, sel.model())
	})
}

type FilterValueDTO struct {
	Text  string `json:"text"`
	Shown bool   `json:"shown"`
}

func (s *SheetService) FilterValues(ctx context.Context, sheet, col int) []FilterValueDTO {
	out := []FilterValueDTO{}
	s.at(ctx).read(func(d *document.Document) {
		for _, v := range d.FilterValues(sheet, col, shownText(d.Workbook())) {
			out = append(out, FilterValueDTO{Text: v.Text, Shown: v.Shown})
		}
	})
	return out
}

// SetFilter sets the values a filter column shows; all clears it.
func (s *SheetService) SetFilter(ctx context.Context, sheet, col int, shown []string, all bool) error {
	return s.exec(ctx, sheet, func(d *document.Document) (command.Command, error) {
		if all {
			shown = nil
		} else if shown == nil {
			shown = []string{}
		}
		return d.FilterCommand(sheet, col, shown, shownText(d.Workbook()))
	})
}

// SortFilter sorts the filter range by one of its columns.
func (s *SheetService) SortFilter(ctx context.Context, sheet, col int, desc bool) error {
	return s.exec(ctx, sheet, func(d *document.Document) (command.Command, error) {
		f := d.FilterForColumn(sheet, col)
		if f == nil {
			return nil, errors.New("沒有篩選")
		}
		return d.SortCommand(sheet, f.Range, []document.SortKey{{Col: col, Desc: desc}}, true)
	})
}
