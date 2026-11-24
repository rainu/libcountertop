package nativeapi

import (
	"context"
	"log/slog"

	esphome "github.com/ygelfand/go-esphome-device"
	"github.com/ygelfand/go-esphome-device/api"
	"google.golang.org/protobuf/proto"
)

// actionHandler keeps opt-in long-running actions off the connection read loop.
// The shared bound prevents authenticated clients from spawning unbounded work.
type actionHandler struct {
	// Retain the SDK-private server binding hook. Embedding only Handler erases
	// this method and silently drops all live HA state updates.
	*esphome.Entities
	next  esphome.Handler
	slots chan struct{}
}

func (h *actionHandler) Handle(ctx context.Context, c *esphome.Conn, msg proto.Message) error {
	request, ok := msg.(*api.ExecuteServiceRequest)
	if !ok {
		return h.next.Handle(ctx, c, msg)
	}
	return h.dispatch(
		ctx,
		request,
		c.Send,
		func() error { return h.next.Handle(ctx, c, msg) },
	)
}
func (h *actionHandler) dispatch(
	ctx context.Context,
	request *api.ExecuteServiceRequest,
	send func(proto.Message) error,
	work func() error,
) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	select {
	case h.slots <- struct{}{}:
	default:
		if request.GetCallId() == 0 {
			return nil
		}
		return send(
			&api.ExecuteServiceResponse{
				CallId:       request.GetCallId(),
				Success:      false,
				ErrorMessage: "native_action_busy",
			},
		)
	}
	go func() {
		defer func() { <-h.slots }()
		defer func() {
			if recover() != nil {
				slog.Error("native action panicked")
				if request.GetCallId() != 0 {
					_ = send(
						&api.ExecuteServiceResponse{
							CallId:       request.GetCallId(),
							Success:      false,
							ErrorMessage: "native_action_failed",
						},
					)
				}
			}
		}()
		if err := work(); err != nil {
			slog.Warn("native action response could not be sent")
		}
	}()
	return nil
}

// Preserve lifecycle cleanup for every handler in the delegated chain.
func (h *actionHandler) ConnClosed(c *esphome.Conn) {
	if next, ok := h.next.(esphome.ConnCloser); ok {
		next.ConnClosed(c)
	}
}
