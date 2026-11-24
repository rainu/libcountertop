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
	} {
		v, yes, err := Parse(raw)
		if !yes || err != nil || v.ID != "M7lc1UVf-VE" {
			t.Fatalf("%s: %+v %t %v", raw, v, yes, err)
		}
	}
	for _, tc := range []struct {
		suffix string
		want   time.Duration
	}{{"&t=62", 62 * time.Second}, {"&t=1m2s", 62 * time.Second}, {"&start=90", 90 * time.Second}} {
		v, _, err := Parse("https://youtube.com/watch?v=M7lc1UVf-VE" + tc.suffix)
		if err != nil || v.Start != tc.want {
			t.Fatal(tc, v, err)
		}
	}
}
func TestUnsupportedYouTubeNeverFallsThroughAsDirectMedia(t *testing.T) {
	for _, raw := range []string{
		"https://youtube.com/playlist?list=abc", "https://music.youtube.com/", "https://youtu.be/M7lc1UVf-VE/extra",
		"https://youtube.com/watch?v=invalid", "https://youtube.com/watch?v=M7lc1UVf-VE&v=aaaaaaaaaaa",
		"https://youtube.com/watch?v=M7lc1UVf-VE&t=bad", "https://youtube.com/watch?v=M7lc1UVf-VE&t=99999999",
		"https://user:pass@youtube.com/watch?v=M7lc1UVf-VE", "https://youtube.com:444/watch?v=M7lc1UVf-VE",
		"https://youtube.com/watch?v=M7lc1UVf-VE#fragment", "ftp://youtu.be/M7lc1UVf-VE",
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
