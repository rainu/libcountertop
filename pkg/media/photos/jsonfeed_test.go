package photos

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/ygelfand/libcountertop/pkg/settings/schema"
)

func feedConfig(raw string) schema.Photos {
	c := schema.DefaultPhotos()
	c.Feed = schema.JSONFeed{Enabled: true, URL: raw}
	c.AppSource = schema.SourceFeed
	return c
}
func TestJSONFeedRankedPaginationThroughSharedLibrary(t *testing.T) {
	pages, downloads := 0, 0
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/image/") {
			downloads++
			w.Write(twoTone(8, 4))
			return
		}
		pages++
		page := 0
		fmt.Sscanf(r.URL.Path, "/page/%d", &page)
		items := []any{}
		for i := range 30 {
			n := page*30 + i
			items = append(items, map[string]any{"id": fmt.Sprint(n), "attachments": []any{map[string]any{"url": fmt.Sprintf("%s/image/%d?signature=opaque", srv.URL, n), "mime_type": "image/jpeg"}}})
		}
		next := ""
		if page < 3 {
			next = fmt.Sprintf("%s/page/%d", srv.URL, page+1)
		}
		json.NewEncoder(w).Encode(map[string]any{"version": "https://jsonfeed.org/version/1.1", "title": "Ranked", "items": items, "next_url": next})
	}))
	defer srv.Close()
	l, _ := library(t, feedConfig(srv.URL+"/page/0?query=ordinary"), t.TempDir())
	app := l.Source("app")
	l.step(context.Background())
	if pages != 1 || downloads != 4 {
		t.Fatalf("eager fetch: pages=%d downloads=%d", pages, downloads)
	}
	for i := range 120 {
		l.step(context.Background())
		p, ok := app.Next()
		if !ok {
			t.Fatalf("missing ranked item %d, state %+v", i, l.State("feed"))
		}
		// IDs end in their sequential (1-based) attachment position, irrespective of pages.
		if !strings.HasSuffix(p.ID, fmt.Sprintf("-%06x", i+1)) {
			t.Fatalf("out of order at %d: %s", i, p.ID)
		}
		if len(l.feedLibrary.ready) > 8 || len(l.feedLibrary.feeds[0].deck) > 48 {
			t.Fatal("unbounded cache/deck")
		}
	}
	l.step(context.Background())
	if _, ok := app.Next(); ok {
		t.Fatal("ranked list silently looped its final cache")
	}
	if pages != 4 || downloads != 120 {
		t.Fatalf("pages %d downloads %d", pages, downloads)
	}
}
func TestJSONFeedMixedMediaAndBadLinks(t *testing.T) {
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/good":
			w.Write(twoTone(8, 4))
		case "/expired":
			http.Error(w, "expired", 403)
		case "/bad":
			w.Write([]byte("not an image"))
		default:
			json.NewEncoder(w).Encode(map[string]any{
				"version": "https://jsonfeed.org/version/1.1", "title": "Mixed", "items": []any{
					map[string]any{"id": "x", "attachments": []any{
						map[string]string{"url": srv.URL + "/expired?secret=hidden", "mime_type": "image/jpeg"},
						map[string]string{"url": srv.URL + "/bad", "mime_type": "image/png"},
						map[string]string{"url": "file:///secret", "mime_type": "image/jpeg"},
						map[string]string{"url": srv.URL + "/gif", "mime_type": "image/gif"},
						map[string]string{"url": srv.URL + "/good", "mime_type": "image/jpeg"},
						map[string]string{"url": srv.URL + "/movie?token=hidden", "mime_type": "video/mp4"},
					}},
				},
			})
		}
	}))
	defer srv.Close()
	l, _ := library(t, feedConfig(srv.URL+"/feed"), t.TempDir())
	l.step(context.Background())
	l.step(context.Background())
	app := l.Source("app")
	first, ok := app.Next()
	if !ok || len(first.Data) == 0 || first.URL != "" {
		t.Fatalf("image missing %+v", first)
	}
	second, ok := app.Next()
	if !ok || second.URL != srv.URL+"/movie?token=hidden" || second.MIME != "video/mp4" || len(second.Data) != 0 {
		t.Fatalf("video missing")
	}
	if l.State("feed").Status != FeedPartial {
		t.Fatalf("bad attachments not reported: %+v", l.State("feed"))
	}
	if strings.Contains(l.State("feed").Says(), "secret") {
		t.Fatal("secret in status")
	}
}
func TestJSONFeedExplicitSelectionPreservesOtherProviders(t *testing.T) {
	c := feedConfig("https://example.com/feed")
	c.Immich = schema.Immich{Enabled: true, Server: "https://existing.example", Key: "unchanged"}
	c.Google.Enabled = true
	c.Google.Albums[0].URL = "https://photos.example/album"
	before := c
	table := NewTable()
	for _, change := range []struct{ name, value string }{{"feed.url", "https://example.com/feed?q=opaque"}, {"feed.enabled", "on"}, {"app.source", "feed"}, {"wallpaper.source", "mixed"}, {"saver.source", "existing"}} {
		row, ok := table.Find(change.name)
		if !ok {
			t.Fatal(change.name)
		}
		if e := row.Write(&c, change.value); e != nil {
			t.Fatal(e)
		}
	}
	if c.Immich != before.Immich || c.Google != before.Google {
		t.Fatal("existing sources mutated")
	}
	if c.AppSource != schema.SourceFeed || c.WallpaperSource != schema.SourceMixed {
		t.Fatal("selection missing")
	}
	row, _ := table.Find("feed.url")
	if row.Visible(&c) != "set" {
		t.Fatal("feed URL exposed to UI/entity")
	}
}
func TestJSONFeedValidationLimitsAndPrivacy(t *testing.T) {
	for _, raw := range []string{"file:///secret", "https://u:p@example.com/f", "ftp://example.com/f", "https://example.com/f#fragment", " " + strings.Repeat("x", 5000)} {
		if validFeedURL(raw) {
			t.Fatal("unsafe URL accepted")
		}
	}
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/oversize":
			w.Write([]byte(strings.Repeat(" ", feedPageBytes+1)))
		case "/bad":
			w.Write([]byte("{invalid"))
		case "/cycle":
			fmt.Fprintf(w, `{"version":"https://jsonfeed.org/version/1.1","title":"x","items":[],"next_url":%q}`, srv.URL+"/cycle")
		case "/redirect":
			http.Redirect(w, r, "file:///secret", 302)
		default:
			w.Write([]byte(`{"version":"https://jsonfeed.org/version/1","title":"old","items":[]}`))
		}
	}))
	defer srv.Close()
	for _, path := range []string{"/oversize", "/bad", "/old", "/redirect"} {
		_, _, e := JSONFeed().Open(context.Background(), feedConfig(srv.URL+path+"?token=do-not-leak"))
		if e == nil || strings.Contains(e.Error(), "do-not-leak") || strings.Contains(e.Error(), srv.URL) {
			t.Fatalf("unsafe error %v", e)
		}
	}
	sess, _, e := JSONFeed().Open(context.Background(), feedConfig(srv.URL+"/cycle"))
	if e != nil {
		t.Fatal(e)
	}
	s := sess.(*feedSession)
	_, _, e = s.Draw(context.Background(), time.Now(), true)
	if e != nil || s.next != "" || s.Notice() == "" {
		t.Fatal("cycle not stopped")
	}
	if !s.RefreshDue(time.Now().Add(feedRefresh + time.Second)) {
		t.Fatal("automatic refresh missing")
	}
}
func TestJSONFeedRefreshClearsOnlyItsOwnCache(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/image" {
			w.Write(twoTone(8, 4))
			return
		}
		fmt.Fprintf(w, `{"version":"https://jsonfeed.org/version/1.1","title":"x","items":[{"id":"1","attachments":[{"url":%q,"mime_type":"image/jpeg"}]}]}`, "http://"+r.Host+"/image")
	}))
	defer srv.Close()
	l, c := library(t, feedConfig(srv.URL), t.TempDir())
	l.step(context.Background())
	p, ok := l.Source("app").Next()
	if !ok {
		t.Fatal("missing first image")
	}
	c.Feed.Refresh = !c.Feed.Refresh
	l.step(context.Background())
	p2, ok := l.Source("app").Next()
	if !ok || p.ID != p2.ID {
		t.Fatal("refresh did not restart")
	}
	c.Feed.Enabled = false
	l.step(context.Background())
	if _, ok := l.Source("app").Next(); ok {
		t.Fatal("disabled feed leaked into default source")
	}
	if l.State("feed").Status != Off {
		t.Fatal("feed still active")
	}
}

