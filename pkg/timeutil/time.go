package timeutil

import "time"

// Now returns the current time in UTC
// Always use this instead of time.Now() to ensure timezone consistency
func Now() time.Time {
	return time.Now().UTC()
}

// ParseDate parses a date string and returns a UTC time
func ParseDate(layout, value string) (time.Time, error) {
	t, err := time.Parse(layout, value)
	if err != nil {
		return time.Time{}, err
	}
	return t.UTC(), nil
}

// StartOfDay returns the start of the day (midnight) in UTC
func StartOfDay(t time.Time) time.Time {
	year, month, day := t.UTC().Date()
	return time.Date(year, month, day, 0, 0, 0, 0, time.UTC)
}

// EndOfDay returns the end of the day (23:59:59.999999999) in UTC
func EndOfDay(t time.Time) time.Time {
	year, month, day := t.UTC().Date()
	return time.Date(year, month, day, 23, 59, 59, 999999999, time.UTC)
}

// ToUTC converts a time.Time to UTC if it isn't already
func ToUTC(t time.Time) time.Time {
	return t.UTC()
}
