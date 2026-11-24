package webreceiver

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ygelfand/libcountertop/pkg/media/cast"
	"github.com/ygelfand/libcountertop/pkg/media/urlplayer"
)

func TestYouTubeValidationAndCancelledLoadCannotOverwriteState(t *testing.T) {
	y := NewYouTube(cast.Env{Output: quietOutput{}}, Options{})
	if err := y.Load(
		cast.Media{ContentID: "https://youtube.com/playlist?list=x"},
		0,
		true,
	); err == nil ||
		y.Active() {
		t.Fatal("accepted playlist")
	}
	if err := y.Load(cast.Media{ContentID: "https://youtu.be/M7lc1UVf-VE"}, 0, true); err != nil {
		t.Fatal(err)
	}
	if y.Snapshot().State != "loading" {
		t.Fatal("no loading status")
	}
	if err := y.Stop(); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	y.fail(ctx, "stale failure")
	if y.Snapshot().State != "idle" || y.Active() {
		t.Fatal("cancelled load changed status")
	}
	y.mu.Lock()
	y.status = urlplayer.Snapshot{State: "playing"}
	y.mu.Unlock()
	y.fail(ctx, "old error")
	if y.Snapshot().State != "playing" {
		t.Fatal("old failure replaced new status")
	}
	if err := y.Pause(); err == nil {
		t.Fatal("idle pause accepted")
	}
}
func TestYouTubeWrapperAndSanitizedErrors(t *testing.T) {
	y := NewYouTube(cast.Env{Output: quietOutput{}}, Options{})
	for _, tc := range []struct {
		path string
		code int
	}{{"/youtube?v=M7lc1UVf-VE", 200}, {"/youtube?v=%22bad", 400}} {
		w := httptest.NewRecorder()
		y.serve(w, httptest.NewRequest("GET", tc.path, nil))
		if w.Code != tc.code {
			t.Fatal(w.Code)
		}
		if tc.code == 200 &&
			(!strings.Contains(w.Body.String(), "https://www.youtube.com/iframe_api") || !strings.Contains(w.Body.String(), "origin:location.origin")) {
			t.Fatal("official embed/origin missing")
		}
	}
	for _, code := range []int{2, 5, 100, 101, 150, 152, 153, -1, -2, 999} {
		s := youtubeError(code)
		if s == "" || strings.Contains(s, "http") {
			t.Fatal("missing/unsafe error", code)
		}
	}
}

func TestWebsiteUsesBrowserIdentityAndNoCastPlatform(t *testing.T) {
	ua := youtubeUserAgent()
	if ua == "" || strings.Contains(ua, "CrKey") || !strings.Contains(ua, "Chrome/") {
		t.Fatal("website does not have a browser identity")
	}
	normal := mediaHooks(cast.Env{}, 8013, false)
	castHooks := hooks(cast.Env{}, 8012)
	if !strings.Contains(normal, "if (false)") || !strings.Contains(castHooks, "if (true)") {
		t.Fatal("Cast platform emulation must not leak into ordinary YouTube playback")
	}
	for _, expected := range []string{"document.querySelector", "playVideo()", "pauseVideo()", "seekTo(t)", "site:true", "advertising()"} {
		if !strings.Contains(youtubeSiteJS, expected) {
			t.Fatalf("missing browser bridge contract %q", expected)
		}
	}
	y := NewYouTube(cast.Env{Output: quietOutput{}}, Options{})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := y.openWebsite(ctx, "cJjMmDeOAZ4", 0, true); err == nil {
		t.Fatal("navigation with no live page accepted")
	}
}
