package schema

import (
	"encoding/json"
	"testing"
	"time"
)

func TestDatePatterns(t *testing.T) {
	at := time.Date(2026, 3, 3, 23, 0, 0, 0, time.FixedZone("local", 3600))
	for _, tc := range []struct{ pattern, want string }{
		{"dd.MM.yyyy", "03.03.2026"}, {"MM/dd/yy", "03/03/26"}, {"yyyy-MM-dd", "2026-03-03"},
		{"d MMMM yyyy", "3 March 2026"}, {"EEE, d MMM yyyy", "Tue, 3 Mar 2026"},
		{"'Day' d 'of' MMMM", "Day 3 of March"}, {"d '' MM", "3 ' 03"},
	} {
		got, err := PatternDate(at, tc.pattern)
		if err != nil || got != tc.want {
			t.Errorf("%s: %q, %v; want %q", tc.pattern, got, err, tc.want)
		}
	}
	for _, p := range []string{"", " ", "YYYY-MM-DD", "yyyy '", "HH:mm", "'nothing'"} {
		if _, err := PatternDate(at, p); err == nil {
			t.Errorf("accepted %q", p)
		}
	}
}
func TestClockDateCompatibility(t *testing.T) {
	at := time.Date(2026, 3, 3, 0, 0, 0, 0, time.UTC)
	var c Clock
	if err := json.Unmarshal([]byte("{\"date\":false}"), &c); err != nil {
		t.Fatal(err)
	}
	if c.FormatDate(at) == "" || c.Date {
		t.Fatal("legacy date visibility or format changed")
	}
	c.DateFormat = "custom"
	c.DatePattern = "dd.MM.yyyy"
	b, _ := json.Marshal(c)
	var restored Clock
	json.Unmarshal(b, &restored)
	if restored.FormatDate(at) != "03.03.2026" || restored.Date {
		t.Fatal("round trip changed settings")
	}
	c.DatePattern = "unsupported"
	if c.FormatDate(at) == "" {
		t.Fatal("invalid stored pattern must fall back")
	}
}
