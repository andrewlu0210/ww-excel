package document

import (
	"errors"

	"ww-excel/internal/command"
	"ww-excel/internal/formula/ast"
	"ww-excel/internal/model"
)

// ClipCell is a copied cell, independent of the source workbook so it
// can be pasted into another document.
type ClipCell struct {
	Row, Col int        // offset from the copied range's top-left
	Value    model.Cell // the (calculated) value; S and F are cleared
	Formula  ast.Node   // nil for constants
	Style    model.Style
}

// Clip is the internal clipboard: what ⌘C or ⌘X captured. Pasting it
// keeps formulas and formats; other apps get the text form.
type Clip struct {
	Doc    *Document
	Sheet  *model.Sheet
	Src    model.Range
	Cells  []ClipCell
	Merges []model.Range // offsets from Src's top-left
	Cut    bool
}

func (c *Clip) Rows() int { return c.Src.R1 - c.Src.R0 + 1 }
func (c *Clip) Cols() int { return c.Src.C1 - c.Src.C0 + 1 }

var ErrClipTooLarge = errors.New("選取範圍太大，無法複製")

// Copy captures a range of a sheet.
func (d *Document) Copy(sheet int, rg model.Range, cut bool) (*Clip, error) {
	if sheet < 0 || sheet >= len(d.wb.Sheets) {
		return nil, errors.New("無效的工作表")
	}
	sh := d.wb.Sheets[sheet]
	rg = rg.Clamp()
	clip := &Clip{Doc: d, Sheet: sh, Src: rg, Cut: cut}
	st := d.wb.StyleTable()
	var err error
	sh.EachInRange(rg, func(row, col int, c model.Cell) {
		if err != nil {
			return
		}
		if len(clip.Cells) >= command.MaxFillCells {
			err = ErrClipTooLarge
			return
		}
		cc := ClipCell{Row: row - rg.R0, Col: col - rg.C0, Value: c.Value(), Style: st.Get(sh.EffectiveStyle(row, col))}
		cc.Value.S, cc.Value.F = 0, nil
		if c.F != nil {
			cc.Formula = d.calc.Parsed(c.F, sheet)
		}
		clip.Cells = append(clip.Cells, cc)
	})
	if err != nil {
		return nil, err
	}
	for _, m := range sh.Layout.Merges {
		if m.R0 >= rg.R0 && m.R1 <= rg.R1 && m.C0 >= rg.C0 && m.C1 <= rg.C1 {
			clip.Merges = append(clip.Merges, model.Range{R0: m.R0 - rg.R0, C0: m.C0 - rg.C0, R1: m.R1 - rg.R0, C1: m.C1 - rg.C0})
		}
	}
	return clip, nil
}

// PasteMode selects what a paste writes (選擇性貼上).
type PasteMode int

const (
	PasteAll PasteMode = iota
	PasteValues
	PasteFormats
	PasteFormulas
)

type PasteOptions struct {
	Mode       PasteMode
	Transpose  bool
	SkipBlanks bool
}

// PasteArea is where a clip lands when pasted onto selection dst: the
// clip is tiled when dst is a multiple of its size, else pasted once at
// dst's top-left.
func PasteArea(rows, cols int, dst model.Range) model.Range {
	dst = dst.Clamp()
	h, w := dst.R1-dst.R0+1, dst.C1-dst.C0+1
	if h%rows != 0 || w%cols != 0 {
		h, w = rows, cols
	}
	return model.Range{R0: dst.R0, C0: dst.C0, R1: dst.R0 + h - 1, C1: dst.C0 + w - 1}
}

