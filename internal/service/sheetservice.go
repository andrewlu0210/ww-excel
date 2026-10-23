// Package service is the thin Wails-bound layer over internal packages.
// Keep logic out of here so it can be tested without Wails.
package service

import (
	"sync"

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

type SheetInfo struct {
	Name      string `json:"name"`
	Rows      int    `json:"rows"`
	Cols      int    `json:"cols"`
	UsedRows  int    `json:"usedRows"`
	UsedCols  int    `json:"usedCols"`
	BlockRows int    `json:"blockRows"`
	BlockCols int    `json:"blockCols"`
}

type SheetService struct {
	mu sync.RWMutex
	wb *model.Workbook
}

func NewSheetService(wb *model.Workbook) *SheetService {
	return &SheetService{wb: wb}
}

func (s *SheetService) sheet(index int) *model.Sheet {
	if index < 0 || index >= len(s.wb.Sheets) {
		return nil
	}
	return s.wb.Sheets[index]
}

// Info returns grid dimensions and block geometry for a sheet.
func (s *SheetService) Info(sheet int) SheetInfo {
	s.mu.RLock()
	defer s.mu.RUnlock()

	info := SheetInfo{
		Rows:      model.MaxRows,
		Cols:      model.MaxCols,
		BlockRows: model.BlockRows,
		BlockCols: model.BlockCols,
	}
	if sh := s.sheet(sheet); sh != nil {
		info.Name = sh.Name
		info.UsedRows, info.UsedCols = sh.UsedRange()
	}
	return info
}

// GetBlocks returns the requested blocks in one call. Empty blocks are
// returned with no cells so the frontend can mark them as loaded.
func (s *SheetService) GetBlocks(sheet int, refs []BlockRef) []BlockDTO {
	s.mu.RLock()
	defer s.mu.RUnlock()

	sh := s.sheet(sheet)
	out := make([]BlockDTO, 0, len(refs))
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
		out = append(out, dto)
	}
	return out
}
