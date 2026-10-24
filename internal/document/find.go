package document

import (
	"errors"
	"strings"

	"ww-excel/internal/command"
	"ww-excel/internal/model"
)

// FindOptions control 尋找與取代. Text is matched in each cell's display
// text, or in its formula when InFormulas is set.
type FindOptions struct {
	Text       string
	MatchCase  bool
	WholeCell  bool
	InFormulas bool
	AllSheets  bool
}

// Hit is a matching cell.
type Hit struct {
	Sheet, Row, Col int
}

// DisplayFunc renders a cell's text as shown (number formats applied).
type DisplayFunc func(sh *model.Sheet, row, col int, c model.Cell) string

func (o FindOptions) matches(s string) bool {
	text := o.Text
	if !o.MatchCase {
		s, text = strings.ToLower(s), strings.ToLower(text)
	}
	if o.WholeCell {
		return s == text
	}
	return strings.Contains(s, text)
}

func (d *Document) cellText(sh *model.Sheet, row, col int, c model.Cell, o FindOptions, display DisplayFunc) string {
	if o.InFormulas {
		return c.EditText()
	}
	return display(sh, row, col, c)
}

// FindAll lists the matching cells in row-major order, sheet by sheet
// (only sheet unless AllSheets). limit bounds the result (0 = no limit).
func (d *Document) FindAll(sheet int, o FindOptions, display DisplayFunc, limit int) []Hit {
	if o.Text == "" {
		return nil
	}
	var out []Hit
	for si, sh := range d.wb.Sheets {
		if !o.AllSheets && si != sheet {
			continue
		}
		done := false
		sh.EachInRange(model.Range{R1: model.MaxRows - 1, C1: model.MaxCols - 1}, func(row, col int, c model.Cell) {
			if done || !c.HasContent() {
				return
			}
			if o.matches(d.cellText(sh, row, col, c, o, display)) {
				out = append(out, Hit{Sheet: si, Row: row, Col: col})
				done = limit > 0 && len(out) >= limit
			}
		})
		if done {
			break
		}
	}
	return out
}

// FindNext returns the first match after (row, col) on sheet, going
// forward or backward and wrapping around, like Excel's 找下一個.
func (d *Document) FindNext(sheet, row, col int, o FindOptions, display DisplayFunc, forward bool) (Hit, bool) {
	hits := d.FindAll(sheet, o, display, 0)
	if len(hits) == 0 {
		return Hit{}, false
	}
	after := func(h Hit) bool {
		if h.Sheet != sheet {
			return h.Sheet > sheet
		}
		if h.Row != row {
			return h.Row > row
		}
		return h.Col > col
	}
	before := func(h Hit) bool {
		if h.Sheet != sheet {
			return h.Sheet < sheet
		}
		if h.Row != row {
			return h.Row < row
		}
		return h.Col < col
	}
	if forward {
		for _, h := range hits {
			if after(h) {
				return h, true
			}
		}
		return hits[0], true
	}
	for i := len(hits) - 1; i >= 0; i-- {
		if before(hits[i]) {
			return hits[i], true
		}
	}
	return hits[len(hits)-1], true
}

// ReplaceCommand replaces the search text in the given cells' input text
// (formulas included) and enters the result as if typed. Cells whose
// result would be an invalid formula are skipped. Returns how many
// cells changed.
func (d *Document) ReplaceCommand(hits []Hit, o FindOptions, with string) (command.Command, int, error) {
	if o.Text == "" {
		return nil, 0, errors.New("請輸入要尋找的內容")
	}
	bySheet := map[int][]command.CellChange{}
	var order []int
	count := 0
	for _, h := range hits {
		if h.Sheet < 0 || h.Sheet >= len(d.wb.Sheets) {
			continue
		}
		sh := d.wb.Sheets[h.Sheet]
		old := sh.Get(h.Row, h.Col)
		text := replaceText(old.EditText(), o, with)
		if text == old.EditText() {
			continue
		}
		in, err := d.InputCell(h.Sheet, text)
		if err != nil {
			continue
		}
		cell := in.Cell
		cell.S = old.S
		if _, ok := bySheet[h.Sheet]; !ok {
			order = append(order, h.Sheet)
		}
		bySheet[h.Sheet] = append(bySheet[h.Sheet], command.CellChange{Row: h.Row, Col: h.Col, After: cell})
		count++
	}
	if count == 0 {
		return nil, 0, nil
	}
	cmds := make([]command.Command, 0, len(order))
	for _, si := range order {
		cmds = append(cmds, &command.SetCells{Sheet: si, Name: "取代", Changes: bySheet[si]})
	}
	if len(cmds) == 1 {
		return cmds[0], count, nil
	}
	return &command.Batch{Name: "取代", Cmds: cmds}, count, nil
}

func replaceText(s string, o FindOptions, with string) string {
	if o.WholeCell {
		if o.matches(s) {
			return with
		}
		return s
	}
	if o.MatchCase {
		return strings.ReplaceAll(s, o.Text, with)
	}
	// Case-insensitive replace, keeping the rest of the text as is.
	lower, needle := strings.ToLower(s), strings.ToLower(o.Text)
	if len(lower) != len(s) {
		return strings.ReplaceAll(s, o.Text, with) // non-ASCII case folding changed lengths
	}
	var b strings.Builder
	for {
		i := strings.Index(lower, needle)
		if i < 0 {
			b.WriteString(s)
			return b.String()
		}
		b.WriteString(s[:i])
		b.WriteString(with)
		s, lower = s[i+len(needle):], lower[i+len(needle):]
	}
}