// PasteCommand pastes a clip onto dst (the selection) of a sheet and
// returns the area written. A cut clip from this document moves its
// cells, and formulas that referred to them follow.
func (d *Document) PasteCommand(clip *Clip, sheet int, dst model.Range, opt PasteOptions) (command.Command, model.Range, error) {
	if sheet < 0 || sheet >= len(d.wb.Sheets) {
		return nil, model.Range{}, errors.New("無效的工作表")
	}
	rows, cols := clip.Rows(), clip.Cols()
	if opt.Transpose {
		rows, cols = cols, rows
	}
	if clip.Cut {
		opt = PasteOptions{} // Excel only pastes cut cells as they are
		dst = model.Range{R0: dst.R0, C0: dst.C0, R1: dst.R0, C1: dst.C0}
	}
	area := PasteArea(rows, cols, dst)
	if area.R1 >= model.MaxRows || area.C1 >= model.MaxCols {
		return nil, model.Range{}, errors.New("無法貼上：貼上的區域超出工作表")
	}
	if tiles := (area.R1 - area.R0 + 1) / rows * ((area.C1 - area.C0 + 1) / cols); tiles*len(clip.Cells) > command.MaxFillCells {
		return nil, model.Range{}, ErrClipTooLarge
	}
	sh := d.wb.Sheets[sheet]
	if err := checkMergeOverlap(sh.Layout.Merges, area); err != nil {
		return nil, model.Range{}, err
	}

	cells := d.pasteCells(clip, sheet, area, rows, cols, opt)
	name := "貼上"
	if opt.Mode != PasteAll || opt.Transpose || opt.SkipBlanks {
		name = "選擇性貼上"
	}
	if clip.Cut && clip.Doc == d {
		return d.moveCommand(clip, sheet, area, cells), area, nil
	}

	set := &command.SetCells{Sheet: sheet, Name: name, Changes: cells, Area: &area}
	if opt.Mode != PasteAll && opt.Mode != PasteFormats {
		return set, area, nil
	}
	// Formats bring their merges: replace the ones in the target area.
	after := sh.Layout.Clone()
	var merges []model.Range
	for _, m := range after.Merges {
		if !m.Overlaps(area) {
			merges = append(merges, m)
		}
	}
	for tr := area.R0; tr <= area.R1; tr += rows {
		for tc := area.C0; tc <= area.C1; tc += cols {
			for _, m := range clip.Merges {
				if opt.Transpose {
					m = model.Range{R0: m.C0, C0: m.R0, R1: m.C1, C1: m.R1}
				}
				merges = append(merges, model.Range{R0: tr + m.R0, C0: tc + m.C0, R1: tr + m.R1, C1: tc + m.C1})
			}
		}
	}
	if len(merges) == len(sh.Layout.Merges) && len(clip.Merges) == 0 {
		return set, area, nil
	}
	after.Merges = merges
	after.SortMerges()
	return &command.Layout{Sheet: sheet, Name: name, After: after, Cells: set, Area: &area}, area, nil
}

// checkMergeOverlap rejects pastes that would split a merged cell.
func checkMergeOverlap(merges []model.Range, area model.Range) error {
	for _, m := range merges {
		inside := m.R0 >= area.R0 && m.R1 <= area.R1 && m.C0 >= area.C0 && m.C1 <= area.C1
		if m.Overlaps(area) && !inside {
			return errors.New("無法對合併儲存格變更部分內容")
		}
	}
	return nil
}

