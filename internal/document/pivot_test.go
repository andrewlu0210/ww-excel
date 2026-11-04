package document

import (
	"os"
	"path/filepath"
	"testing"
	"ww-excel/internal/model"
)

func pivotFixture(t *testing.T) *Document {
	t.Helper()
	d := New()
	sh := d.Workbook().Sheets[0]
	rows := [][]any{{"產品", "銷售人員", "金額"}, {"啤酒", "甲", 1400.0}, {"啤酒", "乙", 750.0}, {"紅酒", "甲", 1600.0}, {"紅酒", "乙", 1000.0}, {"汽水", "甲", 690.0}, {"汽水", "乙", 510.0}}
	for r, row := range rows {
		for c, v := range row {
			if s, ok := v.(string); ok {
				sh.Set(r, c, model.StringCell(s))
			} else {
				sh.Set(r, c, model.NumberCell(v.(float64)))
			}
		}
	}
	sh.Layout.Pivots = []model.Pivot{{Name: "SalesPivot", Source: "工作表1!A1:C7", Range: model.Range{R0: 10, C0: 4, R1: 10, C1: 4}, Rows: []string{"產品"}, Values: []model.PivotValue{{Field: "金額", Function: "sum", Caption: "加總 - 金額"}}, RowTotals: true, ColTotals: true}}
	return d
}
func refreshPivot(t *testing.T, d *Document, p model.Pivot) {
	t.Helper()
	cmd, err := d.PivotCommand(0, p)
	if err != nil {
		t.Fatal(err)
	}
	d.Exec(cmd)
}
func pivotNumber(d *Document, label string) float64 {
	sh := d.Workbook().Sheets[0]
	p := sh.Layout.Pivots[0]
	for r := p.Range.R0; r <= p.Range.R1; r++ {
		if sh.Get(r, 4).Str == label {
			return sh.Get(r, 5).Num
		}
	}
	return -1
}
func TestPivotRefreshFilterFieldsAndUndo(t *testing.T) {
	d := pivotFixture(t)
	p := d.Workbook().Sheets[0].Layout.Pivots[0]
	refreshPivot(t, d, p)
	if got := pivotNumber(d, "啤酒"); got != 2150 {
		t.Fatalf("beer=%v", got)
	}
	if got := pivotNumber(d, "總計"); got != 5950 {
		t.Fatalf("total=%v", got)
	}
	p = d.Workbook().Sheets[0].Layout.Pivots[0].Clone()
	p.Filters = map[string][]string{"產品": {"啤酒"}}
	refreshPivot(t, d, p)
	if got := pivotNumber(d, "總計"); got != 2150 {
		t.Fatalf("filtered=%v", got)
	}
	d.Workbook().Sheets[0].Set(1, 2, model.NumberCell(2000))
	refreshPivot(t, d, p)
	if got := pivotNumber(d, "啤酒"); got != 2750 {
		t.Fatalf("refreshed=%v", got)
	}
	d.Undo()
	if got := pivotNumber(d, "啤酒"); got != 2150 {
		t.Fatalf("undo=%v", got)
	}
	d.Redo()
	if got := pivotNumber(d, "啤酒"); got != 2750 {
		t.Fatalf("redo=%v", got)
	}
	p = d.Workbook().Sheets[0].Layout.Pivots[0].Clone()
	p.Filters = nil
	p.Rows = []string{"銷售人員"}
	refreshPivot(t, d, p)
	if got := pivotNumber(d, "甲"); got != 4290 {
		t.Fatalf("salesperson=%v", got)
	}
	if got := pivotNumber(d, "總計"); got != 6550 {
		t.Fatalf("total=%v", got)
	}
	for _, ext := range []string{".wwx", ".xlsx"} {
		path := filepath.Join(t.TempDir(), "pivot"+ext)
		if err := d.SaveAs(path); err != nil {
			t.Fatal(err)
		}
		back, err := Load(path)
		if err != nil {
			t.Fatal(err)
		}
		if len(back.Workbook().Sheets[0].Layout.Pivots) != 1 {
			t.Fatal("lost pivot")
		}
		refreshPivot(t, back, back.Workbook().Sheets[0].Layout.Pivots[0])
		if pivotNumber(back, "總計") != 6550 {
			t.Fatal("round trip")
		}
	}
}
func TestPivotWeightedAverageCountAndCollision(t *testing.T) {
	d := pivotFixture(t)
	sh := d.Workbook().Sheets[0]
	p := sh.Layout.Pivots[0].Clone()
	p.Values[0].Function = "average"
	p.Columns = []string{"銷售人員"}
	refreshPivot(t, d, p)
	p = sh.Layout.Pivots[0].Clone()
	total := sh.Get(p.Range.R1, p.Range.C1)
	if total.Num != 5950.0/6 {
		t.Fatalf("average total=%v", total)
	}
	p.Values[0].Function = "count"
	refreshPivot(t, d, p)
	if sh.Get(p.Range.R1, p.Range.C1).Num != 6 {
		t.Fatal("count")
	}
	p = sh.Layout.Pivots[0].Clone()
	p.Columns = nil
	p.Rows = []string{"產品", "銷售人員"}
	sh.Set(p.Range.R1+1, p.Range.C0, model.StringCell("keep"))
	if _, err := d.PivotCommand(0, p); err == nil {
		t.Fatal("overwrote adjacent data")
	}
	p.Rows = []string{"missing"}
	if _, err := d.PivotCommand(0, p); err == nil {
		t.Fatal("missing field accepted")
	}
}

