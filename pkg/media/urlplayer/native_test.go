package urlplayer

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"io"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ygelfand/libcountertop/pkg/media/cast"
	"github.com/ygelfand/libcountertop/pkg/media/playback"
)

func TestNativeProtocolBounds(t *testing.T) {
	var h [24]byte
	binary.LittleEndian.PutUint32(h[:], 0x31564c45)
	binary.LittleEndian.PutUint32(h[4:], 3)
	binary.LittleEndian.PutUint32(h[8:], 4<<20+1)
	if _, err := readNative(bytes.NewReader(h[:])); nativeMessage(err) == "" {
		t.Fatal("unbounded record accepted")
	}
	binary.LittleEndian.PutUint32(h[8:], 0)
	if _, err := readNative(bytes.NewReader(h[:])); err != nil {
		t.Fatal(err)
	}
	binary.LittleEndian.PutUint32(h[4:], 6)
	if _, err := readNative(bytes.NewReader(h[:])); nativeMessage(err) == "" {
		t.Fatal("unknown record accepted")
	}
	if nativeMessage(nativeFailure("secret-source-url")) != "Video decoder: HELPER_FAILED" {
		t.Fatal("untrusted error was exposed")
	}
}

func TestNativeAudioTimeline(t *testing.T) {
	d := &nativeDecoder{ctx: context.Background(), hasAudio: true, audio: make(chan nativeAudio, 3)}
	d.audio <- nativeAudio{at: time.Second / 48000 * 2, data: []int16{1, 2, 3, 4}}
	d.audio <- nativeAudio{at: 0, data: []int16{5, 6, 7, 8}}
	d.audio <- nativeAudio{at: time.Second * 3, data: []int16{9, 10}}
	out, err := d.next()
	if err != nil || len(out) != 6 || out[2] != 1 {
		t.Fatalf("gap: %v %v", out, err)
	}
	if _, err = d.next(); nativeMessage(err) != "Video decoder: AUDIO_TIMESTAMP_GAP" {
		t.Fatalf("unexpected %v", err)
	}
}

// Opt-in decode contract, runnable as a CGO-disabled ARM test binary on Android.
// Its generated/public fixtures and decoder are supplied explicitly, not fetched.
func TestNativeFixtureMatrix(t *testing.T) {
	helper, root := os.Getenv("ECHOLOCAL_NATIVE_HELPER"), os.Getenv("ECHOLOCAL_NATIVE_FIXTURES")
	if helper == "" || root == "" {
		t.Skip("set explicit native helper and fixture directory")
	}
	var manifest []struct {
		File      string
		Generator []string
	}
	raw, _ := os.ReadFile(filepath.Join(root, "manifest.json"))
	_ = json.Unmarshal(raw, &manifest)
	synthetic := map[string]bool{}
	for _, m := range manifest {
		synthetic[m.File] = len(m.Generator) > 0
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.FileServer(http.Dir(root)))
	defer server.Close()
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) == ".json" {
			continue
		}
		t.Run(e.Name(), func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
			defer cancel()
			p := New(Options{Native: &Native{Helper: helper}, Screen: true, HTTP: server.Client()})
			tr := p.track(cast.Media{ContentID: server.URL + "/" + e.Name()}, nil, 0)
			defer tr.close()
			// Exercise the shared native contract independently of the primary hardware
			// path; the application integration suite covers normal routing separately.
			if err := tr.openNative(ctx); err != nil {
				t.Fatal(err)
			}
			var count atomic.Int64
			var flashes []time.Duration
			images := make(chan error, 1)
			go func() {
				white := false
				for {
					b, at, err := tr.picture.Next(ctx)
					if err != nil {
						images <- err
						return
					}
					if len(b) == 0 {
						images <- errors.New("empty picture")
						return
					}
					if synthetic[e.Name()] && tr.picture.Codec == playback.RGBA && len(b) >= 4 {
						lit := int(b[0])+int(b[1])+int(b[2]) > 650
						if lit && !white {
							flashes = append(flashes, at)
						}
						white = lit
					}
					count.Add(1)
				}
			}()
			samples := 0
			var pulses []time.Duration
			lastPulse := time.Duration(-1 << 62)
			for {
				pcm, err := tr.dec.next()
				for i := 0; i < len(pcm); i += 2 {
					at := time.Duration(int64(samples+i/2) * int64(time.Second) / 48000)
					if (pcm[i] > 2500 || pcm[i] < -2500) && at-lastPulse > 200*time.Millisecond {
						pulses = append(pulses, at)
						lastPulse = at
					}
				}
				samples += len(pcm) / 2
				if errors.Is(err, io.EOF) {
					break
				}
				if err != nil {
					t.Fatal(err)
				}
			}
			if err := <-images; !errors.Is(err, io.EOF) {
				t.Fatal(err)
			}
			if count.Load() < 1 || samples < 1 {
				t.Fatal("missing decoded A/V")
			}
			if synthetic[e.Name()] && tr.picture.Codec == playback.RGBA {
				matched := 0
				for _, flash := range flashes {
					if flash < 500*time.Millisecond {
						continue
					}
					best := math.MaxFloat64
					for _, pulse := range pulses {
						best = min(best, math.Abs((pulse - flash).Seconds()))
					}
					if best > 0.18 {
						t.Fatalf("decoded A/V pulse offset %.3fs at %v", best, flash)
					}
					matched++
				}
				if matched < 1 {
					t.Fatal("missing synchronization markers")
				}
			}
			t.Logf("codec=%08x size=%dx%d frames=%d pcm_frames=%d duration=%v", tr.picture.Codec, tr.picture.Width, tr.picture.Height, count.Load(), samples, tr.length)
			if tr.picture.Codec != playback.H264 && tr.picture.Codec != playback.RGBA {
				t.Fatal("unexpected output")
			}
		})
	}
}

