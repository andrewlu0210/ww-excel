package document

import (
	"reflect"
	"testing"

	"ww-excel/internal/command"
	"ww-excel/internal/model"
)

func rg(r0, c0, r1, c1 int) model.Range { return model.Range{R0: r0, C0: c0, R1: r1, C1: c1} }

func exec(t *testing.T, d *Document, cmd command.Command, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
	if cmd == nil {
		t.Fatal("no command")
	}
	d.Exec(cmd)
}

func plain(sh *model.Sheet, row, col int, c model.Cell) string { return c.Display() }

func TestCopyPasteTranslatesAndTiles(t *testing.T) {
	d := New()
	enter(t, d, 0, 0, 0, "1")
	enter(t, d, 0, 0, 1, "=A1*2")
	clip, err := d.Copy(0, rg(0, 0, 0, 1), false)
	if err != nil {
		t.Fatal(err)
	}
	cmd, area, err := d.PasteCommand(clip, 0, rg(2, 0, 5, 1), PasteOptions{})
	exec(t, d, cmd, err)
	if area != rg(2, 0, 5, 1) {
		t.Fatalf("tiled area = %+v", area)
	}
	if got := editText(d, 0, 5, 1); got != "=A6*2" || display(d, 0, 5, 1) != "2" {
		t.Fatalf("translated = %q (%q)", got, display(d, 0, 5, 1))
	}
	// Values only, transposed.
	cmd, _, err = d.PasteCommand(clip, 0, rg(0, 4, 0, 4), PasteOptions{Mode: PasteValues, Transpose: true})
	exec(t, d, cmd, err)
	if editText(d, 0, 1, 4) != "2" || editText(d, 0, 0, 4) != "1" {
		t.Fatalf("transposed values: %q %q", editText(d, 0, 0, 4), editText(d, 0, 1, 4))
	}
}

func TestCutPasteMovesReferences(t *testing.T) {
	d := New()
	enter(t, d, 0, 0, 0, "5")
	enter(t, d, 0, 0, 2, "=A1+1")
	clip, _ := d.Copy(0, rg(0, 0, 0, 0), true)
	cmd, _, err := d.PasteCommand(clip, 0, rg(3, 1, 3, 1), PasteOptions{})
	exec(t, d, cmd, err)
	sh := d.Workbook().Sheets[0]
	if sh.Get(0, 0).HasContent() || display(d, 0, 3, 1) != "5" {
		t.Fatal("cell not moved")
	}
	if got := editText(d, 0, 0, 2); got != "=B4+1" || display(d, 0, 0, 2) != "6" {
		t.Fatalf("dependent = %q (%q)", got, display(d, 0, 0, 2))
	}
	d.Undo()
	if got := editText(d, 0, 0, 2); got != "=A1+1" || display(d, 0, 0, 0) != "5" || sh.Get(3, 1).HasContent() {
		t.Fatalf("undo move: %q", got)
	}
}

func TestPasteText(t *testing.T) {
	if got := ParseTSV("a\t\"x\ty\"\r\n1\t\"say \"\"hi\"\"\"\n"); !reflect.DeepEqual(got, [][]string{{"a", "x\ty"}, {"1", `say "hi"`}}) {
		t.Fatalf("ParseTSV = %q", got)
	}
	d := New()
	cmd, area, err := d.PasteTextCommand(0, "名稱\t數量\n蘋果\t12%\n香蕉\t=1+1\n", rg(1, 1, 1, 1))
	exec(t, d, cmd, err)
	if area != rg(1, 1, 3, 2) || display(d, 0, 2, 2) != "0.12" || numFmt(d, 0, 2, 2) != "0%" || display(d, 0, 3, 2) != "2" {
		t.Fatalf("pasted %+v: %q %q %q", area, display(d, 0, 2, 2), numFmt(d, 0, 2, 2), display(d, 0, 3, 2))
	}
}

func TestFillSeries(t *testing.T) {
	d := New()
	enter(t, d, 0, 0, 0, "1")
	enter(t, d, 0, 1, 0, "3")
	enter(t, d, 0, 0, 1, "星期一")
	enter(t, d, 0, 0, 2, "項目09")
	enter(t, d, 0, 0, 3, "=A1*10")
	enter(t, d, 0, 0, 4, "2026/1/31")
	must(t, d)(d.FillCommand(0, rg(0, 0, 1, 0), rg(0, 0, 4, 0), false))
	must(t, d)(d.FillCommand(0, rg(0, 1, 0, 4), rg(0, 1, 3, 4), false))
	want := map[[2]int]string{
		{4, 0}: "9", {2, 1}: "星期三", {3, 2}: "項目12", {3, 3}: "=A4*10", {1, 4}: "46054",
	}
	for p, w := range want {
		if got := editText(d, 0, p[0], p[1]); got != w && display(d, 0, p[0], p[1]) != w {
			t.Errorf("%v = %q, want %q", p, got, w)
		}
	}
	// ⌘D copies instead of continuing.
	must(t, d)(d.FillCommand(0, rg(0, 0, 0, 0), rg(0, 0, 2, 0), true))
	if display(d, 0, 2, 0) != "1" {
		t.Fatalf("copy fill = %q", display(d, 0, 2, 0))
	}
}

