package document

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"path/filepath"
	"testing"

	"ww-excel/internal/model"
)

func testPNG(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for x := 0; x < w; x++ {
		img.Set(x, x%h, color.RGBA{R: 200, A: 255})
	}
	var b bytes.Buffer
	if err := png.Encode(&b, img); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

func TestChartsAndPictures(t *testing.T) {
	d := New()
	rows := [][]string{{"月份", "收入", "支出"}, {"一月", "100", "80"}, {"二月", "150", "90"}, {"三月", "120", "70"}}
	for r, row := range rows {
		for c, v := range row {
			enter(t, d, 0, r, c, v)
		}
	}
	chart, err := d.ChartFromRange(0, rg(0, 0, 3, 2), "column")
	if err != nil {
		t.Fatal(err)
	}
	if len(chart.Series) != 2 || chart.Series[0].Values != "工作表1!$B$2:$B$4" || chart.Series[0].Name != "工作表1!$B$1" || chart.Series[0].Categories != "工作表1!$A$2:$A$4" {
		t.Fatalf("chart = %+v", chart)
	}
	cmd, id, err := d.AddObjectCommand(0, model.Object{Kind: model.ObjectChart, Anchor: model.Anchor{Row: 5, Col: 1}, W: 480, H: 288, Chart: chart})
	must(t, d)(cmd, err)
	data := d.ChartData(0, chart, plain)
	if len(data.Series) != 2 || data.Series[1].Name != "支出" || *data.Series[0].Values[1] != 150 || data.Categories[2] != "三月" {
		t.Fatalf("data = %+v", data)
	}
	pic := testPNG(t, 40, 20)
	cmd, _, err = d.AddObjectCommand(0, model.Object{Kind: model.ObjectImage, Anchor: model.Anchor{Row: 1, Col: 6, DX: 4}, W: 80, H: 40, Image: &model.Image{Data: pic, Ext: ".png"}})
	must(t, d)(cmd, err)

	// Inserting rows above moves the chart and its references.
	shift(t, d, 0, true, 0, 2)
	sh := d.Workbook().Sheets[0]
	o := sh.Layout.ObjectByID(id)
	if o.Anchor.Row != 7 || o.Chart.Series[0].Values != "工作表1!$B$4:$B$6" {
		t.Fatalf("after insert: %+v %+v", o.Anchor, o.Chart.Series[0])
	}
	d.Undo()
	if o := sh.Layout.ObjectByID(id); o.Anchor.Row != 5 || o.Chart.Series[0].Values != "工作表1!$B$2:$B$4" {
		t.Fatalf("undo insert: %+v", o)
	}
	must(t, d)(d.PageSetupCommand(0, model.PageSetup{Paper: "A4", Landscape: true, Top: 1, Bottom: 1, Left: 0.5, Right: 0.5, Scale: 80, PrintArea: &model.Range{R1: 3, C1: 2}, FooterText: "第 &P 頁"}))

	for _, ext := range []string{".xlsx", ".wwx"} {
		path := filepath.Join(t.TempDir(), "objects"+ext)
		if err := d.SaveAs(path); err != nil {
			t.Fatal(err)
		}
		back, err := Load(path)
		if err != nil {
			t.Fatal(err)
		}
		l := back.Workbook().Sheets[0].Layout
		if len(l.Objects) != 2 {
			t.Fatalf("%s objects = %+v", ext, l.Objects)
		}
		var c, p *model.Object
		for i := range l.Objects {
			if l.Objects[i].Chart != nil {
				c = &l.Objects[i]
			} else {
				p = &l.Objects[i]
			}
		}
		if c == nil || p == nil {
			t.Fatalf("%s kinds: %+v", ext, l.Objects)
		}
		if c.Anchor.Row != 5 || c.Anchor.Col != 1 || c.Chart.Type != "column" || len(c.Chart.Series) != 2 || c.Chart.Series[1].Values != "工作表1!$C$2:$C$4" {
			t.Fatalf("%s chart = %+v %+v", ext, c.Anchor, c.Chart)
		}
		if c.W < 470 || c.W > 490 || c.H < 280 || c.H > 296 {
			t.Fatalf("%s chart size = %v x %v", ext, c.W, c.H)
		}
		if p.Anchor.Row != 1 || p.Anchor.Col != 6 || !bytes.Equal(p.Image.Data, pic) || p.W < 78 || p.W > 82 {
			t.Fatalf("%s picture = %+v %vx%v", ext, p.Anchor, p.W, p.H)
		}
		ps := l.PageSetup
		if ps == nil || !ps.Landscape || ps.Scale != 80 || ps.PrintArea == nil || *ps.PrintArea != (model.Range{R1: 3, C1: 2}) || ps.FooterText != "第 &P 頁" || ps.Top != 1 {
			t.Fatalf("%s page setup = %+v", ext, ps)
		}
		if ext == ".xlsx" {
			// Move the picture and save in place: it must not be doubled.
			moved := *p
			moved.Anchor.Row = 10
			must(t, back)(back.UpdateObjectCommand(0, moved, "移動"))
			if err := back.SaveAs(path); err != nil {
				t.Fatal(err)
			}
			again, _ := Load(path)
			objs := again.Workbook().Sheets[0].Layout.Objects
			if len(objs) != 2 {
				t.Fatalf("in-place objects = %+v", objs)
			}
		}
	}
}
