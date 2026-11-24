// Package youtubeurl recognizes individual YouTube videos, not media-stream URLs.
package youtubeurl

import (
	"errors"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var videoID = regexp.MustCompile(`^[A-Za-z0-9_-]{11}$`)
var offset = regexp.MustCompile(`^(?:(\d+)h)?(?:(\d+)m)?(?:(\d+)s)?$`)
var ErrURL = errors.New("use a YouTube watch URL or youtu.be link for a single public video")

type Video struct {
	ID    string
	Start time.Duration
}

// Parse distinguishes a malformed/unsupported YouTube link from a non-YouTube URL.
// Exact hosts prevent lookalike domains from being routed to the embedded player.
func Parse(raw string) (v Video, youtube bool, err error) {
	u, e := url.Parse(raw)
	if e != nil {
		return v, false, nil
	}
	host := strings.ToLower(u.Hostname())
	switch host {
	case "youtube.com",
		"www.youtube.com",
		"m.youtube.com",
		"music.youtube.com",
		"youtu.be",
		"www.youtu.be":
	default:
		return v, false, nil
	}
	if (u.Scheme != "https" && u.Scheme != "http") || u.User != nil || u.Port() != "" ||
		u.Fragment != "" ||
		len(raw) > 8192 {
		return v, true, ErrURL
	}
	if host == "youtu.be" || host == "www.youtu.be" {
		v.ID = strings.TrimPrefix(u.Path, "/")
	} else {
		switch {
		case u.Path == "/watch":
			if len(u.Query()["v"]) != 1 {
				return v, true, ErrURL
			}
			v.ID = u.Query().Get("v")
		case strings.HasPrefix(u.Path, "/embed/"):
			v.ID = strings.TrimPrefix(u.Path, "/embed/")
		case strings.HasPrefix(u.Path, "/shorts/"):
			v.ID = strings.TrimPrefix(u.Path, "/shorts/")
		default:
			return v, true, ErrURL
		}
	}
	if !videoID.MatchString(v.ID) {
		return Video{}, true, ErrURL
	}
	start := u.Query().Get("t")
	if start == "" {
		start = u.Query().Get("start")
	}
	if start != "" {
		if n, e := strconv.ParseUint(start, 10, 32); e == nil {
			v.Start = time.Duration(n) * time.Second
		} else {
			parts := offset.FindStringSubmatch(start)
			if parts == nil || start == "" {
				return Video{}, true, ErrURL
			}
			for i, unit := range []time.Duration{time.Hour, time.Minute, time.Second} {
				if parts[i+1] != "" {
					n, e := strconv.ParseUint(parts[i+1], 10, 32)
					if e != nil || n > 86400 {
						return Video{}, true, ErrURL
					}
					v.Start += time.Duration(n) * unit
				}
			}
		}
		if v.Start > 24*time.Hour {
			return Video{}, true, ErrURL
		}
	}
	return v, true, nil
}
