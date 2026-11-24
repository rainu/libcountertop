package settings

import (
	"github.com/ygelfand/libcountertop/pkg/display/menu"
	"github.com/ygelfand/libcountertop/pkg/say"
	"testing"
)

func TestAlternateConversationRowsReplaceOnlySelectedSlotsPipeline(t *testing.T) {
	p, _ := thresholds()
	p.options.ConversationRows = func(slot int) ([]menu.Row, []func(int), bool) {
		return []menu.Row{{Label: "synthetic route"}}, []func(int){nil}, slot == 0
	}
	for slot := 0; slot < 2; slot++ {
		rows, acts := p.Slot(slot).Build()
		if len(rows) != len(acts) {
			t.Fatal("row action alignment")
		}
		found := false
		for _, row := range rows {
			if row.Label == say.T("assistants.pipeline") {
				found = true
			}
		}
		if found != (slot == 1) {
			t.Fatal("HA pipeline selection was removed from the wrong slot")
		}
		if rows[2].Label != "synthetic route" {
			t.Fatal("route was not integrated alongside wake word")
		}
	}
}

func TestNativeFeedbackHidesOnlySelectedSlotsHAFeedback(t *testing.T) {
	p, _ := thresholds()
	p.options.OwnsFeedback = func(slot int) bool { return slot == 0 }
	for slot := 0; slot < 2; slot++ {
		rows, acts := p.Slot(slot).Build()
		if len(rows) != len(acts) {
			t.Fatal("feedback row/action alignment")
		}
		found := false
		for _, row := range rows {
			if row.Label == say.T("assistants.tone") || row.Label == say.T("assistants.reply") {
				found = true
			}
		}
		if found != (slot == 1) {
			t.Fatal("HA feedback hidden for wrong slot")
		}
	}
}
