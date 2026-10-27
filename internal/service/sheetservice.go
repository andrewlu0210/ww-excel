package service

import (
	"context"
	"sort"

	"ww-excel/internal/command"
	"ww-excel/internal/document"
	"ww-excel/internal/formula"
	"ww-excel/internal/model"
)

// CellDTO is a single non-empty cell inside a block. R and C are offsets
// within the block; short JSON keys keep the payload small.
type CellDTO struct {
	Runs []TextRunDTO `json:"runs,omitempty"`
	R    int          `json:"r"`
	C    int          `json:"c"`
	V    string       `json:"v"`
	// N: number (right-aligned). X: logical or error (centered).
	N bool `json:"n,omitempty"`
	X bool `json:"x,omitempty"`
	// St is the style ID; Fc a color from the number format ([Red]).
	St uint32 `json:"st,omitempty"`
	Fc string `json:"fc,omitempty"`
	// Data bar from conditional formatting: length 0..1 and color.
	Bar float64 `json:"bar,omitempty"`
	Bc  string  `json:"bc,omitempty"`
}

type BlockRef struct {
	Row int `json:"row"`
	Col int `json:"col"`
}

type BlockDTO struct {
	Row   int       `json:"row"`
	Col   int       `json:"col"`
	Cells []CellDTO `json:"cells"`
}

// BlocksResult carries the document version the blocks were read at, so
// the frontend can drop results that raced with an edit, and any styles
// the caller does not know yet.
type BlocksResult struct {
	Version uint64     `json:"version"`
	Blocks  []BlockDTO `json:"blocks"`
	Styles  []StyleDTO `json:"styles"`
}

type SheetInfo struct {
	Name      string `json:"name"`
	Rows      int    `json:"rows"`
	Cols      int    `json:"cols"`
	UsedRows  int    `json:"usedRows"`
	UsedCols  int    `json:"usedCols"`
	BlockRows int    `json:"blockRows"`
	BlockCols int    `json:"blockCols"`
}

type CellPos struct {
	Row int `json:"row"`
	Col int `json:"col"`
}

// TextSample is a cell's display text and style, for measuring when
// auto-fitting columns and rows.
type TextSample struct {
	Row int    `json:"row"`
	Col int    `json:"col"`
	V   string `json:"v"`
	St  uint32 `json:"st"`
}

type SheetService struct {
	src Sessions
	// The internal clipboard, shared by all windows.
	clip clipState
}

func NewSheetService(src Sessions) *SheetService {
	return &SheetService{src: src}
}

func (s *SheetService) at(ctx context.Context) *Session { return s.src.For(ctx) }

func sheetAt(d *document.Document, index int) *model.Sheet {
	sheets := d.Workbook().Sheets
	if index < 0 || index >= len(sheets) {
		return nil
	}
	return sheets[index]
}

func validCell(row, col int) bool {
	return row >= 0 && row < model.MaxRows && col >= 0 && col < model.MaxCols
}

func toModelRanges(ranges []RangeDTO) []model.Range {
	out := make([]model.Range, len(ranges))
	for i, r := range ranges {
		out[i] = r.model().Clamp()
	}
	return out
}

// Info returns grid dimensions and block geometry for a sheet.
func (s *SheetService) Info(ctx context.Context, sheet int) SheetInfo {
	info := SheetInfo{
		Rows:      model.MaxRows,
		Cols:      model.MaxCols,
		BlockRows: model.BlockRows,
		BlockCols: model.BlockCols,
	}
	s.at(ctx).read(func(d *document.Document) {
		if sh := sheetAt(d, sheet); sh != nil {
			info.Name = sh.Name
			info.UsedRows, info.UsedCols = sh.UsedRange()
		}
	})
	return info
}

