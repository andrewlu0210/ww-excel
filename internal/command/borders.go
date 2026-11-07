package command

import (
	"sort"
	"ww-excel/internal/model"
)

// BorderPatch describes range edges. Nil leaves an edge untouched; an empty
// line clears it. Inside edges are written to both cells sharing the boundary.
type BorderPatch struct {
	Restyle                                    *string
	Recolor                                    *string
	Top, Bottom, Left, Right, InsideH, InsideV *model.BorderLine
}

// ApplyBorderStyle plans all cell, neighbor and inherited-style changes before
// returning one undoable command. The workbook is not mutated during planning.
func ApplyBorderStyle(wb *model.Workbook, sheet int, ranges []model.Range, name string, patch Patch, borders BorderPatch) (Command, error) {
	sh := wb.Sheets[sheet]
	lineEdit := borders.Recolor != nil || borders.Restyle != nil
	ranges = ExpandFormatRanges(ranges, sh.Layout.Merges)
	cmd, err := ApplyStyle(wb, sheet, ranges, name, patch)
	if err != nil || cmd == nil {
		return cmd, err
	}
	var cells *SetCells
	if c, ok := cmd.(*SetCells); ok {
		cells = c
	}
	layout := sh.Layout.Clone()
	layoutChanged := false
	if l, ok := cmd.(*Layout); ok {
		cells = l.Cells
		layout = l.After.Clone()
		layoutChanged = true
	}
	changes := make(map[CellRef]model.Cell, len(cells.Changes))
	for _, ch := range cells.Changes {
		changes[CellRef{ch.Row, ch.Col}] = ch.After
	}
	var base map[CellRef]model.Cell
	var baseLayout model.Layout
	styles := wb.StyleTable()
	getStyle := func(source map[CellRef]model.Cell, l model.Layout, row, col int) model.Style {
		if row < 0 {
			return styles.Get(l.ColStyles[col])
		}
		if col < 0 {
			return styles.Get(l.RowStyles[row])
		}
		c, ok := source[CellRef{row, col}]
		if !ok {
			c = sh.Get(row, col)
		}
		id := c.S
		if id == 0 {
			id = l.StyleAt(row, col)
		}
		return styles.Get(id)
	}
	set := func(row, col, side int, line model.BorderLine) {
		if lineEdit && line.Style == "" {
			return
		}
		rc := CellRef{row, col}
		c, ok := changes[rc]
		if !ok {
			c = sh.Get(row, col)
		}
		st := getStyle(changes, layout, row, col)
		*borderSide(&st.Border, side) = line
		c.S = styles.Intern(st)
		if c.S == 0 && layout.StyleAt(row, col) != 0 {
			c.S = styles.InternExplicit(st)
		}
		changes[rc] = c
	}
	// A strip can be a complete row/column. Keep it sparse and preserve explicit
	// styles at intersections instead of materializing a million empty cells.
	paint := func(rg model.Range, side int, value func(int, int) model.BorderLine) error {
		if rg.R0 < 0 || rg.C0 < 0 || rg.R1 >= model.MaxRows || rg.C1 >= model.MaxCols || rg.R0 > rg.R1 || rg.C0 > rg.C1 {
			return nil
		}
		fullCols := rg.R0 == 0 && rg.R1 == model.MaxRows-1
		fullRows := rg.C0 == 0 && rg.C1 == model.MaxCols-1
		positions := map[CellRef]bool{}
		if fullCols || fullRows {
			sh.EachInRange(rg, func(r, c int, _ model.Cell) { positions[CellRef{r, c}] = true })
			for rc := range changes {
				if rg.Contains(rc.Row, rc.Col) {
					positions[rc] = true
				}
			}
			if fullCols {
				if len(layout.RowStyles)*(rg.C1-rg.C0+1) > MaxFillCells {
					return ErrFillTooLarge
				}
				for r := range layout.RowStyles {
					for c := rg.C0; c <= rg.C1; c++ {
						positions[CellRef{r, c}] = true
					}
				}
			} else {
				if len(layout.ColStyles)*(rg.R1-rg.R0+1) > MaxFillCells {
					return ErrFillTooLarge
				}
				for c := range layout.ColStyles {
					for r := rg.R0; r <= rg.R1; r++ {
						positions[CellRef{r, c}] = true
					}
				}
			}
		} else {
			if rg.CellCount() > MaxFillCells {
				return ErrFillTooLarge
			}
			for r := rg.R0; r <= rg.R1; r++ {
				for c := rg.C0; c <= rg.C1; c++ {
					positions[CellRef{r, c}] = true
				}
			}
		}
		if len(positions) > MaxFillCells {
			return ErrFillTooLarge
		}
		for rc := range positions {
			if _, merged := sh.Layout.MergeAt(rc.Row, rc.Col); !merged {
				set(rc.Row, rc.Col, side, value(rc.Row, rc.Col))
			}
		}
		if fullCols {
			for c := rg.C0; c <= rg.C1; c++ {
				st := styles.Get(layout.ColStyles[c])
				line := value(-1, c)
				if lineEdit && line.Style == "" {
					continue
				}
				*borderSide(&st.Border, side) = line
				next := styles.Intern(st)
				if old, exists := layout.ColStyles[c]; exists || styles.Get(next) != styles.Get(old) {
					layout.ColStyles[c] = next
				}
			}
			layoutChanged = true
		} else if fullRows {
			for r := rg.R0; r <= rg.R1; r++ {
				st := styles.Get(layout.RowStyles[r])
				line := value(r, -1)
				if lineEdit && line.Style == "" {
					continue
				}
				*borderSide(&st.Border, side) = line
				next := styles.Intern(st)
				if old, exists := layout.RowStyles[r]; exists || styles.Get(next) != styles.Get(old) {
					layout.RowStyles[r] = next
				}
			}
			layoutChanged = true
		}
		for _, m := range sh.Layout.Merges {
			if !m.Overlaps(rg) {
				continue
			}
			edge := m
			switch side {
			case 0:
				edge.R1 = edge.R0
			case 1:
				edge.R0 = edge.R1
			case 2:
				edge.C1 = edge.C0
			case 3:
				edge.C0 = edge.C1
			}
			if edge.Overlaps(rg) {
				line := value(m.R0, m.C0)
				for r := edge.R0; r <= edge.R1; r++ {
					for c := edge.C0; c <= edge.C1; c++ {
						set(r, c, side, line)
					}
				}
				set(m.R0, m.C0, side, line)
			}
			// Inherited line changes must not add seams inside a merge.
			if fullCols || fullRows {
				if m.CellCount() > MaxFillCells {
					return ErrFillTooLarge
				}
				for r := m.R0; r <= m.R1; r++ {
					for c := m.C0; c <= m.C1; c++ {
						if (r != m.R0 || c != m.C0) && !edge.Contains(r, c) {
							set(r, c, side, model.BorderLine{})
						}
					}
				}
			}
		}
		if len(changes) > MaxFillCells {
			return ErrFillTooLarge
		}
		return nil
	}
	constant := func(line *model.BorderLine) func(int, int) model.BorderLine {
		return func(int, int) model.BorderLine { return *line }
	}
	original := func(side int) func(int, int) model.BorderLine {
		return func(r, c int) model.BorderLine {
			st := getStyle(base, baseLayout, r, c)
			return *borderSide(&st.Border, side)
		}
	}
	for _, rg := range ranges {
		if (borders.InsideH != nil && rg.R0 == 0 && rg.R1 == model.MaxRows-1) || (borders.InsideV != nil && rg.C0 == 0 && rg.C1 == model.MaxCols-1) {
			base = make(map[CellRef]model.Cell, len(changes))
			for rc, c := range changes {
				base[rc] = c
			}
			baseLayout = layout.Clone()
		}
		// Paint interior seams before the outside edges. Complete columns/rows use
		// inherited styles, restoring just the outer boundaries to their old values.
		if borders.InsideH != nil && rg.R1 > rg.R0 {
			v := constant(borders.InsideH)
			if rg.R0 == 0 && rg.R1 == model.MaxRows-1 {
				if err = paint(rg, 0, v); err != nil {
					return nil, err
				}
				if err = paint(rg, 1, v); err != nil {
					return nil, err
				}
				if err = paint(model.Range{R0: rg.R0, C0: rg.C0, R1: rg.R0, C1: rg.C1}, 0, original(0)); err != nil {
					return nil, err
				}
				if err = paint(model.Range{R0: rg.R1, C0: rg.C0, R1: rg.R1, C1: rg.C1}, 1, original(1)); err != nil {
					return nil, err
				}
			} else {
				lower, upper := rg, rg
				lower.R0++
				upper.R1--
				if err = paint(lower, 0, v); err != nil {
					return nil, err
				}
				if err = paint(upper, 1, v); err != nil {
					return nil, err
				}
			}
		}
		if borders.InsideV != nil && rg.C1 > rg.C0 {
			v := constant(borders.InsideV)
			if rg.C0 == 0 && rg.C1 == model.MaxCols-1 {
				if err = paint(rg, 2, v); err != nil {
					return nil, err
				}
				if err = paint(rg, 3, v); err != nil {
					return nil, err
				}
				if err = paint(model.Range{R0: rg.R0, C0: rg.C0, R1: rg.R1, C1: rg.C0}, 2, original(2)); err != nil {
					return nil, err
				}
				if err = paint(model.Range{R0: rg.R0, C0: rg.C1, R1: rg.R1, C1: rg.C1}, 3, original(3)); err != nil {
					return nil, err
				}
			} else {
				left, right := rg, rg
				left.C1--
				right.C0++
				if err = paint(left, 3, v); err != nil {
					return nil, err
				}
				if err = paint(right, 2, v); err != nil {
					return nil, err
				}
			}
		}
		for side, line := range []*model.BorderLine{borders.Top, borders.Bottom, borders.Left, borders.Right} {
			if line == nil && !lineEdit {
				continue
			}
			edge, neighbor := rg, rg
			switch side {
			case 0:
				edge.R1 = edge.R0
				neighbor = edge
				neighbor.R0--
				neighbor.R1--
			case 1:
				edge.R0 = edge.R1
				neighbor = edge
				neighbor.R0++
				neighbor.R1++
			case 2:
				edge.C1 = edge.C0
				neighbor = edge
				neighbor.C0--
				neighbor.C1--
			case 3:
				edge.C0 = edge.C1
				neighbor = edge
				neighbor.C0++
				neighbor.C1++
			}
			if lineEdit {
				neighborValue := func(r, c int) model.BorderLine {
					sr, sc := r, c
					switch side {
					case 0:
						sr++
					case 1:
						sr--
					case 2:
						sc++
					case 3:
						sc--
					}
					if sr >= 0 && sc >= 0 {
						if m, ok := sh.Layout.MergeAt(sr, sc); ok {
							sr, sc = m.R0, m.C0
						}
					}
					source := getStyle(changes, layout, sr, sc)
					selectedLine := *borderSide(&source.Border, side)
					if selectedLine.Style == "" {
						return model.BorderLine{}
					}
					target := getStyle(changes, layout, r, c)
					line := *borderSide(&target.Border, side^1)
					if line.Style == "" {
						line = selectedLine
					}
					if borders.Recolor != nil {
						line.Color = *borders.Recolor
					}
					if borders.Restyle != nil {
						line.Style = *borders.Restyle
					}
					return line
				}
				if err = paint(neighbor, side^1, neighborValue); err != nil {
					return nil, err
				}
				continue
			}
			if err = paint(edge, side, constant(line)); err != nil {
				return nil, err
			}
			if err = paint(neighbor, side^1, constant(line)); err != nil {
				return nil, err
			}
		}
	}
	out := &SetCells{Sheet: sheet, Name: name}
	keys := make([]CellRef, 0, len(changes))
	for rc := range changes {
		keys = append(keys, rc)
	}
	sort.Slice(keys, func(i, j int) bool { a, b := keys[i], keys[j]; return a.Row < b.Row || a.Row == b.Row && a.Col < b.Col })
	for _, rc := range keys {
		out.Changes = append(out.Changes, CellChange{Row: rc.Row, Col: rc.Col, After: changes[rc]})
	}
	// Preserve selection bounds; neighbor changes still appear in the cell diff.
	for _, rg := range ranges {
		out.Area = union(out.Area, rg)
	}
	if layoutChanged {
		return &Layout{Sheet: sheet, Name: name, After: layout, Cells: out, Area: out.Area}, nil
	}
	return out, nil
}

func borderSide(b *model.Borders, side int) *model.BorderLine {
	switch side {
	case 0:
		return &b.Top
	case 1:
		return &b.Bottom
	case 2:
		return &b.Left
	default:
		return &b.Right
	}
}

// ExpandFormatRanges includes complete merges touched by each selection.
func ExpandFormatRanges(ranges, merges []model.Range) []model.Range {
	out := make([]model.Range, len(ranges))
	for i, rg := range ranges {
		rg = rg.Clamp()
		for changed := true; changed; {
			changed = false
			for _, m := range merges {
				if rg.Overlaps(m) {
					next := model.Range{R0: min(rg.R0, m.R0), C0: min(rg.C0, m.C0), R1: max(rg.R1, m.R1), C1: max(rg.C1, m.C1)}
					if next != rg {
						rg = next
						changed = true
					}
				}
			}
		}
		out[i] = rg
	}
	return out
}
