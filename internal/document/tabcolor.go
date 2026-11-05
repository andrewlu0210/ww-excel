package document

import (
	"errors"
	"strconv"
	"strings"
	"ww-excel/internal/command"
	"ww-excel/internal/model"
)

// SetTabColorCommand changes one tab color; an empty value restores the default.
func (d *Document) SetTabColorCommand(sheet int, color string) (command.Command, error) {
	if sheet < 0 || sheet >= len(d.wb.Sheets) {
		return nil, errors.New("無效的工作表")
	}
	color = strings.ToUpper(strings.TrimPrefix(strings.TrimSpace(color), "#"))
	if color != "" {
		if _, err := strconv.ParseUint(color, 16, 24); err != nil || len(color) != 6 {
			return nil, errors.New("分頁色彩必須是六位十六進位色碼")
		}
	}
	if d.wb.Sheets[sheet].Layout.TabColor == color {
		return nil, nil
	}
	return d.layoutCommand(sheet, "分頁色彩", nil, func(l *model.Layout) { l.TabColor = color })
}
