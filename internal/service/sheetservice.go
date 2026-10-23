package service

import (
	"ww-excel/internal/command"
	"ww-excel/internal/document"
	"ww-excel/internal/formula"
	"ww-excel/internal/model"
)

// CellDTO is a single non-empty cell inside a block. R and C are offsets
// within the block; short JSON keys keep the payload small.
type CellDTO struct {
	R int    `json:"r"`
	C int    `json:"c"`
	V string `json:"v"`
	// N: number (right-aligned). X: logical or error (centered).
	N bool `json:"n,omitempty"`
	X bool `json:"x,omitempty"`
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
// the frontend can drop results that raced with an edit.
type BlocksResult struct {
	Version uint64     `json:"version"`
	Blocks  []BlockDTO `json:"blocks"`
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

type SheetService struct {
	s *Session
}

func NewSheetService(s *Session) *SheetService {
	return &SheetService{s: s}
}

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
func (s *SheetService) Info(sheet int) SheetInfo {
	info := SheetInfo{
		Rows:      model.MaxRows,
		Cols:      model.MaxCols,
		BlockRows: model.BlockRows,
		BlockCols: model.BlockCols,
	}
	s.s.read(func(d *document.Document) {
		if sh := sheetAt(d, sheet); sh != nil {
			info.Name = sh.Name
			info.UsedRows, info.UsedCols = sh.UsedRange()
		}
	})
	return info
}

// GetBlocks returns the requested blocks in one call. Empty blocks are
// returned with no cells so the frontend can mark them as loaded.
func (s *SheetService) GetBlocks(sheet int, refs []BlockRef) BlocksResult {
	res := BlocksResult{Blocks: make([]BlockDTO, 0, len(refs))}
	s.s.read(func(d *document.Document) {
		res.Version = d.Version()
		sh := sheetAt(d, sheet)
		for _, ref := range refs {
			dto := BlockDTO{Row: ref.Row, Col: ref.Col, Cells: []CellDTO{}}
			if sh != nil {
				if b := sh.Block(model.BlockKey{Row: int32(ref.Row), Col: int32(ref.Col)}); b != nil {
					dto.Cells = make([]CellDTO, 0, b.Count())
					b.Each(func(r, c int, cell model.Cell) {
						v, n, x := cellDisplay(cell)
						dto.Cells = append(dto.Cells, CellDTO{R: r, C: c, V: v, N: n, X: x})
					})
				}
			}
			res.Blocks = append(res.Blocks, dto)
		}
	})
	return res
}

// EditText returns the text to show when editing a cell (F2).
func (s *SheetService) EditText(sheet, row, col int) string {
	var text string
	s.s.read(func(d *document.Document) {
		if sh := sheetAt(d, sheet); sh != nil && validCell(row, col) {
			text = sh.Get(row, col).EditText()
		}
	})
	return text
}

// SetCell commits typed text into one cell. A formula that cannot be
// entered (syntax error, wrong argument count) returns an error whose
// message is shown to the user; nothing changes.
func (s *SheetService) SetCell(sheet, row, col int, text string) error {
	var err error
	s.s.change(false, func(d *document.Document) (document.Change, bool) {
		if sheetAt(d, sheet) == nil || !validCell(row, col) {
			return document.Change{}, false
		}
		var cell model.Cell
		if cell, err = d.InputCell(sheet, text); err != nil {
			return document.Change{}, false
		}
		return d.Exec(command.SetCell(d.Workbook(), sheet, row, col, cell))
	})
	return err
}

// Fill writes text into every cell of the ranges (⌘Enter). A formula is
// entered at the active cell and adjusted for the others.
func (s *SheetService) Fill(sheet int, ranges []RangeDTO, text string, activeRow, activeCol int) error {
	var err error
	s.s.change(false, func(d *document.Document) (document.Change, bool) {
		if sheetAt(d, sheet) == nil {
			return document.Change{}, false
		}
		var cellAt func(int, int) model.Cell
		if cellAt, err = d.FillCells(sheet, text, activeRow, activeCol); err != nil {
			return document.Change{}, false
		}
		var cmd command.Command
		if cmd, err = command.FillRanges(sheet, toModelRanges(ranges), cellAt); err != nil {
			return document.Change{}, false
		}
		return d.Exec(cmd)
	})
	return err
}

// Clear empties the ranges (Delete key).
func (s *SheetService) Clear(sheet int, ranges []RangeDTO) {
	s.s.change(false, func(d *document.Document) (document.Change, bool) {
		if sheetAt(d, sheet) == nil {
			return document.Change{}, false
		}
		return d.Exec(command.ClearRanges(d.Workbook(), sheet, toModelRanges(ranges)))
	})
}

func (s *SheetService) Undo() bool {
	return s.s.change(true, func(d *document.Document) (document.Change, bool) { return d.Undo() })
}

func (s *SheetService) Redo() bool {
	return s.s.change(true, func(d *document.Document) (document.Change, bool) { return d.Redo() })
}

// Functions lists worksheet functions for autocomplete and hints.
func (s *SheetService) Functions() []formula.FuncInfo {
	return formula.Functions()
}

// DataEdge returns the ⌘+arrow target from (row, col).
func (s *SheetService) DataEdge(sheet, row, col, dr, dc int) CellPos {
	pos := CellPos{Row: row, Col: col}
	if !validCell(row, col) || abs(dr)+abs(dc) != 1 {
		return pos
	}
	s.s.read(func(d *document.Document) {
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
