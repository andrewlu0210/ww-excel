//go:build !darwin || !cgo

package xlsxfile

import "ww-excel/internal/model"

func fontDigitWidth(font model.Font) float64 {
	if font.Size <= 0 {
		font.Size = 11
	}
	return 7 * font.Size / 11
}
