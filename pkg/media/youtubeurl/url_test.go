package youtubeurl

import (
	"testing"
	"time"
)

func TestSupportedSingleVideos(t *testing.T) {
	for _, raw := range []string{
		"https://www.youtube.com/watch?v=M7lc1UVf-VE",
		"https://youtube.com/watch?v=M7lc1UVf-VE&list=ignored",
		"https://youtu.be/M7lc1UVf-VE?si=ignored",
		"https://music.youtube.com/watch?v=M7lc1UVf-VE",
		"https://m.youtube.com/watch?v=M7lc1UVf-VE",
		"https://www.youtube.com/shorts/M7lc1UVf-VE",
		"https://www.youtube.com/embed/M7lc1UVf-VE",
		"https://music.youtube.com/podcast/M7lc1UVf-VE",
		"https://www.youtube.com/live/M7lc1UVf-VE?si=share",
		"https://m.youtube.com/shorts/M7lc1UVf-VE/?feature=share",
		"https://www.youtube-nocookie.com/embed/M7lc1UVf-VE",
		"https://gaming.youtube.com/watch?v=M7lc1UVf-VE",
		"https://youtube.com/v/M7lc1UVf-VE",
		"https://youtube.com/e/M7lc1UVf-VE",
		"https://youtube.com/watch/M7lc1UVf-VE",
		"https://youtube.com/watch/?v=M7lc1UVf-VE",
		"https://youtu.be/M7lc1UVf-VE/",
		"https://youtube.com/watch?v=M7lc1UVf-VE#details",
		"https://youtube.com/attribution_link?u=%2Fwatch%3Fv%3DM7lc1UVf-VE%26feature%3Dshare",
		"https://youtube.com/redirect?q=https%3A%2F%2Fyoutu.be%2FM7lc1UVf-VE",
		"https://youtube.com/redirect?url=https%3A%2F%2Fmusic.youtube.com%2Fpodcast%2FM7lc1UVf-VE",
	} {
		v, yes, err := Parse(raw)
		if !yes || err != nil || v.ID != "M7lc1UVf-VE" {
			t.Fatalf("%s: %+v %t %v", raw, v, yes, err)
		}
	}
	for _, tc := range []struct {
		suffix string
		want   time.Duration
	}{{"&t=62", 62 * time.Second}, {"&t=1m2s", 62 * time.Second}, {"&start=90", 90 * time.Second}, {"#t=1m2s", 62 * time.Second}, {"#start=90", 90 * time.Second}} {
		v, _, err := Parse("https://youtube.com/watch?v=M7lc1UVf-VE" + tc.suffix)
		if err != nil || v.Start != tc.want {
			t.Fatal(tc, v, err)
		}
	}
}
func TestUnsupportedYouTubeNeverFallsThroughAsDirectMedia(t *testing.T) {
	for _, raw := range []string{
		"https://youtube.com/playlist?list=abc",
		"https://music.youtube.com/podcast/invalid",
		"https://youtube.com/live/M7lc1UVf-VE/extra",
		"https://music.youtube.com/podcast/M7lc1UVf-VE/extra",
		"https://youtube.com/clip/opaque-clip-id",
		"https://youtube.com/@channel", "https://youtube.com/results?search_query=test",
		"https://youtube.com/redirect?q=https://example.com/file.mp4",
		"https://youtube.com/redirect?q=//youtube.com.example.com/watch",
		"https://youtube.com/redirect?q=https://user:pass@youtube.com/watch?v=M7lc1UVf-VE",
		"https://youtube.com/redirect?q=/watch%3Fv=M7lc1UVf-VE&q=/watch%3Fv=aaaaaaaaaaa",
		"https://youtube.com/redirect?q=",
		"https://youtube.com/attribution_link?u=/attribution_link%3Fu%3D/attribution_link%253Fu%253D/attribution_link%25253Fu%25253D/watch%2525253Fv%2525253DM7lc1UVf-VE",
		"https://youtu.be/M7lc1UVf-VE#t=invalid", "https://music.youtube.com/", "https://youtu.be/M7lc1UVf-VE/extra",
		"https://youtube.com/watch?v=invalid", "https://youtube.com/watch?v=M7lc1UVf-VE&v=aaaaaaaaaaa",
		"https://youtube.com/watch?v=M7lc1UVf-VE&t=bad", "https://youtube.com/watch?v=M7lc1UVf-VE&t=99999999",
		"https://user:pass@youtube.com/watch?v=M7lc1UVf-VE", "https://youtube.com:444/watch?v=M7lc1UVf-VE",
		"ftp://youtu.be/M7lc1UVf-VE",
	} {
		_, yes, err := Parse(raw)
		if !yes || err == nil {
			t.Fatalf("accepted/fell through %s", raw)
		}
	}
	for _, raw := range []string{"https://youtube.com.example.com/watch?v=M7lc1UVf-VE", "https://example.com/youtube.com/video.mp4", "https://notyoutube.com/watch?v=M7lc1UVf-VE"} {
		_, yes, err := Parse(raw)
		if yes || err != nil {
			t.Fatalf("misrouted %s", raw)
		}
	}
}

func TestOwnerPodcastURL(t *testing.T) {
	v, youtube, err := Parse("https://music.youtube.com/podcast/cJjMmDeOAZ4")
	if err != nil || !youtube || v.ID != "cJjMmDeOAZ4" {
		t.Fatalf("podcast share link rejected: %+v %t %v", v, youtube, err)
	}
}
