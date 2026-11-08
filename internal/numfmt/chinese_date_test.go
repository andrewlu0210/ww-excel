package numfmt

import "testing"

func TestChineseDateTypes(t *testing.T) {
	date := DateSerial(2012, 3, 14)
	for _, tc := range []struct{ code, want string }{
		{`m"月"d"日"`, "3月14日"},
		{`[DBNum1][$-404]yyyy"年"m"月"d"日"`, "二○一二年三月十四日"},
		{`[DBNum1][$-404]m"月"d"日"`, "三月十四日"},
		{`[$-404]dddd`, "星期三"},
		{`[$-404]ddd`, "週三"},
		{`[$-zh-TW]dddd`, "星期三"},
		{`ddd`, "Wed"},
		{`dddd`, "Wednesday"},
		{`[Red][DBNum1][$-404]yyyy"年"m"月"d"日";@`, "二○一二年三月十四日"},
		{`[DBNum1][$-404]yyyy" year 2020 "m"月"d"日"`, "二○一二 year 2020 三月十四日"},
	} {
		t.Run(tc.code, func(t *testing.T) {
			if !IsDate(tc.code) {
				t.Fatal("not a date format")
			}
			if got := Format(date, tc.code); got != tc.want {
				t.Fatalf("%q want %q", got, tc.want)
			}
		})
	}
	for _, tc := range []struct {
		year, month, day int
		want             string
	}{{2000, 1, 1, "二○○○年一月一日"}, {2010, 10, 10, "二○一○年十月十日"}, {2020, 11, 20, "二○二○年十一月二十日"}, {2024, 2, 29, "二○二四年二月二十九日"}, {2026, 12, 31, "二○二六年十二月三十一日"}} {
		if got := Format(DateSerial(tc.year, tc.month, tc.day), `[DBNum1][$-404]yyyy"年"m"月"d"日"`); got != tc.want {
			t.Fatalf("%s want %s", got, tc.want)
		}
	}
}
func TestTraditionalWeekdays(t *testing.T) {
	for i, suffix := range []string{"日", "一", "二", "三", "四", "五", "六"} {
		day := DateSerial(2012, 3, 11+i)
		if got := Format(day, `[$-404]dddd`); got != "星期"+suffix {
			t.Fatal(got)
		}
		if got := Format(day, `[$-404]ddd`); got != "週"+suffix {
			t.Fatal(got)
		}
	}
}
