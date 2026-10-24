package document

import (
	"strings"

	"ww-excel/internal/formula/ast"
	"ww-excel/internal/model"
	"ww-excel/internal/numfmt"
)

// Input is typed text turned into a cell, plus formatting Excel applies
// automatically: a number format for "12%" or "2026/10/23", and text
// wrapping when the text contains a line break.
type Input struct {
	Cell   model.Cell
	NumFmt string
	Wrap   bool
}

// InputCell turns typed text into a cell, compiling formulas. A formula
// the user cannot enter returns an error with a message for the user.
func (d *Document) InputCell(sheet int, text string) (Input, error) {
	cell := model.ParseInput(text)
	in := Input{Cell: cell, Wrap: strings.Contains(text, "\n")}
	switch {
	case cell.F != nil:
		f, err := d.calc.Compile(cell.F.Text, sheet)
		if err != nil {
			return Input{}, err
		}
		in.Cell = model.Cell{F: f}
		in.NumFmt = d.formulaFormat(sheet, d.calc.Parsed(f, sheet))
	case cell.Kind == model.KindString && !strings.HasPrefix(text, "'"):
		if v, code, ok := numfmt.Detect(text, d.Now()); ok {
			in.Cell = model.NumberCell(v)
			in.NumFmt = code
		}
	}
	return in, nil
}

// formulaFormat infers a format for a new formula like Excel does: date
// functions get a date format; arithmetic and aggregates take the number
// format of the first referenced cell (=A1+7 with a date in A1 is a
// date, =SUM(B:B) of amounts keeps their currency format).
func (d *Document) formulaFormat(sheet int, node ast.Node) string {
	switch x := node.(type) {
	case ast.Paren:
		return d.formulaFormat(sheet, x.X)
	case ast.Unary:
		return d.formulaFormat(sheet, x.X)
	case ast.Binary:
		switch x.Op {
		case "+", "-", "*", "/":
			if f := d.formulaFormat(sheet, x.L); f != "" {
				return f
			}
			return d.formulaFormat(sheet, x.R)
		}
	case ast.Call:
		switch x.Name {
		case "TODAY", "DATE", "EDATE", "EOMONTH":
			return "yyyy/m/d"
		case "NOW":
			return "yyyy/m/d h:mm"
		case "TIME":
			return "h:mm AM/PM"
		case "SUM", "AVERAGE", "MIN", "MAX", "ROUND", "ROUNDUP", "ROUNDDOWN", "ABS":
			if len(x.Args) > 0 {
				return d.formulaFormat(sheet, x.Args[0])
			}
		}
	case ast.Ref:
		target := sheet
		if x.Sheet != "" {
			if target = d.wb.SheetIndex(x.Sheet); target < 0 {
				return ""
			}
		}
		sh := d.wb.Sheets[target]
		return d.wb.StyleTable().Get(sh.EffectiveStyle(x.Area.R0, x.Area.C0)).NumFmt
	}
	return ""
}

// FillCells returns a cell factory for ⌘Enter: a formula typed at the
// active cell is adjusted for every other cell, like Excel's fill.
func (d *Document) FillCells(sheet int, text string, activeRow, activeCol int) (func(row, col int) model.Cell, error) {
	in, err := d.InputCell(sheet, text)
	if err != nil {
		return nil, err
	}
	cell := in.Cell
	if cell.F == nil {
		return func(int, int) model.Cell { return cell }, nil
	}
	node := d.calc.Parsed(cell.F, sheet)
	return func(row, col int) model.Cell {
		if row == activeRow && col == activeCol {
			return cell
		}
		moved := ast.Translate(node, row-activeRow, col-activeCol)
		return model.Cell{F: d.calc.CompileNode(moved, sheet)}
	}, nil
}

// address formats a cell address, optionally with the sheet name.
func (d *Document) address(sheet, row, col int, withSheet bool) string {
	addr := ast.FormatArea(ast.CellAt(row, col, false, false))
	if withSheet && sheet < len(d.wb.Sheets) {
		addr = ast.QuoteSheet(d.wb.Sheets[sheet].Name) + "!" + addr
	}
	return addr
}
