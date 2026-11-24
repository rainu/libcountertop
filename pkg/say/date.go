package say

import (
	"strings"
	"time"
)

// Date formats Go time layout names in the selected UI language.
// Numeric fields, time zones and literal text retain time.Format semantics.
func Date(at time.Time, layout string) string {
	var out strings.Builder
	for len(layout) > 0 {
		matched := false
		for _, token := range []string{"January", "Monday", "Jan", "Mon"} {
			if !strings.HasPrefix(layout, token) {
				continue
			}
			if len(token) == 3 && len(layout) > 3 && layout[3] >= 'a' && layout[3] <= 'z' {
				continue
			}
			var key string
			switch token {
			case "January":
				key = "date.month." + strings.ToLower(at.Month().String())
			case "Jan":
				key = "date.month_short." + strings.ToLower(at.Month().String())
			case "Monday":
				key = "date.weekday." + strings.ToLower(at.Weekday().String())
			case "Mon":
				key = "date.weekday_short." + strings.ToLower(at.Weekday().String())
			}
			out.WriteString(T(key))
			layout = layout[len(token):]
			matched = true
			break
		}
		if matched {
			continue
		}
		n := 1
		for n < len(layout) && !strings.HasPrefix(layout[n:], "Jan") && !strings.HasPrefix(layout[n:], "Mon") {
			n++
		}
		out.WriteString(at.Format(layout[:n]))
		layout = layout[n:]
	}
	return out.String()
}
