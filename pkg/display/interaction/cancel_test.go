package interaction

import (
	gogui "github.com/go-gui-org/go-gui/gui"
	"testing"
)

func TestCancelNeverFiresPendingPress(t *testing.T) {
	w := gogui.SimpleWindow("cancel", 400, 400, &struct{}{}, func(w *gogui.Window) {
		w.SetView(func(*gogui.Window) gogui.View { return gogui.Column(gogui.ContainerCfg{}) })
	})
	fired := false
	p := New()
	p.armed = "button"
	p.pending = func(gogui.EventCtx) { fired = true }
	p.wave.gen = 3
	p.Cancel(w)
	p.Lift(w)
	if fired || p.armed != "" || p.pending != nil || p.wave.gen == 3 {
		t.Fatal("cancel retained action")
	}
}
