package document

import (
	"errors"
	"fmt"

	"ww-excel/internal/command"
	"ww-excel/internal/formula/ast"
	"ww-excel/internal/model"
)

// formulaSnapshot records every formula cell before a structural edit so
// undo can restore them exactly, including references that became #REF!.
type formulaSnapshot []struct {
	sheet    *model.Sheet
	row, col int
	cell     model.Cell
}

func (d *Document) snapshotFormulas() formulaSnapshot {
	var out formulaSnapshot
	for _, sh := range d.wb.Sheets {
		sh.EachInRange(model.Range{R1: model.MaxRows - 1, C1: model.MaxCols - 1}, func(row, col int, c model.Cell) {
			if c.F != nil {
				out = append(out, struct {
					sheet    *model.Sheet
					row, col int
					cell     model.Cell
				}{sh, row, col, c})
			}
		})
	}
	return out
}

func (fs formulaSnapshot) restore() {
	for _, f := range fs {
		f.sheet.Set(f.row, f.col, f.cell)
	}
}

// ShiftCommand inserts (count > 0) or deletes (count < 0) whole rows or
// columns of a sheet at index at. Formulas everywhere are adjusted like
// Excel: references move, ranges grow or shrink, and references to
// deleted cells become #REF!.
func (d *Document) ShiftCommand(sheet int, rows bool, at, count int) (command.Command, error) {
	if sheet < 0 || sheet >= len(d.wb.Sheets) || count == 0 {
		return nil, errors.New("無效的插入或刪除")
	}
	sh := d.wb.Sheets[sheet]
	limit := model.MaxCols
	if rows {
		limit = model.MaxRows
	}
	if at < 0 || at >= limit || count < 0 && at-count > limit {
		return nil, errors.New("超出工作表範圍")
	}
	if count > 0 {
		usedRows, usedCols := sh.UsedRange()
		used := usedCols
		if rows {
			used = usedRows
		}
		if used > at && used+count > limit {
			return nil, errors.New("無法插入：會把非空白的儲存格移出工作表")
		}
	}
	what := "欄"
	if rows {
		what = "列"
	}
	name := fmt.Sprintf("插入%s", what)
	if count < 0 {
		name = fmt.Sprintf("刪除%s", what)
	}

	var snap formulaSnapshot
	var names nameState
	var objects objectsState
	var removed []model.CellAt
	var layoutBefore model.Layout
	shift := &command.Shift{Rows: rows, At: at, Count: count}

	return &command.Func{
		Name: name,
		DoFn: func(wb *model.Workbook) command.Effect {
			snap = d.snapshotFormulas()
			names = d.saveNames()
			objects = d.saveObjects()
			layoutBefore = sh.Layout.Clone()
			removed = sh.Shift(rows, at, count)
			eff := command.Effect{Sheet: sheet, LayoutChanged: true, Shift: shift}
			eff.Others = d.adjustFormulas(ast.Shift{Sheet: sh.Name, Rows: rows, At: at, Count: count})
			d.adjustObjectFormulas(func(n ast.Node, sheet string) (ast.Node, bool) {
				return ast.Adjust(n, sheet, ast.Shift{Sheet: sh.Name, Rows: rows, At: at, Count: count})
			})
			adjustRuleFormulas(sh, func(n ast.Node) (ast.Node, bool) {
				return ast.Adjust(n, sh.Name, ast.Shift{Sheet: sh.Name, Rows: rows, At: at, Count: count})
			})
			eff.Names = d.adjustNames(func(n ast.Node, sheet string) (ast.Node, bool) {
				return ast.Adjust(n, sheet, ast.Shift{Sheet: sh.Name, Rows: rows, At: at, Count: count})
			})
			area := model.Range{R0: at, C0: 0, R1: at + max(count, 1) - 1, C1: model.MaxCols - 1}
			if !rows {
				area = model.Range{R0: 0, C0: at, R1: model.MaxRows - 1, C1: at + max(count, 1) - 1}
			}
			eff.Bounds = &area
			return eff
		},
		UndoFn: func(wb *model.Workbook) command.Effect {
			sh.Shift(rows, at, -count)
			for _, c := range removed {
				sh.Set(c.Row, c.Col, c.Cell)
			}
			snap.restore()
			names.restore(wb)
			objects.restore(wb)
			sh.Layout = layoutBefore.Clone()
			inverse := &command.Shift{Rows: rows, At: at, Count: -count}
			eff := command.Effect{Sheet: sheet, LayoutChanged: true, Shift: inverse, Others: map[int][]command.CellRef{}, Names: len(wb.Names) > 0}
			// Restored cells and formulas are edits for in-place saves.
			for _, c := range removed {
				eff.Cells = append(eff.Cells, command.CellRef{Row: c.Row, Col: c.Col})
			}
			for _, f := range snap {
				if i := d.wb.SheetIndex(f.sheet.Name); i >= 0 {
					eff.Others[i] = append(eff.Others[i], command.CellRef{Row: f.row, Col: f.col})
				}
			}
			return eff
		},
	}, nil
}

// adjustFormulas applies a row/column shift to every formula in the
// workbook (cells have already moved) and returns the cells rewritten.
func (d *Document) adjustFormulas(s ast.Shift) map[int][]command.CellRef {
	out := map[int][]command.CellRef{}
	for si, sh := range d.wb.Sheets {
		sh.EachInRange(model.Range{R1: model.MaxRows - 1, C1: model.MaxCols - 1}, func(row, col int, c model.Cell) {
			if c.F == nil {
				return
			}
			node, changed := ast.Adjust(d.calc.Parsed(c.F, si), sh.Name, s)
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