func TestPivotOrderedFieldsFiltersAndEmptySelectionRoundTrip(t *testing.T) {
	for _, allowed := range [][]string{{"啤酒"}, {}} {
		d := pivotFixture(t)
		p := d.Workbook().Sheets[0].Layout.Pivots[0].Clone()
		p.Rows = []string{"銷售人員", "產品"}
		p.Subtotals = true
		p.Filters = map[string][]string{"產品": allowed}
		refreshPivot(t, d, p)
		for _, ext := range []string{".xlsx", ".wwx"} {
			path := filepath.Join(t.TempDir(), "pivot"+ext)
			if err := d.SaveAs(path); err != nil {
				t.Fatal(err)
			}
			back, err := Load(path)
			if err != nil {
				t.Fatal(err)
			}
			bp := back.Workbook().Sheets[0].Layout.Pivots[0]
			if len(bp.Rows) != 2 || bp.Rows[0] != "銷售人員" || !bp.Subtotals {
				t.Fatalf("lost layout: %+v", bp)
			}
			if selected, ok := bp.Filters["產品"]; !ok || len(selected) != len(allowed) {
				t.Fatalf("lost selection: %+v", bp.Filters)
			}
			refreshPivot(t, back, bp)
		}
	}
}
func TestPivotNamedTableExpansionAndReferenceEdits(t *testing.T) {
	d := pivotFixture(t)
	sh := d.Workbook().Sheets[0]
	cmd, name, err := d.CreateTableCommand(0, model.Range{R0: 0, C0: 0, R1: 6, C1: 2}, true, "")
	if err != nil {
		t.Fatal(err)
	}
	d.Exec(cmd)
	p := sh.Layout.Pivots[0].Clone()
	p.Source = name
	refreshPivot(t, d, p)
	sh.Set(7, 0, model.StringCell("啤酒"))
	sh.Set(7, 1, model.StringCell("甲"))
	sh.Set(7, 2, model.NumberCell(100))
	d.Exec(d.ExpandTableCommand(0, 7, 0))
	refreshPivot(t, d, sh.Layout.Pivots[0])
	if got := pivotNumber(d, "啤酒"); got != 2250 {
		t.Fatalf("table expansion=%v", got)
	}
	rename, err := d.RenameTableCommand(0, name, "SalesData")
	if err != nil {
		t.Fatal(err)
	}
	d.Exec(rename)
	if sh.Layout.Pivots[0].Source != "SalesData" {
		t.Fatal("table rename")
	}
	d.Undo()
	if sh.Layout.Pivots[0].Source != name {
		t.Fatal("table undo")
	}
	// A direct reference tracks source row insertion and sheet renaming.
	p = sh.Layout.Pivots[0].Clone()
	p.Source = "工作表1!A1:C8"
	refreshPivot(t, d, p)
	shift, err := d.ShiftCommand(0, true, 3, 1)
	if err != nil {
		t.Fatal(err)
	}
	d.Exec(shift)
	if sh.Layout.Pivots[0].Range.R0 != 11 {
		t.Fatal("destination did not move")
	}
	refreshPivot(t, d, sh.Layout.Pivots[0])
	if pivotNumber(d, "啤酒") != 2250 {
		t.Fatal("source did not expand")
	}
	d.Undo()
	d.Undo()
	rename, err = d.RenameSheetCommand(0, "Data Sheet")
	if err != nil {
		t.Fatal(err)
	}
	d.Exec(rename)
	refreshPivot(t, d, sh.Layout.Pivots[0])
	if pivotNumber(d, "啤酒") != 2250 {
		t.Fatal("sheet rename")
	}
}

