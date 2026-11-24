package localvideo

import (
	"github.com/ygelfand/libcountertop/pkg/media/cast"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestRoutingAndRejectedLoadKeepCurrentSource(t *testing.T) {
	d, y := &fake{}, &fake{}
	r := Route(d, y)
	if err := r.Load(cast.Media{ContentID: "https://example.com/video.mp4"}, 0, true); err != nil || d.loads != 1 || y.loads != 0 {
		t.Fatal("direct route", err)
	}
	if err := r.Load(cast.Media{ContentID: "https://music.youtube.com/watch?v=M7lc1UVf-VE"}, 0, true); err != nil || y.loads != 1 || d.stops != 2 {
		t.Fatal("YouTube route", err, d.stops)
	}
	stops := y.stops
	if err := r.Load(cast.Media{ContentID: "https://youtube.com/playlist?list=x"}, 0, true); err == nil || y.stops != stops {
		t.Fatal("invalid load disrupted source")
	}
	r.Seek(time.Second)
	r.Stop()
	if y.seeks != 1 || y.stops != stops+1 {
		t.Fatal("controls addressed wrong player")
	}
	r.Load(cast.Media{ContentID: "https://example.com/audio.wav"}, 0, true)
	if d.loads != 2 || y.stops != stops+2 {
		t.Fatal("return to direct source")
	}
}
func TestUnsupportedYouTubeHasActionableHTTPError(t *testing.T) {
	h := New(Route(&fake{}, &fake{}))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("POST", "/", strings.NewReader(`{"action":"load","url":"https://youtube.com/playlist?list=unsupported"}`)))
	if w.Code != 400 || !strings.Contains(w.Body.String(), "single public video") {
		t.Fatal(w.Code, w.Body.String())
	}
}
