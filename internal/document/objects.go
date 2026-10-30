package document

import (
	"errors"
	"fmt"
	"strings"

	"ww-excel/internal/command"
	"ww-excel/internal/formula/ast"
	"ww-excel/internal/formula/eval"
	"ww-excel/internal/formula/parser"
	"ww-excel/internal/i18n"
	"ww-excel/internal/model"
)

// AddObjectCommand adds a chart or picture; the new ID is reported
// through id (set when the command runs the first time).
func (d *Document) AddObjectCommand(sheet int, obj model.Object) (command.Command, int, error) {
	if sheet < 0 || sheet >= len(d.wb.Sheets) {
		return nil, 0, errors.New("無效的工作表")
	}
	if obj.Kind == model.ObjectChart {
		if err := d.checkChart(obj.Chart); err != nil {
			return nil, 0, err
		}
	}
	id := d.wb.Sheets[sheet].Layout.NextObjectID()
	obj.ID = id
	name := "插入圖表"
	if obj.Kind == model.ObjectImage {
		name = "插入圖片"
	}
	cmd, err := d.layoutCommand(sheet, name, nil, func(l *model.Layout) {
		l.Objects = append(l.Objects, obj)
	})
	return cmd, id, err
}

// UpdateObjectCommand replaces an object's position, size or chart.
func (d *Document) UpdateObjectCommand(sheet int, obj model.Object, label string) (command.Command, error) {
	if sheet < 0 || sheet >= len(d.wb.Sheets) || d.wb.Sheets[sheet].Layout.ObjectByID(obj.ID) == nil {
		return nil, errors.New("物件不存在")
	}
	if obj.Kind == model.ObjectChart {
		if err := d.checkChart(obj.Chart); err != nil {
			return nil, err
		}
	}
	return d.layoutCommand(sheet, label, nil, func(l *model.Layout) {
		if o := l.ObjectByID(obj.ID); o != nil {
			*o = obj
		}
	})
}

// DeleteObjectCommand removes objects by ID.
func (d *Document) DeleteObjectCommand(sheet int, ids []int) (command.Command, error) {
	if sheet < 0 || sheet >= len(d.wb.Sheets) {
		return nil, errors.New("無效的工作表")
	}
	drop := map[int]bool{}
	for _, id := range ids {
		drop[id] = true
	}
	return d.layoutCommand(sheet, "刪除物件", nil, func(l *model.Layout) {
		var keep []model.Object
		for _, o := range l.Objects {
			if !drop[o.ID] {
				keep = append(keep, o)
			}
		}
		l.Objects = keep
	})
}

func (d *Document) checkChart(c *model.Chart) error {
	if c == nil {
		return errors.New("缺少圖表設定")
	}
	switch c.Type {
	case "column", "bar", "line", "pie", "doughnut", "scatter", "area":
	default:
		return fmt.Errorf("不支援的圖表類型「%s」", c.Type)
	}
	if len(c.Series) == 0 {
		return errors.New("圖表至少要有一個數列")
	}
	for i := range c.Series {
		s := &c.Series[i]
		for _, f := range []*string{&s.Name, &s.Categories, &s.Values} {
			*f = strings.TrimPrefix(strings.TrimSpace(*f), "=")
			if *f == "" || strings.HasPrefix(*f, `"`) {
				continue
			}
			if _, err := parser.Parse(*f); err != nil {
				return fmt.Errorf("數列參照有誤：%v", err)
			}
		}
		if s.Values == "" {
			return errors.New("每個數列都要有值的範圍")
		}
	}
	return nil
}

// ChartData is a chart's series evaluated for drawing.
type ChartData struct {
	Categories []string
	Series     []SeriesData
}

type SeriesData struct {
	Name string
	// Values: nil entries are blanks or text (gaps in the chart).
	Values []*float64
	// X values for scatter charts.
	X []*float64
}

