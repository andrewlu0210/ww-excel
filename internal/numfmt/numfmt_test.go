package numfmt

import (
	"math"
	"testing"
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
