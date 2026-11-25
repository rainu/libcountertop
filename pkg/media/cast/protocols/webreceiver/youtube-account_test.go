package webreceiver

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestYouTubeConnectionDoesNotConfuseCodeWithAuthentication(t *testing.T) {
	now := time.Unix(1000, 0)
	p := youtubeAccountProbe{Code: "TEST-CODE", Premium: true}
	s := accountState(p, false, time.Time{}, now)
	if s.State != "disconnected" || s.Code != "" || s.Premium {
		t.Fatal("anonymous status disclosed code or claimed premium")
	}
	s = accountState(p, true, now.Add(time.Minute), now)
	if s.State != "linking" || s.Code == "" || s.Premium {
		t.Fatal("activation is not authentication")
	}
	s = accountState(p, true, now, now)
	if s.State != "expired" || s.Code != "" {
		t.Fatal("expired code retained")
	}
	p.SignedIn = true
	s = accountState(p, true, now.Add(time.Minute), now)
	if s.State != "connected" || !s.Premium || s.Code != "" || s.VerificationURL != "" {
		t.Fatal("connected state retained activation data")
	}
}
func TestYouTubeAccountAPIUsesPostAndNoStore(t *testing.T) {
	a := newYouTubeAccount(nil, func() error { return nil })
	a.set(YouTubeConnection{State: "connected", Premium: true})
	h := a.Handler()
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("GET", "/", nil))
	if w.Code != 200 || w.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("cacheable metadata")
	}
	var s YouTubeConnection
	if json.Unmarshal(w.Body.Bytes(), &s) != nil || s.State != "connected" {
		t.Fatal("bad status")
	}
	for _, method := range []string{"PUT", "DELETE", "HEAD"} {
		w = httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest(method, "/", nil))
		if w.Code != 405 {
			t.Fatal("unsafe method", method)
		}
	}
	for _, body := range []string{`{"action":"token-export"}`, `{"action":"connect","password":"synthetic"}`} {
		w = httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest("POST", "/", strings.NewReader(body)))
		if w.Code < 400 {
			t.Fatal("accepted unsupported credential operation")
		}
	}
	w = httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("POST", "/", strings.NewReader(`{"action":"connect"}`)))
	if w.Code != 202 || <-a.commands != "connect" {
		t.Fatal("missing queued action")
	}
}
