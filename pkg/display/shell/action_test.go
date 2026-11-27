package shell

import "testing"

func TestActionDefaultAndDynamicEnablement(t *testing.T) {
	n := 0
	a := Action{Do: func() { n++ }}
	if !a.IsEnabled() {
		t.Fatal("existing actions must default to enabled")
	}
	a.Run()
	enabled := false
	a.Enabled = func() bool { return enabled }
	a.Run()
	if a.IsEnabled() || n != 1 {
		t.Fatal("disabled action dispatched")
	}
	enabled = true
	a.Run()
	if n != 2 {
		t.Fatal("enabled action did not dispatch")
	}
	// Render-time state may be stale: dispatch must recheck.
	if !a.IsEnabled() {
		t.Fatal("enabled")
	}
	enabled = false
	a.Run()
	if n != 2 {
		t.Fatal("stale action dispatched")
	}
	Action{}.Run()
}
