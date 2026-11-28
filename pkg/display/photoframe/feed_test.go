package photoframe

import (
	"github.com/ygelfand/libcountertop/pkg/media/photos"
	"testing"
	"time"
)

type feedTestSource struct{}

func (feedTestSource) Next() (photos.Picture, bool) {
	return photos.Picture{ID: "v", URL: "https://example.com/v.mp4", MIME: "video/mp4"}, true
}
func TestFeedVideoUsesNavigableCardNotImageDecoder(t *testing.T) {
	f := New("app", feedTestSource{})
	if !f.Next(time.Now()) {
		t.Fatal("video tried to decode as an image")
	}
	p, ok := f.Showing()
	if !ok || p.MIME != "video/mp4" {
		t.Fatal("video metadata lost")
	}
}
