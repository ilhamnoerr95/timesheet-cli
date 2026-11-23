package timesheet

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var clockRe = regexp.MustCompile(`^(\d{1,2})[:.](\d{2})$`)

// ParseClock normalises a time typed by a person to "HH:MM".
// Accepts 7:30, 07:30 and 7.30 (a dot is a common way to type it), and "now".
// An empty string or "-" means "no time" and returns "".
func ParseClock(s string, now time.Time) (string, error) {
	s = strings.ToLower(strings.TrimSpace(s))
	switch s {
	case "", "-":
		return "", nil
	case "now":
		return now.Format("15:04"), nil
	}
	m := clockRe.FindStringSubmatch(s)
	if m == nil {
		return "", fmt.Errorf("time %q: use HH:MM, e.g. 07:30 (or 'now')", s)
	}
	h, _ := strconv.Atoi(m[1])
	min, _ := strconv.Atoi(m[2])
	if h > 23 || min > 59 {
		return "", fmt.Errorf("time %q is out of range", s)
	}
	return fmt.Sprintf("%02d:%02d", h, min), nil
}
