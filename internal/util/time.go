package util

import (
	"fmt"
	"regexp"
	"strconv"
	"time"
)

// ParseDuration parses a string like "20 days" or "1m" into a time.Duration.
func ParseDuration(durationStr string) (time.Duration, error) {
	re := regexp.MustCompile(`^(\d+)\s*(hour|day|month|year)s?$`)
	matches := re.FindStringSubmatch(durationStr)

	unit := ""
	valueStr := ""

	if len(matches) == 3 {
		valueStr = matches[1]
		unit = matches[2]
	} else {
		re = regexp.MustCompile(`^(\d+)\s*(h|d|m|y)$`)
		matches = re.FindStringSubmatch(durationStr)
		if len(matches) == 3 {
			valueStr = matches[1]
			unit = matches[2]
		} else {
			return 0, fmt.Errorf("invalid duration format: %s. Use a format like '20 days' or '1m'", durationStr)
		}
	}

	value, err := strconv.Atoi(valueStr)
	if err != nil {
		return 0, err // Should not happen with the regex
	}

	now := time.Now()
	var then time.Time

	switch unit {
	case "h", "hour":
		return time.Duration(value) * time.Hour, nil
	case "d", "day":
		return time.Duration(value) * 24 * time.Hour, nil
	case "m", "month":
		then = now.AddDate(0, -value, 0)
		return now.Sub(then), nil
	case "y", "year":
		then = now.AddDate(-value, 0, 0)
		return now.Sub(then), nil
	default:
		return 0, fmt.Errorf("unknown time unit: %s", unit)
	}
}