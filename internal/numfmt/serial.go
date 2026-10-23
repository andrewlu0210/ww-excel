package numfmt

import (
	"math"
	"time"
)

// Excel's 1900 date system: serial 1 is 1900-01-01 and serial 60 is the
// non-existent 1900-02-29 (kept for Lotus 1-2-3 compatibility). From
// serial 61 (1900-03-01) on, serials count days since 1899-12-30.

var epoch = time.Date(1899, 12, 30, 0, 0, 0, 0, time.UTC)

// DateSerial converts a calendar date to a serial. Months and days
// overflow like Excel's DATE (month 13 is January of the next year).
func DateSerial(y, m, d int) float64 {
	if y == 1900 && m == 2 && d == 29 {
		return 60
	}
	t := time.Date(y, time.Month(m), d, 0, 0, 0, 0, time.UTC)
	days := math.Round(t.Sub(epoch).Hours() / 24)
	if days < 61 {
		days-- // dates before 1900-03-01 sit before the phantom leap day
	}
	return days
}

// TimeSerial converts a wall-clock time to a serial (date part + fraction).
func TimeSerial(t time.Time) float64 {
	day := DateSerial(t.Year(), int(t.Month()), t.Day())
	secs := t.Hour()*3600 + t.Minute()*60 + t.Second()
	return day + float64(secs)/86400 + float64(t.Nanosecond())/86400e9
}

// YMD returns the calendar date of a serial, including Excel's special
// cases: serial 0 is 1900-01-00 and 60 is 1900-02-29.
func YMD(serial float64) (y, m, d int) {
	days := int(math.Floor(serial))
	switch {
	case days == 0:
		return 1900, 1, 0
	case days == 60:
		return 1900, 2, 29
	case days < 60:
		days++
	}
	t := epoch.AddDate(0, 0, days)
	return t.Year(), int(t.Month()), t.Day()
}

// HMS returns the time of day of a serial, rounded to the nearest second.
func HMS(serial float64) (h, m, s int) {
	frac := serial - math.Floor(serial)
	secs := int(math.Round(frac * 86400))
	if secs >= 86400 {
		secs = 86399
	}
	return secs / 3600, secs / 60 % 60, secs % 60
}

// Weekday of a serial, 0 = Sunday. Serial 1 (1900-01-01) is a Sunday in
// Excel's calendar because of the phantom leap day.
func Weekday(serial float64) int {
	days := int(math.Floor(serial))
	return ((days-1)%7 + 7) % 7
}