func TestJSONFeedWallpaperAndSaverNeverDecodeVideo(t *testing.T) {
	for _, consumer := range []string{"wallpaper", "saver"} {
		c := feedConfig("https://example.com/feed")
		c.WallpaperSource = schema.SourceFeed
		c.SaverSource = schema.SourceFeed
		l, _ := library(t, c, t.TempDir())
		child := l.feedLibrary
		os.MkdirAll(child.o.Dir, 0700)
		child.ready = []entry{{Source: "feed", ID: "video", URL: "https://example.com/v.mp4", MIME: "video/mp4"}, {Source: "feed", ID: "image"}}
		if err := os.WriteFile(child.file(child.ready[1]), twoTone(8, 4), 0600); err != nil {
			t.Fatal(err)
		}
		p, ok := l.Source(consumer).Next()
		if !ok || p.ID != "image" || p.URL != "" {
			t.Fatalf("%s selected a video", consumer)
		}
	}
}
func TestJSONFeedDefaultDoesNotMixAndMixedKeepsFeedOrder(t *testing.T) {
	c := feedConfig("https://example.com/feed")
	c.AppSource = schema.SourceExisting
	l, settings := library(t, c, t.TempDir())
	l.ready = []entry{{Source: "immich", ID: "old"}}
	l.feedLibrary.ready = []entry{{Source: "feed", ID: "one", URL: "https://example.com/1", MIME: "video/mp4"}, {Source: "feed", ID: "two", URL: "https://example.com/2", MIME: "video/mp4"}}
	if e := os.WriteFile(l.file(l.ready[0]), twoTone(8, 4), 0600); e != nil {
		t.Fatal(e)
	}
	source := l.Source("app")
	for range 3 {
		if p, ok := source.Next(); !ok || p.Source != "immich" {
			t.Fatal("feed changed the existing selection")
		}
	}
	settings.AppSource = schema.SourceMixed
	var ranked []string
	for range 4 {
		p, ok := source.Next()
		if ok && p.Source == "feed" {
			ranked = append(ranked, p.ID)
		}
	}
	if strings.Join(ranked, ",") != "one,two" {
		t.Fatalf("rank lost: %v", ranked)
	}
}

func TestUnsupportedOnlyFeedReportsPartialRatherThanHidingTheProblem(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"version":"https://jsonfeed.org/version/1.1","title":"Unsupported","items":[{"id":"1","attachments":[{"url":"https://example.com/a.gif","mime_type":"image/gif"}]}]}`))
	}))
	defer srv.Close()
	l, _ := library(t, feedConfig(srv.URL), t.TempDir())
	l.step(context.Background())
	if l.State("feed").Status != FeedPartial {
		t.Fatal("unsupported-only feed hid the reason")
	}
}
