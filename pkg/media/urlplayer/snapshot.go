package urlplayer

import (
	"time"

	"github.com/ygelfand/libcountertop/pkg/media/playback"
)

// Snapshot is a URL-free view suitable for a local playback control interface.
type Snapshot struct {
	State    string  `json:"state"`
	Position float64 `json:"position"`
	Duration float64 `json:"duration"`
	Width    int     `json:"width"`
	Height   int     `json:"height"`
	Frames   int64   `json:"frames"`
	Seekable bool    `json:"seekable"`
	Error    string  `json:"error,omitempty"`
}

func (p *Player) Snapshot() Snapshot {
	p.mu.Lock()
	t, shown, paused, failedPlayback := p.cur, p.shown, p.paused, p.failed
	p.mu.Unlock()
	s := Snapshot{State: "idle"}
	if failedPlayback {
		s.State = "error"
		s.Error = "Video playback failed. Try a supported codec or a different stream."
		return s
	}
	if t == nil {
		return s
	}
	t.mu.Lock()
	started, failed := t.started, t.failed
	s.Width, s.Height, s.Duration, s.Seekable = int(
		t.picture.Width,
	), int(
		t.picture.Height,
	), t.length.Seconds(), t.seekable &&
		!t.media.Streaming()
	t.mu.Unlock()
	s.Position = p.Elapsed().Round(time.Millisecond).Seconds()
	s.Frames = p.Frames()
	switch {
	case failed != nil:
		s.State, s.Error = "error", "Could not open or decode this media. Use a direct supported video or stream URL."
	case !started || shown != t:
		s.State = "loading"
	default:
		s.State = "playing"
		if active, ok := p.o.Output.(interface{ Active(playback.Source) bool }); ok &&
			!active.Active(t) {
			s.State = "idle"
		} else if paused {
			s.State = "paused"
		} else if t.waiting.Load() {
			s.State = "loading"
		}
	}
	return s
}
