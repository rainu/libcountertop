package photosview

import (
	"github.com/ygelfand/libcountertop/pkg/display/shell"
	"testing"
)

func TestIndependentAppInstancesHaveDistinctShellIdentity(t *testing.T) {
	a := New(Options{Shell: shell.New()})
	b := New(Options{Shell: shell.New()})
	if a.app == b.app {
		t.Fatal("independent Photos apps alias the same shell identity")
	}
	if a.Screen(b.app) != nil || b.Screen(a.app) != nil {
		t.Fatal("Photos routed another instance to its own screen")
	}
}