func TestPivotColumnsValuesAndPagesRoundTrip(t *testing.T) {
	d := pivotFixture(t)
	sh := d.Workbook().Sheets[0]
	p := sh.Layout.Pivots[0].Clone()
	p.Rows = []string{"產品"}
	p.Pages = []string{"銷售人員"}
	p.Filters = map[string][]string{"銷售人員": {"甲"}}
	p.Values = append(p.Values, model.PivotValue{Field: "金額", Function: "count", Caption: "筆數"})
	refreshPivot(t, d, p)
	if pivotNumber(d, "啤酒") != 1400 {
		t.Fatal("page filter")
	}
	path := filepath.Join(t.TempDir(), "pivot.xlsx")
	if err := d.SaveAs(path); err != nil {
		t.Fatal(err)
	}
	back, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	bp := back.Workbook().Sheets[0].Layout.Pivots[0]
	if len(bp.Pages) != 1 || len(bp.Values) != 2 || bp.Range.R0 != 10 {
		t.Fatalf("page/value config: %+v", bp)
	}
	refreshPivot(t, back, bp)
	if pivotNumber(back, "啤酒") != 1400 {
		t.Fatal("page filter round trip")
	}
	p = sh.Layout.Pivots[0].Clone()
	p.Pages = nil
	p.Filters = nil
	p.Columns = []string{"銷售人員"}
	refreshPivot(t, d, p)
	if err := d.SaveAs(filepath.Join(t.TempDir(), "columns.xlsx")); err != nil {
		t.Fatal(err)
	}
}

func TestPivotLocalExcelFixture(t *testing.T) {
	path := filepath.Join("..", "..", "test_data", "test5.xlsx")
	if _, err := os.Stat(path); err != nil {
		t.Skip("local Excel fixture is not checked in")
	}
	d, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	idx := d.Workbook().SheetIndex("10.樞紐分析表")
	sh := d.Workbook().Sheets[idx]
	if len(sh.Layout.Pivots) != 1 {
		t.Fatalf("metadata: %+v", sh.Layout.Pivots)
	}
	p := sh.Layout.Pivots[0].Clone()
	refresh := func(p model.Pivot) {
		cmd, err := d.PivotCommand(idx, p)
		if err != nil {
			t.Fatal(err)
		}
		d.Exec(cmd)
	}
	find := func(label string) float64 {
		for r := p.Range.R0; r <= sh.Layout.Pivots[0].Range.R1; r++ {
			if sh.Get(r, p.Range.C0).Str == label {
				return sh.Get(r, p.Range.C0+1).Num
			}
		}
		return -1
	}
	refresh(p)
	if find("啤酒") != 2150 || find("總計") != 5950 {
		t.Fatalf("cached source refresh: beer %v total %v", find("啤酒"), find("總計"))
	}
	p = sh.Layout.Pivots[0].Clone()
	p.Filters = map[string][]string{"產品": {"啤酒"}}
	refresh(p)
	if find("總計") != 2150 {
		t.Fatal("filter")
	}
	src, err := d.ResolvePivotSource(idx, p.Source)
	if err != nil {
		t.Fatal(err)
	}
	source := d.Workbook().Sheets[src.Sheet]
	for r := src.Range.R0 + 1; r <= src.Range.R1; r++ {
		if source.Get(r, src.Range.C0+3).Num == 1400 {
			source.Set(r, src.Range.C0+3, model.NumberCell(2000))
		}
	}
	refresh(p)
	if find("啤酒") != 2750 {
		t.Fatalf("updated beer=%v", find("啤酒"))
	}
	out := filepath.Join(t.TempDir(), "edited.xlsx")
	if err := d.SaveAs(out); err != nil {
		t.Fatal(err)
	}
	back, err := Load(out)
	if err != nil {
		t.Fatal(err)
	}
	bp := back.Workbook().Sheets[idx].Layout.Pivots[0]
	if len(bp.Filters["產品"]) != 1 {
		t.Fatalf("lost filter=%+v", bp.Filters)
	}
	cmd, err := back.PivotCommand(idx, bp)
	if err != nil {
		t.Fatal(err)
	}
	back.Exec(cmd)
}

