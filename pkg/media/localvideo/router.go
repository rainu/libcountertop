package localvideo

import (
	"sync"
	"time"

	"github.com/ygelfand/libcountertop/pkg/media/cast"
	"github.com/ygelfand/libcountertop/pkg/media/urlplayer"
	"github.com/ygelfand/libcountertop/pkg/media/youtubeurl"
)

// Router preserves direct-media playback and selects the official YouTube player
// only for recognized video URLs. Commands always address the selected player.
type Router struct {
	mu                       sync.Mutex
	direct, youtube, current Player
}

func Route(direct, youtube Player) *Router {
	return &Router{direct: direct, youtube: youtube, current: direct}
}
func (p *Router) Load(m cast.Media, at time.Duration, play bool) error {
	_, recognized, err := youtubeurl.Parse(m.URL())
	if err != nil {
		return err
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	next := p.direct
	if recognized {
		next = p.youtube
	}
	if err := p.current.Stop(); err != nil {
		return err
	}
	p.current = next
	return next.Load(m, at, play)
}
func (p *Router) Play() error  { p.mu.Lock(); defer p.mu.Unlock(); return p.current.Play() }
func (p *Router) Pause() error { p.mu.Lock(); defer p.mu.Unlock(); return p.current.Pause() }
func (p *Router) Stop() error  { p.mu.Lock(); defer p.mu.Unlock(); return p.current.Stop() }
func (p *Router) Seek(at time.Duration) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.current.Seek(at)
}
func (p *Router) Snapshot() urlplayer.Snapshot {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.current.Snapshot()
}
