package xlsxfile

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"path"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"ww-excel/internal/formula/parser"

	"github.com/xuri/excelize/v2"
	"ww-excel/internal/formula/ast"
	"ww-excel/internal/model"
	"ww-excel/internal/numfmt"
)

func (r *reader) readPivots(sh *model.Sheet, part string) {
	// Excelize reads the field layout; inspect raw XML as well so advanced
	// calculations never silently become an ordinary sum during refresh.
	options, err := r.f.GetPivotTables(sh.Name)
	if err != nil {
		return
	}
	for _, o := range options {
		cut := strings.LastIndex(o.PivotTableRange, "!")
		if cut < 0 {
			continue
		}
		ranges := parseSqref(o.PivotTableRange[cut+1:])
		if len(ranges) != 1 {
			continue
		}
		p := model.Pivot{NoBands: !o.ShowRowStripes, Name: o.Name, Source: o.DataRange, Range: ranges[0], Style: o.PivotTableStyleName, Filters: map[string][]string{}, RowTotals: o.RowGrandTotals, ColTotals: o.ColGrandTotals}
		read := func(fs []excelize.PivotTableField) []string {
			out := []string{}
			for _, f := range fs {
				out = append(out, f.Data)

			}
			return out
		}
		for _, f := range o.Rows {
			p.Subtotals = p.Subtotals || f.DefaultSubtotal
		}
		p.Rows = read(o.Rows)
		p.Columns = read(o.Columns)
		p.Pages = read(o.Filter)
		for _, v := range o.Data {
			fn := strings.ToLower(v.Subtotal)
			if fn == "" {
				fn = "sum"
			}
			if fn != "sum" && fn != "average" && fn != "count" || v.ShowValuesAs.Type != excelize.PivotTableShowValuesAsNoCalculation {
				p.Unsupported = "此樞紐分析表包含尚未支援的計算，保留原檔結果，無法重新整理"
			}
			p.Values = append(p.Values, model.PivotValue{Field: v.Data, Function: fn, Caption: v.Name})
		}
		var rels struct {
			Items []struct {
				ID   string `xml:"Id,attr"`
				Type string `xml:"Type,attr"`
			} `xml:"Relationship"`
		}
		i := strings.LastIndex(part, "/")
		decodePart(r.parts, part[:i+1]+"_rels/"+part[i+1:]+".rels", &rels)
		firstData := 1
		for _, rel := range rels.Items {
			if !strings.HasSuffix(rel.Type, "/pivotTable") {
				continue
			}
			pt := relTarget(r.parts, part, rel.ID)
			var raw struct {
				RowTotals *bool `xml:"rowGrandTotals,attr"`
				ColTotals *bool `xml:"colGrandTotals,attr"`
				Style     struct {
					Name string `xml:"name,attr"`
				} `xml:"pivotTableStyleInfo"`
				Name     string `xml:"name,attr"`
				CacheID  int    `xml:"cacheId,attr"`
				Location struct {
					First int `xml:"firstDataRow,attr"`
				} `xml:"location"`
				RowFields []struct {
					X int `xml:"x,attr"`
				} `xml:"rowFields>field"`
				ColFields []struct {
					X int `xml:"x,attr"`
				} `xml:"colFields>field"`
				PageFields []struct {
					Fld int `xml:"fld,attr"`
				} `xml:"pageFields>pageField"`
				PivotFields []struct {
					Items []struct {
						X *int `xml:"x,attr"`
						H bool `xml:"h,attr"`
					} `xml:"items>item"`
				} `xml:"pivotFields>pivotField"`
				Filters    *struct{} `xml:"filters"`
				Calculated *struct{} `xml:"calculatedItems"`
			}
			decodePart(r.parts, pt, &raw)
			if raw.Name != p.Name {
				continue
			}
			p.RowTotals = raw.RowTotals == nil || *raw.RowTotals
			p.ColTotals = raw.ColTotals == nil || *raw.ColTotals
			p.Style = raw.Style.Name
			firstData = max(1, raw.Location.First)
			if raw.Filters != nil || raw.Calculated != nil {
				p.Unsupported = "此樞紐分析表包含尚未支援的計算，保留原檔結果，無法重新整理"
			}
			cache := relOfType(r.parts, pt, "/pivotCacheDefinition")
			var cd struct {
				Source struct {
					Worksheet struct {
						Name  string `xml:"name,attr"`
						Sheet string `xml:"sheet,attr"`
						Ref   string `xml:"ref,attr"`
					} `xml:"worksheetSource"`
				} `xml:"cacheSource"`
				Fields []struct {
					Name   string `xml:"name,attr"`
					Shared struct {
						Items []struct {
							XMLName xml.Name
							V       string `xml:"v,attr"`
						} `xml:",any"`
					} `xml:"sharedItems"`
					Group   *struct{} `xml:"fieldGroup"`
					Formula string    `xml:"formula,attr"`
				} `xml:"cacheFields>cacheField"`
			}
			decodePart(r.parts, cache, &cd)
			if ws := cd.Source.Worksheet; ws.Name != "" {
				p.Source = ws.Name
			} else if rs := parseSqref(ws.Ref); len(rs) == 1 {
				r := rs[0]
				p.Source = ast.Format(ast.Ref{Sheet: ws.Sheet, Area: ast.Area{Kind: ast.CellArea, R0: r.R0, C0: r.C0, R1: r.R1, C1: r.C1}})
			}
			ordered := func(indices []int) []string {
				out := []string{}
				for _, idx := range indices {
					if idx >= 0 && idx < len(cd.Fields) {
						out = append(out, cd.Fields[idx].Name)
					}
				}
				return out
			}
			ids := []int{}
			for _, f := range raw.RowFields {
				ids = append(ids, f.X)
			}
			p.Rows = ordered(ids)
			ids = nil
			for _, f := range raw.ColFields {
				ids = append(ids, f.X)
			}
			p.Columns = ordered(ids)
			ids = nil
			for _, f := range raw.PageFields {
				ids = append(ids, f.Fld)
			}
			p.Pages = ordered(ids)
			for i, field := range raw.PivotFields {
				if i >= len(cd.Fields) {
					break
				}
				hidden := false
				allowed := []string{}
				for _, item := range field.Items {
					hidden = hidden || item.H
					if !item.H && item.X != nil && *item.X >= 0 && *item.X < len(cd.Fields[i].Shared.Items) {
						member := cd.Fields[i].Shared.Items[*item.X]
						text := member.V
						if member.XMLName.Local == "n" {
							if number, err := strconv.ParseFloat(text, 64); err == nil {
								text = model.NumberCell(number).Display()
							}
						}
						if member.XMLName.Local == "b" {
							text = model.BoolCell(text == "1" || text == "true").Display()
						}
						allowed = append(allowed, text)
					}
				}
				if hidden {
					p.Filters[cd.Fields[i].Name] = allowed
				}
			}
			for _, f := range cd.Fields {
				if f.Group != nil || f.Formula != "" {
					p.Unsupported = "此樞紐分析表包含尚未支援的計算，保留原檔結果，無法重新整理"
				}
			}
		}
		style := func(row, col int) uint32 {
			s := sh.Get(row, col).S
			if s == 0 {
				s = sh.Layout.StyleAt(row, col)
			}
			return s
		}
		p.HeaderStyle = style(p.Range.R0, p.Range.C0)
		p.TotalStyle = style(p.Range.R1, p.Range.C0)
		p.RowStyle = style(min(p.Range.R1, p.Range.R0+firstData), min(p.Range.C1, p.Range.C0+max(1, len(p.Rows))))
		p.Appearance = r.tableAppearance(p.Style)
		for i := range p.Values {
			fmtID := o.Data[i].NumFmt
			valueStyle := style(min(p.Range.R1, p.Range.R0+firstData), min(p.Range.C1, p.Range.C0+max(1, len(p.Rows))+i))
			p.Values[i].Format = r.wb.StyleTable().Get(valueStyle).NumFmt
			if fmtID > 0 {
				if code, ok := numfmt.Builtin(fmtID); ok {
					p.Values[i].Format = code
				} else if r.f.Styles != nil && r.f.Styles.NumFmts != nil {
					for _, nf := range r.f.Styles.NumFmts.NumFmt {
						if nf.NumFmtID == fmtID {
							p.Values[i].Format = nf.FormatCode
						}
					}
				}
			}
		}

		// Report filters live above Excel's location rectangle.
		if len(p.Pages) > 0 {
			p.Range.R0 = max(0, p.Range.R0-len(p.Pages)-1)
		}
		sh.Layout.Pivots = append(sh.Layout.Pivots, p)
	}
}

