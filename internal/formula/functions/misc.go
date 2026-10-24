package functions

import (
	"math"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"ww-excel/internal/formula/ast"
	"ww-excel/internal/formula/eval"
	"ww-excel/internal/formula/parser"
	"ww-excel/internal/model"
	"ww-excel/internal/numfmt"
)

func init() {
	// ---- Text ----
	reg(catText, "CHAR", 1, 1, "number", "傳回字碼對應的字元", func(a *eval.Args) eval.Value {
		n, err := a.Int(0, 0)
		if err.IsErr() {
			return err
		}
		if n < 1 || n > 255 {
			return errValue
		}
		return eval.Text(string(rune(n)))
	})
	reg(catText, "UNICHAR", 1, 1, "number", "傳回 Unicode 字碼對應的字元", func(a *eval.Args) eval.Value {
		n, err := a.Int(0, 0)
		if err.IsErr() {
			return err
		}
		if n < 1 || n > unicode.MaxRune || !utf8.ValidRune(rune(n)) {
			return errValue
		}
		return eval.Text(string(rune(n)))
	})
	codeOf := func(name, desc string) {
		reg(catText, name, 1, 1, "text", desc, func(a *eval.Args) eval.Value {
			s, err := a.Text(0, "")
			if err.IsErr() {
				return err
			}
			if s == "" {
				return errValue
			}
			r, _ := utf8.DecodeRuneInString(s)
			return eval.Number(float64(r))
		})
	}
	codeOf("CODE", "傳回文字第一個字元的字碼")
	codeOf("UNICODE", "傳回文字第一個字元的 Unicode 字碼")
	reg(catText, "CLEAN", 1, 1, "text", "移除文字中無法列印的字元", textMap(func(s string) string {
		return strings.Map(func(r rune) rune {
			if r < 32 {
				return -1
			}
			return r
		}, s)
	}))
	reg(catText, "T", 1, 1, "value", "值為文字時傳回文字，否則傳回空字串", func(a *eval.Args) eval.Value {
		v := a.Scalar(0)
		switch v.Kind {
		case eval.KString:
			return v
		case eval.KError:
			return v
		}
		return eval.Text("")
	})
	fixed := func(name, desc string, currency bool) {
		sig := "number, [decimals], [no_commas]"
		if currency {
			sig = "number, [decimals]"
		}
		reg(catText, name, 1, 3, sig, desc, func(a *eval.Args) eval.Value {
			n, err := a.Number(0, 0)
			if err.IsErr() {
				return err
			}
			d, err := a.Int(1, 2)
			if err.IsErr() {
				return err
			}
			noCommas := false
			if !currency {
				if noCommas, err = a.Bool(2, false); err.IsErr() {
					return err
				}
			}
			n = numfmt.Round(n, d)
			code := "#,##0"
			if noCommas {
				code = "0"
			}
			if d > 0 {
				code += "." + strings.Repeat("0", min(d, 127))
			}
			if currency {
				code = `"$"` + code + `;\("$"` + code + `\)`
			}
			return eval.Text(numfmt.Format(n, code))
		})
	}
	fixed("FIXED", "將數字四捨五入並格式化為文字", false)
	fixed("DOLLAR", "將數字格式化為貨幣文字", true)
	reg(catText, "NUMBERVALUE", 1, 3, "text, [decimal_separator], [group_separator]", "依指定的分隔符號將文字轉為數字", func(a *eval.Args) eval.Value {
		s, err := a.Text(0, "")
		if err.IsErr() {
			return err
		}
		dec, err := a.Text(1, ".")
		if err.IsErr() {
			return err
		}
		group, err := a.Text(2, ",")
		if err.IsErr() {
			return err
		}
		s = strings.Join(strings.Fields(s), "")
		if s == "" {
			return eval.Number(0)
		}
		if group != "" {
			s = strings.ReplaceAll(s, group[:1], "")
		}
		if dec != "" && dec[:1] != "." {
			s = strings.ReplaceAll(s, dec[:1], ".")
		}
		pct := 0
		for strings.HasSuffix(s, "%") {
			s = strings.TrimSuffix(s, "%")
			pct++
		}
		n, ok := model.ParseNumber(s)
		if !ok {
			return errValue
		}
		return eval.Number(n / math.Pow(100, float64(pct)))
	})
	beforeAfter := func(name, desc string, after bool) {
		reg(catText, name, 2, 6, "text, delimiter, [instance_num], [match_mode], [match_end], [if_not_found]", desc, func(a *eval.Args) eval.Value {
			s, err := a.Text(0, "")
			if err.IsErr() {
				return err
			}
			d, err := a.Text(1, "")
			if err.IsErr() {
				return err
			}
			inst, err := a.Int(2, 1)
			if err.IsErr() {
				return err
			}
			mode, err := a.Int(3, 0)
			if err.IsErr() {
				return err
			}
			if inst == 0 || d == "" {
				return errValue
			}
			hay, needle := s, d
			if mode == 1 {
				hay, needle = strings.ToLower(s), strings.ToLower(d)
			}
			// Byte offsets of every match.
			var at []int
			for i := 0; i <= len(hay)-len(needle); {
				k := strings.Index(hay[i:], needle)
				if k < 0 {
					break
				}
				at = append(at, i+k)
				i += k + len(needle)
			}
			idx := inst - 1
			if inst < 0 {
				idx = len(at) + inst
			}
			if idx < 0 || idx >= len(at) {
				if a.Has(5) {
					return a.Scalar(5)
				}
				return errNA
			}
			if after {
				return eval.Text(s[at[idx]+len(d):])
			}
			return eval.Text(s[:at[idx]])
		})
	}
	beforeAfter("TEXTBEFORE", "傳回分隔符號之前的文字", false)
	beforeAfter("TEXTAFTER", "傳回分隔符號之後的文字", true)

	// ---- Date ----
	reg(catDate, "DATEVALUE", 1, 1, "date_text", "將日期文字轉換為序列值", func(a *eval.Args) eval.Value {
		s, err := a.Text(0, "")
		if err.IsErr() {
			return err
		}
		v, code, ok := numfmt.Detect(s, a.Ev().Now)
		if !ok || !numfmt.IsDate(code) {
			return errValue
		}
		return eval.Number(math.Floor(v))
	})
	reg(catDate, "TIMEVALUE", 1, 1, "time_text", "將時間文字轉換為序列值", func(a *eval.Args) eval.Value {
		s, err := a.Text(0, "")
		if err.IsErr() {
			return err
		}
		v, code, ok := numfmt.Detect(s, a.Ev().Now)
		if !ok || !numfmt.IsDate(code) {
			return errValue
		}
		return eval.Number(v - math.Floor(v))
	})
	reg(catDate, "WEEKNUM", 1, 2, "serial_number, [return_type]", "傳回日期是一年中的第幾週", func(a *eval.Args) eval.Value {
		s, err := serialArg(a, 0)
		if err.IsErr() {
			return err
		}
		kind, err := a.Int(1, 1)
		if err.IsErr() {
			return err
		}
		if kind == 21 {
			return eval.Number(float64(isoWeek(s)))
		}
		start := map[int]int{1: 0, 17: 0, 2: 1, 11: 1, 12: 2, 13: 3, 14: 4, 15: 5, 16: 6}
		first, ok := start[kind]
		if !ok {
			return errNum
		}
		y, _, _ := numfmt.YMD(s)
		jan1 := numfmt.DateSerial(y, 1, 1)
		offset := (numfmt.Weekday(jan1) - first + 7) % 7
		return eval.Number(math.Floor((math.Floor(s)-jan1+float64(offset))/7) + 1)
	})
	reg(catDate, "ISOWEEKNUM", 1, 1, "date", "傳回日期的 ISO 週數", func(a *eval.Args) eval.Value {
		s, err := serialArg(a, 0)
		if err.IsErr() {
			return err
		}
		return eval.Number(float64(isoWeek(s)))
	})
	reg(catDate, "NETWORKDAYS", 2, 3, "start_date, end_date, [holidays]", "傳回兩日期之間的工作日數", func(a *eval.Args) eval.Value {
		return networkdays(a, -1)
	})
	reg(catDate, "NETWORKDAYS.INTL", 2, 4, "start_date, end_date, [weekend], [holidays]", "傳回兩日期之間的工作日數（可自訂週末）", func(a *eval.Args) eval.Value {
		return networkdays(a, 2)
	})
	reg(catDate, "WORKDAY", 2, 3, "start_date, days, [holidays]", "傳回幾個工作日之前或之後的日期", func(a *eval.Args) eval.Value {
		return workday(a, -1)
	})
	reg(catDate, "WORKDAY.INTL", 2, 4, "start_date, days, [weekend], [holidays]", "傳回幾個工作日之前或之後的日期（可自訂週末）", func(a *eval.Args) eval.Value {
		return workday(a, 2)
	})
	reg(catDate, "DAYS360", 2, 3, "start_date, end_date, [method]", "以一年 360 天計算兩日期之間的天數", func(a *eval.Args) eval.Value {
		s, err := serialArg(a, 0)
		if err.IsErr() {
			return err
		}
		e, err := serialArg(a, 1)
		if err.IsErr() {
			return err
		}
		european, err := a.Bool(2, false)
		if err.IsErr() {
			return err
		}
		return eval.Number(days360(s, e, european))
	})
	reg(catDate, "YEARFRAC", 2, 3, "start_date, end_date, [basis]", "傳回兩日期之間占一年的比例", func(a *eval.Args) eval.Value {
		s, err := serialArg(a, 0)
		if err.IsErr() {
			return err
		}
		e, err := serialArg(a, 1)
		if err.IsErr() {
			return err
		}
		basis, err := a.Int(2, 0)
		if err.IsErr() {
			return err
		}
		s, e = math.Floor(s), math.Floor(e)
		if s > e {
			s, e = e, s
		}
		switch basis {
		case 0:
			return eval.Number(days360(s, e, false) / 360)
		case 1:
			y0, m0, d0 := numfmt.YMD(s)
			y1, m1, d1 := numfmt.YMD(e)
			yearLen := func(y int) float64 { return numfmt.DateSerial(y+1, 1, 1) - numfmt.DateSerial(y, 1, 1) }
			leap := func(y int) bool { return yearLen(y) == 366 }
			if y0 == y1 {
				return eval.Number((e - s) / yearLen(y0))
			}
			if y1 == y0+1 && (m1 < m0 || m1 == m0 && d1 <= d0) {
				// Not more than a year apart: 366 if a Feb 29 is spanned.
				den := 365.0
				if leap(y0) && (m0 < 3) || leap(y1) && (m1 > 2 || m1 == 2 && d1 == 29) {
					den = 366
				}
				return eval.Number((e - s) / den)
			}
			total := 0.0
			for y := y0; y <= y1; y++ {
				total += yearLen(y)
			}
			return eval.Number((e - s) / (total / float64(y1-y0+1)))
		case 2:
			return eval.Number((e - s) / 360)
		case 3:
			return eval.Number((e - s) / 365)
		case 4:
			return eval.Number(days360(s, e, true) / 360)
		}
		return errNum
	})

	// ---- Lookup and reference ----
	reg(catLookup, "LOOKUP", 2, 3, "lookup_value, lookup_vector, [result_vector]", "在單列或單欄中尋找值（資料需排序）", func(a *eval.Args) eval.Value {
		v := a.Scalar(0)
		if v.IsErr() {
			return v
		}
		g, err := a.Grid(1)
		if err.IsErr() {
			return err
		}
		// Array form: search the first row or column, return from the
		// last one.
		vertical := g.Rows() >= g.Cols()
		search := line{g, vertical}
		if !vertical {
			search = line{g, false}
		}
		i := findSorted(search, v, true)
		if i < 0 {
			return errNA
		}
		if a.Has(2) {
			r, err := a.Grid(2)
			if err.IsErr() {
				return err
			}
			if r.Cols() == 1 {
				if i >= r.Rows() {
					return errNA
				}
				return r.At(i, 0)
			}
			if i >= r.Cols() {
				return errNA
			}
			return r.At(0, i)
		}
		if vertical {
			return g.At(i, g.Cols()-1)
		}
		return g.At(g.Rows()-1, i)
	})
	eval.Register(eval.Def{
		Name: "INDIRECT", Min: 1, Max: 2, Volatile: true, Category: catLookup,
		Sig: "ref_text, [a1]", Desc: "傳回文字所指定的參照",
		Fn: func(a *eval.Args) eval.Value {
			s, err := a.Text(0, "")
			if err.IsErr() {
				return err
			}
			a1, err := a.Bool(1, true)
			if err.IsErr() {
				return err
			}
			if !a1 {
				return errRef // R1C1 references are not supported yet
			}
			node, perr := parser.Parse(strings.TrimSpace(s))
			if perr != nil {
				return errRef
			}
			switch node.(type) {
			case ast.Ref, ast.Name:
			default:
				return errRef
			}
			v := a.Ev().Reference(node)
			if v.Kind != eval.KRef {
				return errRef
			}
			return v
		},
	})
	eval.Register(eval.Def{
		Name: "OFFSET", Min: 3, Max: 5, Volatile: true, Category: catLookup,
		Sig: "reference, rows, cols, [height], [width]", Desc: "傳回從參照位移指定列數與欄數的參照",
		Fn: func(a *eval.Args) eval.Value {
			v := a.Raw(0)
			if v.Kind == eval.KError {
				return v
			}
			if v.Kind != eval.KRef {
				return errValue
			}
			r := *v.Ref
			dr, err := a.Int(1, 0)
			if err.IsErr() {
				return err
			}
			dc, err := a.Int(2, 0)
			if err.IsErr() {
				return err
			}
			h, err := a.Int(3, r.Rows())
			if err.IsErr() {
				return err
			}
			w, err := a.Int(4, r.Cols())
			if err.IsErr() {
				return err
			}
			if h < 1 || w < 1 {
				return errRef
			}
			out := eval.Ref{Sheet: r.Sheet, R0: r.R0 + dr, C0: r.C0 + dc}
			out.R1, out.C1 = out.R0+h-1, out.C0+w-1
			if out.R0 < 0 || out.C0 < 0 || out.R1 >= model.MaxRows || out.C1 >= model.MaxCols {
				return errRef
			}
			return eval.RefOf(&out)
		},
	})
	reg(catLookup, "ADDRESS", 2, 5, "row_num, column_num, [abs_num], [a1], [sheet_text]", "傳回儲存格位址的文字", func(a *eval.Args) eval.Value {
		row, err := a.Int(0, 0)
		if err.IsErr() {
			return err
		}
		col, err := a.Int(1, 0)
		if err.IsErr() {
			return err
		}
		abs, err := a.Int(2, 1)
		if err.IsErr() {
			return err
		}
		a1, err := a.Bool(3, true)
		if err.IsErr() {
			return err
		}
		sheet, err := a.Text(4, "")
		if err.IsErr() {
			return err
		}
		if row < 1 || col < 1 || row > model.MaxRows || col > model.MaxCols || abs < 1 || abs > 4 {
			return errValue
		}
		var s string
		if a1 {
			absR, absC := abs == 1 || abs == 2, abs == 1 || abs == 3
			s = ast.FormatArea(ast.CellAt(row-1, col-1, absR, absC))
		} else {
			rs, cs := itoa(row), itoa(col)
			if abs == 2 || abs == 4 {
				cs = "[" + cs + "]"
			}
			if abs == 3 || abs == 4 {
				rs = "[" + rs + "]"
			}
			s = "R" + rs + "C" + cs
		}
		if sheet != "" {
			s = ast.QuoteSheet(sheet) + "!" + s
		}
		return eval.Text(s)
	})
	reg(catLookup, "HYPERLINK", 1, 2, "link_location, [friendly_name]", "建立超連結（顯示名稱或位址）", func(a *eval.Args) eval.Value {
		if a.Has(1) {
			return a.Scalar(1)
		}
		return a.Scalar(0)
	})
	reg(catLookup, "AREAS", 1, 1, "reference", "傳回參照中的區域數", func(a *eval.Args) eval.Value {
		if v := a.Raw(0); v.Kind != eval.KRef {
			return errValue
		}
		return eval.Number(1)
	})

	// ---- Information ----
	parity := func(name, desc string, odd bool) {
		reg(catInfo, name, 1, 1, "number", desc, func(a *eval.Args) eval.Value {
			n, err := a.Number(0, 0)
			if err.IsErr() {
				return err
			}
			isOdd := math.Mod(math.Abs(math.Trunc(n)), 2) == 1
			return eval.Bool(isOdd == odd)
		})
	}
	parity("ISEVEN", "數字為偶數時傳回 TRUE", false)
	parity("ISODD", "數字為奇數時傳回 TRUE", true)
	reg(catInfo, "ISREF", 1, 1, "value", "值為參照時傳回 TRUE", func(a *eval.Args) eval.Value {
		return eval.Bool(a.Raw(0).Kind == eval.KRef)
	})
	reg(catInfo, "N", 1, 1, "value", "將值轉換為數字", func(a *eval.Args) eval.Value {
		v := a.Scalar(0)
		switch v.Kind {
		case eval.KNumber, eval.KBool:
			return eval.Number(v.Num)
		case eval.KError:
			return v
		}
		return eval.Number(0)
	})
	reg(catInfo, "TYPE", 1, 1, "value", "傳回值的類型（1 數字、2 文字、4 邏輯、16 錯誤、64 陣列）", func(a *eval.Args) eval.Value {
		v := a.Raw(0)
		if v.Kind == eval.KRef && !v.Ref.Single() {
			return eval.Number(16)
		}
		if v.Kind != eval.KArray {
			v = a.Scalar(0)
		}
		switch v.Kind {
		case eval.KString:
			return eval.Number(2)
		case eval.KBool:
			return eval.Number(4)
		case eval.KError:
			return eval.Number(16)
		case eval.KArray:
			return eval.Number(64)
		}
		return eval.Number(1)
	})
	reg(catInfo, "ERROR.TYPE", 1, 1, "error_val", "傳回錯誤值對應的數字", func(a *eval.Args) eval.Value {
		v := a.Scalar(0)
		if !v.IsErr() {
			return errNA
		}
		codes := map[string]float64{model.ErrNull: 1, model.ErrDiv0: 2, model.ErrValue: 3, model.ErrRef: 4, model.ErrName: 5, model.ErrNum: 6, model.ErrNA: 7}
		if n, ok := codes[v.Str]; ok {
			return eval.Number(n)
		}
		return errNA
	})
	reg(catInfo, "SHEETS", 0, 1, "[reference]", "傳回參照中的工作表數", func(a *eval.Args) eval.Value {
		if a.Has(0) {
			return eval.Number(1)
		}
		return eval.Number(float64(a.Ev().R.SheetCount()))
	})
	reg(catInfo, "SHEET", 0, 1, "[value]", "傳回工作表的編號", func(a *eval.Args) eval.Value {
		if !a.Has(0) {
			return eval.Number(float64(a.Ev().Sheet + 1))
		}
		v := a.Raw(0)
		switch v.Kind {
		case eval.KRef:
			return eval.Number(float64(v.Ref.Sheet + 1))
		case eval.KString:
			if i, ok := a.Ev().R.SheetIndex(v.Str); ok {
				return eval.Number(float64(i + 1))
			}
		}
		return errNA
	})
}

