package nativeapi

import (
	"bufio"
	"context"
	"encoding/binary"
	"io"
	"log/slog"
	"net"
	"testing"
	"time"

	esphome "github.com/ygelfand/go-esphome-device"
	"github.com/ygelfand/go-esphome-device/api"
	"google.golang.org/protobuf/proto"
)

// Exercise the real SDK server binding, not only direct action dispatch.
func TestConcurrentActionWrapperPreservesLiveEntityStates(t *testing.T) {
	player := &esphome.MediaPlayer{Base: esphome.Base{ObjectID: "video", DeviceID: 3}}
	player.SetState(esphome.MediaPlayerIdle)
	ents := esphome.NewEntities()
	if err := ents.Add(player); err != nil {
		t.Fatal(err)
	}
	receiptDelivered := make(chan bool, 1)
	action := &esphome.Action{
		Name:    "synthetic_transfer",
		Answers: true,
		Run: func(esphome.Call) (any, error) {
			return &wireReceipt{delivered: receiptDelivered}, nil
		},
	}
	if err := ents.AddActions(action); err != nil {
		t.Fatal(err)
	}
	closed := make(chan struct{}, 1)
	next := esphome.Chain(ents, &closedHandler{closed: closed})
	h := &actionHandler{
		Entities: ents,
		next:     next,
		slots:    make(chan struct{}, 2),
		actions:  deliveryActionMap([]*esphome.Action{action}, []string{action.Name}),
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	server := &esphome.Server{
		Info:    esphome.Info{Name: "synthetic-video"},
		Handler: h,
		Logger:  slog.New(slog.DiscardHandler),
	}
	done := make(chan struct{})
	go func() { defer close(done); _ = server.Serve(ctx, listener) }()
	conn, err := net.Dial("tcp", listener.Addr().String())
	if err != nil {
		cancel()
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close(); cancel(); <-done })
	reader := bufio.NewReader(conn)
	send := func(m proto.Message) {
		t.Helper()
		info, ok := esphome.Lookup(m)
		if !ok {
			t.Fatal("unknown message")
		}
		raw, e := proto.Marshal(m)
		if e != nil {
			t.Fatal(e)
		}
		head := []byte{0}
		head = binary.AppendUvarint(head, uint64(len(raw)))
		head = binary.AppendUvarint(head, uint64(info.ID))
		if _, e = conn.Write(append(head, raw...)); e != nil {
			t.Fatal(e)
		}
	}
	recv := func() proto.Message {
		t.Helper()
		_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
		magic, e := reader.ReadByte()
		if e != nil || magic != 0 {
			t.Fatalf("frame preamble: %v %d", e, magic)
		}
		size, e := binary.ReadUvarint(reader)
		if e != nil {
			t.Fatal(e)
		}
		kind, e := binary.ReadUvarint(reader)
		if e != nil {
			t.Fatal(e)
		}
		raw := make([]byte, size)
		if _, e = io.ReadFull(reader, raw); e != nil {
			t.Fatal(e)
		}
		m, ok := esphome.NewMessage(uint32(kind))
		if !ok {
			t.Fatal("unknown response")
		}
		if e = proto.Unmarshal(raw, m); e != nil {
			t.Fatal(e)
		}
		return m
	}
	send(
		&api.HelloRequest{
			ClientInfo:      "synthetic-native-test",
			ApiVersionMajor: esphome.APIVersionMajor,
			ApiVersionMinor: esphome.APIVersionMinor,
		},
	)
	if _, ok := recv().(*api.HelloResponse); !ok {
		t.Fatal("no hello")
	}
	// Use the key actually published by SDK discovery, not our own hash.
	send(&api.ListEntitiesRequest{})
	var discovered uint32
discovery:
	for {
		switch m := recv().(type) {
		case *api.ListEntitiesServicesResponse:
			if m.Name == action.Name {
				discovered = m.Key
			}
		case *api.ListEntitiesDoneResponse:
			break discovery
		}
	}
	if discovered == 0 {
		t.Fatal("receipt-aware action not discovered")
	}
	send(&api.ExecuteServiceRequest{Key: discovered, CallId: 71, ReturnResponse: true})
	answer, ok := recv().(*api.ExecuteServiceResponse)
	if !ok || !answer.Success || answer.CallId != 71 ||
		string(answer.ResponseData) != `{"accepted":true,"state":"accepted"}` {
		t.Fatalf("correlated acceptance response missing: %v", answer)
	}
	select {
	case delivered := <-receiptDelivered:
		if !delivered {
			t.Fatal("successful write cancelled receipt")
		}
	case <-time.After(time.Second):
		t.Fatal("receipt callback missing")
	}
	send(&api.SubscribeStatesRequest{})
	initial, ok := recv().(*api.MediaPlayerStateResponse)
	if !ok || initial.GetState() != esphome.MediaPlayerIdle {
		t.Fatal("initial state missing")
	}
	for _, state := range []esphome.MediaPlayerState{esphome.MediaPlayerPlaying, esphome.MediaPlayerPaused, esphome.MediaPlayerIdle} {
		player.SetState(state)
		update, ok := recv().(*api.MediaPlayerStateResponse)
		if !ok || update.GetState() != state || update.GetDeviceId() != 3 {
			t.Fatalf("live update missing: %v", update)
		}
	}
	_ = conn.Close()
	select {
	case <-closed:
	case <-time.After(2 * time.Second):
		t.Fatal("connection cleanup was not forwarded")
	}
}

type closedHandler struct{ closed chan struct{} }

func (h *closedHandler) Handle(context.Context, *esphome.Conn, proto.Message) error { return nil }
func (h *closedHandler) ConnClosed(*esphome.Conn) {
	select {
	case h.closed <- struct{}{}:
	default:
	}
}

type wireReceipt struct{ delivered chan bool }

func (*wireReceipt) MarshalJSON() ([]byte, error) {
	return []byte(`{"accepted":true,"state":"accepted"}`), nil
}
func (r *wireReceipt) ActionResultDelivered(ok bool) { r.delivered <- ok }
