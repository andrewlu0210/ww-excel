package service

import (
	"ww-excel/internal/command"
	"ww-excel/internal/document"
	"ww-excel/internal/model"
)

// CellDTO is a single non-empty cell inside a block. R and C are offsets
// within the block; short JSON keys keep the payload small.
type CellDTO struct {
	R int    `json:"r"`
	C int    `json:"c"`
	V string `json:"v"`
	// N marks numeric values (right-aligned in the grid).
	N bool `json:"n,omitempty"`
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
						dto.Cells = append(dto.Cells, CellDTO{
							R: r, C: c, V: cell.Display(), N: cell.Kind == model.KindNumber,
						})
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

// SetCell commits typed text into one cell.
func (s *SheetService) SetCell(sheet, row, col int, text string) {
	s.s.change(false, func(d *document.Document) (command.Effect, bool) {
		if sheetAt(d, sheet) == nil || !validCell(row, col) {
			return command.Effect{}, false
		}
		return d.Exec(command.EnterText(d.Workbook(), sheet, row, col, text))
	})
}

// Fill writes text into every cell of the ranges (⌘Enter).
func (s *SheetService) Fill(sheet int, ranges []RangeDTO, text string) error {
	var err error
	s.s.change(false, func(d *document.Document) (command.Effect, bool) {
		if sheetAt(d, sheet) == nil {
			return command.Effect{}, false
		}
		var cmd command.Command
		cmd, err = command.FillRanges(sheet, toModelRanges(ranges), text)
		if err != nil {
			return command.Effect{}, false
		}
		return d.Exec(cmd)
	})
	return err
}

// Clear empties the ranges (Delete key).
func (s *SheetService) Clear(sheet int, ranges []RangeDTO) {
	s.s.change(false, func(d *document.Document) (command.Effect, bool) {
		if sheetAt(d, sheet) == nil {
			return command.Effect{}, false
		}
		return d.Exec(command.ClearRanges(d.Workbook(), sheet, toModelRanges(ranges)))
	})
}

func (s *SheetService) Undo() bool {
	return s.s.change(true, func(d *document.Document) (command.Effect, bool) { return d.Undo() })
}

func (s *SheetService) Redo() bool {
	return s.s.change(true, func(d *document.Document) (command.Effect, bool) { return d.Redo() })
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
