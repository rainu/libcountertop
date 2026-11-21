package webapi

import (
	"context"
	"crypto/tls"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	setting "github.com/ygelfand/libcountertop/pkg/settings"
	"github.com/ygelfand/libcountertop/pkg/settings/schema"
)

func TestDAVOptInAuthenticationTLSAndScope(t *testing.T) {
	m := &memory{v: schema.Frontend{Password: "owner-test"}}
	registry := &setting.Registry{}
	reached := 0
	a := New(Options{Settings: m, Registry: registry, Pages: map[string]http.Handler{
		"webdav": http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { reached++; w.WriteHeader(207) }),
		"files":  http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) }),
	}})
	registry.Add(a.Controls())
	h := a.Handler()
	do := func(method, path, password string, secure bool) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, Prefix+path, nil)
		if secure {
			r.TLS = &tls.ConnectionState{}
		}
		if password != "" {
			r.SetBasicAuth("owner", password)
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w
	}
	if m.v.WebDAV {
		t.Fatal("enabled by default")
	}
	if w := do("PROPFIND", "/webdav/", "owner-test", true); w.Code != 404 {
		t.Fatal(w.Code)
	}
	if err := a.SetWebDAV(true); err != nil {
		t.Fatal(err)
	}
	if w := do("PROPFIND", "/webdav/", "owner-test", false); w.Code != 403 {
		t.Fatal(w.Code)
	}
	for _, password := range []string{"", "wrong"} {
		if w := do("PROPFIND", "/webdav/", password, true); w.Code != 401 || !strings.HasPrefix(w.Header().Get("WWW-Authenticate"), "Basic ") {
			t.Fatal(w.Code, w.Header())
		}
	}
	for _, method := range []string{"OPTIONS", "PROPFIND", "PUT", "DELETE"} {
		if w := do(method, "/webdav/", "owner-test", true); w.Code != 207 {
			t.Fatal(method, w.Code)
		}
	}
	if reached != 4 {
		t.Fatal(reached)
	}
	if w := do("PROPFIND", "/files/", "owner-test", true); w.Code != 405 {
		t.Fatal("DAV leaked to other Pages", w.Code)
	}
	if err := a.SetWebDAV(false); err != nil {
		t.Fatal(err)
	}
	if w := do("GET", "/webdav/x", "owner-test", true); w.Code != 404 {
		t.Fatal(w.Code)
	}
	if w := do("GET", "/files", "owner-test", true); w.Code != 200 {
		t.Fatal("normal API disabled", w.Code)
	}
	// Generic configuration API uses the same bound setting without new secrets.
	if w := do("GET", "/settings/access/webdav", "owner-test", true); w.Code != 200 || !strings.Contains(w.Body.String(), "\"kind\":\"toggle\"") {
		t.Fatal(w.Code, w.Body)
	}
	if w := do("GET", "/settings/access/password", "owner-test", true); strings.Contains(w.Body.String(), "owner-test") {
		t.Fatal("secret disclosed")
	}
}

type davMemory struct {
	mu sync.Mutex
	v  schema.Frontend
}

func (m *davMemory) Read() schema.Frontend { m.mu.Lock(); defer m.mu.Unlock(); return m.v }
func (m *davMemory) Update(f func(*schema.Frontend)) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	f(&m.v)
	return nil
}

func TestDAVDisableCancelsBlockedNetworkUpload(t *testing.T) {
	m := &davMemory{v: schema.Frontend{Password: "owner-test", WebDAV: true}}
	started := make(chan struct{})
	cancelled := make(chan error, 1)
	a := New(Options{Settings: m, Pages: map[string]http.Handler{
		"webdav": http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			close(started)
			_, err := io.Copy(io.Discard, r.Body)
			if err == nil {
				err = r.Context().Err()
			}
			cancelled <- err
		}),
	}})
	srv := httptest.NewTLSServer(a.Handler())
	defer srv.Close()
	pr, pw := io.Pipe()
	defer pw.Close()
	req, _ := http.NewRequest("PUT", srv.URL+Prefix+"/webdav/new", pr)
	req.SetBasicAuth("owner", "owner-test")
	done := make(chan struct{})
	go func() {
		defer close(done)
		resp, _ := srv.Client().Do(req)
		if resp != nil {
			resp.Body.Close()
		}
	}()
	select {
	case <-started:
	case <-time.After(3 * time.Second):
		t.Fatal("not admitted")
	}
	if err := a.SetWebDAV(false); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-cancelled:
		if err == nil {
			t.Fatal("uncancelled read")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("blocked body survived disable")
	}
	pw.CloseWithError(context.Canceled)
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("client still blocked")
	}
	a.dav.mu.Lock()
	defer a.dav.mu.Unlock()
	if len(a.dav.active) != 0 {
		t.Fatal("request retained")
	}
}
