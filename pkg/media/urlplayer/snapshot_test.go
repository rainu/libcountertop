package urlplayer

import (
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/ygelfand/libcountertop/pkg/media/cast"
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
	if err := p.Load(
		cast.Media{ContentID: serve(t, "text/html", []byte("<html>not a video</html>"), false)},
		0,
		true,
	); err != nil {
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

// Native transport controls need the decoded duration when no Cast sender supplied metadata.
func TestNativeControlsUseDecodedDurationWithoutSenderMetadata(t *testing.T) {
	body, err := os.ReadFile("testdata/clip.mp4")
	if err != nil {
		t.Fatal(err)
	}
	tr := newTrack(serve(t, "video/mp4", body, true), 0, nil)
	defer tr.close()
	if err := tr.ensure(); err != nil {
		t.Fatal(err)
	}
	if got := tr.Showing().Length; got < 2*time.Second || got > 2100*time.Millisecond {
		t.Fatalf("native duration %v, want approximately two seconds", got)
	}
	// UI/transport metadata must not wait on a decoder read held under tr.mu.
	tr.mu.Lock()
	done := make(chan struct{})
	go func() { _ = tr.Showing(); close(done) }()
	select {
	case <-done:
		tr.mu.Unlock()
	case <-time.After(time.Second):
		tr.mu.Unlock()
		t.Fatal("native metadata blocked on decoder mutex")
	}
	tr.media.Duration = 3
	if got := tr.Showing().Length; got != 3*time.Second {
		t.Fatalf("sender duration was not preserved: %v", got)
	}
}

// A web/HA status poll must complete even while decoder input is stalled or paused.
func TestSnapshotDoesNotWaitForTheDecoderMutex(t *testing.T) {
	body, err := os.ReadFile("testdata/clip.mp4")
	if err != nil {
		t.Fatal(err)
	}
	out := &fakeOutput{}
	p := New(Options{Output: out, Screen: true})
	if err = p.Load(cast.Media{ContentID: serve(t, "video/mp4", body, true)}, 0, true); err != nil {
		t.Fatal(err)
	}
	defer p.Stop()
	out.wait(t, "play")
	if err = p.Pause(); err != nil {
		t.Fatal(err)
	}
	tr := p.showing()
	tr.mu.Lock()
	done := make(chan Snapshot, 1)
	go func() { done <- p.Snapshot() }()
	select {
	case s := <-done:
		tr.mu.Unlock()
		if s.State != "paused" || s.Width != 64 || s.Height != 64 || s.Duration < 2 {
			t.Fatalf("cached snapshot: %+v", s)
		}
	case <-time.After(time.Second):
		tr.mu.Unlock()
		t.Fatal("status poll blocked on the decoder mutex")
	}
}

// Seeking must be able to replace a decoder even while its current read is blocked.
func TestSeekDoesNotWaitForTheDecoderMutex(t *testing.T) {
	body, err := os.ReadFile("testdata/clip.mp4")
	if err != nil {
		t.Fatal(err)
	}
	out := &fakeOutput{}
	p := New(Options{Output: out, Screen: true})
	if err = p.Load(cast.Media{ContentID: serve(t, "video/mp4", body, true)}, 0, true); err != nil {
		t.Fatal(err)
	}
	defer p.Stop()
	out.wait(t, "play")
	if err = p.Pause(); err != nil {
		t.Fatal(err)
	}
	tr := p.showing()
	tr.mu.Lock()
	done := make(chan error, 1)
	go func() { done <- p.Seek(time.Second) }()
	select {
	case err := <-done:
		tr.mu.Unlock()
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		tr.mu.Unlock()
		t.Fatal("seek blocked on the decoder mutex")
	}
}
