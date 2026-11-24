package engine

import (
	"errors"
	"reflect"
	"testing"
)

func TestFrameAdapterBeforeResumeAndSiteIsolationUnchanged(t *testing.T) {
	var calls []string
	if err := initializeFrame(func(m string, p any) error { calls = append(calls, m); return nil }, "adapter"); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(calls, []string{"Page.enable", "Runtime.enable", "Page.addScriptToEvaluateOnNewDocument", "Target.setAutoAttach"}) {
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
