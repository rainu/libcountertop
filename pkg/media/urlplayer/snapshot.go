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

// Open-time decoder metadata is immutable once published. Status readers must
// never take the decoder mutex: a paused decoder can be blocked on its queues.
type trackSnapshot struct {
	failed        bool
	width, height int
	duration      time.Duration
	seekable      bool
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
	meta := t.snapshot.Load()
	if meta == nil {
		s.State = "loading"
		return s
	}
	s.Width, s.Height = meta.width, meta.height
	s.Duration, s.Seekable = meta.duration.Seconds(), meta.seekable
	s.Position = p.Elapsed().Round(time.Millisecond).Seconds()
	s.Frames = p.Frames()
	switch {
	case meta.failed:
		s.State, s.Error = "error", "Could not open or decode this media. Use a direct supported video or stream URL."
	case shown != t:
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