func TestPivotTypedGroupsAndSourceFormatting(t *testing.T) {
	d := pivotFixture(t)
	sh := d.Workbook().Sheets[0]
	sh.Set(1, 0, model.NumberCell(1))
	sh.Set(2, 0, model.StringCell("1"))
	sh.Set(3, 0, model.NumberCell(10))
	sh.Set(4, 0, model.NumberCell(2))
	st := d.Workbook().StyleTable().Default()
	st.NumFmt = "yyyy-mm-dd"
	id := d.Workbook().StyleTable().InternExplicit(st)
	c := model.NumberCell(45000)
	c.S = id
	sh.Set(5, 0, c)
	refreshPivot(t, d, sh.Layout.Pivots[0])
	p := sh.Layout.Pivots[0]
	numeric, text, date := 0, 0, false
	for r := p.Range.R0 + 1; r < p.Range.R1; r++ {
		c := sh.Get(r, p.Range.C0)
		if c.Kind == model.KindNumber && c.Num == 1 {
			numeric++
		}
		if c.Kind == model.KindString && c.Str == "1" {
			text++
		}
		if c.Num == 45000 {
			date = d.Workbook().StyleTable().Get(c.S).NumFmt == "yyyy-mm-dd"
		}
	}
	if numeric != 1 || text != 1 || !date {
		t.Fatalf("typed groups or date style lost: %d %d %v", numeric, text, date)
	}
}

func TestPivotCrossSheetSourceRenameSave(t *testing.T) {
	d := pivotFixture(t)
	source := d.Workbook().Sheets[0]
	target := model.NewSheet("Summary")
	p := source.Layout.Pivots[0]
	source.Layout.Pivots = nil
	target.Layout.Pivots = []model.Pivot{p}
	d.Workbook().Sheets = append(d.Workbook().Sheets, target)
	cmd, err := d.PivotCommand(1, p)
	if err != nil {
		t.Fatal(err)
	}
	d.Exec(cmd)
	path := filepath.Join(t.TempDir(), "cross-sheet.xlsx")
	if err := d.SaveAs(path); err != nil {
		t.Fatal(err)
	}
	cmd, err = d.RenameSheetCommand(0, "New Source")
	if err != nil {
		t.Fatal(err)
	}
	d.Exec(cmd)
	if err := d.SaveAs(path); err != nil {
		t.Fatal(err)
	}
	back, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	cmd, err = back.RefreshPivotCommand(1, p.Name)
	if err != nil {
		t.Fatal(err)
	}
	back.Exec(cmd)
	sh := back.Workbook().Sheets[1]
	bp := sh.Layout.Pivots[0]
	if sh.Get(bp.Range.R1, bp.Range.C1).Num != 5950 {
		t.Fatal("cross-sheet source lost on save")
	}
}
