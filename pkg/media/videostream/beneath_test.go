package videostream

import (
	"testing"

	"github.com/ygelfand/libcountertop/pkg/display/surface"
	"github.com/ygelfand/libcountertop/pkg/media/playback"
)

func TestAKeptDecoderServesAnySizeOfTheSameCodecAndSession(t *testing.T) {
	c := &surface.Client{}
	var b Beneath
	kept := Spec{Codec: playback.H264, W: 426, H: 240, Session: 3}
	b.Adopt(c, 7, kept)
	for _, tc := range []struct {
		name string
		spec Spec
		ok   bool
	}{
		{"larger", Spec{Codec: playback.H264, W: 1280, H: 720, Session: 3}, true},
		{"smaller", Spec{Codec: playback.H264, W: 256, H: 144, Session: 3}, true},
		{"another session", Spec{Codec: playback.H264, W: 426, H: 240, Session: 4}, false},
		{"clear", Spec{Codec: playback.H264, W: 426, H: 240}, false},
		{"another codec", Spec{Codec: playback.H264 + 1, W: 426, H: 240, Session: 3}, false},
	} {
		id, spec, ok := b.Reuse(c, tc.spec)
		if ok != tc.ok || (ok && (id != 7 || spec != kept)) {
			t.Errorf("%s: Reuse(%+v) = %d, %+v, %v; want 7, %+v, %v",
				tc.name, tc.spec, id, spec, ok, kept, tc.ok)
		}
	}
	if _, _, ok := b.Reuse(&surface.Client{}, kept); ok {
		t.Error("another helper reused the kept decoder")
	}
}

func TestAKeptRawSurfaceRequiresIdenticalDimensions(t *testing.T) {
	c := &surface.Client{}
	var b Beneath
	kept := Spec{Codec: surface.RGBA, W: 480, H: 270}
	b.Adopt(c, 7, kept)
	if id, spec, ok := b.Reuse(c, kept); !ok || id != 7 || spec != kept {
		t.Fatal("same-size raw surface was not reusable")
	}
	for _, size := range [][2]int{{640, 480}, {320, 240}, {112, 80}, {270, 480}, {480, 360}} {
		want := Spec{Codec: surface.RGBA, W: size[0], H: size[1]}
		if _, _, ok := b.Reuse(c, want); ok {
			t.Fatalf("raw surface reused old geometry for %dx%d", size[0], size[1])
		}
	}
}
