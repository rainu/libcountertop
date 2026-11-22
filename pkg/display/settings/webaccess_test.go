package settings

import (
	"testing"

	"github.com/ygelfand/libcountertop/pkg/display/menu"
	"github.com/ygelfand/libcountertop/pkg/say"
)

type davAdmin struct{ on bool }

func (a *davAdmin) Password() string         { return "" }
func (a *davAdmin) SetPassword(string) error { return nil }
func (a *davAdmin) WebDAVEnabled() bool      { return a.on }
func (a *davAdmin) SetWebDAV(on bool) error  { a.on = on; return nil }

func TestWebAccessDAVToggleUsesPersistedCapability(t *testing.T) {
	say.Use("en")
	a := &davAdmin{}
	page := (WebAccess{Admin: a}).Page(func() string { return "device.example" })
	rows, actions := page.Build()
	at := -1
	for i, r := range rows {
		if r.Label == say.T("access.setting.webdav") {
			at = i
		}
	}
	if at < 0 || rows[at].Kind != menu.Toggle || rows[at].On || actions[at] == nil {
		t.Fatal("missing opt-in toggle", rows)
	}
	actions[at](0)
	rows, actions = page.Build()
	if !a.on || !rows[at].On {
		t.Fatal("toggle did not persist")
	}
	actions[at](0)
	if a.on {
		t.Fatal("disable did not persist")
	}
}

func TestWebAccessShowsURLOnlyWhenEnabled(t *testing.T) {
	say.Use("en")
	a := &davAdmin{}
	p := (WebAccess{Admin: a}).Page(func() string { return "192.0.2.40" })
	rows, _ := p.Build()
	for _, r := range rows {
		if r.Label == say.T("webaccess.webdav_address") {
			t.Fatal("URL visible while disabled")
		}
	}
	a.on = true
	rows, _ = p.Build()
	found := false
	for _, r := range rows {
		if r.Label == say.T("webaccess.webdav_address") {
			found = r.Value == "https://192.0.2.40/api/v1/webdav/"
		}
	}
	if !found {
		t.Fatal("missing complete URL")
	}
}
