package urlplayer

import (
	"context"
	"encoding/binary"
	"errors"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ygelfand/libcountertop/pkg/media/playback"
)

// Native enables the optional, separately installed on-device decoder. The
// helper receives only source bytes, never a URL or transport credentials.
type Native struct{ Helper string }

type nativeError string

func (e nativeError) Error() string { return "Video decoder: " + string(e) }
func nativeMessage(err error) string {
	var e nativeError
	if errors.As(err, &e) {
		return e.Error()
	}
	return ""
}

var nativeCodes = map[string]bool{}

func init() {
	for _, s := range strings.Fields("MISSING_TIMESTAMPS DECODER_MEMORY UNSUPPORTED_VIDEO_CODEC UNSUPPORTED_AUDIO_CODEC UNSUPPORTED_CODEC_PROFILE INVALID_COMPRESSED_DATA VIDEO_RESOURCE_LIMIT VIDEO_CONVERSION_FAILED AUDIO_RESOURCE_LIMIT AUDIO_CONVERSION_FAILED DECODE_FAILED H264_PACKET_FAILED UNSUPPORTED_OR_DAMAGED_CONTAINER NO_USABLE_VIDEO_TRACK SEEK_UNSUPPORTED MEDIA_READ_FAILED AUDIO_TIMESTAMP_GAP STARTUP_TIMEOUT HELPER_UNAVAILABLE HELPER_PROTOCOL HELPER_FAILED") {
		nativeCodes[s] = true
	}
}
func nativeFailure(code string) error {
	if !nativeCodes[code] {
		code = "HELPER_FAILED"
	}
	return nativeError(code)
}

type nativeAudio struct {
	data []int16
	at   time.Duration
	tail bool
}
type nativeFrame struct {
	data []byte
	at   time.Duration
}
type nativeDecoder struct {
	ctx      context.Context
	cancel   context.CancelFunc
	cmd      *exec.Cmd
	source   io.Closer
	pipes    []*os.File
	once     sync.Once
	audio    chan nativeAudio
	video    chan nativeFrame
	done     chan struct{}
	mu       sync.Mutex
	err      error
	hasAudio bool
	length   time.Duration
	cursor   int64
	end      atomic.Int64
}

func (d *nativeDecoder) Close() error {
	d.once.Do(func() {
		d.cancel()
		_ = d.source.Close()
		for _, f := range d.pipes {
			_ = f.Close()
		}
	})
	return nil
}
func (d *nativeDecoder) rate() int      { return outRate }
func (d *nativeDecoder) failure() error { d.mu.Lock(); defer d.mu.Unlock(); return d.err }
func (d *nativeDecoder) finish(err error) {
	d.mu.Lock()
	d.err = err
	d.mu.Unlock()
	close(d.audio)
	close(d.video)
	close(d.done)
}
func (d *nativeDecoder) next() ([]int16, error) {
	if !d.hasAudio {
		select {
		case <-d.done:
			if err := d.failure(); err != nil {
				return nil, err
			}
			if d.cursor >= int64(time.Duration(d.end.Load()).Seconds()*outRate) {
				return nil, io.EOF
			}
		default:
		}
		if d.length > 0 && d.cursor >= int64(d.length.Seconds()*outRate) {
			return nil, io.EOF
		}
		d.cursor += 960
		return make([]int16, 1920), nil
	}
	select {
	case a, ok := <-d.audio:
		if !ok {
			if err := d.failure(); err != nil {
				return nil, err
			}
			end := int64(time.Duration(d.end.Load()).Seconds() * outRate)
			if d.cursor >= end {
				return nil, io.EOF
			}
			n := min(int64(960), end-d.cursor)
			d.cursor += n
			return make([]int16, n*2), nil
		}
		at := d.cursor
		if !a.tail {
			at = int64(a.at.Seconds() * outRate)
		}
		gap := at - d.cursor
		if gap > 2*outRate {
			return nil, nativeFailure("AUDIO_TIMESTAMP_GAP")
		}
		if gap > 0 {
			a.data = append(make([]int16, gap*2), a.data...)
		}
		if gap < 0 {
			a.data = a.data[min(int64(len(a.data)), -gap*2):]
		}
		d.cursor += int64(len(a.data) / 2)
		if len(a.data) == 0 {
			return d.next()
		}
		return a.data, nil
	case <-d.ctx.Done():
		return nil, d.ctx.Err()
	}
}
func (d *nativeDecoder) picture(ctx context.Context) ([]byte, time.Duration, error) {
	select {
	case f, ok := <-d.video:
		if !ok {
			if err := d.failure(); err != nil {
				return nil, 0, err
			}
			return nil, 0, io.EOF
		}
		return f.data, f.at, nil
	case <-ctx.Done():
		return nil, 0, ctx.Err()
	case <-d.ctx.Done():
		return nil, 0, d.ctx.Err()
	}
}