func must(t *testing.T, d *Document) func(command.Command, error) {
	return func(cmd command.Command, err error) { t.Helper(); exec(t, d, cmd, err) }
}

func TestSortAndRegion(t *testing.T) {
	d := New()
	rows := [][]string{{"名稱", "分數"}, {"乙", "70"}, {"甲", ""}, {"丙", "90"}, {"丁", "80"}}
	for r, row := range rows {
		for c, v := range row {
			if v != "" {
				enter(t, d, 0, r+2, c+1, v)
			}
		}
	}
	enter(t, d, 0, 3, 3, "=C4*2")
	region, header, ok := d.SortRange(0, rg(4, 2, 4, 2))
	if !ok || !header || region != rg(2, 1, 6, 3) {
		t.Fatalf("region %+v header %v", region, header)
	}
	must(t, d)(d.SortCommand(0, region, []SortKey{{Col: 2, Desc: true}}, header))
	var got []string
	for r := 3; r <= 6; r++ {
		got = append(got, display(d, 0, r, 1))
	}
	if !reflect.DeepEqual(got, []string{"丙", "丁", "乙", "甲"}) {
		t.Fatalf("sorted = %v", got)
	}
	if editText(d, 0, 5, 3) != "=C6*2" || display(d, 0, 5, 3) != "140" {
		t.Fatalf("formula moved with its row: %q", editText(d, 0, 5, 3))
	}
}

func TestFindReplace(t *testing.T) {
	d := New()
	enter(t, d, 0, 0, 0, "Apple pie")
	enter(t, d, 0, 2, 1, "apple")
	enter(t, d, 0, 3, 0, "=LEN(\"apple\")")
	o := FindOptions{Text: "apple"}
	hits := d.FindAll(0, o, plain, 0)
	if len(hits) != 2 {
		t.Fatalf("hits = %v", hits)
	}
	if h, _ := d.FindNext(0, 0, 0, o, plain, true); h.Row != 2 {
		t.Fatalf("next = %+v", h)
	}
	if h, _ := d.FindNext(0, 2, 1, o, plain, true); h.Row != 0 {
		t.Fatalf("wraps = %+v", h)
	}
	o.InFormulas = true
	cmd, n, err := d.ReplaceCommand(d.FindAll(0, o, plain, 0), o, "pear")
	exec(t, d, cmd, err)
	if n != 3 || display(d, 0, 0, 0) != "pear pie" || display(d, 0, 3, 0) != "4" {
		t.Fatalf("replaced %d: %q %q", n, display(d, 0, 0, 0), display(d, 0, 3, 0))
	}
}

func TestFilterAndHide(t *testing.T) {
	d := New()
	for r, v := range []string{"水果", "蘋果", "香蕉", "蘋果", ""} {
		if v != "" {
			enter(t, d, 0, r, 0, v)
		}
	}
	enter(t, d, 0, 4, 1, "x")
	must(t, d)(d.ToggleFilterCommand(0, rg(1, 0, 1, 0)))
	sh := d.Workbook().Sheets[0]
	if f := sh.Layout.Filter; f == nil || f.Range != rg(0, 0, 4, 1) {
		t.Fatalf("filter = %+v", sh.Layout.Filter)
	}
	vals := d.FilterValues(0, 0, plain)
	if len(vals) != 3 || vals[2].Text != "" {
		t.Fatalf("values = %+v", vals)
	}
	must(t, d)(d.FilterCommand(0, 0, []string{"蘋果"}, plain))
	if sh.Layout.RowHeight(2) != 0 || sh.Layout.RowHeight(4) != 0 || sh.Layout.RowHeight(3) == 0 {
		t.Fatal("rows not filtered")
	}
	must(t, d)(d.ToggleFilterCommand(0, rg(0, 0, 0, 0)))
	if sh.Layout.Filter != nil || sh.Layout.RowHeight(2) == 0 {
		t.Fatal("filter not removed")
	}

	must(t, d)(d.HideCommand(0, false, 1, 2, true))
	if sh.Layout.ColWidth(1) != 0 || sh.Layout.ColWidth(3) == 0 {
		t.Fatal("columns not hidden")
	}
	must(t, d)(d.HideCommand(0, false, 0, 3, false))
	if sh.Layout.ColWidth(2) != model.DefaultColWidth || len(sh.Layout.Cols) != 0 {
		t.Fatal("columns not shown")
	}
	must(t, d)(d.FreezeCommand(0, 1, 0))
	d.Undo()
	if sh.Layout.FreezeRows != 0 {
		t.Fatal("undo freeze")
	}
}
