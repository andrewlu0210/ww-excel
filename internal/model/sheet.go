// Package model holds the in-memory workbook representation.
// The Go side is the single source of truth; the frontend only caches
// blocks around the viewport.
package model

import "strconv"

const (
	MaxRows = 1 << 20 // 1,048,576 rows, same as Excel
	MaxCols = 1 << 14 // 16,384 columns (A..XFD)

	// Cells are stored and transferred in fixed-size blocks.
	BlockRows = 64
	BlockCols = 32
)

type CellKind uint8

const (
	KindEmpty CellKind = iota
	KindNumber
	KindString
)

type Cell struct {
	Kind CellKind
	Num  float64
	Str  string
}

func NumberCell(n float64) Cell { return Cell{Kind: KindNumber, Num: n} }
func StringCell(s string) Cell  { return Cell{Kind: KindString, Str: s} }

// Display returns the text shown in the grid.
// Number formats arrive in v0.4; until then numbers use the shortest repr.
func (c Cell) Display() string {
	switch c.Kind {
	case KindNumber:
		return strconv.FormatFloat(c.Num, 'f', -1, 64)
	case KindString:
		return c.Str
	default:
		return ""
	}
}

// BlockKey addresses a block by block indices (not cell indices).
type BlockKey struct {
	Row, Col int32
}

func KeyOf(row, col int) BlockKey {
	return BlockKey{Row: int32(row / BlockRows), Col: int32(col / BlockCols)}
}

type Block struct {
	cells [BlockRows * BlockCols]Cell
	count int
}

// Cell returns the cell at the given offset within the block.
func (b *Block) Cell(r, c int) Cell { return b.cells[r*BlockCols+c] }

// Count reports how many non-empty cells the block holds.
func (b *Block) Count() int { return b.count }

// Each calls fn for every non-empty cell in row-major order.
func (b *Block) Each(fn func(r, c int, cell Cell)) {
	if b.count == 0 {
		return
	}
	for i := range b.cells {
		if b.cells[i].Kind != KindEmpty {
			fn(i/BlockCols, i%BlockCols, b.cells[i])
		}
	}
}

// Sheet stores cells sparsely: only blocks containing data are allocated.
type Sheet struct {
	Name   string
	blocks map[BlockKey]*Block

	// Used range bounds (exclusive), i.e. the last used row/col + 1.
	usedRows, usedCols int
}

func NewSheet(name string) *Sheet {
	return &Sheet{Name: name, blocks: make(map[BlockKey]*Block)}
}

func (s *Sheet) Get(row, col int) Cell {
	b := s.blocks[KeyOf(row, col)]
	if b == nil {
		return Cell{}
	}
	return b.Cell(row%BlockRows, col%BlockCols)
}

func (s *Sheet) Set(row, col int, cell Cell) {
	key := KeyOf(row, col)
	b := s.blocks[key]
	if b == nil {
		if cell.Kind == KindEmpty {
			return
		}
		b = &Block{}
		s.blocks[key] = b
	}

	i := (row%BlockRows)*BlockCols + col%BlockCols
	wasEmpty := b.cells[i].Kind == KindEmpty
	b.cells[i] = cell

	switch {
	case wasEmpty && cell.Kind != KindEmpty:
		b.count++
	case !wasEmpty && cell.Kind == KindEmpty:
		b.count--
		if b.count == 0 {
			delete(s.blocks, key)
		}
	}

	if cell.Kind != KindEmpty {
		s.usedRows = max(s.usedRows, row+1)
		s.usedCols = max(s.usedCols, col+1)
	}
}

// Block returns the block at key, or nil if it holds no data.
func (s *Sheet) Block(key BlockKey) *Block { return s.blocks[key] }

func (s *Sheet) BlockCount() int { return len(s.blocks) }

// UsedRange returns the used row/col counts. Shrinking on delete is not
// tracked yet; it only grows.
func (s *Sheet) UsedRange() (rows, cols int) { return s.usedRows, s.usedCols }

type Workbook struct {
	Sheets []*Sheet
}

func NewWorkbook() *Workbook {
	return &Workbook{Sheets: []*Sheet{NewSheet("Sheet1")}}
}