// pasteCells computes the new cells of the target area.
func (d *Document) pasteCells(clip *Clip, sheet int, area model.Range, rows, cols int, opt PasteOptions) []command.CellChange {
	sh := d.wb.Sheets[sheet]
	st := d.wb.StyleTable()
	styles := map[model.Style]uint32{}
	intern := func(s model.Style) uint32 {
		id, ok := styles[s]
		if !ok {
			id = st.Intern(s)
			styles[s] = id
		}
		return id
	}

	written := map[[2]int]bool{}
	var out []command.CellChange
	for tr := area.R0; tr <= area.R1; tr += rows {
		for tc := area.C0; tc <= area.C1; tc += cols {
			for _, cc := range clip.Cells {
				dr, dc := cc.Row, cc.Col
				if opt.Transpose {
					dr, dc = dc, dr
				}
				row, col := tr+dr, tc+dc
				if opt.SkipBlanks && !cc.Value.HasContent() && cc.Formula == nil {
					continue
				}
				old := sh.Get(row, col)
				cell := old
				switch opt.Mode {
				case PasteAll, PasteFormulas, PasteValues:
					cell = cc.Value
					cell.S = old.S
					if opt.Mode != PasteValues && cc.Formula != nil {
						node := cc.Formula
						if !clip.Cut {
							node = ast.Translate(node, row-(clip.Src.R0+cc.Row), col-(clip.Src.C0+cc.Col))
						}
						cell.F = d.calc.CompileNode(node, sheet)
					}
				}
				if opt.Mode == PasteAll || opt.Mode == PasteFormats {
					cell.S = intern(cc.Style)
				} else if cell.S == 0 {
					cell.S = sh.Layout.StyleAt(row, col)
				}
				written[[2]int{row, col}] = true
				out = append(out, command.CellChange{Row: row, Col: col, After: cell})
			}
		}
	}
	if opt.SkipBlanks {
		return out
	}
	// Blank source cells clear what they land on.
	sh.EachInRange(area, func(row, col int, old model.Cell) {
		if written[[2]int{row, col}] {
			return
		}
		cell := model.Cell{}
		switch opt.Mode {
		case PasteValues, PasteFormulas:
			cell.S = old.S
		case PasteFormats:
			cell = old
			cell.S = 0
		}
		if cell.SameInput(old) && cell.S == old.S {
			return
		}
		out = append(out, command.CellChange{Row: row, Col: col, After: cell})
	})
	return out
}

// moveCommand pastes cut cells: the source is cleared and every formula
// referring to the moved cells is rewritten to their new place.
func (d *Document) moveCommand(clip *Clip, sheet int, area model.Range, cells []command.CellChange) command.Command {
	srcSheet := d.wb.SheetIndex(clip.Sheet.Name)
	dst := d.wb.Sheets[sheet]
	dr, dc := area.R0-clip.Src.R0, area.C0-clip.Src.C0
	var snap formulaSnapshot
	type saved struct {
		sh       *model.Sheet
		row, col int
		cell     model.Cell
	}
	var before []saved

	// The source keeps its merges' area unmerged, the target gets them.
	srcLayout, dstLayout := clip.Sheet.Layout.Clone(), dst.Layout.Clone()

	return &command.Func{
		Name: "移動",
		DoFn: func(wb *model.Workbook) command.Effect {
			snap = d.snapshotFormulas()
			before = before[:0]
			eff := command.Effect{Sheet: sheet, Bounds: &area, Others: map[int][]command.CellRef{}}
			mark := func(sh *model.Sheet, si, row, col int) {
				before = append(before, saved{sh, row, col, sh.Get(row, col)})
				if si == sheet {
					eff.Cells = append(eff.Cells, command.CellRef{Row: row, Col: col})
				} else {
					eff.Others[si] = append(eff.Others[si], command.CellRef{Row: row, Col: col})
				}
			}
			if srcSheet >= 0 {
				clip.Sheet.EachInRange(clip.Src, func(row, col int, c model.Cell) {
					if srcSheet == sheet && area.Contains(row, col) {
						return
					}
					mark(clip.Sheet, srcSheet, row, col)
					clip.Sheet.Set(row, col, model.Cell{})
				})
			}
			for _, ch := range cells {
				mark(dst, sheet, ch.Row, ch.Col)
				dst.Set(ch.Row, ch.Col, ch.After)
			}
			if len(clip.Merges) > 0 && srcSheet >= 0 {
				moveMerges(clip, dst, area)
				eff.LayoutChanged = true
			}
			for si, refs := range d.moveFormulas(clip.Sheet.Name, clip.Src, dst.Name, dr, dc) {
				if si == sheet {
					eff.Cells = append(eff.Cells, refs...)
				} else {
					eff.Others[si] = append(eff.Others[si], refs...)
				}
			}
			return eff
		},
		UndoFn: func(wb *model.Workbook) command.Effect {
			eff := command.Effect{Sheet: sheet, Bounds: &area, Others: map[int][]command.CellRef{}}
			for i := len(before) - 1; i >= 0; i-- {
				b := before[i]
				b.sh.Set(b.row, b.col, b.cell)
				if si := d.wb.SheetIndex(b.sh.Name); si == sheet {
					eff.Cells = append(eff.Cells, command.CellRef{Row: b.row, Col: b.col})
				} else if si >= 0 {
					eff.Others[si] = append(eff.Others[si], command.CellRef{Row: b.row, Col: b.col})
				}
			}
			snap.restore()
			for _, f := range snap {
				if si := d.wb.SheetIndex(f.sheet.Name); si >= 0 {
					eff.Others[si] = append(eff.Others[si], command.CellRef{Row: f.row, Col: f.col})
				}
			}
			if len(clip.Merges) > 0 && srcSheet >= 0 {
				clip.Sheet.Layout.Merges = srcLayout.Clone().Merges
				dst.Layout.Merges = dstLayout.Clone().Merges
				eff.LayoutChanged = true
			}
			return eff
		},
	}
}

