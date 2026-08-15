package parser

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// JakartaLocation is a fixed UTC+7 zone (Indonesia has no DST), used
// instead of the server's OS timezone so bookings parse and display
// correctly regardless of where the app is hosted (Railway/Render
// default to UTC).
var JakartaLocation = time.FixedZone("WIB", 7*60*60)

var indonesianMonths = map[string]time.Month{
	"januari": time.January, "februari": time.February, "maret": time.March,
	"april": time.April, "mei": time.May, "juni": time.June,
	"juli": time.July, "agustus": time.August, "september": time.September,
	"oktober": time.October, "november": time.November, "desember": time.December,
}

// singleDateTimeRe matches "20 Agustus 08:00" (day, month name, hour:minute)
var singleDateTimeRe = regexp.MustCompile(`(?i)(\d{1,2})\s+([a-zA-Z]+)\s+(\d{1,2}):(\d{2})`)

// ParseDateTime parses a single Indonesian date+time like "20 Agustus 08:00".
// Assumes the current year; if the resulting date is more than a month in
// the past, assumes next year instead (handles bookings made near year-end).
func ParseDateTime(input string) (time.Time, error) {
	match := singleDateTimeRe.FindStringSubmatch(input)
	if match == nil {
		return time.Time{}, fmt.Errorf("format tanggal tidak dikenali, gunakan contoh: 20 Agustus 08:00")
	}

	day, _ := strconv.Atoi(match[1])
	monthName := strings.ToLower(match[2])
	hour, _ := strconv.Atoi(match[3])
	minute, _ := strconv.Atoi(match[4])

	month, ok := indonesianMonths[monthName]
	if !ok {
		return time.Time{}, fmt.Errorf("nama bulan tidak dikenali: %s", match[2])
	}

	now := time.Now().In(JakartaLocation)
	year := now.Year()
	result := time.Date(year, month, day, hour, minute, 0, 0, JakartaLocation)

	// If the date is more than ~30 days in the past, assume it's meant
	// for next year (e.g. booking made in December for early January).
	if result.Before(now.AddDate(0, 0, -30)) {
		result = time.Date(year+1, month, day, hour, minute, 0, 0, JakartaLocation)
	}

	return result, nil
}

// ParseDateTimeRange parses "20 Agustus 08:00 - 22 Agustus 17:00" into a
// start and end time.
func ParseDateTimeRange(input string) (start, end time.Time, err error) {
	parts := strings.SplitN(input, "-", 2)
	// Careful: the day number itself might briefly look like it has a
	// hyphen if formatting is inconsistent, so we match datetimes
	// explicitly instead of blindly splitting on every hyphen.
	matches := singleDateTimeRe.FindAllStringSubmatch(input, -1)
	if len(matches) != 2 {
		return time.Time{}, time.Time{}, fmt.Errorf(
			"format rentang waktu tidak dikenali, gunakan contoh: 20 Agustus 08:00 - 22 Agustus 17:00")
	}
	_ = parts // not used directly; kept matches-based parsing for reliability

	start, err = ParseDateTime(matches[0][0])
	if err != nil {
		return time.Time{}, time.Time{}, err
	}
	end, err = ParseDateTime(matches[1][0])
	if err != nil {
		return time.Time{}, time.Time{}, err
	}

	if !end.After(start) {
		return time.Time{}, time.Time{}, fmt.Errorf("waktu kembali harus setelah waktu jemput")
	}

	return start, end, nil
}
