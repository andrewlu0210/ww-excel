package document

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"ww-excel/internal/command"
	"ww-excel/internal/formula/ast"
	"ww-excel/internal/formula/parser"
	"ww-excel/internal/model"
)

// nameState is a snapshot of the defined names, for undo. Pointers stay
// the same so other snapshots referring to them remain valid.
type nameState struct {
	list []*model.DefinedName
	vals []model.DefinedName
}

func (d *Document) saveNames() nameState {
	s := nameState{list: slices.Clone(d.wb.Names)}
	for _, n := range d.wb.Names {
		s.vals = append(s.vals, *n)
	}
	return s
}

func (s nameState) restore(wb *model.Workbook) {
	wb.Names = slices.Clone(s.list)
	for i, n := range s.list {
		*n = s.vals[i]
	}
}

// adjustNames rewrites the formulas of all names; fn gets the parsed
// formula and the name of the sheet relative references belong to.
func (d *Document) adjustNames(fn func(n ast.Node, sheet string) (ast.Node, bool)) bool {
	changed := false
	for _, n := range d.wb.Names {
		node, err := parser.Parse(n.Formula)
		if err != nil {
			continue
		}
		sheet := ""
		if n.Scope != nil {
			sheet = n.Scope.Name
		}
		if out, ok := fn(node, sheet); ok {
			n.Formula = ast.Format(out)
			changed = true
		}
	}
	return changed
}

// NameInput is a name as entered in 定義名稱.
type NameInput struct {
	Name     string
	Scope    int // sheet index, or -1 for the workbook
	RefersTo string
	Comment  string
}

// SetNameCommand adds a name (old == nil) or changes one.
func (d *Document) SetNameCommand(old *model.DefinedName, in NameInput) (command.Command, error) {
	name := strings.TrimSpace(in.Name)
	if !model.ValidName(name) {
		return nil, fmt.Errorf("「%s」不是有效的名稱：名稱要以字母或底線開頭，不能有空格，也不能和儲存格位址相同", name)
	}
	var scope *model.Sheet
	if in.Scope >= 0 {
		if in.Scope >= len(d.wb.Sheets) {
			return nil, errors.New("工作表不存在")
		}
		scope = d.wb.Sheets[in.Scope]
	}
	for _, n := range d.wb.Names {
		if n != old && n.Scope == scope && strings.EqualFold(n.Name, name) {
			return nil, fmt.Errorf("已經有名為「%s」的名稱", name)
		}
	}
	text := strings.TrimPrefix(strings.TrimSpace(in.RefersTo), "=")
	node, err := parser.Parse(text)
	if err != nil {
		return nil, fmt.Errorf("參照位置有誤：%v", err)
	}
	next := model.DefinedName{Name: name, Scope: scope, Formula: ast.Format(node), Comment: in.Comment}
	label := "定義名稱"
	if old != nil {
		label = "編輯名稱"
	}
	var before nameState
	return &command.Func{
		Name: label,
		DoFn: func(wb *model.Workbook) command.Effect {
			before = d.saveNames()
			if old != nil {
				*old = next
			} else {
				n := next
				wb.Names = append(wb.Names, &n)
			}
			return command.Effect{Names: true}
		},
		UndoFn: func(wb *model.Workbook) command.Effect {
			before.restore(wb)
			return command.Effect{Names: true}
		},
	}, nil
}

// DeleteNameCommand removes a name; formulas using it show #NAME?.
func (d *Document) DeleteNameCommand(n *model.DefinedName) (command.Command, error) {
	if !slices.Contains(d.wb.Names, n) {
		return nil, errors.New("名稱不存在")
	}
	var before nameState
	return &command.Func{
		Name: "刪除名稱",
		DoFn: func(wb *model.Workbook) command.Effect {
			before = d.saveNames()
			wb.Names = slices.DeleteFunc(wb.Names, func(x *model.DefinedName) bool { return x == n })
			return command.Effect{Names: true}
		},
		UndoFn: func(wb *model.Workbook) command.Effect {
			before.restore(wb)
			return command.Effect{Names: true}
		},
	}, nil
}

// FindName returns the name with this name and scope (-1 = workbook).
func (d *Document) FindName(name string, scope int) *model.DefinedName {
	var sh *model.Sheet
	if scope >= 0 && scope < len(d.wb.Sheets) {
		sh = d.wb.Sheets[scope]
	}
	for _, n := range d.wb.Names {
		if n.Scope == sh && strings.EqualFold(n.Name, name) {
			return n
		}
	}
	return nil
}

// Target is where a name or address typed in the name box points.
type Target struct {
	Sheet int
	Range model.Range
}

// Resolve interprets name-box text on sheet: an address (A1, B2:C5,
// 工作表2!A1, A:A) or a defined name that refers to a range.
func (d *Document) Resolve(sheet int, text string) (Target, bool) {
	node, err := parser.Parse(strings.TrimSpace(text))
	if err != nil {
		return Target{}, false
	}
	for depth := 0; depth < 16; depth++ {
		switch x := node.(type) {
		case ast.Ref:
			s := sheet
			if x.Sheet != "" {
				if s = d.wb.SheetIndex(x.Sheet); s < 0 {
					return Target{}, false
				}
			}
			return Target{Sheet: s, Range: x.Area.Range()}, true
		case ast.Name:
			var sh *model.Sheet
			if sheet >= 0 && sheet < len(d.wb.Sheets) {
				sh = d.wb.Sheets[sheet]
			}
			n := d.wb.LookupName(x.Name, sh)
			if n == nil {
				return Target{}, false
			}
			if node, err = parser.Parse(n.Formula); err != nil {
				return Target{}, false
			}
			if n.Scope != nil {
				sheet = d.wb.SheetIndex(n.Scope.Name)
			}
		default:
			return Target{}, false
		}
	}
	return Target{}, false
}
