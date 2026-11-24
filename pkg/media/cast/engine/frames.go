package engine

import (
	"context"
	"encoding/json"
	"time"

	"github.com/ygelfand/libcountertop/pkg/runtime/safe"
)

// InjectFrames installs a main-world media adapter before scripts execute, also
// in out-of-process frames. Site isolation stays enabled. Each paused target is
// resumed even if initialization fails; no unrelated browser targets are attached.
func (p *Page) InjectFrames(ctx context.Context, source string) error {
	unlisten := p.Events.Listen(func(e Event) {
		if e.Method != "Target.attachedToTarget" {
			return
		}
		var a struct {
			Session string `json:"sessionId"`
			Info    struct {
				Type string `json:"type"`
			} `json:"targetInfo"`
		}
		if json.Unmarshal(e.Params, &a) != nil || a.Session == "" {
			return
		}
		p.in.mu.Lock()
		p.in.pages[a.Session] = p
		p.in.mu.Unlock()
		safe.Go("embedded media frame", func() {
			bounded, cancel := context.WithTimeout(ctx, 10*time.Second)
			defer cancel()
			request := func(m string, v any) error { _, err := p.in.conn.request(bounded, a.Session, m, v); return err }
			defer p.in.conn.call(a.Session, "Runtime.runIfWaitingForDebugger", nil)
			if a.Info.Type != "iframe" {
				return
			}
			_ = initializeFrame(request, source)
		})
	})
	stop := context.AfterFunc(ctx, unlisten)
	safe.Go("embedded media frame cleanup", func() {
		select {
		case <-ctx.Done():
		case <-p.Done():
		}
		stop()
		unlisten()
	})
	return initializeFrame(func(m string, v any) error { _, err := p.Request(ctx, m, v); return err }, source)
}

func initializeFrame(request func(string, any) error, source string) error {
	for _, c := range []struct {
		method string
		params any
	}{
		{"Page.enable", nil},
		{"Runtime.enable", nil},
		{"Page.addScriptToEvaluateOnNewDocument", map[string]any{"source": source, "runImmediately": true}},
		{"Target.setAutoAttach", map[string]any{"autoAttach": true, "flatten": true, "waitForDebuggerOnStart": true}},
	} {
		if err := request(c.method, c.params); err != nil {
			return err
		}
	}
	return nil
}
