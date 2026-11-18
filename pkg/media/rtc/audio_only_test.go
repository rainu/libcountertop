package rtc

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestAudioOnlyOfferHasNoDataChannel(t *testing.T) {
	l, e := NewAudioOnly(Media{Audio: &tone{freq: 440}}, nil)
	if e != nil {
		t.Fatal(e)
	}
	defer l.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	offer, e := l.Offer(ctx)
	if e != nil {
		t.Fatal(e)
	}
	if !strings.Contains(offer, "m=audio ") || strings.Contains(offer, "m=application ") || strings.Contains(offer, "m=video ") {
		t.Fatal("not an audio-only offer")
	}
	if e = l.Send("status", nil); e != ErrNotOpen {
		t.Fatal("unexpected control channel")
	}
}
