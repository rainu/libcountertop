package photos

import (
	"github.com/ygelfand/libcountertop/pkg/settings/schema"
	"sync"
)

// Source reuses the normal photo pipeline, with explicit independent choices
// for the app, wallpaper and screen saver. Existing providers are never changed.
func (l *Library) Source(consumer string) *Selection {
	return &Selection{library: l, consumer: consumer}
}

type Selection struct {
	mu       sync.Mutex
	library  *Library
	consumer string
	turn     bool
}

func (s *Selection) Next() (Picture, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	cfg := s.library.o.Settings.Read()
	mode := cfg.AppSource
	switch s.consumer {
	case "wallpaper":
		mode = cfg.WallpaperSource
	case "saver":
		mode = cfg.SaverSource
	}
	if !cfg.Feed.Enabled || (mode == schema.SourceExisting || mode == "") {
		return s.library.Next()
	}
	feed := func() (Picture, bool) {
		child := s.library.feedSurfaces[s.consumer]
		if child == nil {
			return Picture{}, false
		}
		for range 2 * child.o.Keep {
			p, ok := child.Next()
			if !ok {
				return p, false
			}
			if s.consumer == "app" || p.URL == "" {
				return p, true
			}
		}
		return Picture{}, false
	}
	if mode == schema.SourceFeed {
		return feed()
	}
	s.turn = !s.turn
	if s.turn {
		if p, ok := s.library.Next(); ok {
			return p, true
		}
		return feed()
	}
	if p, ok := feed(); ok {
		return p, true
	}
	return s.library.Next()
}
