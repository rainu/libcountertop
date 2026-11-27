package nativeapi

import (
	"context"
	"encoding/json"
	"errors"
	"hash/fnv"
	"log/slog"

	"github.com/ygelfand/libcountertop/pkg/actions"

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
	next    esphome.Handler
	slots   chan struct{}
	actions map[uint32]*esphome.Action
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
		func() error {
			if action := h.actions[request.GetKey()]; action != nil {
				return executeAction(ctx, action, request, c.Send)
			}
			return h.next.Handle(ctx, c, msg)
		},
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

// actionKey matches the SDK/ESPHome FNV-1 action key.
func actionKey(name string) uint32 {
	h := fnv.New32()
	_, _ = h.Write([]byte("action:" + name))
	return h.Sum32()
}

func executeAction(
	ctx context.Context,
	action *esphome.Action,
	request *api.ExecuteServiceRequest,
	send func(proto.Message) error,
) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	args := map[string]any{}
	if len(request.Args) != len(action.Args) {
		if request.CallId == 0 {
			return nil
		}
		return send(
			&api.ExecuteServiceResponse{CallId: request.CallId, ErrorMessage: "invalid_params"},
		)
	}
	for i, arg := range action.Args {
		value := request.Args[i]
		switch arg.Type {
		case esphome.ArgString:
			args[arg.Name] = value.GetString_()
		case esphome.ArgBool:
			args[arg.Name] = value.GetBool_()
		case esphome.ArgFloat:
			args[arg.Name] = float64(value.GetFloat_())
		case esphome.ArgInt:
			n := value.GetInt_()
			if n == 0 {
				n = value.GetLegacyInt()
			}
			args[arg.Name] = int(n)
		}
	}
	call, err := esphome.NewCall(args)
	if err != nil {
		return err
	}
	answer, err := action.Run(call)
	delivered := false
	defer func() { actions.ResultDelivered(answer, delivered) }()
	if request.CallId == 0 {
		return nil
	}
	reply := &api.ExecuteServiceResponse{CallId: request.CallId, Success: err == nil}
	if err != nil {
		reply.ErrorMessage = err.Error()
	} else if answer != nil && request.ReturnResponse {
		reply.ResponseData, err = json.Marshal(answer)
		if err != nil {
			reply.Success = false
			reply.ErrorMessage = "native_action_encoding_failed"
		}
	}
	// A deferred action needs an answering, correlated invocation.
	_, deferred := answer.(actions.DeliveryResult)
	if deferred && !request.ReturnResponse {
		reply.Success = false
		reply.ErrorMessage = "action_response_required"
	}
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if err = send(reply); err != nil {
		return err
	}
	delivered = reply.Success && (!deferred || request.ReturnResponse) && ctx.Err() == nil
	if !delivered && deferred {
		return errors.New("action_result_not_delivered")
	}
	return nil
}
