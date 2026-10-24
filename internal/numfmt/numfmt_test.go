package numfmt

import (
	"math"
	"testing"
	"time"
)

func TestDateSerial(t *testing.T) {
	tests := []struct {
		y, m, d int
		want    float64
	}{
		{1900, 1, 1, 1},
		{1900, 2, 28, 59},
		{1900, 2, 29, 60}, // Excel's phantom leap day
		{1900, 3, 1, 61},
		{2026, 10, 23, 46318},
		{2026, 13, 1, 46388}, // month overflow -> 2027-01-01
		{2024, 2, 29, 45351},
	}
	for _, tt := range tests {
		if got := DateSerial(tt.y, tt.m, tt.d); got != tt.want {
			t.Errorf("DateSerial(%d,%d,%d) = %v, want %v", tt.y, tt.m, tt.d, got, tt.want)
		}
		if tt.m <= 12 {
			if y, m, d := YMD(tt.want); y != tt.y || m != tt.m || d != tt.d {
				t.Errorf("YMD(%v) = %d-%d-%d", tt.want, y, m, d)
			}
		}
	}
	if y, m, d := YMD(0); y != 1900 || m != 1 || d != 0 {
		t.Errorf("YMD(0) = %d-%d-%d, want 1900-1-0", y, m, d)
	}
	if Weekday(1) != 0 || Weekday(46318) != 5 {
		t.Errorf("weekday: 1 -> %d (Sun), 46318 -> %d (Fri)", Weekday(1), Weekday(46318))
	}
	if h, m, s := HMS(0.75 + 1.0/86400*30); h != 18 || m != 0 || s != 30 {
		t.Errorf("HMS = %d:%d:%d", h, m, s)
	}
}

func TestGeneralAndRound(t *testing.T) {
	general := []struct {
		in   float64
		want string
	}{
		{0.1 + 0.2, "0.3"},
		{1234.5, "1234.5"},
		{-0.000123, "-0.000123"},
		{1e21, "1E+21"},
		{123456789012345678, "1.23457E+17"},
		{1.5e-10, "1.5E-10"},
		{100, "100"},
		{1.0 / 3, "0.333333333333333"},
	}
	for _, tt := range general {
		if got := General(tt.in); got != tt.want {
			t.Errorf("General(%v) = %q, want %q", tt.in, got, tt.want)
		}
	}

	rounding := []struct {
		in   float64
		d    int
		want float64
	}{
		{2.675, 2, 2.68},
		{2.5, 0, 3},
		{-2.5, 0, -3},
		{1234.5678, -2, 1200},
		{0.125, 2, 0.13},
		{9.995, 2, 10},
		{5, -1, 10},
		{4, -1, 0},
	}
	for _, tt := range rounding {
		if got := Round(tt.in, tt.d); math.Abs(got-tt.want) > 1e-12 {
			t.Errorf("Round(%v, %d) = %v, want %v", tt.in, tt.d, got, tt.want)
		}
	}
}

func TestFormat(t *testing.T) {
	date := DateSerial(2026, 10, 23) + (13*3600+5*60+9)/86400.0
	tests := []struct {
		v    float64
		code string
		want string
	}{
		{1234.567, "#,##0.00", "1,234.57"},
		{-1234.5, "#,##0", "-1,235"},
		{1234567.891, "#,##0", "1,234,568"},
		{0.5, "0%", "50%"},
		{0.1234, "0.0%", "12.3%"},
		{5, "000", "005"},
		{0.123, "#.##", ".12"},
		{2.5, "#.##", "2.5"},
		{3, "#.##", "3."},
		{2.675, "0.00", "2.68"},
		{-5, "0.0;(0.0)", "(5.0)"},
		{0, `0;-0;"zero"`, "zero"},
		{1234567, "#,##0,", "1,235"},
		{12345678, `0.0,,"M"`, "12.3M"},
		{12345.678, "0.00E+00", "1.23E+04"},
		{0.000123, "0.0E+00", "1.2E-04"},
		{99999, "0.00E+00", "1.00E+05"},
		{150, "[>100]\"big\";\"small\"", "big"},
		{1234, "General", "1234"},
		{42, `"NT$"#,##0`, "NT$42"},
		{7, `0\ "個"`, "7 個"},
		{0.5, "# ?/?", "0 1/2"}, // fractions not supported; must not panic
		{date, "yyyy/mm/dd", "2026/10/23"},
		{date, "yyyy-mm-dd hh:mm:ss", "2026-10-23 13:05:09"},
		{date, "m/d/yy", "10/23/26"},
		{date, "mmm d, yyyy", "Oct 23, 2026"},
		{date, "mmmm", "October"},
		{date, "dddd", "Friday"},
		{date, "ddd", "Fri"},
		{date, "h:mm AM/PM", "1:05 PM"},
		{date, "hh:mm", "13:05"},
		{1.5, "[h]:mm", "36:00"},
	}
	for _, tt := range tests {
		got := Format(tt.v, tt.code)
		if tt.code == "# ?/?" {
			continue // only checks it does not panic
		}
		if got != tt.want {
			t.Errorf("Format(%v, %q) = %q, want %q", tt.v, tt.code, got, tt.want)
		}
	}
}