// GetBlocks returns the requested blocks in one call. Empty blocks are
// returned with no cells so the frontend can mark them as loaded.
// knownStyles is how many styles the caller has; newer ones are included.
func (s *SheetService) GetBlocks(ctx context.Context, sheet int, refs []BlockRef, knownStyles int) BlocksResult {
	res := BlocksResult{Blocks: make([]BlockDTO, 0, len(refs))}
	s.at(ctx).read(func(d *document.Document) {
		wb := d.Workbook()
		res.Version = d.Version()
		sh := sheetAt(d, sheet)
		look := newLook(d, sheet)
		for _, ref := range refs {
			dto := BlockDTO{Row: ref.Row, Col: ref.Col, Cells: []CellDTO{}}
			if sh != nil {
				r0, c0 := ref.Row*model.BlockRows, ref.Col*model.BlockCols
				b := sh.Block(model.BlockKey{Row: int32(ref.Row), Col: int32(ref.Col)})
				if look == nil {
					if b != nil {
						dto.Cells = make([]CellDTO, 0, b.Count())
						b.Each(func(r, c int, cell model.Cell) {
							v, n, x, st, fc := cellView(wb, sh, r0+r, c0+c, cell)
							dto.Cells = append(dto.Cells, CellDTO{R: r, C: c, V: v, N: n, X: x, St: st, Fc: fc, Runs: richTextDTO(cell.Rich, wb.StyleTable().Get(st).Font)})
						})
					}
				} else {
					dto.Cells = lookBlock(look, sh, b, r0, c0)
				}
			}
			res.Blocks = append(res.Blocks, dto)
		}
		// Styles go last: the look may have created some.
		res.Styles = stylesSince(wb, knownStyles)
	})
	return res
}

// lookBlock renders a block with table styles and conditional formats,
// including blank cells inside their ranges.
func lookBlock(look *sheetLook, sh *model.Sheet, b *model.Block, r0, c0 int) []CellDTO {
	block := model.Range{R0: r0, C0: c0, R1: r0 + model.BlockRows - 1, C1: c0 + model.BlockCols - 1}
	seen := map[[2]int]bool{}
	out := []CellDTO{}
	add := func(r, c int, cell model.Cell) {
		if seen[[2]int{r, c}] {
			return
		}
		seen[[2]int{r, c}] = true
		if dto, show := look.view(r0+r, c0+c, cell); show {
			dto.R, dto.C = r, c
			out = append(out, dto)
		}
	}
	if b != nil {
		b.Each(add)
	}
	for _, a := range look.areas() {
		if !a.Overlaps(block) {
			continue
		}
		for row := max(a.R0, block.R0); row <= min(a.R1, block.R1); row++ {
			for col := max(a.C0, block.C0); col <= min(a.C1, block.C1); col++ {
				if b == nil || !b.Cell(row-r0, col-c0).HasContent() {
					add(row-r0, col-c0, sh.Get(row, col))
				}
			}
		}
	}
	return out
}

// Layout returns a sheet's sizes, row/column styles and merges.
func (s *SheetService) Layout(ctx context.Context, sheet int, knownStyles int) LayoutDTO {
	var out LayoutDTO
	s.at(ctx).read(func(d *document.Document) {
		sh := sheetAt(d, sheet)
		if sh == nil {
			out = layoutDTO(d.Workbook(), model.NewLayout(), knownStyles)
			return
		}
		out = layoutDTO(d.Workbook(), sh.Layout, knownStyles)
	})
	return out
}

// EditText returns the text to show when editing a cell (F2).
func (s *SheetService) EditText(ctx context.Context, sheet, row, col int) string {
	var text string
	s.at(ctx).read(func(d *document.Document) {
		if sh := sheetAt(d, sheet); sh != nil && validCell(row, col) {
			text = sh.Get(row, col).EditText()
		}
	})
	return text
}

// SetCellResult reports a notice for the user about an accepted input,
// e.g. a data validation warning.
type SetCellResult struct {
	Notice string `json:"notice"`
}

