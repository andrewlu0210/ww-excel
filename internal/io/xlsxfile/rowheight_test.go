package xlsxfile

import (
	"archive/zip"
	"github.com/xuri/excelize/v2"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadRowHeightWithoutCustomHeight(t *testing.T) {
	f := excelize.NewFile()
	defer f.Close()
	for row, height := range map[int]float64{7: 140, 8: 56, 9: 28} {
		if err := f.SetRowHeight("Sheet1", row, height); err != nil {
			t.Fatal(err)
		}
	}
	if err := f.SetRowVisible("Sheet1", 8, false); err != nil {
		t.Fatal(err)
	}
	buf, err := f.WriteToBuffer()
	if err != nil {
		t.Fatal(err)
	}
	src, err := zip.NewReader(strings.NewReader(buf.String()), int64(buf.Len()))
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "autoheight.xlsx")
	out, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(out)
	for _, part := range src.File {
		in, err := part.Open()
		if err != nil {
			t.Fatal(err)
		}
		data, err := io.ReadAll(in)
		in.Close()
		if err != nil {
			t.Fatal(err)
		}
		if part.Name == "xl/worksheets/sheet1.xml" {
			text := strings.ReplaceAll(string(data), ` customHeight="true"`, "")
			data = []byte(strings.ReplaceAll(text, ` customHeight="1"`, ""))
		}
		dst, err := zw.Create(part.Name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = dst.Write(data); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := out.Close(); err != nil {
		t.Fatal(err)
	}
	wb, base, err := Read(path)
	if err != nil {
		t.Fatal(err)
	}
	defer base.Close()
	layout := wb.Sheets[0].Layout
	for row, height := range map[int]float64{6: 187, 7: 75, 8: 37} {
		size := layout.Rows[row]
		if size.Px != height {
			t.Errorf("row %d height=%v want %v", row+1, size.Px, height)
		}
	}
	if !layout.Rows[7].Hidden {
		t.Fatal("hidden row lost")
	}
}
