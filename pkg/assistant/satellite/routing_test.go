package satellite

import "testing"

func TestAlternateRouteNeverStartsHAPipeline(t *testing.T) {
	s, _, _, _, _, _ := fixture("first", "second")
	var slots []int
	s.options.Route = func(slot int, heard bool) bool { slots = append(slots, slot); return true }
	s.Start(0)
	s.Heard(1)
	s.Action()
	s.ActionHold(1)
	if s.Turn().Pending() || s.Turn().Busy() {
		t.Fatal("alternate route entered HA")
	}
	if len(slots) != 4 || slots[0] != 0 || slots[1] != 1 || slots[2] != 0 || slots[3] != 1 {
		t.Fatal("activation slot lost")
	}
	s.options.StopAlternate = func() bool { return true }
	if !s.Stop() {
		t.Fatal("alternate stop was not consumed")
	}
}
func TestUnconsumedRouteKeepsHAStart(t *testing.T) {
	s, _, _, _, _, _ := fixture("first")
	s.options.Route = func(int, bool) bool { return false }
	s.Start(0)
	if !s.Turn().Pending() {
		t.Fatal("HA activation lost before event processing")
	}
}