// SetCell commits typed text into one cell. A formula that cannot be
// entered (syntax error, wrong argument count) or a value refused by
// data validation returns an error whose message is shown to the user;
// nothing changes. Typing right below or next to a table extends it.
func (s *SheetService) SetCell(ctx context.Context, sheet, row, col int, text string) (SetCellResult, error) {
	var err error
	var res SetCellResult
	s.at(ctx).change(false, func(d *document.Document) (document.Change, bool) {
		if sheetAt(d, sheet) == nil || !validCell(row, col) {
			return document.Change{}, false
		}
		var in document.Input
		if in, err = d.InputCell(sheet, text); err != nil {
			return document.Change{}, false
		}
		if in.Cell.F == nil {
			if verr := d.Validate(sheet, row, col, in.Cell, shownText(d.Workbook())); verr != nil {
				if verr.Stop {
					err = verr
					return document.Change{}, false
				}
				res.Notice = verr.Message
			}
		}
		cmd := command.SetCell(d.Workbook(), sheet, row, col, in.Cell, in.NumFmt, in.Wrap)
		if cmd == nil {
			return document.Change{}, false
		}
		if in.Cell.HasContent() {
			if grow := d.ExpandTableCommand(sheet, row, col); grow != nil {
				// The table grows first so structured references resolve.
				cmd = &command.Batch{Name: "輸入", Cmds: []command.Command{grow, cmd}}
			}
		}
		return d.Exec(cmd)
	})
	return res, err
}

// Fill writes text into every cell of the ranges (⌘Enter). A formula is
// entered at the active cell and adjusted for the others.
func (s *SheetService) Fill(ctx context.Context, sheet int, ranges []RangeDTO, text string, activeRow, activeCol int) error {
	var err error
	s.at(ctx).change(false, func(d *document.Document) (document.Change, bool) {
		if sheetAt(d, sheet) == nil {
			return document.Change{}, false
		}
		var cellAt func(int, int) model.Cell
		if cellAt, err = d.FillCells(sheet, text, activeRow, activeCol); err != nil {
			return document.Change{}, false
		}
		var cmd command.Command
		if cmd, err = command.FillRanges(d.Workbook(), sheet, toModelRanges(ranges), cellAt); err != nil {
			return document.Change{}, false
		}
		return d.Exec(cmd)
	})
	return err
}

// Clear empties the ranges: mode "contents" (Delete key), "formats" or
// "all".
func (s *SheetService) Clear(ctx context.Context, sheet int, ranges []RangeDTO, mode string) {
	m := command.ClearContents
	switch mode {
	case "formats":
		m = command.ClearFormats
	case "all":
		m = command.ClearAll
	}
	s.at(ctx).change(false, func(d *document.Document) (document.Change, bool) {
		if sheetAt(d, sheet) == nil {
			return document.Change{}, false
		}
		return d.Exec(command.ClearRanges(d.Workbook(), sheet, toModelRanges(ranges), m))
	})
}

// Format applies a formatting change to the ranges.
func (s *SheetService) Format(ctx context.Context, sheet int, ranges []RangeDTO, patch FormatPatch) error {
	label := patch.Label
	if label == "" {
		label = "格式設定"
	}
	var err error
	s.at(ctx).change(false, func(d *document.Document) (document.Change, bool) {
		if sheetAt(d, sheet) == nil {
			return document.Change{}, false
		}
		var cmd command.Command
		if cmd, err = command.ApplyStyle(d.Workbook(), sheet, toModelRanges(ranges), label, patch.apply); err != nil || cmd == nil {
			return document.Change{}, false
		}
		return d.Exec(cmd)
	})
	return err
}

// SetSizes sets row heights (rows = true) or column widths in pixels;
// px < 0 restores the default.
func (s *SheetService) SetSizes(ctx context.Context, sheet int, rows bool, indices []int, px float64) {
	s.at(ctx).change(false, func(d *document.Document) (document.Change, bool) {
		if sheetAt(d, sheet) == nil || len(indices) == 0 {
			return document.Change{}, false
		}
		return d.Exec(command.SetSizes(d.Workbook(), sheet, rows, indices, px))
	})
}

// Merge merges rg; center also centers it (跨欄置中).
func (s *SheetService) Merge(ctx context.Context, sheet int, rg RangeDTO, center bool) error {
	var err error
	s.at(ctx).change(true, func(d *document.Document) (document.Change, bool) {
		if sheetAt(d, sheet) == nil {
			return document.Change{}, false
		}
		var cmd command.Command
		if cmd, err = command.Merge(d.Workbook(), sheet, rg.model(), center); err != nil || cmd == nil {
			return document.Change{}, false
		}
		return d.Exec(cmd)
	})
	return err
}

