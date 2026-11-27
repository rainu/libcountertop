package nativeapi

import (
	"context"
	"encoding/json"
	"errors"
	esphome "github.com/ygelfand/go-esphome-device"
	"github.com/ygelfand/go-esphome-device/api"
	"google.golang.org/protobuf/proto"
	"testing"
)

type deliveryReceipt struct {
	Accepted bool   `json:"accepted"`
	State    string `json:"state"`
	calls    []bool
	fail     bool
}

func (r *deliveryReceipt) ActionResultDelivered(ok bool) { r.calls = append(r.calls, ok) }
func (r *deliveryReceipt) MarshalJSON() ([]byte, error) {
	if r.fail {
		return nil, errors.New("synthetic encoding failure")
	}
	return json.Marshal(map[string]any{"accepted": r.Accepted, "state": r.State})
}
func TestDeferredNativeActionDelivery(t *testing.T) {
	for _, tc := range []struct {
		name                                 string
		id                                   uint32
		answer, sendFail, encodeFail, cancel bool
		delivered                            bool
	}{
		{"accepted", 7, true, false, false, false, true},
		{"lost_write", 7, true, true, false, false, false},
		{"uncorrelated", 0, true, false, false, false, false},
		{"no_answer_requested", 7, false, false, false, false, false},
		{"encoding_failed", 7, true, false, true, false, false},
		{"cancel_during_work", 7, true, false, false, true, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			receipt := &deliveryReceipt{Accepted: true, State: "accepted", fail: tc.encodeFail}
			a := &esphome.Action{Name: "transfer", Answers: true, Args: []esphome.Arg{{Name: "session_id", Type: esphome.ArgString}},
				Run: func(c esphome.Call) (any, error) {
					if c.String("session_id") != "agent:test:next" {
						t.Fatal("target changed")
					}
					if tc.cancel {
						cancel()
					}
					return receipt, nil
				}}
			sent := 0
			_ = executeAction(ctx, a, &api.ExecuteServiceRequest{CallId: tc.id, ReturnResponse: tc.answer, Args: []*api.ExecuteServiceArgument{{String_: "agent:test:next"}}}, func(m proto.Message) error {
				sent++
				r := m.(*api.ExecuteServiceResponse)
				if r.CallId != tc.id {
					t.Fatal("correlation lost")
				}
				if tc.sendFail {
					return errors.New("synthetic lost write")
				}
				return nil
			})
			if len(receipt.calls) != 1 || receipt.calls[0] != tc.delivered {
				t.Fatal(receipt.calls)
			}
			if (tc.cancel || tc.id == 0) && sent != 0 {
				t.Fatal("sent after cancellation/without correlation")
			}
		})
	}
}
func TestNativeActionKeyMatchesDiscovery(t *testing.T) {
	a := &esphome.Action{Name: "test_delivery", Run: func(esphome.Call) (any, error) { return nil, nil }}
	ents := esphome.NewEntities()
	if err := ents.AddActions(a); err != nil {
		t.Fatal(err)
	}
	// SDK discovery must use exactly the same action key as our delivery wrapper.
	seen := false
	for _, a := range ents.Actions() {
		if actionMap([]*esphome.Action{&a})[actionKey(a.Name)] == nil {
			t.Fatal("missing action")
		}
		seen = true
	}
	if !seen {
		t.Fatal("missing discovery")
	}
}
