package document

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"unicode/utf8"

	"ww-excel/internal/command"
	"ww-excel/internal/formula/ast"
	"ww-excel/internal/model"
)

// DefaultSheetPrefix names new sheets 工作表1, 工作表2, ... like Excel in
// Traditional Chinese.
const DefaultSheetPrefix = "工作表"

// ValidateSheetName applies Excel's rules for sheet names.
func (d *Document) ValidateSheetName(name string, except int) error {
	switch {
	case strings.TrimSpace(name) == "":
		return errors.New("工作表名稱不能空白")
	case utf8.RuneCountInString(name) > 31:
		return errors.New("工作表名稱不能超過 31 個字元")
	case strings.ContainsAny(name, `:\/?*[]`):
		return errors.New(`工作表名稱不能包含下列字元：: \ / ? * [ ]`)
	case strings.HasPrefix(name, "'") || strings.HasSuffix(name, "'"):
		return errors.New("工作表名稱的開頭和結尾不能是單引號")
	}
	if i := d.wb.SheetIndex(name); i >= 0 && i != except {
		return fmt.Errorf("已經有名為「%s」的工作表", name)
	}
	return nil
}

// NextSheetName returns the first unused 工作表N.
func (d *Document) NextSheetName() string {
	for n := len(d.wb.Sheets) + 1; ; n++ {
		name := fmt.Sprintf("%s%d", DefaultSheetPrefix, n)
		if d.wb.SheetIndex(name) < 0 {
			return name
		}
	}
}

func structural() command.Effect { return command.Effect{Structure: true} }

// AddSheetCommand inserts a new empty sheet at index.
func (d *Document) AddSheetCommand(index int, name string) (command.Command, error) {
	if err := d.ValidateSheetName(name, -1); err != nil {
		return nil, err
	}
	index = max(0, min(index, len(d.wb.Sheets)))
	sh := model.NewSheet(name)
	return &command.Func{
		Name: "插入工作表",
		DoFn: func(wb *model.Workbook) command.Effect {
			wb.Sheets = slices.Insert(wb.Sheets, index, sh)
			return structural()
		},
		UndoFn: func(wb *model.Workbook) command.Effect {
			wb.Sheets = slices.Delete(wb.Sheets, index, index+1)
			return structural()
		},
	}, nil
}

// formulaEdit is one formula rewritten by a sheet operation.
type formulaEdit struct {
	sheet         *model.Sheet
	row, col      int
	before, after model.Cell
}

// rewriteFormulas applies fn to every formula in the workbook (except on
// skip) and returns the cells that changed.
func (d *Document) rewriteFormulas(skip *model.Sheet, fn func(ast.Node) (ast.Node, bool)) []formulaEdit {
	var edits []formulaEdit
	for si, sh := range d.wb.Sheets {
		if sh == skip {
			continue
		}
		sh.EachInRange(model.Range{R1: model.MaxRows - 1, C1: model.MaxCols - 1}, func(row, col int, c model.Cell) {
			if c.F == nil {
				return
			}
			node, changed := fn(d.calc.Parsed(c.F, si))
			if !changed {
				return
			}
			after := c
			after.F = d.calc.CompileNode(node, si)
			edits = append(edits, formulaEdit{sheet: sh, row: row, col: col, before: c, after: after})
		})
	}
	return edits
}

func applyEdits(wb *model.Workbook, edits []formulaEdit, undo bool) map[int][]command.CellRef {
	others := map[int][]command.CellRef{}
	for _, e := range edits {
		c := e.after
		if undo {
			c = e.before
		}
		e.sheet.Set(e.row, e.col, c)
		if i := slices.Index(wb.Sheets, e.sheet); i >= 0 {
			others[i] = append(others[i], command.CellRef{Row: e.row, Col: e.col})
		}
	}
	return others
}

// DeleteSheetCommand removes a sheet; references to it become #REF!.
func (d *Document) DeleteSheetCommand(index int) (command.Command, error) {
	if len(d.wb.Sheets) <= 1 {
		return nil, errors.New("活頁簿至少要有一個工作表")
	}
	if index < 0 || index >= len(d.wb.Sheets) {
		return nil, errors.New("工作表不存在")
	}
	sh := d.wb.Sheets[index]
	edits := d.rewriteFormulas(sh, func(n ast.Node) (ast.Node, bool) { return ast.RemoveSheet(n, sh.Name) })
	var names nameState
	return &command.Func{
		Name: "刪除工作表",
		DoFn: func(wb *model.Workbook) command.Effect {
			names = d.saveNames()
			// Names scoped to the sheet go with it.
			wb.Names = slices.DeleteFunc(wb.Names, func(n *model.DefinedName) bool { return n.Scope == sh })
			d.adjustNames(func(n ast.Node, _ string) (ast.Node, bool) { return ast.RemoveSheet(n, sh.Name) })
			wb.Sheets = slices.Delete(wb.Sheets, index, index+1)
			eff := structural()
			eff.Names = true
			eff.Others = applyEdits(wb, edits, false)
			return eff
		},
		UndoFn: func(wb *model.Workbook) command.Effect {
			wb.Sheets = slices.Insert(wb.Sheets, index, sh)
			names.restore(wb)
			eff := structural()
			eff.Names = true
			eff.Others = applyEdits(wb, edits, true)
			return eff
		},
	}, nil
}

// RenameSheetCommand renames a sheet and updates formulas referring to it.
func (d *Document) RenameSheetCommand(index int, name string) (command.Command, error) {
	if index < 0 || index >= len(d.wb.Sheets) {
		return nil, errors.New("工作表不存在")
	}
	if err := d.ValidateSheetName(name, index); err != nil {
		return nil, err
	}
	sh := d.wb.Sheets[index]
	old := sh.Name
	if old == name {
		return nil, nil
	}
	edits := d.rewriteFormulas(nil, func(n ast.Node) (ast.Node, bool) { return ast.RenameSheet(n, old, name) })
	var names nameState
	return &command.Func{
		Name: "重新命名工作表",
		DoFn: func(wb *model.Workbook) command.Effect {
			names = d.saveNames()
			d.adjustNames(func(n ast.Node, _ string) (ast.Node, bool) { return ast.RenameSheet(n, old, name) })
			sh.Name = name
			eff := structural()
			eff.Names = len(wb.Names) > 0
			eff.Others = applyEdits(wb, edits, false)
			return eff
		},
		UndoFn: func(wb *model.Workbook) command.Effect {
			sh.Name = old
			names.restore(wb)
			eff := structural()
			eff.Names = len(wb.Names) > 0
			eff.Others = applyEdits(wb, edits, true)
			return eff
		},
	}, nil
}

// MoveSheetCommand moves the sheet at from to position to.
func (d *Document) MoveSheetCommand(from, to int) (command.Command, error) {
	n := len(d.wb.Sheets)
	if from < 0 || from >= n || to < 0 || to >= n {
		return nil, errors.New("工作表不存在")
	}
	if from == to {
		return nil, nil
	}
	move := func(wb *model.Workbook, a, b int) {
		sh := wb.Sheets[a]
		wb.Sheets = slices.Delete(wb.Sheets, a, a+1)
		wb.Sheets = slices.Insert(wb.Sheets, b, sh)
	}
	return &command.Func{
		Name:   "移動工作表",
		DoFn:   func(wb *model.Workbook) command.Effect { move(wb, from, to); return structural() },
		UndoFn: func(wb *model.Workbook) command.Effect { move(wb, to, from); return structural() },
	}, nil
}
