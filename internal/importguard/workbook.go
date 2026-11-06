package importguard

import (
	"bytes"
	"context"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"math"
	"unicode/utf8"

	"ww-excel/internal/formula/parser"
	"ww-excel/internal/model"
)

// Workbook validates model-level resources before formula calculation/rendering.
func Workbook(ctx context.Context, wb *model.Workbook) error {
	l := Policy(ctx)
	if len(wb.Sheets) > l.Sheets {
		return Exceeded("工作表數", int64(l.Sheets))
	}
	if wb.StyleTable().Len() > 65536 {
		return Exceeded("樣式數", 65536)
	}
	cells, objects := 0, 0
	var pixels float64
	formulaBytes := 0
	decodedPixels := int64(0)
	checkFormula := func(src string) error {
		formulaBytes += len(src)
		if formulaBytes > 32<<20 {
			return Exceeded("公式總長度", 32<<20)
		}
		return parser.CheckResources(src)
	}
	var err error
	for _, n := range wb.Names {
		if err = checkFormula(n.Formula); err != nil {
			return err
		}
	}
	for _, sh := range wb.Sheets {
		sh.EachInRange(model.Range{R1: model.MaxRows - 1, C1: model.MaxCols - 1}, func(row, col int, c model.Cell) {
			if err != nil {
				return
			}
			err = ctx.Err()
			if err != nil {
				return
			}
			cells++
			if cells > l.Cells {
				err = Exceeded("儲存格數", int64(l.Cells))
				return
			}
			if utf8.RuneCountInString(c.Str) > 32767 {
				err = Exceeded("儲存格文字字元數", 32767)
				return
			}
			if c.F != nil {
				err = checkFormula(c.F.Text)
			}
		})
		if err != nil {
			return err
		}
		for _, o := range sh.Layout.Objects {
			if err = ctx.Err(); err != nil {
				return err
			}
			objects++
			if objects > 2000 {
				return Exceeded("圖形數", 2000)
			}
			if math.IsNaN(o.W) || math.IsNaN(o.H) || math.IsInf(o.W, 0) || math.IsInf(o.H, 0) || o.W < 0 || o.H < 0 || o.W > 8192 || o.H > 8192 || o.W*o.H > 8_000_000 {
				return Exceeded("圖形大小／像素數", 8_000_000)
			}
			pixels += o.W * o.H
			if pixels > 32_000_000 {
				return Exceeded("圖形總像素數", 32_000_000)
			}
			if o.Image != nil {
				if int64(len(o.Image.Data)) > l.Data {
					return Exceeded("圖片資料大小", l.Data)
				}
				cfg, _, e := image.DecodeConfig(bytes.NewReader(o.Image.Data))
				if e == nil && (cfg.Width <= 0 || cfg.Height <= 0 || cfg.Width > 64_000_000/cfg.Height) {
					return Exceeded("圖片像素數", 64_000_000)
				}
				if e == nil {
					decodedPixels += int64(cfg.Width) * int64(cfg.Height)
					if decodedPixels > 64_000_000 {
						return Exceeded("圖片總像素數", 64_000_000)
					}
				}
			}
		}
	}
	return ctx.Err()
}
