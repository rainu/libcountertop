package photos

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"time"

	esphome "github.com/ygelfand/go-esphome-device"

	"github.com/ygelfand/libcountertop/pkg/say"
	setting "github.com/ygelfand/libcountertop/pkg/settings"
	"github.com/ygelfand/libcountertop/pkg/settings/schema"
	"github.com/ygelfand/libcountertop/pkg/settings/storage"
)

const (
	tick       = 5 * time.Second
	retryAfter = time.Minute
	index      = "index.json"
	bytesMost  = 48 << 20
)

type Status int

const (
	Unset Status = iota
	Off
	Connecting
	Ready
	BadKey
	TooOld
	Unreachable
	Unknown
	Empty
	NoPermission
	FeedInvalid
	FeedPartial
)

type State struct {
	Status  Status
	Version string
	Missing string
}

type Picture struct {
	URL    string
	MIME   string
	ID     string
	Source string
	Data   []byte
	Taken  time.Time
	Place  string
	People []string
	Year   int
	Faces  []Box
}

type Options struct {
	Settings  storage.Section[schema.Photos]
	Dir       string
	Keep      int
	Now       func() time.Time
	DeviceID  uint32
	Providers []Provider
	Most      func() int
	BytesMost int64
	Ordered   bool
}

type entry struct {
	URL    string   `json:"url,omitempty"`
	MIME   string   `json:"mime,omitempty"`
	Source string   `json:"source"`
	ID     string   `json:"id"`
	Taken  string   `json:"taken,omitempty"`
	Place  string   `json:"place,omitempty"`
	People []string `json:"people,omitempty"`
	Year   int      `json:"year,omitempty"`
	Faces  []Box    `json:"faces,omitempty"`
	Size   int64    `json:"size,omitempty"`
}

type feed struct {
	p       Provider
	sensor  *esphome.TextSensor
	state   State
	applied schema.Photos
	known   bool
	session Session
	version string
	deck    []entry
	failed  time.Time
}

type Library struct {
	o           Options
	knobs       *Controls
	feedLibrary *Library
	feeds       []*feed

	mu    sync.Mutex
	ready []entry
	next  int
}

func New(o Options) *Library {
	if o.Keep <= 0 {
		o.Keep = 24
	}
	if o.Now == nil {
		o.Now = time.Now
	}
	defaultProviders := o.Providers == nil
	if o.Providers == nil {
		o.Providers = []Provider{Immich(), Google(o.Most)}
	}
	if o.BytesMost <= 0 {
		o.BytesMost = bytesMost
	}
	l := &Library{o: o}
	l.knobs = &Controls{
		Table:  NewTable(),
		Device: o.DeviceID,
		Read:   o.Settings.Read,
		Save:   func(s Setting, v string) error { return setting.Store(o.Settings, s, v) },
	}
	for _, p := range o.Providers {
		l.feeds = append(l.feeds, &feed{p: p, sensor: &esphome.TextSensor{Base: esphome.Base{
			ObjectID: "photos_" + p.Key() + "_status",
			Name:     say.F("photos.entity.status", map[string]any{"Source": p.Name()}),
			Icon:     "mdi:image-multiple",
			Category: esphome.CategoryDiagnostic,
			DeviceID: o.DeviceID,
		}}})
	}
	l.load()
	if defaultProviders {
		child := o
		child.Dir = filepath.Join(o.Dir, "feed")
		child.Providers = []Provider{JSONFeed()}
		child.Ordered = true
		l.feedLibrary = New(child)
	}
	return l
}

func (l *Library) Name() string { return "photos" }

func (l *Library) Entities() []esphome.Entity {
	out := l.knobs.Entities()
	for _, f := range l.feeds {
		out = append(out, f.sensor)
	}
	if l.feedLibrary != nil {
		for _, f := range l.feedLibrary.feeds {
			out = append(out, f.sensor)
		}
	}
	return out
}

func (l *Library) Settings() *Controls { return l.knobs }

func (l *Library) Providers() []Provider {
	out := slices.Clone(l.o.Providers)
	if l.feedLibrary != nil {
		out = append(out, l.feedLibrary.o.Providers...)
	}
	return out
}

