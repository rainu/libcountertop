package say

import (
	"testing"
	"time"
)

func TestDate(t *testing.T) {
	old := Chosen()
	t.Cleanup(func() { Use(old) })
	at := time.Date(2026, time.March, 3, 15, 4, 5, 0, time.UTC)
	for _, tt := range []struct{ lang, layout, want string }{
		{"de", "Monday, 2. January", "Dienstag, 3. März"},
		{"de-DE", "Mon, 02 Jan 2006 15:04 MST", "Di., 03 März 2026 15:04 UTC"},
		{"en", "Monday, 2 January", "Tuesday, 3 March"},
		{"it", "Monday, 2 January", "martedì, 3 marzo"},
		{"vi", "Monday, 2 January", "Thứ ba, 3 tháng 3"},
		{"unknown", "Monday, 2 January", "Tuesday, 3 March"},
		{"de", "2006-01-02 / 02.01.2006 / 3:04PM", "2026-03-03 / 03.03.2026 / 3:04PM"},
		{"de", "Month January Janitor Mondayish", "Month März Janitor Dienstagish"},
	} {
		Use(tt.lang)
		if got := Date(at, tt.layout); got != tt.want {
			t.Errorf("%s %q: got %q, want %q", tt.lang, tt.layout, got, tt.want)
		}
	}
	Use("de")
	if got := Date(time.Date(2026, 11, 17, 0, 0, 0, 0, time.UTC), T("date.layout")); got != "Dienstag, 17. November" {
		t.Fatal(got)
	}
	// Language changes must affect subsequent readings without restarting.
	Use("en")
	if got := Date(at, T("date.layout")); got != "Tuesday, 3 March" {
		t.Fatal(got)
	}
}
