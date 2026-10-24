package numfmt

// Builtin maps xlsx built-in number format IDs to format codes. IDs
// whose display depends on the locale use the Taiwan (zh-TW) codes, as
// Excel shows them on a Traditional Chinese system.
func Builtin(id int) (string, bool) {
	code, ok := builtinCodes[id]
	return code, ok
}

var builtinCodes = map[int]string{
	0:  "",
	1:  "0",
	2:  "0.00",
	3:  "#,##0",
	4:  "#,##0.00",
	5:  `"$"#,##0_);\("$"#,##0\)`,
	6:  `"$"#,##0_);[Red]\("$"#,##0\)`,
	7:  `"$"#,##0.00_);\("$"#,##0.00\)`,
	8:  `"$"#,##0.00_);[Red]\("$"#,##0.00\)`,
	9:  "0%",
	10: "0.00%",
	11: "0.00E+00",
	12: "# ?/?",
	13: "# ??/??",
	14: "yyyy/m/d",
	15: "d-mmm-yy",
	16: "d-mmm",
	17: "mmm-yy",
	18: "h:mm AM/PM",
	19: "h:mm:ss AM/PM",
	20: "h:mm",
	21: "h:mm:ss",
	22: "yyyy/m/d h:mm",
	27: `[$-404]e/m/d`,
	28: `[$-404]e"年"m"月"d"日"`,
	29: `[$-404]e"年"m"月"d"日"`,
	30: "m/d/yy",
	31: `yyyy"年"m"月"d"日"`,
	32: `hh"時"mm"分"`,
	33: `hh"時"mm"分"ss"秒"`,
	34: `上午/下午 hh"時"mm"分"`,
	35: `上午/下午 hh"時"mm"分"ss"秒"`,
	36: `[$-404]e/m/d`,
	37: "#,##0 ;(#,##0)",
	38: "#,##0 ;[Red](#,##0)",
	39: "#,##0.00;(#,##0.00)",
	40: "#,##0.00;[Red](#,##0.00)",
	45: "mm:ss",
	46: "[h]:mm:ss",
	47: "mm:ss.0",
	48: "##0.0E+0",
	49: "@",
	50: `[$-404]e/m/d`,
	51: `[$-404]e"年"m"月"`,
	52: `上午/下午 hh"時"mm"分"`,
	53: `上午/下午 hh"時"mm"分"ss"秒"`,
	54: `[$-404]e"年"m"月"d"日"`,
	55: `上午/下午 hh"時"mm"分"`,
	56: `上午/下午 hh"時"mm"分"ss"秒"`,
	57: `[$-404]e/m/d`,
	58: `[$-404]e"年"m"月"d"日"`,
}

// BuiltinID returns the built-in ID to store a format code under, if it
// has a portable one. Currency (5-8) and locale-specific IDs are stored
// as custom codes instead: their meaning depends on the reader's locale
// and excelize drops them when writing.
func BuiltinID(code string) (int, bool) {
	for id, c := range builtinCodes {
		if c == code && portable(id) {
			return id, true
		}
	}
	return 0, false
}

func portable(id int) bool {
	return id >= 1 && id <= 4 || id >= 9 && id <= 22 || id >= 37 && id <= 40 || id >= 45 && id <= 49
}