// ChartData evaluates the references of a chart.
func (d *Document) ChartData(sheet int, c *model.Chart, display DisplayFunc) ChartData {
	var out ChartData
	for i, s := range c.Series {
		sd := SeriesData{Name: fmt.Sprintf(i18n.T("數列%d"), i+1)}
		if s.Name != "" {
			if names := d.chartTexts(sheet, s.Name, display); len(names) > 0 {
				sd.Name = strings.Join(names, " ")
			}
		}
		sd.Values = d.chartNumbers(sheet, s.Values)
		if c.Type == "scatter" && s.Categories != "" {
			sd.X = d.chartNumbers(sheet, s.Categories)
		}
		if i == 0 && s.Categories != "" {
			out.Categories = d.chartTexts(sheet, s.Categories, display)
		} else if i == 0 && s.CategoryLabels != nil {
			out.Categories = append([]string{}, s.CategoryLabels...)
		}
		out.Series = append(out.Series, sd)
	}
	n := 0
	for _, s := range out.Series {
		n = max(n, len(s.Values))
	}
	for len(out.Categories) < n {
		label := fmt.Sprint(len(out.Categories) + 1)
		if len(c.Series) > 0 && c.Series[0].Categories == "" && c.Series[0].CategoryLabels != nil {
			label = ""
		}
		out.Categories = append(out.Categories, label)
	}
	return out
}

// chartCells visits the cells of a reference formula in order.
func (d *Document) chartCells(sheet int, formula string, fn func(sh *model.Sheet, row, col int, c model.Cell)) bool {
	if strings.HasPrefix(formula, `"`) {
		return false
	}
	node, err := parser.Parse(formula)
	if err != nil {
		return false
	}
	v := d.calc.ReferenceAt(node, sheet, 0, 0)
	if v.Kind != eval.KRef {
		return false
	}
	r := v.Ref
	if r.Sheet < 0 || r.Sheet >= len(d.wb.Sheets) {
		return false
	}
	sh := d.wb.Sheets[r.Sheet]
	rg := r.Range()
	if rows, cols := sh.UsedRange(); rg.R1-rg.R0 > 10000 || rg.C1-rg.C0 > 10000 {
		rg.R1, rg.C1 = min(rg.R1, max(rg.R0, rows-1)), min(rg.C1, max(rg.C0, cols-1))
	}
	for row := rg.R0; row <= rg.R1; row++ {
		for col := rg.C0; col <= rg.C1; col++ {
			fn(sh, row, col, sh.Get(row, col))
		}
	}
	return true
}

func (d *Document) chartNumbers(sheet int, formula string) []*float64 {
	var out []*float64
	d.chartCells(sheet, formula, func(_ *model.Sheet, _, _ int, c model.Cell) {
		if c.Kind == model.KindNumber {
			v := c.Num
			out = append(out, &v)
		} else {
			out = append(out, nil)
		}
	})
	return out
}

func (d *Document) chartTexts(sheet int, formula string, display DisplayFunc) []string {
	if strings.HasPrefix(formula, `"`) {
		return []string{strings.Trim(formula, `"`)}
	}
	var out []string
	d.chartCells(sheet, formula, func(sh *model.Sheet, row, col int, c model.Cell) {
		if c.HasContent() {
			out = append(out, display(sh, row, col, c))
		} else {
			out = append(out, "")
		}
	})
	return out
}

