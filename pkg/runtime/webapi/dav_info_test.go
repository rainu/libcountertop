package webapi

import (
	"encoding/json"
	"github.com/ygelfand/libcountertop/pkg/settings/schema"
	"net/http/httptest"
	"testing"
)

type infoMemory struct{ v schema.Frontend }

func (m *infoMemory) Read() schema.Frontend                 { return m.v }
func (m *infoMemory) Update(f func(*schema.Frontend)) error { f(&m.v); return nil }
func TestDAVConnectionInfoIsConditionalAndOwnerOnly(t *testing.T) {
	m := &infoMemory{schema.Frontend{Password: "synthetic"}}
	a := New(Options{Settings: m, Address: func() string { return "192.0.2.40" }})
	call := func(auth bool) *httptest.ResponseRecorder {
		r := httptest.NewRequest("GET", "https://spoofed.example"+Prefix+"/webdav-info", nil)
		if auth {
			r.SetBasicAuth("owner", "synthetic")
		}
		w := httptest.NewRecorder()
		a.Handler().ServeHTTP(w, r)
		return w
	}
	if w := call(false); w.Code != 401 {
		t.Fatal(w.Code)
	}
	w := call(true)
	var v map[string]any
	json.Unmarshal(w.Body.Bytes(), &v)
	if v["url"] != "" || v["enabled"] != false {
		t.Fatal(v)
	}
	a.SetWebDAV(true)
	w = call(true)
	json.Unmarshal(w.Body.Bytes(), &v)
	if v["url"] != "https://192.0.2.40/api/v1/webdav/" || v["root"] != "/" {
		t.Fatal(v)
	}
}
