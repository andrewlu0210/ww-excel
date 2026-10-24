package document

import (
	"errors"
	"strconv"
	"strings"
	"unicode"

	"ww-excel/internal/command"
	"ww-excel/internal/formula/ast"
	"ww-excel/internal/model"
	"ww-excel/internal/numfmt"
)

// CustomLists are the fill series Excel (Traditional Chinese) knows.
var CustomLists = [][]string{
	{"星期日", "星期一", "星期二", "星期三", "星期四", "星期五", "星期六"},
	{"週日", "週一", "週二", "週三", "週四", "週五", "週六"},
	{"一月", "二月", "三月", "四月", "五月", "六月", "七月", "八月", "九月", "十月", "十一月", "十二月"},
	{"第一季", "第二季", "第三季", "第四季"},
	{"子", "丑", "寅", "卯", "辰", "巳", "午", "未", "申", "酉", "戌", "亥"},
	{"甲", "乙", "丙", "丁", "戊", "己", "庚", "辛", "壬", "癸"},
	{"Sun", "Mon", "Tue", "Wed", "Thu", "Fri", "Sat"},
	{"Sunday", "Monday", "Tuesday", "Wednesday", "Thursday", "Friday", "Saturday"},
	{"Jan", "Feb", "Mar", "Apr", "May", "Jun", "Jul", "Aug", "Sep", "Oct", "Nov", "Dec"},
	{"January", "February", "March", "April", "May", "June", "July", "August", "September", "October", "November", "December"},
}

// FillCommand extends src over dst with the fill handle (or ⌘D / ⌘R
// when copy is true). dst contains src and extends it in one direction;
// a dst inside src clears the cells dragged back over.
func (d *Document) FillCommand(sheet int, src, dst model.Range, copyOnly bool) (command.Command, error) {
	if sheet < 0 || sheet >= len(d.wb.Sheets) {
		return nil, errors.New("無效的工作表")
	}
	src, dst = src.Clamp(), dst.Clamp()
	sh := d.wb.Sheets[sheet]
	// Shrinking: clear the part of src that is no longer selected.
	if dst.R0 == src.R0 && dst.C0 == src.C0 && dst.R1 <= src.R1 && dst.C1 <= src.C1 {
		var ranges []model.Range
		if dst.R1 < src.R1 {
			ranges = append(ranges, model.Range{R0: dst.R1 + 1, C0: src.C0, R1: src.R1, C1: src.C1})
		}
		if dst.C1 < src.C1 {
			ranges = append(ranges, model.Range{R0: src.R0, C0: dst.C1 + 1, R1: src.R1, C1: src.C1})
		}
		if len(ranges) == 0 {
			return nil, nil
		}
		return command.ClearRanges(d.wb, sheet, ranges, command.ClearContents), nil
	}

	vertical := dst.C0 == src.C0 && dst.C1 == src.C1
	if !vertical && (dst.R0 != src.R0 || dst.R1 != src.R1) {
		return nil, errors.New("無法填滿：目標範圍必須與來源同列或同欄")
	}
	if dst.CellCount() > command.MaxFillCells {
		return nil, command.ErrFillTooLarge
	}
	// Lines run along the fill direction; n source cells per line.
	n, lines := src.R1-src.R0+1, src.C1-src.C0+1
	start, end, srcStart := dst.R0, dst.R1, src.R0
	if !vertical {
		n, lines = src.C1-src.C0+1, src.R1-src.R0+1
		start, end, srcStart = dst.C0, dst.C1, src.C0
	}
	pos := func(line, i int) (int, int) {
		if vertical {
			return i, src.C0 + line
		}
		return src.R0 + line, i
	}

	st := d.wb.StyleTable()
	var changes []command.CellChange
	for line := 0; line < lines; line++ {
		seq := make([]model.Cell, n)
		for k := range seq {
			r, c := pos(line, srcStart+k)
			seq[k] = sh.Get(r, c)
		}
		gen := d.series(sheet, seq, copyOnly, func(c model.Cell) string {
			return st.Get(c.S).NumFmt
		})
		for i := start; i <= end; i++ {
			k := i - srcStart
			if k >= 0 && k < n {
				continue
			}
			r, c := pos(line, i)
			// The source cell this one repeats (cyclically).
			m := ((k % n) + n) % n
			sr, sc := pos(line, srcStart+m)
			cell := gen(k, m, r-sr, c-sc)
			cell.S = sh.Get(sr, sc).S
			if cell.S == 0 {
				cell.S = sh.Layout.StyleAt(r, c)
			}
			changes = append(changes, command.CellChange{Row: r, Col: c, After: cell})
		}
	}
	return &command.SetCells{Sheet: sheet, Name: "填滿", Changes: changes, Area: &dst}, nil
}

