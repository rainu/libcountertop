// Package localvideo exposes a local URL player behind the host's authenticated web API.
package localvideo

import (
	"encoding/json"
	"errors"
	"io"
	"math"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/ygelfand/libcountertop/pkg/media/cast"
	"github.com/ygelfand/libcountertop/pkg/media/smburl"
	"github.com/ygelfand/libcountertop/pkg/media/urlplayer"
	"github.com/ygelfand/libcountertop/pkg/media/youtubeurl"
)

type Player interface {
	Load(cast.Media, time.Duration, bool) error
	Play() error
	Pause() error
	Stop() error
	Seek(time.Duration) error
	Snapshot() urlplayer.Snapshot
}
type Handler struct {
	mu     sync.Mutex
	player Player
}

func New(p Player) *Handler { return &Handler{player: p} }

func ValidURL(raw string) bool {
	if strings.HasPrefix(raw, "smb:") {
		_, err := smburl.Parse(raw)
		return err == nil
	}
	if len(raw) > 8192 {
		return false
	}
	u, err := url.Parse(raw)
	if err == nil && (u.Scheme == "http" || u.Scheme == "https") && u.User == nil {
		if _, youtube, _ := youtubeurl.Parse(raw); youtube {
			// Let the common router produce the actionable YouTube validation error.
			return true
		}
	}
	return err == nil && (u.Scheme == "http" || u.Scheme == "https") && u.Hostname() != "" &&
		u.User == nil &&
		u.Fragment == ""
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if r.URL.Path != "/" && r.URL.Path != "" {
		http.NotFound(w, r)
		return
	}
	if r.Method == http.MethodGet {
		h.mu.Lock()
		s := h.player.Snapshot()
		h.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(s)
		return
	}
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", "GET, POST")
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	var cmd struct {
		Action   string  `json:"action"`
		URL      string  `json:"url"`
		Position float64 `json:"position"`
	}
	d := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16<<10))
	d.DisallowUnknownFields()
	if d.Decode(&cmd) != nil || d.Decode(new(any)) != io.EOF {
		http.Error(w, "Invalid playback request", 400)
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	var err error
	switch cmd.Action {
	case "load":
		cmd.URL = strings.TrimSpace(cmd.URL)
		if !ValidURL(cmd.URL) {
			http.Error(
				w,
				"Use an HTTP(S) media URL or smb://media/folder/video.mp4 without embedded credentials",
				400,
			)
			return
		}
		err = h.player.Load(
			cast.Media{
				ContentID:  cmd.URL,
				StreamType: cast.StreamBuffered,
				Metadata:   cast.Metadata{Title: "Video"},
			},
			0,
			true,
		)
	case "play":
		err = h.player.Play()
	case "pause":
		err = h.player.Pause()
	case "stop":
		err = h.player.Stop()
	case "seek":
		if math.IsNaN(cmd.Position) || math.IsInf(cmd.Position, 0) || cmd.Position < 0 ||
			cmd.Position > float64((1<<63-1)/int64(time.Second)) {
			http.Error(w, "Invalid playback position", 400)
			return
		}
		if !h.player.Snapshot().Seekable {
			err = errors.New("not seekable")
		} else {
			err = h.player.Seek(time.Duration(cmd.Position * float64(time.Second)))
		}
	default:
		http.Error(w, "Unknown playback action", 400)
		return
	}
	var public interface{ PlaybackError() string }
	if errors.As(err, &public) {
		http.Error(w, public.PlaybackError(), http.StatusBadRequest)
		return
	}
	if errors.Is(err, smburl.ErrURL) {
		http.Error(w, smburl.ErrURL.Error(), http.StatusBadRequest)
		return
	}
	if errors.Is(err, youtubeurl.ErrURL) {
		http.Error(w, youtubeurl.ErrURL.Error(), http.StatusBadRequest)
		return
	}
	if err != nil {
		http.Error(
			w,
			"Playback command is not available for the current media",
			http.StatusConflict,
		)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