type nativeRecord struct {
	kind uint32
	at   int64
	data []byte
}

func readNative(r io.Reader) (nativeRecord, error) {
	var h [24]byte
	if _, err := io.ReadFull(r, h[:]); err != nil {
		return nativeRecord{}, err
	}
	kind, n := binary.LittleEndian.Uint32(h[4:]), binary.LittleEndian.Uint32(h[8:])
	if binary.LittleEndian.Uint32(h[:]) != 0x31564c45 || binary.LittleEndian.Uint32(h[12:]) != 0 ||
		kind < 1 ||
		kind > 5 ||
		n > 4<<20 {
		return nativeRecord{}, nativeFailure("HELPER_PROTOCOL")
	}
	rec := nativeRecord{
		kind: kind,
		at:   int64(binary.LittleEndian.Uint64(h[16:])),
		data: make([]byte, n),
	}
	_, err := io.ReadFull(r, rec.data)
	return rec, err
}
func (d *nativeDecoder) run(r io.Reader, w, h int, codec uint32) {
	var err error
	defer func() {
		if err != nil {
			_ = d.Close()
		}
		wait := d.cmd.Wait()
		_ = d.Close()
		if err == nil && wait != nil {
			err = nativeFailure("HELPER_FAILED")
		}
		d.finish(err)
	}()
	for {
		var rec nativeRecord
		rec, err = readNative(r)
		if err != nil {
			if !errors.As(err, new(nativeError)) {
				err = nativeFailure("HELPER_FAILED")
			}
			return
		}
		switch rec.kind {
		case 2:
			if !d.hasAudio || len(rec.data)%4 != 0 || len(rec.data) > 256<<10 {
				err = nativeFailure("HELPER_PROTOCOL")
				return
			}
			if rec.at != -1<<63 &&
				(rec.at < -60*int64(time.Second/time.Microsecond) || rec.at > int64(30*24*time.Hour/time.Microsecond)) {
				err = nativeFailure("HELPER_PROTOCOL")
				return
			}
			a := nativeAudio{
				data: make([]int16, len(rec.data)/2),
				at:   time.Duration(rec.at) * time.Microsecond,
				tail: rec.at == -1<<63,
			}
			for i := range a.data {
				a.data[i] = int16(binary.LittleEndian.Uint16(rec.data[i*2:]))
			}
			select {
			case d.audio <- a:
			case <-d.ctx.Done():
				err = d.ctx.Err()
				return
			}
		case 3:
			if rec.at < -60*int64(time.Second/time.Microsecond) ||
				rec.at > int64(30*24*time.Hour/time.Microsecond) ||
				(codec == playback.RGBA && len(rec.data) != w*h*4) {
				err = nativeFailure("HELPER_PROTOCOL")
				return
			}
			at := time.Duration(rec.at) * time.Microsecond
			d.end.Store(int64(at + 40*time.Millisecond))
			select {
			case d.video <- nativeFrame{rec.data, at}:
			case <-d.ctx.Done():
				err = d.ctx.Err()
				return
			}
		case 4:
			err = nativeFailure(string(rec.data))
			return
		case 5:
			if len(rec.data) != 0 {
				err = nativeFailure("HELPER_PROTOCOL")
			}
			return
		default:
			err = nativeFailure("HELPER_PROTOCOL")
			return
		}
	}
}

// serveNative is the only input path. All byte access retains the caller's HTTP
// client (including its SMB transport), range limits, authentication and cancel.
func serveNative(ctx context.Context, s stream, requests, responses *os.File) {
	defer requests.Close()
	defer responses.Close()
	var pos int64
	var req [16]byte
	var ans [4]byte
	for {
		if _, err := io.ReadFull(requests, req[:]); err != nil {
			return
		}
		off := int64(binary.LittleEndian.Uint64(req[:]))
		n := binary.LittleEndian.Uint32(req[8:])
		if off < 0 || n == 0 || n > 65536 || binary.LittleEndian.Uint32(req[12:]) != 0 {
			return
		}
		if off != pos {
			if s.ranged == nil {
				return
			}
			if _, err := s.ranged.Seek(off, io.SeekStart); err != nil {
				return
			}
			pos = off
		}
		if ctx.Err() != nil {
			return
		}
		b := make([]byte, n)
		got, err := s.body.Read(b)
		for got == 0 && err == nil {
			got, err = s.body.Read(b)
		}
		count := int32(got)
		if err != nil && !errors.Is(err, io.EOF) && got == 0 {
			count = -1
		}
		binary.LittleEndian.PutUint32(ans[:], uint32(count))
		if _, e := responses.Write(ans[:]); e != nil {
			return
		}
		if got > 0 {
			if _, e := responses.Write(b[:got]); e != nil {
				return
			}
			pos += int64(got)
		}
	}
}

