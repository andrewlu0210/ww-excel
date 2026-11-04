package document

import (
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"

	"ww-excel/internal/command"
	"ww-excel/internal/formula/eval"
	"ww-excel/internal/formula/parser"
	"ww-excel/internal/i18n"
	"ww-excel/internal/model"
	"ww-excel/internal/numfmt"
)

type PivotSource struct {
	Sheet  int
	Range  model.Range
	Fields []string
}

// ResolvePivotSource resolves tables dynamically, including their newly appended rows.
func (d *Document) ResolvePivotSource(sheet int, source string) (PivotSource, error) {
	var out PivotSource
	if t, sh := d.wb.FindTable(source); t != nil {
		out.Sheet = slices.Index(d.wb.Sheets, sh)
		out.Range = t.Range
		if t.TotalsRow {
			out.Range.R1--
		}
	} else {
		node, err := parser.Parse(strings.TrimPrefix(source, "="))
		if err != nil {
			return out, errors.New("樞紐分析表的來源參照無效")
		}
		v := d.calc.ReferenceAt(node, sheet, 0, 0)
		if v.Kind != eval.KRef || v.Ref.Sheet < 0 || v.Ref.Sheet >= len(d.wb.Sheets) {
			return out, errors.New("找不到樞紐分析表的來源資料")
		}
		out.Sheet = v.Ref.Sheet
		out.Range = v.Ref.Range()
	}
	r := out.Range
	if r.R0 < 0 || r.C0 < 0 || r.R1 >= model.MaxRows || r.C1 >= model.MaxCols || r.R1 < r.R0 || r.C1 < r.C0 || (r.R1-r.R0+1)*(r.C1-r.C0+1) > 2_000_000 {
		return out, errors.New("樞紐分析表的來源範圍太大或無效")
	}
	seen := map[string]bool{}
	sh := d.wb.Sheets[out.Sheet]
	for c := r.C0; c <= r.C1; c++ {
		name := sh.Get(r.R0, c).Display()
		if name == "" || seen[name] {
			return out, errors.New("樞紐分析表來源的欄位名稱必須非空白且不重複")
		}
		seen[name] = true
		out.Fields = append(out.Fields, name)
	}
	return out, nil
}
func (d *Document) PivotItems(sheet int, p model.Pivot, field string) ([]string, error) {
	src, err := d.ResolvePivotSource(sheet, p.Source)
	if err != nil {
		return nil, err
	}
	idx := slices.Index(src.Fields, field)
	if idx < 0 {
		return nil, errors.New("樞紐分析表的欄位不存在")
	}
	values := []string{}
	seen := map[string]bool{}
	sh := d.wb.Sheets[src.Sheet]
	for r := src.Range.R0 + 1; r <= src.Range.R1; r++ {
		s := sh.Get(r, src.Range.C0+idx).Display()
		if !seen[s] {
			seen[s] = true
			values = append(values, s)
		}
	}
	slices.Sort(values)
	return values, nil
}

type pivotStat struct {
	Sum            float64
	Numbers, Count int
}

func (s pivotStat) value(fn string) model.Cell {
	switch fn {
	case "count":
		return model.NumberCell(float64(s.Count))
	case "average":
		if s.Numbers == 0 {
			return model.ErrorCell("#DIV/0!")
		}
		return model.NumberCell(s.Sum / float64(s.Numbers))
	default:
		return model.NumberCell(s.Sum)
	}
}

type pivotKey struct {
	Parts  []string
	Cells  []model.Cell
	Total  bool
	Key    string
	Labels []string
}

func pivotGroup(labels []string) string { b, _ := json.Marshal(labels); return string(b) }

