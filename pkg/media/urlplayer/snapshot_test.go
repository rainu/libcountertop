package urlplayer

import (
	"errors"
	"github.com/ygelfand/libcountertop/pkg/media/cast"
	"strings"
	"testing"
	"time"
)

func TestSnapshotTracksPauseFailureAndStopWithoutURLs(t *testing.T) {
	f := &fakeOutput{}
	p := New(Options{Output: f})
	if p.Snapshot().State != "idle" {
		t.Fatal("not idle")
	}
	raw := serve(t, "audio/wav", wavFile(outRate, ramp(outRate)), true)
	if err := p.Load(cast.Media{ContentID: raw}, 0, true); err != nil {
		t.Fatal(err)
	}
	f.wait(t, "play")
	if s := p.Snapshot(); s.State != "playing" || !s.Seekable {
		t.Fatalf("%+v", s)
	}
	_ = p.Pause()
	if p.Snapshot().State != "paused" {
		t.Fatal("not paused")
	}
	_ = p.Play()
	if p.Snapshot().State != "playing" {
		t.Fatal("not playing")
	}
	p.showing().Fail(errors.New("private-url?token=secret"))
	s := p.Snapshot()
	if s.State != "error" || strings.Contains(s.Error, "secret") {
		t.Fatalf("%+v", s)
	}
	_ = p.Stop()
	if p.Snapshot().State != "idle" {
		t.Fatal("not idle after stop")
	}
}
func TestSnapshotReportsOpenFailure(t *testing.T) {
	p := New(Options{Output: &fakeOutput{}})
	if err := p.Load(cast.Media{ContentID: serve(t, "text/html", []byte("<html>not a video</html>"), false)}, 0, true); err != nil {
		t.Fatal(err)
	}
	defer p.Stop()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if s := p.Snapshot(); s.State == "error" {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("opening failure not visible")
}