func (t *track) openNative(ctx context.Context) error {
	s, err := open(ctx, t.client, t.media.URL(), 0)
	if err != nil {
		return err
	}
	processCtx, cancel := context.WithCancel(ctx)
	d := &nativeDecoder{
		ctx:    ctx,
		cancel: cancel,
		source: s.body,
		audio:  make(chan nativeAudio, 64),
		video:  make(chan nativeFrame, 6),
		done:   make(chan struct{}),
		cursor: int64(t.at.Seconds() * outRate),
	}
	var reqR, reqW, respR, respW *os.File
	if reqR, reqW, err = os.Pipe(); err == nil {
		respR, respW, err = os.Pipe()
	}
	d.pipes = []*os.File{reqR, reqW, respR, respW}
	if err != nil {
		for _, f := range d.pipes {
			if f != nil {
				f.Close()
			}
		}
		cancel()
		s.body.Close()
		return nativeFailure("HELPER_UNAVAILABLE")
	}
	if !t.hold(d) {
		_ = d.Close()
		return io.ErrClosedPipe
	}
	seekable := "0"
	if s.ranged != nil {
		seekable = "1"
	}
	d.cmd = exec.CommandContext(
		processCtx,
		t.player.o.Native.Helper,
		strconv.FormatInt(s.size, 10),
		seekable,
		strconv.FormatInt(t.at.Microseconds(), 10),
	)
	d.cmd.ExtraFiles = []*os.File{reqW, respR}
	stdout, err := d.cmd.StdoutPipe()
	if err != nil {
		_ = d.Close()
		return nativeFailure("HELPER_UNAVAILABLE")
	}
	if err = d.cmd.Start(); err != nil {
		_ = d.Close()
		return nativeFailure("HELPER_UNAVAILABLE")
	}
	reqW.Close()
	respR.Close()
	go serveNative(processCtx, s, reqR, respW)
	timedOut := atomic.Bool{}
	timer := time.AfterFunc(30*time.Second, func() { timedOut.Store(true); _ = d.Close() })
	meta, err := readNative(stdout)
	timer.Stop()
	if err == nil && meta.kind == 4 {
		err = nativeFailure(string(meta.data))
	}
	if timedOut.Load() {
		err = nativeFailure("STARTUP_TIMEOUT")
	}
	if err == nil && (meta.kind != 1 || len(meta.data) != 24) {
		err = nativeFailure("HELPER_PROTOCOL")
	}
	if err != nil {
		_ = d.Close()
		_ = d.cmd.Wait()
		if nativeMessage(err) == "" {
			err = nativeFailure("HELPER_FAILED")
		}
		return err
	}
	w, h := int(
		binary.LittleEndian.Uint32(meta.data),
	), int(
		binary.LittleEndian.Uint32(meta.data[4:]),
	)
	codec := binary.LittleEndian.Uint32(meta.data[8:])
	length := int64(binary.LittleEndian.Uint64(meta.data[16:]))
	if w < 1 || h < 1 || w > 1920 || h > 1080 ||
		(codec != playback.H264 && codec != playback.RGBA) ||
		(codec == playback.RGBA && (w > 854 || h > 480)) ||
		length < 0 ||
		length > int64(30*24*time.Hour/time.Microsecond) {
		_ = d.Close()
		_ = d.cmd.Wait()
		return nativeFailure("HELPER_PROTOCOL")
	}
	d.hasAudio = binary.LittleEndian.Uint32(meta.data[12:]) != 0
	d.length = time.Duration(length) * time.Microsecond
	t.length = d.length
	t.seekable = s.ranged != nil
	t.format = "native"
	t.picture = playback.Picture{Codec: codec, Width: w, Height: h, Next: d.picture}
	go d.run(stdout, w, h, codec)
	return t.use(d)
}
