package printing

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"os"
	"testing"
	"time"

	"ww-excel/internal/model"
)

type imgSource struct {
	stubSource
	img []byte
}

func (s imgSource) ObjectImage(model.Object) []byte { return s.img }

// TestRenderObjects draws a picture over cells (written to
// WW_PDF_OUT for a visual check when set).
func TestRenderObjects(t *testing.T) {
	pic := image.NewRGBA(image.Rect(0, 0, 200, 120))
	for x := 0; x < 200; x++ {
		for y := 0; y < 120; y++ {
			pic.Set(x, y, color.RGBA{R: 68, G: 114, B: 196, A: 255})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, pic); err != nil {
		t.Fatal(err)
	}
	img := buf.Bytes()
	sh := sheetWith(30, 6)
	sh.Layout.Objects = []model.Object{{ID: 1, Kind: model.ObjectChart, Anchor: model.Anchor{Row: 3, Col: 1, DX: 10}, W: 384, H: 230, Chart: &model.Chart{Type: "column"}}}
	plan, err := Paginate(sh, model.DefaultPageSetup())
	if err != nil {
		t.Fatal(err)
	}
	data, err := Render(sh, plan, imgSource{stubSource{sh}, img}, Info{Now: time.Now()})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(data, []byte("/Subtype /Image")) {
		t.Fatal("the picture is missing from the PDF")
	}
	if out := os.Getenv("WW_PDF_OUT"); out != "" {
		_ = os.WriteFile(out, data, 0o644)
	}
}
