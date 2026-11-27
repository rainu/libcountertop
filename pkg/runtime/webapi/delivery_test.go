package webapi

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	esphome "github.com/ygelfand/go-esphome-device"
)

type deliveredAnswer struct{ calls []bool }

func (a *deliveredAnswer) ActionResultDelivered(ok bool) { a.calls = append(a.calls, ok) }

type deliveryStates struct {
	fakeStates
	answer *deliveredAnswer
}

func (s *deliveryStates) RunAction(string, esphome.Call) (any, error) { return s.answer, nil }

type failedWriter struct {
	header    http.Header
	flushFail bool
}

func (w *failedWriter) Header() http.Header { return w.header }
func (*failedWriter) WriteHeader(int)       {}
func (w *failedWriter) Write(b []byte) (int, error) {
	if !w.flushFail {
		return 0, errors.New("lost write")
	}
	return len(b), nil
}
func (*failedWriter) FlushError() error { return errors.New("lost flush") }
func TestHTTPDeferredActionDelivery(t *testing.T) {
	for _, mode := range []string{"success", "write_failed", "flush_failed", "cancelled"} {
		t.Run(mode, func(t *testing.T) {
			answer := &deliveredAnswer{}
			a := &API{o: Options{States: &deliveryStates{answer: answer}}}
			r := httptest.NewRequest(
				"POST",
				Prefix+"/actions/transfer",
				strings.NewReader(`{"args":{"session_id":"agent:test:next"}}`),
			)
			route := chi.NewRouteContext()
			route.URLParams.Add("name", "transfer")
			r = r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, route))
			if mode == "cancelled" {
				ctx, cancel := context.WithCancel(r.Context())
				cancel()
				r = r.WithContext(ctx)
			}
			var w http.ResponseWriter = httptest.NewRecorder()
			if mode == "write_failed" || mode == "flush_failed" {
				w = &failedWriter{header: make(http.Header), flushFail: mode == "flush_failed"}
			}
			a.runAction(w, r)
			if len(answer.calls) != 1 || answer.calls[0] != (mode == "success") {
				t.Fatal(answer.calls)
			}
		})
	}
}