func moveMerges(clip *Clip, dst *model.Sheet, area model.Range) {
	var keep []model.Range
	for _, m := range clip.Sheet.Layout.Merges {
		inside := m.R0 >= clip.Src.R0 && m.R1 <= clip.Src.R1 && m.C0 >= clip.Src.C0 && m.C1 <= clip.Src.C1
		if !inside {
			keep = append(keep, m)
		}
	}
	clip.Sheet.Layout.Merges = keep
	keep = nil
	for _, m := range dst.Layout.Merges {
		if !m.Overlaps(area) {
			keep = append(keep, m)
		}
	}
	for _, m := range clip.Merges {
		keep = append(keep, model.Range{R0: area.R0 + m.R0, C0: area.C0 + m.C0, R1: area.R0 + m.R1, C1: area.C0 + m.C1})
	}
	dst.Layout.Merges = keep
	dst.Layout.SortMerges()
}

// moveFormulas rewrites references into a moved range in every formula.
func (d *Document) moveFormulas(srcSheet string, src model.Range, dstSheet string, dr, dc int) map[int][]command.CellRef {
	out := map[int][]command.CellRef{}
	for si, sh := range d.wb.Sheets {
		sh.EachInRange(model.Range{R1: model.MaxRows - 1, C1: model.MaxCols - 1}, func(row, col int, c model.Cell) {
			if c.F == nil {
				return
			}
			node, changed := ast.Move(d.calc.Parsed(c.F, si), sh.Name, srcSheet, src, dstSheet, dr, dc)
			if !changed {
				return
			}
			c.F = d.calc.CompileNode(node, si)
			sh.Set(row, col, c)
			out[si] = append(out[si], command.CellRef{Row: row, Col: col})
		})
	}
	return out
}

// ParseTSV splits clipboard text from Excel or another app into rows of
// fields. Fields with tabs, line breaks or quotes are quoted, as Excel
// writes them; a trailing line break is not an extra row.
func ParseTSV(text string) [][]string {
	var rows [][]string
	var row []string
	var field []rune
	quoted, inQuotes := false, false
	rs := []rune(text)
	for i := 0; i < len(rs); i++ {
		r := rs[i]
		switch {
		case inQuotes:
			if r == '"' {
				if i+1 < len(rs) && rs[i+1] == '"' {
					field = append(field, '"')
					i++
				} else {
					inQuotes = false
				}
			} else {
				field = append(field, r)
			}
		case r == '"' && len(field) == 0 && !quoted:
			inQuotes, quoted = true, true
		case r == '\t':
			row = append(row, string(field))
			field, quoted = field[:0], false
		case r == '\r' || r == '\n':
			if r == '\r' && i+1 < len(rs) && rs[i+1] == '\n' {
				i++
			}
			rows = append(rows, append(row, string(field)))
			row, field, quoted = nil, field[:0], false
		default:
			field = append(field, r)
		}
	}
	if len(field) > 0 || len(row) > 0 || quoted {
		rows = append(rows, append(row, string(field)))
	}
	return rows
}

