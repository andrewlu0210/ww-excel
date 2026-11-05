package xlsxfile

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"path/filepath"
	"testing"
	"ww-excel/internal/io/wwx"
	"ww-excel/internal/model"
)

func TestNormalizeChartDrawingAnchors(t *testing.T) {
	l := model.NewLayout()
	raw := []byte(`<xdr:wsDr xmlns:xdr="` + spreadsheetDrawingNS + `"><xdr:twoCellAnchor editAs="oneCell"><xdr:from><xdr:col>3</xdr:col><xdr:colOff>95250</xdr:colOff><xdr:row>1</xdr:row><xdr:rowOff>47625</xdr:rowOff></xdr:from><xdr:to><xdr:col>3</xdr:col><xdr:colOff>6191250</xdr:colOff><xdr:row>1</xdr:row><xdr:rowOff>3476625</xdr:rowOff></xdr:to><xdr:graphicFrame/><xdr:clientData/></xdr:twoCellAnchor></xdr:wsDr>`)
	fixed := normalizeChartDrawingAnchors(raw, &l)
	root, e := drawingNodes(fixed)
	if e != nil {
		t.Fatal(e)
	}
	a := root.all("oneCellAnchor")
	if len(a) != 1 || a[0].child("to") != nil {
		t.Fatalf("expected explicit one-cell extent: %s", fixed)
	}
	if string(normalizeChartDrawingAnchors(fixed, &l)) != string(fixed) {
		t.Fatal("not idempotent")
	}
	var drawing struct {
		Anchors []rawAnchor `xml:"oneCellAnchor"`
	}
	if err := xml.Unmarshal(fixed, &drawing); err != nil {
		t.Fatal(err)
	}
	anchor := drawing.Anchors[0]

	width, height := anchorSize(&l, anchor)
	if width != 640 || height != 360 {
		t.Fatalf("size changed: %gx%g", width, height)
	}
}

func TestExportChartUsesExplicitSizeAcrossNativeRoundTrip(t *testing.T) {
	wb := model.NewWorkbook()
	sh := wb.Sheets[0]
	sh.Set(0, 0, model.NumberCell(10))
	sh.Layout.Objects = []model.Object{{ID: 1, Kind: model.ObjectChart, Anchor: model.Anchor{Row: 1, Col: 3, DX: 10, DY: 5}, W: 640, H: 360, Chart: &model.Chart{Type: "column", Series: []model.ChartSeries{{Values: "A1"}}}}}
	for _, mode := range []string{"xlsx", "wwx"} {
		if mode == "wwx" {
			var b bytes.Buffer
			if err := wwx.Write(&b, wb); err != nil {
				t.Fatal(err)
			}
			var err error
			wb, err = wwx.Read(&b)
			if err != nil {
				t.Fatal(err)
			}
		}
		filename := filepath.Join(t.TempDir(), mode+".xlsx")
		f, err := Save(filename, wb, nil, Changes{})
		if err != nil {
			t.Fatal(err)
		}
		f.Close()
		z, err := zip.OpenReader(filename)
		if err != nil {
			t.Fatal(err)
		}
		parts := map[string]*zip.File{}
		for _, part := range z.File {
			parts[part.Name] = part
		}
		var drawing struct {
			Anchors []rawAnchor `xml:"oneCellAnchor"`
		}
		if err = xml.Unmarshal(readPart(parts, "xl/drawings/drawing1.xml"), &drawing); err != nil {
			t.Fatal(err)
		}
		z.Close()
		if len(drawing.Anchors) != 1 {
			t.Fatal("chart must have a one-cell anchor with an explicit extent")
		}
		anchor := drawing.Anchors[0]
		if anchor.Ext.CX != 640*emuPerPx || anchor.Ext.CY != 360*emuPerPx || anchor.FromCX != 10*emuPerPx || anchor.FromRY != 5*emuPerPx {
			t.Fatalf("geometry changed: %+v", anchor)
		}
	}
}
