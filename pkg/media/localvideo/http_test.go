package localvideo

import (
	"github.com/ygelfand/libcountertop/pkg/media/cast"
	"github.com/ygelfand/libcountertop/pkg/media/urlplayer"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

type fake struct {
	loads    int
	stops    int
	seeks    int
	snapshot urlplayer.Snapshot
}

func (f *fake) Load(cast.Media, time.Duration, bool) error { f.loads++; return nil }
func (f *fake) Play() error                                { return nil }
func (f *fake) Pause() error                               { return nil }
func (f *fake) Stop() error                                { f.stops++; return nil }
func (f *fake) Seek(time.Duration) error                   { f.seeks++; return nil }
func (f *fake) Snapshot() urlplayer.Snapshot               { return f.snapshot }
func TestURLValidation(t *testing.T) {
	for _, raw := range []string{"file:///etc/passwd", "ftp://example.com/a", "https:///x", "https://user:pass@example.com/a", "https://example.com/a#x", "javascript:alert(1)"} {
		if ValidURL(raw) {
			t.Errorf("accepted %q", raw)
		}
	}
	for _, raw := range []string{"http://example.com/a.mp4", "https://example.com/live.m3u8?token=opaque", "http://localhost:9000/video"} {
		if !ValidURL(raw) {
			t.Errorf("rejected %q", raw)
		}
	}
}
func TestCommandsRejectMalformedOrUnavailableActions(t *testing.T) {
	f := &fake{}
	h := New(f)
	for _, body := range []string{`{"action":"load","url":"file:///etc/passwd"}`, `{"action":"seek","position":-1}`, `{"action":"seek","position":1e30}`, `{"action":"load","url":"http://example.com/a","extra":1}`, `{"action":"stop"} {}`, `{"action":"other"}`} {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest("POST", "/", strings.NewReader(body)))
		if w.Code != 400 {
			t.Errorf("%s: %d", body, w.Code)
		}
	}
	if f.loads+f.stops+f.seeks != 0 {
		t.Fatal("invalid request changed playback")
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("POST", "/", strings.NewReader(`{"action":"seek","position":3}`)))
	if w.Code != 409 {
		t.Fatalf("nonseekable: %d", w.Code)
	}
}
func TestLoadStatusStopAndSeek(t *testing.T) {
	f := &fake{snapshot: urlplayer.Snapshot{State: "playing", Seekable: true}}
	h := New(f)
	for _, body := range []string{`{"action":"load","url":"https://example.com/a.mp4"}`, `{"action":"seek","position":3}`, `{"action":"stop"}`} {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest("POST", "/", strings.NewReader(body)))
		if w.Code != 204 {
			t.Fatalf("%d %s", w.Code, w.Body.String())
		}
	}
	if f.loads != 1 || f.stops != 1 || f.seeks != 1 {
		t.Fatal("commands not dispatched")
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("GET", "/", nil))
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"state":"playing"`) || w.Header().Get("Cache-Control") != "no-store" {
		t.Fatal(w.Body.String())
	}
	w = httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("DELETE", "/", nil))
	if w.Code != 405 {
		t.Fatal(w.Code)
	}
}
