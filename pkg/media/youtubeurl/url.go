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

var ErrURL = errors.New(
	"use a link to a single public video, song, podcast episode, Short or live stream; playlist, channel, search and clip links do not identify a playable video",
)

type Video struct {
	ID    string
	Start time.Duration
}

// Parse distinguishes a malformed/unsupported YouTube link from a non-YouTube URL.
// Exact hosts prevent lookalike domains from being routed to the embedded player.
func Parse(raw string) (v Video, youtube bool, err error) {
	return parse(raw, 0)
}

// Unwrap only YouTube-owned share redirects, without fetching any remote URL.
func parse(raw string, depth int) (v Video, youtube bool, err error) {
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
		"www.music.youtube.com",
		"gaming.youtube.com",
		"youtube-nocookie.com",
		"www.youtube-nocookie.com",
		"youtu.be",
		"www.youtu.be":
	default:
		return v, false, nil
	}
	if (u.Scheme != "https" && u.Scheme != "http") || u.User != nil || u.Port() != "" ||
		depth > 3 ||
		len(raw) > 8192 {
		return v, true, ErrURL
	}
	path := strings.TrimSuffix(u.Path, "/")
	if host == "youtu.be" || host == "www.youtu.be" {
		v.ID = strings.TrimPrefix(path, "/")
	} else {
		switch path {
		case "/attribution_link", "/redirect":
			key := "u"
			if path == "/redirect" {
				key = "q"
				if u.Query().Get(key) == "" {
					key = "url"
				}
			}
			if len(u.Query()[key]) != 1 {
				return v, true, ErrURL
			}
			target, parseErr := url.Parse(u.Query().Get(key))
			if parseErr != nil || target.String() == "" {
				return v, true, ErrURL
			}
			nested, recognized, nestedErr := parse(u.ResolveReference(target).String(), depth+1)
			if !recognized || nestedErr != nil {
				return v, true, ErrURL
			}
			return nested, true, nil
		case "/watch":
			if len(u.Query()["v"]) != 1 {
				return v, true, ErrURL
			}
			v.ID = u.Query().Get("v")
		default:
			parts := strings.Split(strings.TrimPrefix(path, "/"), "/")
			if len(parts) != 2 {
				return v, true, ErrURL
			}
			switch parts[0] {
			case "embed", "shorts", "live", "podcast", "v", "e", "watch":
				v.ID = parts[1]
			default:
				return v, true, ErrURL
			}
		}
	}
	if !videoID.MatchString(v.ID) {
		return Video{}, true, ErrURL
	}
	start := u.Query().Get("t")
	if start == "" {
		start = u.Query().Get("start")
	}
	if start == "" && u.Fragment != "" {
		fragment, fragmentErr := url.ParseQuery(u.Fragment)
		if fragmentErr != nil {
			return Video{}, true, ErrURL
		}
		start = fragment.Get("t")
		if start == "" {
			start = fragment.Get("start")
		}
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