// ChartFromRange builds a chart over a data range like Excel: a text
// first row holds series names and a text first column holds category
// labels; series run down columns unless the range is wider than tall.
func (d *Document) ChartFromRange(sheet int, rg model.Range, typ string) (*model.Chart, error) {
	if sheet < 0 || sheet >= len(d.wb.Sheets) {
		return nil, errors.New("無效的工作表")
	}
	sh := d.wb.Sheets[sheet]
	rg = rg.Clamp()
	if rows, cols := sh.UsedRange(); rg.R1 >= rows || rg.C1 >= cols {
		rg.R1, rg.C1 = min(rg.R1, max(rg.R0, rows-1)), min(rg.C1, max(rg.C0, cols-1))
	}
	isText := func(r, c int) bool {
		cell := sh.Get(r, c)
		return !cell.HasContent() || cell.Kind == model.KindString
	}
	// Header row: every cell after the first is text.
	headerRow := rg.R1 > rg.R0
	for c := rg.C0 + 1; c <= rg.C1 && headerRow; c++ {
		headerRow = isText(rg.R0, c)
	}
	// Label column: the first column below the header is text (or
	// dates are left as values).
	labelCol := rg.C1 > rg.C0
	for r := rg.R0; r <= rg.R1 && labelCol; r++ {
		if r == rg.R0 && headerRow {
			continue
		}
		labelCol = isText(r, rg.C0)
	}
	if rg.C1-rg.C0 > 200 || rg.R1-rg.R0 > 100000 {
		return nil, errors.New("資料範圍太大")
	}
	ref := func(r0, c0, r1, c1 int) string {
		a := ast.Area{Kind: ast.CellArea, R0: r0, C0: c0, R1: r1, C1: c1, AbsR0: true, AbsC0: true, AbsR1: true, AbsC1: true}
		return ast.Format(ast.Ref{Sheet: sh.Name, Area: a})
	}
	data := rg
	if headerRow {
		data.R0++
	}
	if labelCol {
		data.C0++
	}
	if data.R0 > data.R1 || data.C0 > data.C1 {
		return nil, errors.New("請選取含有數字的資料範圍")
	}
	byRow := data.C1-data.C0 > data.R1-data.R0 && typ != "scatter"
	chart := &model.Chart{Type: typ, Legend: "right"}
	if byRow {
		for r := data.R0; r <= data.R1; r++ {
			s := model.ChartSeries{Values: ref(r, data.C0, r, data.C1)}
			if labelCol {
				s.Name = ref(r, rg.C0, r, rg.C0)
			}
			if headerRow {
				s.Categories = ref(rg.R0, data.C0, rg.R0, data.C1)
			}
			chart.Series = append(chart.Series, s)
		}
	} else {
		for c := data.C0; c <= data.C1; c++ {
			s := model.ChartSeries{Values: ref(data.R0, c, data.R1, c)}
			if headerRow {
				s.Name = ref(rg.R0, c, rg.R0, c)
			}
			if labelCol {
				s.Categories = ref(data.R0, rg.C0, data.R1, rg.C0)
			}
			chart.Series = append(chart.Series, s)
		}
	}
	if typ == "scatter" && !labelCol && len(chart.Series) > 1 {
		// Scatter: the first numeric column is X.
		x := chart.Series[0].Values
		chart.Series = chart.Series[1:]
		for i := range chart.Series {
			chart.Series[i].Categories = x
		}
	}
	if typ == "pie" || typ == "doughnut" {
		chart.Series = chart.Series[:1]
	}
	if len(chart.Series) == 1 && chart.Series[0].Name != "" {
		chart.Title = strings.Join(d.chartTexts(sheet, chart.Series[0].Name, func(sh *model.Sheet, row, col int, c model.Cell) string { return c.Display() }), " ")
	}
	return chart, nil
}

// adjustObjectFormulas rewrites chart references in every sheet's
// charts (after rows or columns moved, or a sheet was renamed).
func (d *Document) adjustObjectFormulas(fn func(n ast.Node, sheet string) (ast.Node, bool)) {
	for _, sh := range d.wb.Sheets {
		for i := range sh.Layout.Objects {
			c := sh.Layout.Objects[i].Chart
			if c == nil {
				continue
			}
			for j := range c.Series {
				s := &c.Series[j]
				for _, f := range []*string{&s.Name, &s.Categories, &s.Values} {
					if *f == "" || strings.HasPrefix(*f, `"`) {
						continue
					}
					node, err := parser.Parse(*f)
					if err != nil {
						continue
					}
					if out, ok := fn(node, sh.Name); ok {
						*f = ast.Format(out)
					}
				}
			}
		}
	}
}

// PageSetupCommand replaces a sheet's print settings.
func (d *Document) PageSetupCommand(sheet int, ps model.PageSetup) (command.Command, error) {
	if ps.Scale == 0 {
		ps.Scale = 100
	}
	if ps.Scale < 10 || ps.Scale > 400 {
		return nil, errors.New("縮放比例要在 10% 到 400% 之間")
	}
	return d.layoutCommand(sheet, "版面設定", nil, func(l *model.Layout) {
		l.PageSetup = &ps
	})
}

// objectsState snapshots every sheet's objects, for undoing edits that
// rewrite chart references on other sheets.
type objectsState map[*model.Sheet][]model.Object

func (d *Document) saveObjects() objectsState {
	out := objectsState{}
	for _, sh := range d.wb.Sheets {
		if len(sh.Layout.Objects) > 0 {
			out[sh] = sh.Layout.Clone().Objects
		}
	}
	return out
}

func (s objectsState) restore(wb *model.Workbook) {
	for _, sh := range wb.Sheets {
		if objs, ok := s[sh]; ok {
			sh.Layout.Objects = objs
		}
	}
}
