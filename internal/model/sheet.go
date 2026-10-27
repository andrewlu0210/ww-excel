// Package model holds the in-memory workbook representation.
// The Go side is the single source of truth; the frontend only caches
// blocks around the viewport.
package model

import (
	"strings"

	"ww-excel/internal/numfmt"
)

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
	KindBool  // Num is 1 or 0
	KindError // Str holds the error code, e.g. "#DIV/0!"
)

// Excel error values.
const (
	ErrNull  = "#NULL!"
	ErrDiv0  = "#DIV/0!"
	ErrValue = "#VALUE!"
	ErrRef   = "#REF!"
	ErrName  = "#NAME?"
	ErrNum   = "#NUM!"
	ErrNA    = "#N/A"
	ErrSpill = "#SPILL!"
	ErrCalc  = "#CALC!"
)

var errorCodes = []string{ErrNull, ErrDiv0, ErrValue, ErrRef, ErrName, ErrNum, ErrNA, ErrSpill, ErrCalc}

// ErrorCode returns the canonical error code matching s
// case-insensitively, or "".
func ErrorCode(s string) string {
	for _, code := range errorCodes {
		if len(s) == len(code) && equalFoldASCII(s, code) {
			return code
		}
	}
	return ""
}

func equalFoldASCII(a, b string) bool {
	for i := 0; i < len(a); i++ {
		x, y := a[i], b[i]
		if 'a' <= x && x <= 'z' {
			x -= 'a' - 'A'
		}
		if 'a' <= y && y <= 'z' {
			y -= 'a' - 'A'
		}
		if x != y {
			return false
		}
	}
	return true
}

// Formula is a cell formula. Text is the canonical formula without the
// leading "="; Compiled is owned by the formula engine (parsed AST and
// precedents) and may be nil until compiled.
type Formula struct {
	Text     string
	Compiled any
}

// Cell holds a value. For formula cells (F != nil) the value is the last
// calculated result. S is the style ID (0 = inherit the row/column
// style, else the workbook default); a cell may hold only a style.
type Cell struct {
	Rich *RichText
	Kind CellKind
	S    uint32
	Num  float64
	Str  string
	F    *Formula
}

func NumberCell(n float64) Cell  { return Cell{Kind: KindNumber, Num: n} }
func StringCell(s string) Cell   { return Cell{Kind: KindString, Str: s} }
func ErrorCell(code string) Cell { return Cell{Kind: KindError, Str: code} }

func BoolCell(b bool) Cell {
	if b {
		return Cell{Kind: KindBool, Num: 1}
	}
	return Cell{Kind: KindBool}
}

func FormulaCell(text string) Cell { return Cell{F: &Formula{Text: text}} }

// IsEmpty reports whether the cell holds no value, formula or style.
func (c Cell) IsEmpty() bool { return c.Kind == KindEmpty && c.F == nil && c.S == 0 }

// HasContent reports whether the cell holds a value or formula.
func (c Cell) HasContent() bool { return c.Kind != KindEmpty || c.F != nil }

// Value returns the cell without its style, e.g. to compare contents.
func (c Cell) Value() Cell {
	c.Rich = nil
	c.S = 0
	return c
}

// SameInput reports whether two cells hold the same user input: equal
// formula text, or equal values for non-formula cells.
func (c Cell) SameInput(o Cell) bool {
	if (c.F == nil) != (o.F == nil) {
		return false
	}
	if c.F != nil {
		return c.F.Text == o.F.Text
	}
	return c.Kind == o.Kind && c.Num == o.Num && c.Str == o.Str
}

// SameValue reports whether two cells hold the same value, ignoring
// formulas. Used to detect recalculated cells that actually changed.
func (c Cell) SameValue(o Cell) bool {
	return c.Kind == o.Kind && c.Num == o.Num && c.Str == o.Str
}

