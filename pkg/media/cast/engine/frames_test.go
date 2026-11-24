package engine

import (
	"errors"
	"reflect"
	"testing"
)

func TestFrameAdapterBeforeResumeAndSiteIsolationUnchanged(t *testing.T) {
	var calls []string
	if err := initializeFrame(
		func(m string, p any) error { calls = append(calls, m); return nil },
		"adapter",
	); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(
		calls,
		[]string{
			"Page.enable",
			"Runtime.enable",
			"Page.addScriptToEvaluateOnNewDocument",
			"Target.setAutoAttach",
		},
	) {
		t.Fatal(calls)
	}
	sentinel := errors.New("domain unavailable")
	calls = nil
	err := initializeFrame(func(m string, p any) error {
		calls = append(calls, m)
		if m == "Page.enable" {
			return sentinel
		}
		return nil
	}, "adapter")
	if !errors.Is(err, sentinel) || len(calls) != 1 {
		t.Fatal("continued after failed initialization", err, calls)
	}
}

func TestDetachedChildDoesNotCloseParent(t *testing.T) {
	p := &Page{session: "parent", target: "target", done: make(chan struct{})}
	in := &instance{pages: map[string]*Page{"parent": p, "child": p}}
	in.event("parent", "Target.detachedFromTarget", []byte(`{"sessionId":"child"}`))
	if in.pages["child"] != nil {
		t.Fatal("child mapping leaked")
	}
	select {
	case <-p.Done():
		t.Fatal("child closed parent")
	default:
	}
	in.pages["child"] = p
	in.event("", "Target.detachedFromTarget", []byte(`{"sessionId":"child"}`))
	select {
	case <-p.Done():
		t.Fatal("global child detach closed parent")
	default:
	}
	in.event("", "Target.detachedFromTarget", []byte(`{"sessionId":"parent"}`))
	select {
	case <-p.Done():
	default:
		t.Fatal("parent detach did not close parent")
	}
}