func TestNativeFixtureRouting(t *testing.T) {
	helper, root := os.Getenv("ECHOLOCAL_NATIVE_HELPER"), os.Getenv("ECHOLOCAL_NATIVE_FIXTURES")
	if helper == "" || root == "" {
		t.Skip("explicit native test fixtures required")
	}
	server := httptest.NewServer(http.FileServer(http.Dir(root)))
	defer server.Close()
	entries, _ := os.ReadDir(root)
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) == ".json" {
			continue
		}
		t.Run(e.Name(), func(t *testing.T) {
			p := New(Options{Native: &Native{Helper: helper}, Screen: true, HTTP: server.Client()})
			tr := p.track(cast.Media{ContentID: server.URL + "/" + e.Name()}, nil, 0)
			defer tr.close()
			if err := tr.begin(tr.ctx); err != nil {
				t.Fatal(err)
			}
			if !tr.Pictured() {
				t.Fatal("video silently became audio-only")
			}
			if e.Name() == "mp4-h264-aac-lc.mp4" && tr.format != MP4 {
				t.Fatal("existing hardware path was replaced")
			}
			t.Logf("route=%s codec=%08x", tr.format, tr.picture.Codec)
		})
	}
}
func TestNativeSeekAndStop(t *testing.T) {
	helper, root := os.Getenv("ECHOLOCAL_NATIVE_HELPER"), os.Getenv("ECHOLOCAL_NATIVE_FIXTURES")
	if helper == "" || root == "" {
		t.Skip("explicit native test fixtures required")
	}
	server := httptest.NewServer(http.FileServer(http.Dir(root)))
	defer server.Close()
	for _, name := range []string{"asf-wmv2-wma2.wmv", "avi-h264-mp3.avi"} {
		t.Run(name, func(t *testing.T) {
			p := New(Options{Native: &Native{Helper: helper}, Screen: true, HTTP: server.Client()})
			tr := p.track(cast.Media{ContentID: server.URL + "/" + name}, nil, time.Second)
			defer tr.close()
			if err := tr.openNative(tr.ctx); err != nil {
				t.Fatal(err)
			}
			d := tr.dec.(*nativeDecoder)
			_, at, err := tr.picture.Next(tr.ctx)
			if err != nil {
				t.Fatal(err)
			}
			if tr.picture.Codec == playback.RGBA && at < time.Second {
				t.Fatal("software seek did not discard preroll")
			}
			tr.close()
			select {
			case <-d.done:
			case <-time.After(2 * time.Second):
				t.Fatal("helper did not exit on stop")
			}
			if d.cmd.ProcessState == nil {
				t.Fatal("helper not reaped")
			}
		})
	}
}

func TestNativeBoundaries(t *testing.T) {
	helper, root := os.Getenv("ECHOLOCAL_NATIVE_HELPER"), os.Getenv("ECHOLOCAL_NATIVE_BOUNDARIES")
	if helper == "" || root == "" {
		t.Skip("explicit boundary fixtures required")
	}
	server := httptest.NewServer(http.FileServer(http.Dir(root)))
	defer server.Close()
	for _, name := range []string{"oversize.avi", "overrate.avi", "data.mp4"} {
		t.Run(name, func(t *testing.T) {
			p := New(Options{Native: &Native{Helper: helper}, Screen: true, HTTP: server.Client()})
			tr := p.track(cast.Media{ContentID: server.URL + "/" + name}, nil, 0)
			defer tr.close()
			err := tr.begin(tr.ctx)
			if name == "data.mp4" {
				if err != nil {
					t.Fatal(err)
				}
				if !tr.Pictured() || tr.picture.Codec != playback.H264 {
					t.Fatal("non-media track prevented usable hardware video")
				}
				return
			}
			if nativeMessage(err) != "Video decoder: VIDEO_RESOURCE_LIMIT" {
				t.Fatalf("wanted resource limit, got %v", err)
			}
		})
	}
}