func (s *SheetService) Unmerge(ctx context.Context, sheet int, ranges []RangeDTO) {
	s.at(ctx).change(false, func(d *document.Document) (document.Change, bool) {
		if sheetAt(d, sheet) == nil {
			return document.Change{}, false
		}
		return d.Exec(command.Unmerge(d.Workbook(), sheet, toModelRanges(ranges)))
	})
}

// AutoFitColumn returns the longest texts of a column (by characters,
// per style), which the frontend measures to auto-fit the width.
func (s *SheetService) AutoFitColumn(ctx context.Context, sheet, col int) []TextSample {
	const keep = 40
	var out []TextSample
	s.at(ctx).read(func(d *document.Document) {
		sh := sheetAt(d, sheet)
		if sh == nil || col < 0 || col >= model.MaxCols {
			return
		}
		wb := d.Workbook()
		sh.EachInRange(model.Range{R0: 0, C0: col, R1: model.MaxRows - 1, C1: col}, func(row, c int, cell model.Cell) {
			if !cell.HasContent() {
				return
			}
			v, _, _, st, _ := cellView(wb, sh, row, c, cell)
			out = append(out, TextSample{Row: row, Col: c, V: v, St: st})
		})
	})
	sort.SliceStable(out, func(i, j int) bool { return len([]rune(out[i].V)) > len([]rune(out[j].V)) })
	if len(out) > keep {
		out = out[:keep]
	}
	return out
}

// RowTexts returns the cells of a row, for auto-fitting its height.
func (s *SheetService) RowTexts(ctx context.Context, sheet, row int) []TextSample {
	var out []TextSample
	s.at(ctx).read(func(d *document.Document) {
		sh := sheetAt(d, sheet)
		if sh == nil || row < 0 || row >= model.MaxRows {
			return
		}
		wb := d.Workbook()
		sh.EachInRange(model.Range{R0: row, C0: 0, R1: row, C1: model.MaxCols - 1}, func(r, col int, cell model.Cell) {
			if cell.HasContent() {
				v, _, _, st, _ := cellView(wb, sh, r, col, cell)
				out = append(out, TextSample{Row: r, Col: col, V: v, St: st})
			}
		})
	})
	return out
}

// PreviewFormat renders a cell's value with a format code, for the
// number format dialog. Empty cells preview the number 1234.5678.
func (s *SheetService) PreviewFormat(ctx context.Context, sheet, row, col int, code string) string {
	var out string
	s.at(ctx).read(func(d *document.Document) {
		c := model.NumberCell(1234.5678)
		if sh := sheetAt(d, sheet); sh != nil && validCell(row, col) && sh.Get(row, col).HasContent() {
			c = sh.Get(row, col)
		}
		out, _ = display(c, model.Style{NumFmt: code})
	})
	return out
}

func (s *SheetService) Undo(ctx context.Context) bool {
	return s.at(ctx).change(true, func(d *document.Document) (document.Change, bool) { return d.Undo() })
}

func (s *SheetService) Redo(ctx context.Context) bool {
	return s.at(ctx).change(true, func(d *document.Document) (document.Change, bool) { return d.Redo() })
}

// Functions lists worksheet functions for autocomplete and hints.
func (s *SheetService) Functions(ctx context.Context) []formula.FuncInfo {
	return formula.Functions()
}

// DataEdge returns the ⌘+arrow target from (row, col).
func (s *SheetService) DataEdge(ctx context.Context, sheet, row, col, dr, dc int) CellPos {
	pos := CellPos{Row: row, Col: col}
	if !validCell(row, col) || abs(dr)+abs(dc) != 1 {
		return pos
	}
	s.at(ctx).read(func(d *document.Document) {
		if sh := sheetAt(d, sheet); sh != nil {
			pos.Row, pos.Col = sh.DataEdge(row, col, dr, dc)
		}
	})
	return pos
}

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}
