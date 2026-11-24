package webreceiver

import (
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/ygelfand/libcountertop/pkg/media/cast"
	"github.com/ygelfand/libcountertop/pkg/media/urlplayer"
	"github.com/ygelfand/libcountertop/pkg/media/youtubeurl"
	"github.com/ygelfand/libcountertop/pkg/runtime/safe"
	"github.com/ygelfand/libcountertop/pkg/runtime/service"
)

//go:embed youtube.html
var youtubeHTML string

//go:embed youtube-site.js
var youtubeSiteJS string

// YouTube uses YouTube's official IFrame Player API. The existing media adapter
// supplies the device decoders. Contextually refused embeds can use the official
// full website through standard media controls, not extracted stream URLs.
type YouTube struct {
	r      *Receiver
	op     sync.Mutex
	mu     sync.Mutex
	active bool
	cancel context.CancelFunc
	status urlplayer.Snapshot
}

func NewYouTube(env cast.Env, o Options) *YouTube {
	if o.BusPort == 0 {
		o.BusPort = 8013
	}
	y := &YouTube{status: urlplayer.Snapshot{State: "idle"}}
	y.r = New(env, o)
	y.r.youtube = y
	return y
}

func (y *YouTube) Run(ctx context.Context) error {
	defer y.Stop()
	return service.Keep(ctx, "YouTube player", y.r.run)
}

func (y *YouTube) Active() bool { y.mu.Lock(); defer y.mu.Unlock(); return y.active }

func (y *YouTube) Snapshot() urlplayer.Snapshot { y.mu.Lock(); defer y.mu.Unlock(); return y.status }

func (y *YouTube) Load(m cast.Media, at time.Duration, play bool) error {
	v, recognized, err := youtubeurl.Parse(m.URL())
	if err != nil {
		return err
	}
	if !recognized {
		return youtubeurl.ErrURL
	}
	y.op.Lock()
	defer y.op.Unlock()
	y.stop()
	ctx, cancel := context.WithCancel(context.Background())
	y.mu.Lock()
	y.active = true
	y.cancel = cancel
	y.status = urlplayer.Snapshot{State: "loading"}
	y.mu.Unlock()
	if at == 0 {
		at = v.Start
	}
	safe.Go("YouTube playback", func() { y.open(ctx, v.ID, at, play) })
	return nil
}

func (y *YouTube) open(ctx context.Context, id string, at time.Duration, play bool) {
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()
	deadline := time.Now().Add(60 * time.Second)
	for {
		if time.Now().After(deadline) {
			y.fail(ctx, "The YouTube player did not become ready")
			return
		}
		y.r.mu.Lock()
		tools := y.r.tools
		y.r.mu.Unlock()
		if tools != nil {
			q := url.Values{
				"v":     {id},
				"start": {strconv.FormatFloat(at.Seconds(), 'f', 3, 64)},
				"play":  {strconv.Itoa(map[bool]int{true: 1, false: 0}[play])},
			}
			y.op.Lock()
			if ctx.Err() != nil {
				y.op.Unlock()
				return
			}
			err := tools.Navigate(
				"http://127.0.0.1:" + strconv.Itoa(y.r.o.BusPort) + "/youtube?" + q.Encode(),
			)
			y.op.Unlock()
			if err != nil {
				y.fail(ctx, "The YouTube player could not open")
				return
			}
			break
		}
		select {
		case <-ctx.Done():
			y.fail(ctx, "The YouTube player did not become ready")
			return
		case <-ticker.C:
		}
	}
	// A load deadline does not terminate a successfully playing session.
	loaded := false
	website := false
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
		if time.Now().After(deadline) {
			y.fail(ctx, "YouTube did not start. The video may be unavailable or blocked")
			return
		}
		probeCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
		raw, err := y.r.Eval(
			probeCtx,
			`JSON.stringify({...(window.youtubeStatus||{}),page:location.pathname,host:location.hostname,position:window.youtubePlayer?.getCurrentTime?.()||0,duration:window.youtubePlayer?.getDuration?.()||0})`,
		)
		cancel()
		if err != nil {
			continue
		}
		var state struct {
			Ready              bool
			Page, Host         string
			State              int
			Error              int
			Position, Duration float64
		}
		if json.Unmarshal([]byte(raw), &state) != nil {
			continue
		}
		if website && (state.Host == "consent.youtube.com" || state.Host == "accounts.google.com") {
			y.fail(ctx, "YouTube requires consent or sign-in on this device")
			return
		}
		if website && state.Page != "/watch" {
			continue // Navigation is asynchronous; ignore the old embed's error.
		}
		if !website && (state.Error == 101 || state.Error == 150) {
			website = true
			if err := y.openWebsite(ctx, id, at, play); err != nil {
				y.fail(ctx, "Could not open the YouTube website")
				return
			}
			continue
		}
		if state.Error != 0 {
			y.fail(ctx, youtubeError(state.Error))
			return
		}
		decoded := y.r.player.Snapshot()
		if decoded.State == "error" {
			y.fail(ctx, "YouTube media could not be decoded on this device")
			return
		}
		if !loaded && (decoded.State == "playing" || decoded.State == "paused") {
			loaded = true
		}
		s := decoded
		if !loaded {
			s.State = "loading"
		}
		if state.Ready && state.State == 0 {
			s.State = "idle"
		}
		s.Duration = state.Duration
		s.Seekable = loaded && state.Duration > 0
		y.mu.Lock()
		if ctx.Err() == nil {
			y.status = s
		}
		y.mu.Unlock()
		if loaded {
			// Continue polling after startup without extending the startup deadline.
			// The session cancellation is separate; monitor below owns its lifetime.
			y.monitor(ctx)
			return
		}
	}
}

