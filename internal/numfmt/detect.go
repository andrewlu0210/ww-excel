package numfmt

import (
	"regexp"
	"strconv"
	"strings"
	"time"
)

var (
	reThousands = regexp.MustCompile(`^-?\d{1,3}(,\d{3})+(\.\d+)?$`)
	reCurrency  = regexp.MustCompile(`^(-?)\$(-?)(\d{1,3}(,\d{3})*|\d+)(\.\d+)?$`)
	rePercent   = regexp.MustCompile(`^-?(\d+(\.\d*)?|\.\d+)%$`)
	reDate      = regexp.MustCompile(`^(\d{4})[/-](\d{1,2})[/-](\d{1,2})(?:\s+(\d{1,2}):(\d{2})(?::(\d{2}))?)?$`)
	reMonthDay  = regexp.MustCompile(`^(\d{1,2})/(\d{1,2})$`)
	reTime      = regexp.MustCompile(`^(\d{1,2}):(\d{2})(?::(\d{2}))?(?:\s*([AaPp][Mm]))?$`)
)

// Detect recognizes typed text that Excel converts to a number with a
// format: 12% (percent), 1,234 (thousands), $12.50 (currency),
// 2026/10/23 or 2026-10-23 (date, optionally with a time), 10/23
// (date in the current year) and 13:05 or 1:05 PM (time). It returns
// the value and the format code to apply.
func Detect(text string, now time.Time) (float64, string, bool) {
	s := strings.TrimSpace(text)
	if s == "" {
		return 0, "", false
	}
	if rePercent.MatchString(s) {
		n, err := strconv.ParseFloat(strings.TrimSuffix(s, "%"), 64)
		if err != nil {
			return 0, "", false
		}
		code := "0%"
		if strings.Contains(s, ".") {
			code = "0.00%"
		}
		return n / 100, code, true
	}
	if reThousands.MatchString(s) {
		n, err := strconv.ParseFloat(strings.ReplaceAll(s, ",", ""), 64)
		if err != nil {
			return 0, "", false
		}
		code := "#,##0"
		if strings.Contains(s, ".") {
			code = "#,##0.00"
		}
		return n, code, true
	}
	if m := reCurrency.FindStringSubmatch(s); m != nil {
		n, err := strconv.ParseFloat(strings.ReplaceAll(m[3], ",", "")+m[5], 64)
		if err != nil {
			return 0, "", false
		}
		if m[1] == "-" || m[2] == "-" {
			n = -n
		}
		code := `"$"#,##0_);[Red]\("$"#,##0\)`
		if m[5] != "" {
			code = `"$"#,##0.00_);[Red]\("$"#,##0.00\)`
		}
		return n, code, true
	}
	if m := reDate.FindStringSubmatch(s); m != nil {
		y, _ := strconv.Atoi(m[1])
		mo, _ := strconv.Atoi(m[2])
		d, _ := strconv.Atoi(m[3])
		if !validDate(y, mo, d) {
			return 0, "", false
		}
		v := DateSerial(y, mo, d)
		if m[4] == "" {
			return v, "yyyy/m/d", true
		}
		h, _ := strconv.Atoi(m[4])
		mi, _ := strconv.Atoi(m[5])
		sec, _ := strconv.Atoi(m[6])
		if h > 23 || mi > 59 || sec > 59 {
			return 0, "", false
		}
		return v + float64(h*3600+mi*60+sec)/86400, "yyyy/m/d h:mm", true
	}
	if m := reMonthDay.FindStringSubmatch(s); m != nil {
		mo, _ := strconv.Atoi(m[1])
		d, _ := strconv.Atoi(m[2])
		if !validDate(now.Year(), mo, d) {
			return 0, "", false
		}
		return DateSerial(now.Year(), mo, d), `m"月"d"日"`, true
	}
	if m := reTime.FindStringSubmatch(s); m != nil {
		h, _ := strconv.Atoi(m[1])
		mi, _ := strconv.Atoi(m[2])
		sec, _ := strconv.Atoi(m[3])
		ampm := strings.ToUpper(m[4])
		if mi > 59 || sec > 59 {
			return 0, "", false
		}
		if ampm != "" {
			if h < 1 || h > 12 {
				return 0, "", false
			}
			h %= 12
			if ampm == "PM" {
				h += 12
			}
		}
		if h > 23 {
			return 0, "", false
		}
		v := float64(h*3600+mi*60+sec) / 86400
		code := "h:mm"
		switch {
		case ampm != "" && m[3] != "":
			code = "h:mm:ss AM/PM"
		case ampm != "":
			code = "h:mm AM/PM"
		case m[3] != "":
			code = "h:mm:ss"
		}
		return v, code, true
	}
	return 0, "", false
}

func validDate(y, m, d int) bool {
	if y < 1900 || y > 9999 || m < 1 || m > 12 || d < 1 {
		return false
	}
	return d <= time.Date(y, time.Month(m)+1, 0, 0, 0, 0, 0, time.UTC).Day()
}