// writePivots leaves unchanged imported definitions intact. Edited pivots are
// recreated from their current source, including item selections and formats.
func (w *writer) writePivots(old map[*model.Sheet]model.Layout) error {
	for _, sh := range w.wb.Sheets {
		if len(sh.Layout.Pivots) == 0 {
			continue
		}
		existing, err := w.f.GetPivotTables(sh.Name)
		if err != nil {
			return err
		}
		for _, p := range sh.Layout.Pivots {
			found := false
			for _, o := range existing {
				if o.Name == p.Name {
					found = true
				}
			}
			unchanged := false
			for _, prev := range old[sh].Pivots {
				if reflect.DeepEqual(prev, p) {
					unchanged = true
				}
			}
			if found && (unchanged || old[sh].Pivots == nil) {
				continue
			}
			if p.Unsupported != "" {
				return fmt.Errorf("無法轉存尚未支援的樞紐分析表「%s」", p.Name)
			}
			if found {
				if err := w.f.DeletePivotTable(sh.Name, p.Name); err != nil {
					return err
				}
			}
			rg := p.Range
			if len(p.Pages) > 0 {
				rg.R0 += len(p.Pages) + 1
			}
			ref := sh.Name + "!" + ast.Format(ast.Ref{Area: ast.Area{Kind: ast.CellArea, R0: rg.R0, C0: rg.C0, R1: rg.R1, C1: rg.C1}})
			o := excelize.PivotTableOptions{Name: p.Name, DataRange: p.Source, PivotTableRange: ref, RowGrandTotals: p.RowTotals, ColGrandTotals: p.ColTotals, ClassicLayout: true, ShowRowHeaders: true, ShowColHeaders: true, PivotTableStyleName: p.Style, UseAutoFormatting: true, ShowRowStripes: !p.NoBands}
			list := func(fields []string, subtotals bool) []excelize.PivotTableField {
				out := []excelize.PivotTableField{}
				for _, field := range fields {
					out = append(out, excelize.PivotTableField{Data: field, DefaultSubtotal: subtotals, SelectedItems: nil})
				}
				return out
			}
			if node, err := parser.Parse(p.Source); err == nil {
				if ref, ok := node.(ast.Ref); ok && ref.Sheet != "" {
					o.DataRange = ref.Sheet + "!" + ast.Format(ast.Ref{Area: ref.Area})
				}
			}
			o.Rows = list(p.Rows, p.Subtotals)
			o.Columns = list(p.Columns, false)
			o.Filter = list(p.Pages, false)
			for _, v := range p.Values {
				numFmt := 0
				if v.Format != "" {
					style := w.wb.StyleTable().Default()
					style.NumFmt = v.Format
					id, err := w.f.NewStyle(toExcelize(style))
					if err != nil {
						return err
					}
					st, err := w.f.GetStyle(id)
					if err != nil {
						return err
					}
					numFmt = st.NumFmt
					if st.CustomNumFmt != nil {
						if n := w.f.Styles.CellXfs.Xf[id].NumFmtID; n != nil {
							numFmt = *n
						}
					}
				}
				o.Data = append(o.Data, excelize.PivotTableField{Data: v.Field, Name: v.Caption, Subtotal: strings.ToUpper(v.Function[:1]) + v.Function[1:], NumFmt: numFmt})
			}
			if err := w.f.AddPivotTable(&o); err != nil {
				return err
			}
			if err := w.patchPivotSelections(p); err != nil {
				return err
			}
		}
	}
	return nil
}

