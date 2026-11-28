package photos

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"image"
	"io"
	"mime"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/ygelfand/libcountertop/pkg/fetch"
	"github.com/ygelfand/libcountertop/pkg/settings/schema"
)

const (
	feedPageBytes   = 2 << 20
	feedMediaBytes  = 12 << 20
	feedPages       = 128
	feedItems       = 1024
	feedAttachments = 8192
	feedURLBytes    = 4096
	feedBatch       = 24
	feedRefresh     = 15 * time.Minute
)

var (
	errFeedInvalid = errors.New("photos: invalid JSON Feed; check the feed format and URL")
	errFeedRequest = errors.New("photos: feed unavailable; check access and refresh")
)

type feedSource struct{ client *http.Client }

func JSONFeed() Provider {
	c := fetch.Client(15 * time.Second)
	c.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		if len(via) >= 5 || !validFeedURL(req.URL.String()) {
			return errFeedRequest
		}
		return nil
	}
	return feedSource{client: c}
}
func (feedSource) Key() string  { return "feed" }
func (feedSource) Name() string { return "JSON Feed" }
func (feedSource) Status(c schema.Photos) Status {
	if !c.Feed.Enabled {
		return Off
	}
	if c.Feed.URL == "" {
		return Unset
	}
	return Connecting
}
func (feedSource) Same(a, b schema.Photos) bool { return a.Feed == b.Feed }
func validFeedURL(raw string) bool {
	if len(raw) > feedURLBytes || strings.TrimSpace(raw) != raw {
		return false
	}
	u, e := url.Parse(raw)
	return e == nil && (u.Scheme == "http" || u.Scheme == "https") && u.Hostname() != "" && u.User == nil && u.Fragment == ""
}

type feedDocument struct {
	Version string `json:"version"`
	Title   string `json:"title"`
	Next    string `json:"next_url"`
	Items   []struct {
		ID          string `json:"id"`
		Attachments []struct {
			URL  string `json:"url"`
			MIME string `json:"mime_type"`
		} `json:"attachments"`
	} `json:"items"`
}
type feedMedia struct{ url, mime string }
type feedSession struct {
	src                    feedSource
	next                   string
	pending                []feedMedia
	urls                   map[string]feedMedia
	seen                   map[string]bool
	pages, count, sequence int
	opened                 time.Time
	notice                 string
}

func (f feedSource) Open(ctx context.Context, c schema.Photos) (Session, string, error) {
	if !validFeedURL(c.Feed.URL) {
		return nil, "", errFeedInvalid
	}
	s := &feedSession{src: f, next: c.Feed.URL, urls: map[string]feedMedia{}, seen: map[string]bool{}, opened: time.Now()}
	if err := s.page(ctx); err != nil {
		return nil, "", err
	}
	return s, "", nil
}
func (s *feedSession) Notice() string { return s.notice }
func (s *feedSession) page(ctx context.Context) error {
	if s.next == "" {
		return nil
	}
	if s.pages >= feedPages || s.seen[s.next] {
		s.next = ""
		s.notice = "pagination limit or cycle"
		return nil
	}
	raw := s.next
	if !validFeedURL(raw) {
		s.next = ""
		return errFeedInvalid
	}
	s.seen[raw] = true
	s.pages++
	body, e := s.src.get(ctx, raw, feedPageBytes)
	if e != nil {
		return errFeedRequest
	}
	var doc feedDocument
	if json.Unmarshal(body, &doc) != nil || doc.Version != "https://jsonfeed.org/version/1.1" || doc.Title == "" || doc.Items == nil || len(doc.Items) > feedItems {
		return errFeedInvalid
	}
	s.next = doc.Next
	if s.next != "" && !validFeedURL(s.next) {
		s.next = ""
		s.notice = "invalid pagination URL"
	}
	for _, item := range doc.Items {
		if item.ID == "" {
			s.notice = "item without an ID"
			continue
		}
		for _, a := range item.Attachments {
			if s.count >= feedAttachments {
				s.next = ""
				s.notice = "attachment limit reached"
				return nil
			}
			s.count++
			kind, _, e := mime.ParseMediaType(a.MIME)
			if e != nil || !validFeedURL(a.URL) || !supportedFeedMIME(kind) {
				s.notice = "unsupported attachment or URL"
				continue
			}
			s.pending = append(s.pending, feedMedia{url: a.URL, mime: kind})
		}
	}
	return nil
}
func supportedFeedMIME(m string) bool {
	switch m {
	case "image/jpeg", "image/png", "image/webp", "video/mp4", "video/webm":
		return true
	}
	return false
}
func (s *feedSession) Draw(ctx context.Context, now time.Time, low bool) ([]Found, []Found, error) {

	if !low {
		return nil, nil, nil
	}
	// At most one page per library tick, including empty/unsupported pages.
	if len(s.pending) == 0 && s.next != "" {
		if e := s.page(ctx); e != nil {
			return nil, nil, e
		}
	}
	var out []Found
	for len(s.pending) > 0 && len(out) < feedBatch {
		a := s.pending[0]
		s.pending = s.pending[1:]
		s.sequence++
		sum := sha256.Sum256([]byte(a.url))
		id := hex.EncodeToString(sum[:8]) + "-" + hex.EncodeToString([]byte{byte(s.sequence >> 16), byte(s.sequence >> 8), byte(s.sequence)})
		s.urls[id] = a
		found := Found{ID: id}
		if strings.HasPrefix(a.mime, "video/") {
			found.URL, found.MIME = a.url, a.mime
		}
		out = append(out, found)
	}
	return nil, out, nil
}
func (s *feedSession) Fetch(ctx context.Context, id string) ([]byte, error) {
	a, ok := s.urls[id]
	delete(s.urls, id)
	if !ok {
		return nil, ErrGone
	}
	if strings.HasPrefix(a.mime, "video/") {
		return nil, nil
	}
	data, e := s.src.get(ctx, a.url, feedMediaBytes)
	if e != nil {
		s.notice = "media unavailable; refresh expired links"
		return nil, ErrGone
	}
	cfg, _, e := image.DecodeConfig(bytes.NewReader(data))
	if e != nil || cfg.Width < 2 || cfg.Height < 2 || cfg.Width > 8192 || cfg.Height > 8192 || int64(cfg.Width)*int64(cfg.Height) > 8*1024*1024 {
		s.notice = "invalid or oversized image"
		return nil, ErrGone
	}
	// Verify payload before exposing it to the shared renderer; dimensions are bounded.
	if _, _, e = image.Decode(bytes.NewReader(data)); e != nil {
		s.notice = "invalid image"
		return nil, ErrGone
	}
	return data, nil
}
func (f feedSource) get(ctx context.Context, raw string, most int64) ([]byte, error) {
	if !validFeedURL(raw) {
		return nil, errFeedInvalid
	}
	req, e := http.NewRequestWithContext(ctx, http.MethodGet, raw, nil)
	if e != nil {
		return nil, errFeedRequest
	}
	req.Header.Set("Accept", "application/feed+json, application/json, image/*")
	resp, e := f.client.Do(req)
	if e != nil {
		return nil, errFeedRequest
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK || resp.ContentLength > most {
		return nil, errFeedRequest
	}
	data, e := io.ReadAll(io.LimitReader(resp.Body, most+1))
	if e != nil || int64(len(data)) > most {
		return nil, errFeedRequest
	}
	return data, nil
}

// An automatic refresh never truncates an active ranked traversal.
func (s *feedSession) RefreshDue(now time.Time) bool {
	return now.Sub(s.opened) >= feedRefresh && s.next == "" && len(s.pending) == 0 && len(s.urls) == 0
}
