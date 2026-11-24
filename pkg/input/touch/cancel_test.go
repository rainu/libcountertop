package touch

import (
	"testing"
	"time"
)

func TestCancelledContactNeverFinishesGesture(t *testing.T) {
	r := NewRecognizer(1280, 800)
	r.Feed(Contact{ID: -12, Phase: Down, X: 100, Y: 100, At: time.Now()})
	r.Feed(Contact{ID: -12, Phase: Move, X: 900, Y: 100, At: time.Now()})
	if _, ok := r.Feed(Contact{ID: -12, Phase: Cancel, At: time.Now()}); ok {
		t.Fatal("cancel emitted gesture")
	}
	if _, ok := r.Feed(Contact{ID: -12, Phase: Up, At: time.Now()}); ok {
		t.Fatal("cancelled finger clicked")
	}
	if len(r.tracking) != 0 {
		t.Fatal("contact leaked")
	}
	r.Feed(down(3, 600, 400))
	if _, ok := r.Feed(up(3, 600, 400)); !ok {
		t.Fatal("fresh touch broken")
	}
}