func itoa(n int) string {
	var b [20]byte
	i := len(b)
	for n >= 10 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	i--
	b[i] = byte('0' + n)
	return string(b[i:])
}

func isoWeek(s float64) int {
	y, m, d := numfmt.YMD(s)
	_, w := time.Date(y, time.Month(m), d, 0, 0, 0, 0, time.UTC).ISOWeek()
	return w
}

func days360(s, e float64, european bool) float64 {
	y0, m0, d0 := numfmt.YMD(s)
	y1, m1, d1 := numfmt.YMD(e)
	lastFeb := func(y, m, d int) bool {
		return m == 2 && d == time.Date(y, 3, 0, 0, 0, 0, 0, time.UTC).Day()
	}
	if european {
		d0, d1 = min(d0, 30), min(d1, 30)
	} else {
		if lastFeb(y0, m0, d0) && lastFeb(y1, m1, d1) {
			d1 = 30
		}
		if lastFeb(y0, m0, d0) {
			d0 = 30
		}
		if d1 == 31 && d0 >= 30 {
			d1 = 30
		}
		if d0 == 31 {
			d0 = 30
		}
	}
	return float64((y1-y0)*360 + (m1-m0)*30 + d1 - d0)
}

// weekendMask returns which weekdays (0 = Sunday) are weekend days for
// the weekend argument of the .INTL functions (or Saturday/Sunday).
func weekendMask(a *eval.Args, i int) ([7]bool, eval.Value) {
	mask := [7]bool{true, false, false, false, false, false, true}
	if i < 0 || !a.Has(i) {
		return mask, eval.Empty
	}
	v := a.Scalar(i)
	if v.Kind == eval.KString {
		if len(v.Str) != 7 || strings.Trim(v.Str, "01") != "" || v.Str == "1111111" {
			return mask, errValue
		}
		// The string starts on Monday.
		for k := 0; k < 7; k++ {
			mask[(k+1)%7] = v.Str[k] == '1'
		}
		return mask, eval.Empty
	}
	n, err := a.Int(i, 1)
	if err.IsErr() {
		return mask, err
	}
	pairs := map[int][2]int{1: {6, 0}, 2: {0, 1}, 3: {1, 2}, 4: {2, 3}, 5: {3, 4}, 6: {4, 5}, 7: {5, 6}}
	mask = [7]bool{}
	switch {
	case n >= 1 && n <= 7:
		p := pairs[n]
		mask[p[0]], mask[p[1]] = true, true
	case n >= 11 && n <= 17:
		mask[(n-11+1)%7] = true
	default:
		return mask, errNum
	}
	return mask, eval.Empty
}