// Build member dictionaries from the actual source sheet. Excelize's item
// builder reads the destination sheet and cannot express an empty selection.
func (w *writer) patchPivotSelections(p model.Pivot) error {
	var target string
	var data []byte
	// DeletePivotTable leaves orphan XML parts behind. Follow live worksheet
	// relationships rather than choosing a same-named part from the ZIP map.
	w.f.Relationships.Range(func(key, value any) bool {
		relPart := key.(string)
		if !strings.HasPrefix(relPart, "xl/worksheets/_rels/") {
			return true
		}
		raw, err := xml.Marshal(value)
		if err != nil {
			return true
		}
		var rels struct {
			Items []struct {
				Target string `xml:"Target,attr"`
				Type   string `xml:"Type,attr"`
			} `xml:"Relationship"`
		}
		if xml.Unmarshal(raw, &rels) != nil {
			return true
		}
		for _, rel := range rels.Items {
			if !strings.HasSuffix(rel.Type, "/pivotTable") {
				continue
			}
			name := path.Clean(path.Join("xl/worksheets", rel.Target))
			if bytes, ok := w.f.Pkg.Load(name); ok {
				raw := bytes.([]byte)
				var head struct {
					Name string `xml:"name,attr"`
				}
				if xml.Unmarshal(raw, &head) == nil && head.Name == p.Name {
					target = name
					data = raw
					return false
				}
			}
		}
		return true
	})
	if target == "" {
		return fmt.Errorf("找不到樞紐分析表的來源資料")
	}
	root, err := drawingNodes(data)
	if err != nil {
		return err
	}
	var cachePart string
	relKey := strings.Replace(target, "pivotTables/", "pivotTables/_rels/", 1) + ".rels"
	relData, hasRels := w.f.Pkg.Load(relKey)
	if rels, ok := w.f.Relationships.Load(relKey); ok {
		if data, err := xml.Marshal(rels); err == nil {
			relData = data
			hasRels = true
		}
	}
	if hasRels {
		raw := relData
		var rels struct {
			Items []struct {
				Target string `xml:"Target,attr"`
				Type   string `xml:"Type,attr"`
			} `xml:"Relationship"`
		}
		xml.Unmarshal(raw.([]byte), &rels)
		for _, rel := range rels.Items {
			if strings.HasSuffix(rel.Type, "/pivotCacheDefinition") {
				cachePart = path.Clean(path.Join(path.Dir(target), rel.Target))
			}
		}
	}
	raw, ok := w.f.Pkg.Load(cachePart)
	if !ok {
		return fmt.Errorf("找不到樞紐分析表的來源資料")
	}
	cache, err := drawingNodes(raw.([]byte))
	if err != nil {
		return err
	}
	var source *model.Sheet
	var rg model.Range
	if table, sh := w.wb.FindTable(p.Source); table != nil {
		source = sh
		rg = table.Range
		if table.TotalsRow {
			rg.R1--
		}
	} else {
		node, e := parser.Parse(p.Source)
		if e != nil {
			return e
		}
		for depth := 0; depth < 32; depth++ {
			if n, ok := node.(ast.Name); ok {
				name := w.wb.LookupName(n.Name, w.wb.Sheets[0])
				if name == nil {
					break
				}
				node, e = parser.Parse(name.Formula)
				if e != nil {
					return e
				}
				continue
			}
			if n, ok := node.(ast.Paren); ok {
				node = n.X
				continue
			}
			break
		}
		ref, ok := node.(ast.Ref)
		if !ok || ref.Area.Kind != ast.CellArea {
			return fmt.Errorf("找不到樞紐分析表的來源資料")
		}
		idx := w.wb.SheetIndex(ref.Sheet)
		if idx < 0 {
			return fmt.Errorf("找不到樞紐分析表的來源資料")
		}
		source = w.wb.Sheets[idx]
		rg = model.Range{R0: ref.Area.R0, C0: ref.Area.C0, R1: ref.Area.R1, C1: ref.Area.C1}
	}
	escape := func(s string) string { var b bytes.Buffer; xml.EscapeText(&b, []byte(s)); return b.String() }
	fields := root.all("pivotField")
	edits := []drawingEdit{}
	var cf strings.Builder
	fmt.Fprintf(&cf, `<cacheFields count="%d">`, rg.C1-rg.C0+1)
	for col := rg.C0; col <= rg.C1; col++ {
		name := source.Get(rg.R0, col).Display()
		members := []model.Cell{}
		seen := map[string]bool{}
		for row := rg.R0 + 1; row <= rg.R1; row++ {
			c := source.Get(row, col)
			key := fmt.Sprintf("%d:%s", c.Kind, c.Display())
			if !seen[key] {
				seen[key] = true
				members = append(members, c)
			}
		}
		fmt.Fprintf(&cf, `<cacheField name="%s"><sharedItems count="%d" containsBlank="1" containsMixedTypes="1">`, escape(name), len(members))
		for _, c := range members {
			switch c.Kind {
			case model.KindEmpty:
				cf.WriteString(`<m/>`)
			case model.KindNumber:
				fmt.Fprintf(&cf, `<n v="%s"/>`, strconv.FormatFloat(c.Num, 'g', -1, 64))
			case model.KindBool:
				fmt.Fprintf(&cf, `<b v="%d"/>`, boolInt(c.Num != 0))
			case model.KindError:
				fmt.Fprintf(&cf, `<e v="%s"/>`, escape(c.Display()))
			default:
				fmt.Fprintf(&cf, `<s v="%s"/>`, escape(c.Display()))
			}
		}
		cf.WriteString(`</sharedItems></cacheField>`)
		idx := col - rg.C0
		if idx >= len(fields) {
			continue
		}
		field := fields[idx]
		if field.attr("axis") == "" {
			continue
		}
		edits = append(edits, drawingAttribute(data, field, "multipleItemSelectionAllowed", "1"))
		allowed, on := p.Filters[name]
		var items strings.Builder
		total := p.Subtotals && field.attr("axis") == "axisRow"
		count := len(members)
		if total {
			count++
		}
		fmt.Fprintf(&items, `<items count="%d">`, count)
		for i, c := range members {
			hidden := 0
			if on && !slices.Contains(allowed, c.Display()) {
				hidden = 1
			}
			fmt.Fprintf(&items, `<item x="%d" h="%d"/>`, i, hidden)
		}
		if total {
			items.WriteString(`<item t="default"/>`)
		}
		items.WriteString(`</items>`)
		if node := field.child("items"); node != nil {
			edits = append(edits, drawingEdit{node.start, node.end, items.String()})
		}
	}
	cf.WriteString(`</cacheFields>`)
	nodes := cache.all("cacheFields")
	if len(nodes) != 1 {
		return fmt.Errorf("找不到樞紐分析表的來源資料")
	}
	w.f.Pkg.Store(cachePart, applyDrawingEdits(raw.([]byte), []drawingEdit{{nodes[0].start, nodes[0].end, cf.String()}}))
	if locs := root.all("location"); len(locs) == 1 {
		loc := locs[0]
		value := fmt.Sprintf(`<location ref="%s" firstHeaderRow="1" firstDataRow="%d" firstDataCol="%d"/>`, escape(loc.attr("ref")), max(1, len(p.Columns)), max(1, len(p.Rows)))
		edits = append(edits, drawingEdit{loc.start, loc.end, value})
	}
	w.f.Pkg.Store(target, applyDrawingEdits(data, edits))
	return nil
}