// PivotCommand validates and computes before creating a single atomic undo step.
func (d *Document) PivotCommand(sheet int, p model.Pivot) (command.Command, error) {
	if sheet < 0 || sheet >= len(d.wb.Sheets) {
		return nil, errors.New("無效的工作表")
	}
	sh := d.wb.Sheets[sheet]
	index := -1
	for i, v := range sh.Layout.Pivots {
		if v.Name == p.Name {
			index = i
		}
	}
	if index < 0 {
		return nil, errors.New("樞紐分析表不存在")
	}
	old := sh.Layout.Pivots[index]
	if old.Unsupported != "" {
		return nil, errors.New(old.Unsupported)
	}
	// Geometry and style belong to the existing pivot, not to client input.
	next := old.Clone()
	next.Source = p.Source
	next.Rows = append([]string{}, p.Rows...)
	next.Columns = append([]string{}, p.Columns...)
	next.Pages = append([]string{}, p.Pages...)
	next.Values = append([]model.PivotValue{}, p.Values...)
	next.Filters = p.Clone().Filters
	next.Subtotals = p.Subtotals
	next.RowTotals = p.RowTotals
	next.ColTotals = p.ColTotals
	p = next
	src, err := d.ResolvePivotSource(sheet, p.Source)
	if err != nil {
		return nil, err
	}
	if len(p.Values) == 0 || len(p.Values) > 32 || len(p.Rows)+len(p.Columns)+len(p.Pages) > 32 {
		return nil, errors.New("樞紐分析表需要至少一個值欄位，且欄位數不可超過限制")
	}
	fields := map[string]int{}
	for i, f := range src.Fields {
		fields[f] = src.Range.C0 + i
	}
	seen := map[string]bool{}
	for _, list := range [][]string{p.Rows, p.Columns, p.Pages} {
		for _, f := range list {
			if _, ok := fields[f]; !ok || seen[f] {
				return nil, errors.New("樞紐分析表的欄位不存在或重複")
			}
			seen[f] = true
		}
	}
	for f := range p.Filters {
		if !seen[f] {
			return nil, errors.New("篩選欄位必須位於列、欄或報表篩選區域")
		}
	}
	captions := map[string]bool{}
	for i, v := range p.Values {
		if _, ok := fields[v.Field]; !ok {
			return nil, errors.New("樞紐分析表的欄位不存在")
		}
		if v.Function != "sum" && v.Function != "average" && v.Function != "count" {
			return nil, errors.New("樞紐分析表只支援加總、平均值與筆數")
		}
		if v.Caption == "" {
			p.Values[i].Caption = v.Field
		}
		caption := strings.ToLower(p.Values[i].Caption)
		if captions[caption] {
			return nil, errors.New("值欄位標題不可重複")
		}
		captions[caption] = true
	}
	rows, cols := []pivotKey{}, []pivotKey{}
	rowSeen, colSeen := map[string]bool{}, map[string]bool{}
	stats := map[string][]pivotStat{}
	source := d.wb.Sheets[src.Sheet]
	add := func(rk, ck string, row int) {
		key := rk + "\x00" + ck
		a := stats[key]
		if a == nil {
			a = make([]pivotStat, len(p.Values))
		}
		for i, v := range p.Values {
			c := source.Get(row, fields[v.Field])
			if c.Kind != model.KindEmpty {
				a[i].Count++
			}
			if c.Kind == model.KindNumber {
				a[i].Sum += c.Num
				a[i].Numbers++
			}
		}
		stats[key] = a
	}
	for r := src.Range.R0 + 1; r <= src.Range.R1; r++ {
		blank := true
		for c := src.Range.C0; c <= src.Range.C1; c++ {
			if source.Get(r, c).HasContent() {
				blank = false
				break
			}
		}
		if blank {
			continue
		}
		keep := true
		for f, allowed := range p.Filters {
			if !slices.Contains(allowed, source.Get(r, fields[f]).Display()) {
				keep = false
				break
			}
		}
		if !keep {
			continue
		}
		group := func(fs []string) pivotKey {
			key := pivotKey{Labels: make([]string, len(fs)), Parts: make([]string, len(fs)), Cells: make([]model.Cell, len(fs))}
			for i, f := range fs {
				col := fields[f]
				c := source.Get(r, col)
				if c.S == 0 {
					c.S = source.Layout.StyleAt(r, col)
				}
				c.F = nil
				key.Cells[i] = c
				key.Parts[i] = fmt.Sprintf("%d:%s", c.Kind, c.Display())
				key.Labels[i] = c.Display()
				if c.Kind == model.KindNumber {
					key.Labels[i] = numfmt.Format(c.Num, d.wb.StyleTable().Get(c.S).NumFmt)
				}
			}
			key.Key = pivotGroup(key.Parts)
			return key
		}
		rowKey, colKey := group(p.Rows), group(p.Columns)
		rk, ck := rowKey.Key, colKey.Key
		if !rowSeen[rk] {
			rowSeen[rk] = true
			rows = append(rows, rowKey)
		}
		if !colSeen[ck] {
			colSeen[ck] = true
			cols = append(cols, colKey)
		}
		if p.Subtotals {
			for level := 1; level < len(rowKey.Parts); level++ {
				prefix := pivotGroup(rowKey.Parts[:level])
				add(prefix, ck, r)
				add(prefix, "*", r)
			}
		}

		add(rk, ck, r)
		add(rk, "*", r)
		add("*", ck, r)
		add("*", "*", r)
	}
	sortKeys := func(keys []pivotKey) {
		slices.SortFunc(keys, func(a, b pivotKey) int {
			for i := range a.Labels {
				if a.Cells[i].Kind == model.KindNumber && b.Cells[i].Kind == model.KindNumber {
					if a.Cells[i].Num < b.Cells[i].Num {
						return -1
					}
					if a.Cells[i].Num > b.Cells[i].Num {
						return 1
					}
				}
				if c := strings.Compare(a.Labels[i], b.Labels[i]); c != 0 {
					return c
				}
			}
			return 0
		})
	}
	sortKeys(rows)
	sortKeys(cols)
	if p.Subtotals && len(p.Rows) > 1 {
		expanded := []pivotKey{}
		for i, rk := range rows {
			expanded = append(expanded, rk)
			for level := len(p.Rows) - 1; level >= 1; level-- {
				if i+1 == len(rows) || !slices.Equal(rk.Parts[:level], rows[i+1].Parts[:level]) {
					labels := append([]string{}, rk.Labels[:level]...)
					labels[level-1] += " " + i18n.T("小計")
					expanded = append(expanded, pivotKey{Key: pivotGroup(rk.Parts[:level]), Labels: labels, Total: true})
				}
			}
		}
		rows = expanded
	}
	if len(p.Rows) == 0 {
		rows = []pivotKey{{Key: "[]"}}
	}
	if len(p.Columns) == 0 {
		cols = []pivotKey{{Key: "[]"}}
	}
	rowCols := max(1, len(p.Rows))
	headRows := max(1, len(p.Columns))
	pageRows := 0
	if len(p.Pages) > 0 {
		pageRows = len(p.Pages) + 1
	}
	ncols := rowCols + len(cols)*len(p.Values)
	if len(p.Columns) > 0 && p.RowTotals {
		ncols += len(p.Values)
	}
	nrows := pageRows + headRows + len(rows)
	if len(p.Rows) > 0 && p.ColTotals {
		nrows++
	}
	rg := model.Range{R0: old.Range.R0, C0: old.Range.C0, R1: old.Range.R0 + nrows - 1, C1: old.Range.C0 + ncols - 1}
	if rg.R0 < 0 || rg.C0 < 0 || rg.R1 >= model.MaxRows || rg.C1 >= model.MaxCols || nrows*ncols > 500_000 {
		return nil, errors.New("樞紐分析表的結果範圍太大")
	}
	if src.Sheet == sheet && rg.Overlaps(src.Range) {
		return nil, errors.New("樞紐分析表不能覆蓋來源資料")
	}
	for i, v := range sh.Layout.Pivots {
		if i != index && v.Range.Overlaps(rg) {
			return nil, errors.New("樞紐分析表不能與其他樞紐分析表重疊")
		}
	}
	for _, t := range sh.Layout.Tables {
		if t.Range.Overlaps(rg) {
			return nil, errors.New("樞紐分析表不能與表格重疊")
		}
	}
	for _, m := range sh.Layout.Merges {
		if m.Overlaps(rg) {
			return nil, errors.New("樞紐分析表中不能有合併儲存格")
		}
	}
	for r := rg.R0; r <= rg.R1; r++ {
		for c := rg.C0; c <= rg.C1; c++ {
			if !old.Range.Contains(r, c) && sh.Get(r, c).HasContent() {
				return nil, errors.New("樞紐分析表的結果會覆蓋其他資料")
			}
		}
	}
	cells := map[[2]int]model.Cell{}
	put := func(r, c int, cell model.Cell) { cells[[2]int{rg.R0 + r, rg.C0 + c}] = cell }
	text := func(s string, style uint32) model.Cell { c := model.StringCell(s); c.S = style; return c }
	label := func(s string) string {
		if s == "" {
			return i18n.T("（空白）")
		}
		return s
	}
	for i, f := range p.Pages {
		put(i, 0, text(f, p.HeaderStyle))
		v := i18n.T("（全部）")
		if a, ok := p.Filters[f]; ok {
			v = strings.Join(a, ", ")
		}
		put(i, 1, text(v, p.RowStyle))
	}
	for i, f := range p.Rows {
		put(pageRows, i, text(f, p.HeaderStyle))
	}
	if len(p.Rows) == 0 {
		put(pageRows, 0, text(i18n.T("值"), p.HeaderStyle))
	}
	for ci, ck := range cols {
		for vi, v := range p.Values {
			for h := 0; h < headRows; h++ {
				caption := v.Caption
				if len(p.Columns) > 0 {
					caption = label(ck.Labels[h])
					if h == headRows-1 {
						caption += " / " + v.Caption
					}
				}
				put(pageRows+h, rowCols+ci*len(p.Values)+vi, text(caption, p.HeaderStyle))
			}
		}
	}
	if len(p.Columns) > 0 && p.RowTotals {
		for vi, v := range p.Values {
			put(pageRows, rowCols+len(cols)*len(p.Values)+vi, text(i18n.T("總計")+" / "+v.Caption, p.HeaderStyle))
		}
	}
	valueStyle := func(v model.PivotValue, total bool, row int) uint32 {
		id := p.RowStyle
		if total {
			id = p.TotalStyle
		}
		st := d.wb.StyleTable().Get(id)
		if !total && p.Appearance != nil {
			st.Font = p.Appearance.Whole.Resolve(st.Font)
			tr := rg
			tr.R0 += pageRows + headRows - 1
			if fill := p.Appearance.FillAt(model.Table{Range: tr, NoBands: p.NoBands}, rg.R0+row, ""); fill != "" {
				st.Fill = fill
			}
		}
		if v.Format != "" {
			st.NumFmt = v.Format
		}
		return d.wb.StyleTable().InternExplicit(st)
	}
	writeValues := func(row int, rk string, total bool) {
		keys := append([]pivotKey{}, cols...)
		if len(p.Columns) > 0 && p.RowTotals {
			keys = append(keys, pivotKey{Key: "*"})
		}
		for ci, ck := range keys {
			a := stats[rk+"\x00"+ck.Key]
			for vi, v := range p.Values {
				cell := model.Cell{}
				if a != nil {
					cell = a[vi].value(v.Function)
				} else if total || ck.Key == "*" || len(p.Rows) == 0 {
					cell = pivotStat{}.value(v.Function)
				}
				cell.S = valueStyle(v, total || ck.Key == "*", row)
				put(row, rowCols+ci*len(p.Values)+vi, cell)
			}
		}
	}
	for ri, rk := range rows {
		row := pageRows + headRows + ri
		for i, s := range rk.Labels {
			cell := text(label(s), p.RowStyle)
			if rk.Total {
				cell.S = p.TotalStyle
			} else if i < len(rk.Cells) && rk.Cells[i].Kind != model.KindEmpty {
				cell = rk.Cells[i]
			}
			put(row, i, cell)
		}
		writeValues(row, rk.Key, rk.Total)
	}
	if len(p.Rows) > 0 && p.ColTotals {
		row := nrows - 1
		put(row, 0, text(i18n.T("總計"), p.TotalStyle))
		writeValues(row, "*", true)
	}
	changes := []command.CellChange{}
	// Clear only cells previously owned by this pivot, leaving neighboring content intact.
	for ai, area := range []model.Range{old.Range, rg} {
		if (area.R1-area.R0+1)*(area.C1-area.C0+1) > 500_000 {
			return nil, errors.New("樞紐分析表的結果範圍太大")
		}
		for r := area.R0; r <= area.R1; r++ {
			for c := area.C0; c <= area.C1; c++ {
				if ai == 1 && old.Range.Contains(r, c) {
					continue
				}
				changes = append(changes, command.CellChange{Row: r, Col: c, After: cells[[2]int{r, c}]})
			}
		}
	}
	p.Range = rg
	after := sh.Layout.Clone()
	after.Pivots[index] = p
	after.PivotHeaders = nil
	for _, pv := range after.Pivots {
		base := pv.Range.R0
		if len(pv.Pages) > 0 {
			base += len(pv.Pages) + 1
		}
		if len(pv.Rows) > 0 {
			after.PivotHeaders = append(after.PivotHeaders, model.Range{R0: base, C0: pv.Range.C0, R1: base, C1: pv.Range.C0 + len(pv.Rows) - 1})
		}
		for j := range pv.Pages {
			after.PivotHeaders = append(after.PivotHeaders, model.Range{R0: pv.Range.R0 + j, C0: pv.Range.C0 + 1, R1: pv.Range.R0 + j, C1: pv.Range.C0 + 1})
		}
	}
	return &command.Layout{Sheet: sheet, Name: "更新樞紐分析表", After: after, Cells: &command.SetCells{Sheet: sheet, Changes: changes}}, nil
}

func (d *Document) RefreshPivotCommand(sheet int, name string) (command.Command, error) {
	if sheet >= 0 && sheet < len(d.wb.Sheets) {
		for _, p := range d.wb.Sheets[sheet].Layout.Pivots {
			if p.Name == name {
				return d.PivotCommand(sheet, p)
			}
		}
	}
	return nil, fmt.Errorf("樞紐分析表「%s」不存在", name)
}