func holidays(a *eval.Args, i int) (map[float64]bool, eval.Value) {
	out := map[float64]bool{}
	if !a.Has(i) {
		return out, eval.Empty
	}
	xs, err := numbersOf(a, i, i+1)
	if err.IsErr() {
		return nil, err
	}
	for _, x := range xs {
		out[math.Floor(x)] = true
	}
	return out, eval.Empty
}

// networkdays counts working days; weekendArg is the index of the
// weekend argument (-1 for NETWORKDAYS), holidays follow it.
func networkdays(a *eval.Args, weekendArg int) eval.Value {
	s, err := serialArg(a, 0)
	if err.IsErr() {
		return err
	}
	e, err := serialArg(a, 1)
	if err.IsErr() {
		return err
	}
	mask, err := weekendMask(a, weekendArg)
	if err.IsErr() {
		return err
	}
	hi := 2
	if weekendArg >= 0 {
		hi = weekendArg + 1
	}
	hol, err := holidays(a, hi)
	if err.IsErr() {
		return err
	}
	s, e = math.Floor(s), math.Floor(e)
	sign := 1.0
	if s > e {
		s, e, sign = e, s, -1
	}
	n := 0.0
	for d := s; d <= e; d++ {
		if !mask[numfmt.Weekday(d)] && !hol[d] {
			n++
		}
	}
	return eval.Number(sign * n)
}

func workday(a *eval.Args, weekendArg int) eval.Value {
	s, err := serialArg(a, 0)
	if err.IsErr() {
		return err
	}
	days, err := a.Int(1, 0)
	if err.IsErr() {
		return err
	}
	mask, err := weekendMask(a, weekendArg)
	if err.IsErr() {
		return err
	}
	hi := 2
	if weekendArg >= 0 {
		hi = weekendArg + 1
	}
	hol, err := holidays(a, hi)
	if err.IsErr() {
		return err
	}
	d := math.Floor(s)
	step := 1.0
	if days < 0 {
		step, days = -1, -days
	}
	for days > 0 {
		d += step
		if d < 0 {
			return errNum
		}
		if !mask[numfmt.Weekday(d)] && !hol[d] {
			days--
		}
	}
	return eval.Number(d)
}