// Display returns the text shown in the grid.
// Number formats arrive in v0.4; until then numbers use General.
func (c Cell) Display() string {
	switch c.Kind {
	case KindNumber:
		return numfmt.General(c.Num)
	case KindString, KindError:
		return c.Str
	case KindBool:
		if c.Num != 0 {
			return "TRUE"
		}
		return "FALSE"
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

// Block holds the cells of a BlockRows × BlockCols area sparsely: idx
// maps a position to its slot in cells (0 = empty), so a block with a
// few used columns costs little more than its cells.
type Block struct {
	idx   [BlockRows * BlockCols]uint16
	cells []Cell
	count int
}

// Cell returns the cell at the given offset within the block.
func (b *Block) Cell(r, c int) Cell {
	if j := b.idx[r*BlockCols+c]; j != 0 {
		return b.cells[j-1]
	}
	return Cell{}
}

// set stores a cell at index i and reports whether it was empty.
func (b *Block) set(i int, cell Cell) (wasEmpty bool) {
	j := b.idx[i]
	if j == 0 {
		if cell.IsEmpty() {
			return true
		}
		b.cells = append(b.cells, cell)
		b.idx[i] = uint16(len(b.cells))
		return true
	}
	wasEmpty = b.cells[j-1].IsEmpty()
	b.cells[j-1] = cell
	return wasEmpty
}

// compact drops the slots of cells that became empty.
func (b *Block) compact() {
	cells := make([]Cell, 0, b.count)
	for i, j := range b.idx {
		if j == 0 {
			continue
		}
		if c := b.cells[j-1]; !c.IsEmpty() {
			cells = append(cells, c)
			b.idx[i] = uint16(len(cells))
		} else {
			b.idx[i] = 0
		}
	}
	b.cells = cells
}

// Count reports how many non-empty cells the block holds.
func (b *Block) Count() int { return b.count }

// Each calls fn for every non-empty cell in row-major order.
func (b *Block) Each(fn func(r, c int, cell Cell)) {
	if b.count == 0 {
		return
	}
	for i, j := range b.idx {
		if j != 0 && !b.cells[j-1].IsEmpty() {
			fn(i/BlockCols, i%BlockCols, b.cells[j-1])
		}
	}
}

// Sheet stores cells sparsely: only blocks containing data are allocated.
type Sheet struct {
	Name   string
	Layout Layout
	// SourceName is the sheet's name in the xlsx file it was loaded
	// from ("" for new sheets), so in-place saves can map sheets.
	SourceName string

	blocks map[BlockKey]*Block

	// Used range bounds (exclusive), i.e. the last used row/col + 1.
	usedRows, usedCols int
}

func NewSheet(name string) *Sheet {
	return &Sheet{Name: name, Layout: NewLayout(), blocks: make(map[BlockKey]*Block)}
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
		if cell.IsEmpty() {
			return
		}
		b = &Block{}
		s.blocks[key] = b
	}

	i := (row%BlockRows)*BlockCols + col%BlockCols
	wasEmpty := b.set(i, cell)

	switch {
	case wasEmpty && !cell.IsEmpty():
		b.count++
	case !wasEmpty && cell.IsEmpty():
		b.count--
		if b.count == 0 {
			delete(s.blocks, key)
		} else if len(b.cells) > 64 && b.count < len(b.cells)/2 {
			b.compact()
		}
	}

	if cell.HasContent() {
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
	Styles *StyleTable
	Names  []*DefinedName
}

func NewWorkbook() *Workbook {
	return &Workbook{Sheets: []*Sheet{NewSheet("工作表1")}, Styles: NewStyleTable(Style{Font: DefaultFont})}
}

// StyleTable returns the workbook's styles, creating the default table
// for workbooks built without one.
func (wb *Workbook) StyleTable() *StyleTable {
	if wb.Styles == nil {
		wb.Styles = NewStyleTable(Style{Font: DefaultFont})
	}
	return wb.Styles
}

// SheetIndex finds a sheet by name, case-insensitively like Excel.
func (wb *Workbook) SheetIndex(name string) int {
	for i, s := range wb.Sheets {
		if strings.EqualFold(s.Name, name) {
			return i
		}
	}
	return -1
}

// EffectiveStyle is the style ID that applies at a position: the cell's
// own style, else the row or column style.
func (s *Sheet) EffectiveStyle(row, col int) uint32 {
	if c := s.Get(row, col); c.S != 0 {
		return c.S
	}
	return s.Layout.StyleAt(row, col)
}
