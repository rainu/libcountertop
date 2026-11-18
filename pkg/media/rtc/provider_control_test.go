package rtc

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestProviderControlOfferUsesBrowserBoundAndSeparateChannel(t *testing.T) {
	l, e := NewControlledAudio(Media{Audio: &tone{freq: 440}, Control: &ControlChannel{Name: "oai-events"}}, nil)
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
	if !strings.Contains(offer, "m=audio ") || !strings.Contains(offer, "m=application ") || !strings.Contains(offer, "a=max-message-size:262144") {
		t.Fatal("provider control offer contract")
	}
	if strings.Contains(offer, "m=video ") {
		t.Fatal("unexpected camera")
	}
	if e = l.SendControl(map[string]any{"type": "session.update"}); e != ErrNotOpen {
		t.Fatal("control sent before open")
	}
}
