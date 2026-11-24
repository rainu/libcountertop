package message

import "testing"

func TestCancelTouchReleasesOnlyItsOwnGrab(t *testing.T) {
	m := &Messages{up: true}
	m.swipe = swipe{id: 12, held: true, dx: 100}
	m.CancelTouch(99)
	if !m.swipe.held {
		t.Fatal("unrelated grab cancelled")
	}
	m.CancelTouch(12)
	if m.swipe.held || m.swipe.dx != 0 || !m.up {
		t.Fatal("cancel did not reset safely")
	}
}
