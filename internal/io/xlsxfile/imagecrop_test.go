package xlsxfile

import (
	"bytes"
	"image"
	"os"
	"path/filepath"
	"testing"

	"ww-excel/internal/io/wwx"
	"ww-excel/internal/model"
)

func TestTest9LogoCropRoundTrip(t *testing.T) {
	path := filepath.Join("..", "..", "..", "test_data", "test9.xlsx")
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Skip("local workbook not present")
	}
	wb, base, err := Read(path)
	if err != nil {
		t.Fatal(err)
	}
	defer base.Close()
	var count int
	for _, sh := range wb.Sheets {
		for _, o := range sh.Layout.Objects {
			if o.Image == nil || o.Image.Crop == (model.ImageCrop{}) {
				continue
			}
			count++
			if o.Image.Crop != (model.ImageCrop{Top: 22334, Bottom: 24064}) {
				t.Fatalf("unexpected logo crop: %+v", o.Image.Crop)
			}
			rendered := o.Image.Rendered()
			cfg, _, err := image.DecodeConfig(bytes.NewReader(rendered.Data))
			if err != nil {
				t.Fatal(err)
			}
			t.Logf("%s: logo frame %.2fx%.2f, cropped source %dx%d", sh.Name, o.W, o.H, cfg.Width, cfg.Height)
			// Native saves preserve the original image and editable crop metadata.
			native := model.NewWorkbook()
			native.Sheets[0].Layout.Objects = []model.Object{o}
			var data bytes.Buffer
			if err := wwx.Write(&data, native); err != nil {
				t.Fatal(err)
			}
			back, err := wwx.Read(&data)
			if err != nil {
				t.Fatal(err)
			}
			got := back.Sheets[0].Layout.Objects[0].Image
			if got.Crop != o.Image.Crop || !bytes.Equal(got.Data, o.Image.Data) {
				t.Fatal("native round trip lost crop")
			}
			// Fresh XLSX exports bake the crop into the picture bytes.
			filename := filepath.Join(t.TempDir(), "logo.xlsx")
			f, err := Save(filename, back, nil, Changes{})
			if err != nil {
				t.Fatal(err)
			}
			f.Close()
			exported, source, err := Read(filename)
			if err != nil {
				t.Fatal(err)
			}
			got = exported.Sheets[0].Layout.Objects[0].Image
			source.Close()
			if got.Crop != (model.ImageCrop{}) || !bytes.Equal(got.Data, rendered.Data) {
				t.Fatal("XLSX export lost cropped appearance")
			}
		}
	}
	if count != len(wb.Sheets) {
		t.Fatalf("expected a cropped logo on every sheet, got %d", count)
	}
}
