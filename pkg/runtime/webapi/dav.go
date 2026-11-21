package webapi

import (
	"context"
	"github.com/go-chi/chi/v5"
	"net/http"
	"sync"
	"time"

	setting "github.com/ygelfand/libcountertop/pkg/settings"
	"github.com/ygelfand/libcountertop/pkg/settings/schema"
)

func init() {
	for _, method := range []string{"PROPFIND", "PROPPATCH", "MKCOL", "COPY", "MOVE", "LOCK", "UNLOCK"} {
		chi.RegisterMethod(method)
	}
}

type davAccess struct {
	mu     sync.Mutex
	active map[*http.Request]context.CancelFunc
}

// Serializes opt-in changes with admission. Turning off cancels in-flight
// transfers as well as denying new requests; no separate listener survives.
func (a *API) saveDAV(s setting.Setting[schema.Frontend], v string) error {
	a.dav.mu.Lock()
	defer a.dav.mu.Unlock()
	if err := setting.Store(a.o.Settings, s, v); err != nil {
		return err
	}
	if !a.WebDAVEnabled() {
		for _, cancel := range a.dav.active {
			cancel()
		}
	}
	return nil
}

func (a *API) requireDAV(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		if r.TLS == nil {
			http.Error(w, "HTTPS required", 403)
			return
		}
		a.dav.mu.Lock()
		enabled := a.WebDAVEnabled()
		a.dav.mu.Unlock()
		if !enabled {
			http.NotFound(w, r)
			return
		}
		if !a.authorized(w, r) {
			w.Header().Set("WWW-Authenticate", "Basic realm=\"EchoLocal files\", charset=\"UTF-8\"")
			http.Error(w, "sign in", 401)
			return
		}
		ctx, cancel := context.WithCancel(r.Context())
		defer cancel()
		a.dav.mu.Lock()
		if !a.WebDAVEnabled() {
			a.dav.mu.Unlock()
			http.NotFound(w, r)
			return
		}
		if a.dav.active == nil {
			a.dav.active = make(map[*http.Request]context.CancelFunc)
		}
		a.dav.active[r] = cancel
		a.dav.mu.Unlock()
		// Cancelling Context alone does not interrupt a blocked HTTP Body.Read.
		// Deadlines terminate network I/O; the shared store checks cancellation
		// before publication. Closing Body wakes non-network test/custom readers.
		rc := http.NewResponseController(w)
		stopped := make(chan struct{})
		stop := context.AfterFunc(ctx, func() {
			defer close(stopped)
			_ = rc.SetReadDeadline(time.Now())
			_ = rc.SetWriteDeadline(time.Now())
			_ = r.Body.Close()
		})
		defer func() {
			if !stop() {
				<-stopped
			}
			a.dav.mu.Lock()
			delete(a.dav.active, r)
			a.dav.mu.Unlock()
			_ = rc.SetReadDeadline(time.Time{})
			_ = rc.SetWriteDeadline(time.Time{})
		}()
		h.ServeHTTP(w, r.WithContext(ctx))
	})
}
