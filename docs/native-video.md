# Optional native URL-video fallback

The URL player retains its primary Go demux/audio and hardware-video path.
A host may opt in with `Options.Native.Helper`, naming an installed executable.
If opening the primary path fails, the URL is reopened with the **same HTTP
client** and passed to the helper as bounded byte ranges. Feed/MSE playback and
hosts without that option are unchanged. A supported MP4 must not silently lose
an unsupported video track and continue as audio-only.

The helper never receives URLs, transport credentials, or a listening endpoint.
Transport ownership, HTTP(S)/SMB authentication, bounded range reads and
cancellation remain with Go. Stop closes the source/pipes, terminates and reaps
the process. Startup is limited to 30 seconds. The host is responsible for
shipping/updating a compatible helper, its codec policy, sandbox/resource limits
and corresponding licenses/source. EchoLocal's Athena helper uses a minimal
FFmpeg build with no file/network protocols.

## Contract

All integers are little endian. Child fd 3 writes 16-byte input requests:
uint64 absolute offset, uint32 count (1..65536), uint32 reserved zero.
Child fd 4 reads a signed int32 response count (-1 read failure, 0 EOF), followed
by exactly that many bytes. Non-range sources are sequential only.

Stdout records have a 24-byte header: magic `ELV1`, uint32 kind, uint32 payload
length, uint32 reserved zero, int64 microsecond timestamp. Payloads are capped
at 4 MiB and validated before use.

- 1: first/only metadata, 24 bytes: uint32 width, height, codec FourCC,
  audio-present; int64 duration in microseconds.
- 2: stereo signed-16 little-endian PCM at 48 kHz, at most 65536 frames.
  Timestamp is the first output sample. INT64_MIN means append a resampler tail.
- 3: timestamped H.264 Annex-B access units, or tightly packed RGBA bytes.
- 4: a whitelisted diagnostic code, never an arbitrary upstream error.
- 5: clean end, empty payload.

A/V timestamps share one media origin; PCM gaps up to two seconds are filled,
overlaps are discarded. Larger gaps fail explicitly. Unsupported decoder errors,
bad protocol, memory/profile/dimension/frame-rate limits, source failures and
startup timeouts remain distinguishable without leaking input data.
The implementation helper source and tests define the exact code allowlist.

## Presentation and bounds

H.264 packets use the existing MediaCodec path. Software frames use the same
surface placement and audio-derived clock, pause, hide/preroll, late-frame
policy, flush and lifecycle. RGBA windows are limited to 854x480. The Go screen
queue holds three raw frames, the decoder queue six, and the native surface
queue three plus one in flight; no 90-frame raw allocation. Hardware buffering
is unchanged. PCM has a 64-packet queue; source reads retain existing bounds.
Raw pixels are copied into an ANativeWindow, not encoded and decoded again.

The optional device tests use explicitly supplied synthetic/public fixtures:

```sh
ECHOLOCAL_NATIVE_HELPER=./echolocal-legacy \
ECHOLOCAL_NATIVE_FIXTURES=./fixtures \
ECHOLOCAL_NATIVE_BOUNDARIES=./boundaries \
./urlplayer.test -test.run TestNative -test.skip TestNativeControls -test.v
```

They verify decode/routing, preservation of ordinary H.264/AAC, decoded flash/
audio-pulse alignment, seeking/cancellation, resource refusal and non-media
tracks. They do not claim physical display, speaker, acoustic synchronization
or sustained rendering performance. Those require the consuming device tests.

Refs rainu/echolocal#79.

MP4 AAC probes a bounded eight-frame prefix when native fallback is available: an LC ASC may hide implicit HE-AAC/SBR. Probing uses an isolated decoder, then restarts a fresh one for playback. Unsupported profiles discovered later fail explicitly rather than silently skipping all sound. Device interleave tests gate raw-picture consumption on the PCM clock, reproducing MPEG-PS packet-burst deadlocks that unthrottled decode tests miss.

A kept raw surface is reusable only at identical width and height. Hardware decoder size adaptation is unchanged. The regression covers larger, smaller, changed-height and same-area/different-aspect transitions.