func (y *YouTube) monitor(ctx context.Context) {
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
		bounded, done := context.WithTimeout(ctx, 2*time.Second)
		raw, err := y.r.Eval(
			bounded,
			`JSON.stringify({...(window.youtubeStatus||{}),duration:window.youtubePlayer?.getDuration?.()||0})`,
		)
		done()
		if err != nil {
			y.fail(ctx, "The YouTube player disconnected")
			return
		}
		var state struct {
			State    int
			Error    int
			Site     bool
			Duration float64
		}
		if json.Unmarshal([]byte(raw), &state) != nil {
			continue
		}
		if state.Error != 0 {
			y.fail(ctx, youtubeError(state.Error))
			return
		}
		if state.Site && state.State == 0 {
			// Do not follow the full website's suggested next video.
			y.op.Lock()
			if ctx.Err() == nil {
				y.stop()
			}
			y.op.Unlock()
			return
		}
		s := y.r.player.Snapshot()
		if s.State == "error" {
			y.fail(ctx, "YouTube media could not be decoded on this device")
			return
		}
		y.mu.Lock()
		if ctx.Err() == nil {
			s.Duration = state.Duration
			s.Seekable = s.Duration > 0
			if state.State == 0 {
				s.State = "idle"
				s.Seekable = false
			}
			y.status = s
		}
		y.mu.Unlock()
	}
}

func (y *YouTube) fail(ctx context.Context, message string) {
	y.op.Lock()
	defer y.op.Unlock()
	if ctx.Err() != nil {
		return
	}
	y.stop()
	y.mu.Lock()
	y.status = urlplayer.Snapshot{State: "error", Error: message}
	y.mu.Unlock()
}
func youtubeError(code int) string {
	switch code {
	case 2:
		return "YouTube rejected this video URL"
	case 5:
		return "YouTube could not play this video on this device"
	case 100:
		return "This YouTube video is unavailable, removed, or private"
	case 101, 150:
		return "YouTube refused playback in this embedded-player context"
	case 152, 153:
		return "YouTube refused this embedded player or its client identification"
	case -1:
		return "YouTube blocked autoplay. Try another video"
	case -2:
		return "Could not load the YouTube player. Check the device internet connection"
	default:
		return "YouTube could not play this video. It may require sign-in or be restricted"
	}
}
func (y *YouTube) stop() {
	y.mu.Lock()
	cancel := y.cancel
	y.cancel = nil
	y.active = false
	y.status = urlplayer.Snapshot{State: "idle"}
	y.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	y.r.Ended(cast.App{})
}
func (y *YouTube) Stop() error { y.op.Lock(); defer y.op.Unlock(); y.stop(); return nil }
func (y *YouTube) call(expr string) error {
	y.op.Lock()
	defer y.op.Unlock()
	if !y.Active() {
		return errors.New("no YouTube video is loaded")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	raw, err := y.r.Eval(
		ctx,
		"Boolean(window.youtubePlayer && window.youtubeStatus?.ready && ("+expr+",true))",
	)
	if err != nil || raw != "true" {
		return errors.New("the YouTube player is not ready")
	}
	return nil
}
func (y *YouTube) Play() error  { return y.call("youtubePlayer.playVideo()") }
func (y *YouTube) Pause() error { return y.call("youtubePlayer.pauseVideo()") }
func (y *YouTube) Seek(at time.Duration) error {
	if at < 0 || !y.Snapshot().Seekable || math.IsInf(at.Seconds(), 0) {
		return errors.New("this YouTube video is not seekable")
	}
	return y.call(fmt.Sprintf("youtubePlayer.seekTo(%f,true)", at.Seconds()))
}
func (y *YouTube) serve(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	v, _, err := youtubeurl.Parse(
		"https://www.youtube.com/watch?v=" + url.QueryEscape(req.URL.Query().Get("v")),
	)
	if err != nil || v.ID == "" {
		http.Error(w, "Invalid video", http.StatusBadRequest)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write([]byte(youtubeHTML))
}

// openWebsite keeps playback on the official website when its embed is refused.
// The same media adapter/decoder is initialized by the page's pre-navigation hook.
func (y *YouTube) openWebsite(ctx context.Context, id string, at time.Duration, play bool) error {
	y.r.mu.Lock()
	page := y.r.tools
	y.r.mu.Unlock()
	if page == nil {
		return errors.New("YouTube page is not ready")
	}
	q := url.Values{
		"v":        {id},
		"t":        {strconv.FormatFloat(at.Seconds(), 'f', 3, 64) + "s"},
		"autoplay": {strconv.Itoa(map[bool]int{true: 1, false: 0}[play])},
	}
	y.op.Lock()
	defer y.op.Unlock()
	if ctx.Err() != nil {
		return ctx.Err()
	}
	return page.Navigate("https://www.youtube.com/watch?" + q.Encode())
}

func youtubeUserAgent() string {
	for _, arg := range EngineArgs() {
		if agent, ok := strings.CutPrefix(arg, "--user-agent="); ok {
			browser, _, _ := strings.Cut(agent, " CrKey/")
			return browser
		}
	}
	return ""
}