// series returns the cell at index k of a line (k < 0 before the
// source), given the source cells seq; m is k's source cell and dr/dc
// its offset from it, for translating formulas.
func (d *Document) series(sheet int, seq []model.Cell, copyOnly bool, numFmt func(model.Cell) string) func(k, m, dr, dc int) model.Cell {
	n := len(seq)
	repeat := func(k, m, dr, dc int) model.Cell {
		c := seq[m]
		if c.F != nil {
			node := ast.Translate(d.calc.Parsed(c.F, sheet), dr, dc)
			return model.Cell{F: d.calc.CompileNode(node, sheet)}
		}
		c.S = 0
		return c
	}
	if copyOnly {
		return repeat
	}

	// All numbers: a linear trend (one number repeats, except dates,
	// which count up by a day).
	if nums, ok := numbers(seq); ok {
		if n == 1 {
			if !numfmt.IsDate(numFmt(seq[0])) {
				return repeat
			}
			return func(k, _, _, _ int) model.Cell { return model.NumberCell(nums[0] + float64(k)) }
		}
		a, b := linearFit(nums)
		return func(k, _, _, _ int) model.Cell { return model.NumberCell(numfmt.Normalize(a + b*float64(k))) }
	}

	// Items of a custom list: continue the list.
	if list, idx, ok := listItems(seq); ok {
		step := 1
		if n > 1 {
			step = idx[1] - idx[0]
		}
		size := len(list)
		return func(k, _, _, _ int) model.Cell {
			i := ((idx[0]+k*step)%size + size) % size
			return model.StringCell(list[i])
		}
	}

	// Text ending in a number ("項目1"): the number counts up.
	if prefix, suffix, nums, width, ok := numberedTexts(seq); ok {
		step := 1.0
		base := nums[0]
		if n > 1 {
			base, step = linearFit(nums)
		}
		if n == 1 {
			step = 1
		}
		return func(k, m, dr, dc int) model.Cell {
			v := int(base + step*float64(k) + 0.5)
			if v < 0 {
				v = -v
			}
			s := strconv.Itoa(v)
			for len(s) < width {
				s = "0" + s
			}
			return model.StringCell(prefix + s + suffix)
		}
	}
	return repeat
}

func numbers(seq []model.Cell) ([]float64, bool) {
	out := make([]float64, len(seq))
	for i, c := range seq {
		if c.F != nil || c.Kind != model.KindNumber {
			return nil, false
		}
		out[i] = c.Num
	}
	return out, true
}

// linearFit returns the least-squares line through (i, v[i]), as Excel's
// AutoFill does for several numbers.
func linearFit(v []float64) (a, b float64) {
	n := float64(len(v))
	var sx, sy, sxx, sxy float64
	for i, y := range v {
		x := float64(i)
		sx += x
		sy += y
		sxx += x * x
		sxy += x * y
	}
	den := n*sxx - sx*sx
	if den == 0 {
		return v[0], 0
	}
	b = (n*sxy - sx*sy) / den
	a = (sy - b*sx) / n
	return a, b
}

func listItems(seq []model.Cell) ([]string, []int, bool) {
	for _, list := range CustomLists {
		idx := make([]int, len(seq))
		ok := true
		for i, c := range seq {
			idx[i] = -1
			if c.F == nil && c.Kind == model.KindString {
				for j, item := range list {
					if strings.EqualFold(item, c.Str) {
						idx[i] = j
						break
					}
				}
			}
			if idx[i] < 0 {
				ok = false
				break
			}
		}
		if ok {
			return list, idx, true
		}
	}
	return nil, nil, false
}

// numberedTexts matches texts that share a prefix and suffix around a
// number, e.g. 第1名, 第2名. width keeps leading zeros (A001).
func numberedTexts(seq []model.Cell) (prefix, suffix string, nums []float64, width int, ok bool) {
	for i, c := range seq {
		if c.F != nil || c.Kind != model.KindString {
			return "", "", nil, 0, false
		}
		p, digits, s, found := splitNumber(c.Str)
		if !found || i > 0 && (p != prefix || s != suffix) {
			return "", "", nil, 0, false
		}
		prefix, suffix = p, s
		v, _ := strconv.Atoi(digits)
		nums = append(nums, float64(v))
		if strings.HasPrefix(digits, "0") {
			width = max(width, len(digits))
		}
	}
	return prefix, suffix, nums, width, true
}

// splitNumber finds the last run of digits in s.
func splitNumber(s string) (prefix, digits, suffix string, ok bool) {
	rs := []rune(s)
	end := -1
	for i := len(rs) - 1; i >= 0; i-- {
		if unicode.IsDigit(rs[i]) && rs[i] < 128 {
			end = i
			break
		}
	}
	if end < 0 {
		return "", "", "", false
	}
	begin := end
	for begin > 0 && unicode.IsDigit(rs[begin-1]) && rs[begin-1] < 128 {
		begin--
	}
	if end-begin >= 9 {
		return "", "", "", false
	}
	return string(rs[:begin]), string(rs[begin : end+1]), string(rs[end+1:]), true
}