// TextClip turns pasted text into a clip: each field is entered as if
// typed (numbers, dates, formulas), with the target's formats.
func (d *Document) TextClip(sheet int, text string) (*Clip, error) {
	rows := ParseTSV(text)
	if len(rows) == 0 {
		return nil, errors.New("剪貼簿沒有可以貼上的內容")
	}
	clip := &Clip{Doc: d, Src: model.Range{}}
	width := 0
	st := d.wb.StyleTable()
	for r, fields := range rows {
		width = max(width, len(fields))
		for c, f := range fields {
			if f == "" {
				continue
			}
			in, err := d.InputCell(sheet, f)
			if err != nil {
				in = Input{Cell: model.StringCell(f)}
			}
			cc := ClipCell{Row: r, Col: c, Value: in.Cell}
			if in.Cell.F != nil {
				cc.Formula = d.calc.Parsed(in.Cell.F, sheet)
				cc.Value = model.Cell{}
			}
			if in.NumFmt != "" || in.Wrap {
				s := st.Default()
				s.NumFmt = in.NumFmt
				s.Align.Wrap = in.Wrap
				cc.Style = s
			}
			clip.Cells = append(clip.Cells, cc)
		}
	}
	clip.Src = model.Range{R1: len(rows) - 1, C1: max(width, 1) - 1}
	if clip.Src.R1 >= model.MaxRows || clip.Src.C1 >= model.MaxCols {
		return nil, errors.New("無法貼上：貼上的資料超出工作表")
	}
	return clip, nil
}

// PasteTextCommand pastes text from another app: values only, keeping
// the target's formats except detected number formats.
func (d *Document) PasteTextCommand(sheet int, text string, dst model.Range) (command.Command, model.Range, error) {
	clip, err := d.TextClip(sheet, text)
	if err != nil {
		return nil, model.Range{}, err
	}
	area := PasteArea(clip.Rows(), clip.Cols(), dst)
	if area.R1 >= model.MaxRows || area.C1 >= model.MaxCols {
		return nil, model.Range{}, errors.New("無法貼上：貼上的區域超出工作表")
	}
	sh := d.wb.Sheets[sheet]
	if err := checkMergeOverlap(sh.Layout.Merges, area); err != nil {
		return nil, model.Range{}, err
	}
	cells := d.pasteCells(clip, sheet, area, clip.Rows(), clip.Cols(), PasteOptions{Mode: PasteFormulas})
	// Detected number formats ("12%", dates) are applied like typing.
	detected := map[[2]int]model.Style{}
	for _, cc := range clip.Cells {
		if cc.Style.NumFmt != "" || cc.Style.Align.Wrap {
			detected[[2]int{cc.Row, cc.Col}] = cc.Style
		}
	}
	for i := range cells {
		ch := &cells[i]
		key := [2]int{(ch.Row - area.R0) % clip.Rows(), (ch.Col - area.C0) % clip.Cols()}
		if s, ok := detected[key]; ok {
			ch.After.S = inputStyleID(d.wb, sh, ch.Row, ch.Col, s.NumFmt, s.Align.Wrap)
		}
	}
	return &command.SetCells{Sheet: sheet, Name: "貼上", Changes: cells, Area: &area}, area, nil
}

func inputStyleID(wb *model.Workbook, sh *model.Sheet, row, col int, numFmt string, wrap bool) uint32 {
	st := wb.StyleTable().Get(sh.EffectiveStyle(row, col))
	if numFmt != "" && st.NumFmt == "" {
		st.NumFmt = numFmt
	}
	if wrap {
		st.Align.Wrap = true
	}
	return wb.StyleTable().Intern(st)
}
