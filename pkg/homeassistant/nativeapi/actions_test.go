package nativeapi

import (
	"context"
	"errors"
	"testing"
	"time"

	esphome "github.com/ygelfand/go-esphome-device"
	"github.com/ygelfand/go-esphome-device/api"
	"google.golang.org/protobuf/proto"
)

func TestLongActionsDoNotBlockReadLoopOrStop(t *testing.T) {
	h := &actionHandler{slots: make(chan struct{}, 2)}
	entered, release, stopped := make(chan struct{}), make(chan struct{}), make(chan struct{})
	send := func(proto.Message) error { return nil }
	if err := h.dispatch(context.Background(), &api.ExecuteServiceRequest{}, send, func() error {
		close(entered)
		<-release
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	defer close(release)
	select {
	case <-entered:
	case <-time.After(time.Second):
		t.Fatal("action did not start")
	}
	// A normal message and a second action can run while the first is blocked.
	h.next = esphome.HandlerFunc(func(context.Context, *esphome.Conn, proto.Message) error { return nil })
	if err := h.Handle(context.Background(), nil, &api.PingRequest{}); err != nil {
		t.Fatal(err)
	}
	if err := h.dispatch(context.Background(), &api.ExecuteServiceRequest{}, send, func() error { close(stopped); return nil }); err != nil {
		t.Fatal(err)
	}
	select {
	case <-stopped:
	case <-time.After(time.Second):
		t.Fatal("stop queued behind start")
	}
}
func TestActionAdmissionIsBoundedAndCorrelated(t *testing.T) {
	h := &actionHandler{slots: make(chan struct{}, 1)}
	h.slots <- struct{}{}
	sent := make(chan *api.ExecuteServiceResponse, 1)
	send := func(msg proto.Message) error { sent <- msg.(*api.ExecuteServiceResponse); return nil }
	if err := h.dispatch(context.Background(), &api.ExecuteServiceRequest{CallId: 23}, send, func() error { t.Error("admitted over limit"); return nil }); err != nil {
		t.Fatal(err)
	}
	reply := <-sent
	if reply.CallId != 23 || reply.Success || reply.ErrorMessage != "native_action_busy" {
		t.Fatal(reply)
	}
	if err := h.dispatch(context.Background(), &api.ExecuteServiceRequest{}, send, func() error { t.Error("admitted"); return nil }); err != nil {
		t.Fatal(err)
	}
	select {
	case <-sent:
		t.Fatal("answered uncorrelated request")
	default:
	}
}
func TestPanicsReleaseAdmissionAndReturnFixedError(t *testing.T) {
	h := &actionHandler{slots: make(chan struct{}, 1)}
	sent := make(chan *api.ExecuteServiceResponse, 1)
	send := func(msg proto.Message) error { sent <- msg.(*api.ExecuteServiceResponse); return nil }
	if err := h.dispatch(context.Background(), &api.ExecuteServiceRequest{CallId: 7}, send, func() error { panic("synthetic-private-detail") }); err != nil {
		t.Fatal(err)
	}
	select {
	case reply := <-sent:
		if reply.Success || reply.CallId != 7 || reply.ErrorMessage != "native_action_failed" {
			t.Fatal(reply)
		}
	case <-time.After(time.Second):
		t.Fatal("panic response missing")
	}
	deadline := time.After(time.Second)
	for len(h.slots) != 0 {
		select {
		case <-deadline:
			t.Fatal("admission leaked")
		default:
			time.Sleep(time.Millisecond)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := h.dispatch(ctx, &api.ExecuteServiceRequest{}, send, func() error { return nil }); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
}