func (l *Library) Restore(c schema.Photos) {
	l.knobs.PublishFrom(c)
	if l.feedLibrary != nil {
		l.feedLibrary.Restore(c)
	}
	for _, f := range l.feeds {
		f.sensor.Set(l.State(f.p.Key()).Says())
	}
}

func (l *Library) Run(ctx context.Context) error {
	t := time.NewTicker(tick)
	defer t.Stop()
	for {
		l.step(ctx)
		select {
		case <-ctx.Done():
			return nil
		case <-t.C:
		}
	}
}

func (l *Library) State(key string) State {
	if key == "feed" && l.feedLibrary != nil {
		return l.feedLibrary.State(key)
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	for _, f := range l.feeds {
		if f.p.Key() == key {
			return f.state
		}
	}
	return State{}
}

func (l *Library) Next() (Picture, bool) {
	l.mu.Lock()
	if len(l.ready) == 0 || (l.o.Ordered && l.next >= len(l.ready)) {
		l.mu.Unlock()
		return Picture{}, false
	}
	e := l.ready[l.next%len(l.ready)]
	l.next++
	l.mu.Unlock()

	var data []byte
	var err error
	if e.URL == "" {
		data, err = os.ReadFile(l.file(e))
	}
	if err != nil {
		slog.Warn("reading a cached photo failed", "asset", e.ID, "err", err)
		l.forget(e)
		return Picture{}, false
	}
	taken, _ := time.Parse(time.RFC3339, e.Taken)
	return Picture{
		URL: e.URL, MIME: e.MIME,
		ID:     e.ID,
		Source: e.Source,
		Data:   data,
		Taken:  taken,
		Place:  e.Place,
		People: e.People,
		Year:   e.Year,
		Faces:  e.Faces,
	}, true
}

func (l *Library) step(ctx context.Context) {
	cfg := l.o.Settings.Read()
	if l.feedLibrary != nil {
		l.feedLibrary.step(ctx)
	}
	for _, f := range l.feeds {
		l.draw(ctx, f, cfg)
	}
	l.prefetch(ctx)
	for _, f := range l.feeds {
		l.settle(f)
	}
}

func (l *Library) draw(ctx context.Context, f *feed, cfg schema.Photos) {
	if st := f.p.Status(cfg); st != Connecting {
		l.close(f)
		l.set(f, State{Status: st})
		return
	}
	if !f.known || !f.p.Same(cfg, f.applied) {
		l.restart(f, cfg)
		l.set(f, State{Status: Connecting})
	}
	l.mu.Lock()
	failed := f.failed
	l.mu.Unlock()
	if !failed.IsZero() && l.o.Now().Sub(failed) < retryAfter {
		return
	}
	if f.session == nil {
		s, version, err := f.p.Open(ctx, cfg)
		f.version = version
		if err != nil {
			l.fail(f, err)
			return
		}
		f.session = s
	}
	l.mu.Lock()
	low := len(f.deck) < l.o.Keep
	drained := l.next >= len(l.ready) && len(f.deck) == 0
	l.mu.Unlock()
	if s, ok := f.session.(interface{ RefreshDue(time.Time) bool }); ok && drained && s.RefreshDue(l.o.Now()) {
		l.restart(f, cfg)
		l.drop(f.p.Key())
		l.set(f, State{Status: Connecting})
		return
	}
	front, back, err := f.session.Draw(ctx, l.o.Now(), low)
	if err != nil {
		l.fail(f, err)
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	f.failed = time.Time{}
	seen := map[string]bool{}
	for _, e := range l.ready {
		seen[e.Source+"/"+e.ID] = true
	}
	for _, e := range f.deck {
		seen[e.Source+"/"+e.ID] = true
	}
	keep := func(in []Found) []entry {
		var out []entry
		for _, x := range in {
			e := entry{
				Source: f.p.Key(),
				URL:    x.URL, MIME: x.MIME,
				ID:     x.ID,
				Taken:  x.Taken,
				Place:  x.Place,
				People: x.People,
				Year:   x.Year,
			}
			if !seen[e.Source+"/"+e.ID] {
				seen[e.Source+"/"+e.ID] = true
				out = append(out, e)
			}
		}
		return out
	}
	f.deck = append(append(keep(front), f.deck...), keep(back)...)
}

func (l *Library) prefetch(ctx context.Context) {
	turn := 0
	for {
		l.mu.Lock()
		if len(l.ready)-l.next >= l.o.Keep {
			l.mu.Unlock()
			return
		}
		var f *feed
		for i := range l.feeds {
			c := l.feeds[(turn+i)%len(l.feeds)]
			if c.session != nil && len(c.deck) > 0 && c.failed.IsZero() {
				f = c
				break
			}
		}
		if f == nil {
			l.mu.Unlock()
			return
		}
		e := f.deck[0]
		l.mu.Unlock()
		turn++

		data, err := f.session.Fetch(ctx, e.ID)
		if err == nil && e.URL == "" && l.o.Most != nil {
			data = shrink(data, l.o.Most())
		}
		if err == nil {
			err = os.MkdirAll(l.o.Dir, 0o755)
		}
		if err == nil && e.URL == "" {
			err = os.WriteFile(l.file(e), data, 0o600)
		}
		if errors.Is(err, ErrGone) {
			slog.Warn("photos: skipping a photo", "source", f.p.Key(), "asset", e.ID, "err", err)
			l.mu.Lock()
			f.deck = f.deck[1:]
			l.mu.Unlock()
			continue
		}
		if err != nil {
			l.fail(f, err)
			continue
		}

		e.Size = int64(len(data))
		if ff, ok := f.session.(faceFinder); ok {
			e.Faces = ff.Faces(ctx, e.ID)
		}
		l.mu.Lock()
		f.deck = f.deck[1:]
		l.ready = append(l.ready, e)
		l.trim()
		l.mu.Unlock()
		l.save()
	}
}

func (l *Library) settle(f *feed) {
	l.mu.Lock()
	if f.session == nil || !f.failed.IsZero() {
		l.mu.Unlock()
		return
	}
	st := State{Status: Ready, Version: f.version}
	if s, ok := f.session.(interface{ Notice() string }); ok {
		if note := s.Notice(); note != "" {
			st.Status, st.Missing = FeedPartial, note
		}
	}
	if st.Status == Ready && len(f.deck) == 0 && !slices.ContainsFunc(l.ready, func(e entry) bool {
		return e.Source == f.p.Key()
	}) {
		st.Status = Empty
	}
	l.mu.Unlock()
	l.set(f, st)
}

func (l *Library) trim() {
	most := 2 * l.o.Keep
	if l.next > len(l.ready) {
		l.next = len(l.ready)
	}
	var held int64
	for _, e := range l.ready {
		held += e.Size
	}
	for (len(l.ready) > most || held > l.o.BytesMost) && l.next > 0 {
		held -= l.ready[0].Size
		_ = os.Remove(l.file(l.ready[0]))
		l.ready = l.ready[1:]
		l.next--
	}
}

func (l *Library) restart(f *feed, cfg schema.Photos) {
	l.mu.Lock()
	changed := f.known
	f.applied, f.known = cfg, true
	f.session, f.version, f.deck, f.failed = nil, "", nil, time.Time{}
	l.mu.Unlock()
	if changed {
		l.drop(f.p.Key())
	}
}

func (l *Library) close(f *feed) {
	l.mu.Lock()
	f.known, f.applied = false, schema.Photos{}
	f.session, f.version, f.deck, f.failed = nil, "", nil, time.Time{}
	l.mu.Unlock()
	l.drop(f.p.Key())
}

func (l *Library) drop(source string) {
	l.mu.Lock()
	kept := l.ready[:0]
	removed := false
	for i, e := range l.ready {
		if e.Source != source {
			kept = append(kept, e)
			continue
		}
		_ = os.Remove(l.file(e))
		removed = true
		if i < l.next {
			l.next--
		}
	}
	l.ready = kept
	l.mu.Unlock()
	if removed {
		l.save()
	}
}

func (l *Library) fail(f *feed, err error) {
	l.mu.Lock()
	f.failed = l.o.Now()
	f.session = nil
	if l.o.Ordered {
		f.deck = nil
	}
	version := f.version
	l.mu.Unlock()
	var (
		old     *TooOldError
		missing *MissingError
		perm    *PermissionError
	)
	switch {
	case errors.Is(err, errFeedInvalid):
		l.set(f, State{Status: FeedInvalid})
	case errors.As(err, &old):
		l.set(f, State{Status: TooOld, Version: old.Version})
	case errors.As(err, &perm):
		l.set(f, State{Status: NoPermission, Version: version, Missing: perm.Name})
	case errors.Is(err, ErrKey):
		l.set(f, State{Status: BadKey, Version: version})
	case errors.As(err, &missing):
		l.set(f, State{Status: Unknown, Version: version, Missing: missing.Name})
	default:
		l.set(f, State{Status: Unreachable, Version: version})
	}
	slog.Warn("photos", "source", f.p.Key(), "err", err)
}

func (l *Library) set(f *feed, st State) {
	l.mu.Lock()
	changed := f.state != st
	f.state = st
	l.mu.Unlock()
	if changed {
		f.sensor.Set(st.Says())
	}
}

func (st State) Says() string {
	switch st.Status {
	case Off:
		return say.T("photos.status.off")
	case Connecting:
		return say.T("photos.status.connecting")
	case Ready:
		return say.T("photos.status.ready")
	case BadKey:
		return say.T("photos.status.key")
	case TooOld:
		return say.F("photos.status.old", map[string]any{"Version": st.Version})
	case Unreachable:
		return say.T("photos.status.unreachable")
	case Unknown:
		return say.F("photos.status.unknown", map[string]any{"Name": st.Missing})
	case Empty:
		return say.T("photos.status.empty")
	case FeedInvalid:
		return say.T("photos.status.feed_invalid")
	case FeedPartial:
		return say.T("photos.status.feed_partial")
	case NoPermission:
		return say.F("photos.status.permission", map[string]any{"Name": st.Missing})
	}
	return say.T("photos.status.unset")
}

func (l *Library) forget(gone entry) {
	l.mu.Lock()
	if i := slices.IndexFunc(l.ready, func(e entry) bool {
		return e.Source == gone.Source && e.ID == gone.ID
	}); i >= 0 {
		l.ready = slices.Delete(l.ready, i, i+1)
		if i < l.next {
			l.next--
		}
	}
	l.mu.Unlock()
	l.save()
}

func (l *Library) file(e entry) string {
	return filepath.Join(l.o.Dir, filepath.Base(e.Source+"-"+e.ID)+".jpg")
}

func (l *Library) load() {
	var saved []entry
	if data, err := os.ReadFile(filepath.Join(l.o.Dir, index)); err == nil {
		_ = json.Unmarshal(data, &saved)
	}
	kept := map[string]bool{index: true}
	for _, e := range saved {
		if e.Source == "" {
			continue
		}
		if st, err := os.Stat(l.file(e)); err == nil && e.URL == "" {
			e.Size = st.Size()
			l.ready = append(l.ready, e)
			kept[filepath.Base(l.file(e))] = true
		}
	}
	files, _ := os.ReadDir(l.o.Dir)
	for _, f := range files {
		if !f.IsDir() && !kept[f.Name()] {
			_ = os.Remove(filepath.Join(l.o.Dir, f.Name()))
		}
	}
}

func (l *Library) save() {
	l.mu.Lock()
	saved := l.ready
	if l.o.Ordered {
		saved = nil
	}
	data, err := json.Marshal(saved)
	l.mu.Unlock()
	if err != nil {
		return
	}
	if err := os.MkdirAll(l.o.Dir, 0o755); err != nil {
		return
	}
	tmp := filepath.Join(l.o.Dir, index+".tmp")
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return
	}
	_ = os.Rename(tmp, filepath.Join(l.o.Dir, index))
}