func TestFormatText(t *testing.T) {
	if got := FormatText("abc", `0;0;0;"[" @ "]"`); got != "[ abc ]" {
		t.Errorf("4th section = %q", got)
	}
	if got := FormatText("abc", "0.00"); got != "abc" {
		t.Errorf("no text section = %q", got)
	}
	if got := FormatText("abc", `@" 元"`); got != "abc 元" {
		t.Errorf("lone @ section = %q", got)
	}
}

func TestRenderColorsAndLocale(t *testing.T) {
	date := DateSerial(2026, 10, 23) + (15*3600+4*60)/86400.0
	tests := []struct {
		v                 float64
		code, text, color string
	}{
		{-5, "0;[Red]-0", "-5", "FF0000"},
		{5, "0;[Red]-0", "5", ""},
		{-5, "#,##0;[Color3](#,##0)", "(5)", "FF0000"},
		{date, "[$-404]e/m/d", "115/10/23", ""},
		{date, `[$-404]gge"年"m"月"d"日"`, "民國115年10月23日", ""},
		{date, `上午/下午 hh"時"mm"分"`, "下午 03時04分", ""},
		{1234, "[$NT$-404]#,##0", "NT$1,234", ""},
		{0.5, "General", "0.5", ""},
	}
	for _, tt := range tests {
		text, color := Render(tt.v, tt.code)
		if text != tt.text || color != tt.color {
			t.Errorf("Render(%v, %q) = %q %q, want %q %q", tt.v, tt.code, text, color, tt.text, tt.color)
		}
	}
	for id, want := range map[int]string{14: "2026/10/23", 27: "115/10/23", 22: "2026/10/23 15:04"} {
		code, _ := Builtin(id)
		if got := Format(date, code); got != want {
			t.Errorf("builtin %d (%q) = %q, want %q", id, code, got, want)
		}
	}
	if !IsDate("yyyy/m/d") || IsDate("#,##0.00") {
		t.Error("IsDate")
	}
}

func TestDetect(t *testing.T) {
	now := time.Date(2026, 10, 23, 0, 0, 0, 0, time.UTC)
	tests := []struct {
		in   string
		v    float64
		code string
		ok   bool
	}{
		{"12%", 0.12, "0%", true},
		{"12.5%", 0.125, "0.00%", true},
		{"1,234", 1234, "#,##0", true},
		{"-1,234.5", -1234.5, "#,##0.00", true},
		{"12,34", 0, "", false},
		{"$12", 12, `"$"#,##0_);[Red]\("$"#,##0\)`, true},
		{"$1,234.50", 1234.5, `"$"#,##0.00_);[Red]\("$"#,##0.00\)`, true},
		{"2026/10/23", 46318, "yyyy/m/d", true},
		{"2026-1-5", DateSerial(2026, 1, 5), "yyyy/m/d", true},
		{"2026/2/30", 0, "", false},
		{"2026/10/23 13:30", 46318 + 13.5/24, "yyyy/m/d h:mm", true},
		{"10/23", 46318, `m"月"d"日"`, true},
		{"13:05", (13*60 + 5) / 1440.0, "h:mm", true},
		{"1:05 PM", (13*60 + 5) / 1440.0, "h:mm AM/PM", true},
		{"25:00", 0, "", false},
		{"abc", 0, "", false},
		{"12", 0, "", false}, // plain numbers need no format
	}
	for _, tt := range tests {
		v, code, ok := Detect(tt.in, now)
		if ok != tt.ok || code != tt.code || math.Abs(v-tt.v) > 1e-9 {
			t.Errorf("Detect(%q) = %v %q %v, want %v %q %v", tt.in, v, code, ok, tt.v, tt.code, tt.ok)
		}
	}
}

func TestAdjustDecimals(t *testing.T) {
	tests := []struct {
		code  string
		delta int
		want  string
	}{
		{"", 1, "0.0"},
		{"", -1, "0"},
		{"0", 1, "0.0"},
		{"0.00", 1, "0.000"},
		{"0.00", -1, "0.0"},
		{"0.0", -1, "0"},
		{"#,##0", 1, "#,##0.0"},
		{"0%", 2, "0.00%"},
		{`"$"#,##0.00_);[Red]\("$"#,##0.00\)`, -1, `"$"#,##0.0_);[Red]\("$"#,##0.0\)`},
		{`0.0" 元"`, 1, `0.00" 元"`},
		{"yyyy/m/d", 1, "yyyy/m/d"},
	}
	for _, tt := range tests {
		if got := AdjustDecimals(tt.code, tt.delta); got != tt.want {
			t.Errorf("AdjustDecimals(%q, %d) = %q, want %q", tt.code, tt.delta, got, tt.want)
		}
	}
}
