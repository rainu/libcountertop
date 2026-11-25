package webreceiver

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"sync"
	"time"

	"github.com/ygelfand/libcountertop/pkg/hook"
	"github.com/ygelfand/libcountertop/pkg/media/cast/engine"
)

// YouTubeConnection contains UI-safe metadata only. Codes are short lived and
// must never be logged, stored in settings, or included in diagnostics/exports.
type YouTubeConnection struct {
	State           string    `json:"state"`
	Premium         bool      `json:"premium"`
	Code            string    `json:"code,omitempty"`
	VerificationURL string    `json:"verification_url,omitempty"`
	ExpiresAt       time.Time `json:"expires_at,omitempty"`
	Error           string    `json:"error,omitempty"`
}

type YouTubeAccount struct {
	Changed  hook.Hook[YouTubeConnection]
	mu       sync.Mutex
	state    YouTubeConnection
	commands chan string
	host     *engine.Host
	stop     func() error
}

func newYouTubeAccount(host *engine.Host, stop func() error) *YouTubeAccount {
	return &YouTubeAccount{
		host:     host,
		stop:     stop,
		commands: make(chan string, 1),
		state:    YouTubeConnection{State: "checking"},
	}
}
func (a *YouTubeAccount) Snapshot() YouTubeConnection {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.state
}
func (a *YouTubeAccount) set(s YouTubeConnection) {
	a.mu.Lock()
	changed := a.state != s
	a.state = s
	a.mu.Unlock()
	if changed {
		a.Changed.Emit(s)
	}
}

// Command is nonblocking; the single worker serializes both UI surfaces.
func (a *YouTubeAccount) Command(action string) error {
	switch action {
	case "connect", "refresh", "cancel", "disconnect":
	default:
		return errors.New("unsupported YouTube connection action")
	}
	select {
	case a.commands <- action:
		return nil
	default:
		return errors.New("a YouTube connection operation is already pending")
	}
}

// The query intentionally returns no account names, IDs, cookies or tokens.
const youtubeAccountJS = `JSON.stringify((()=>{
 if(location.hostname!=="www.youtube.com" || !location.pathname.startsWith("/tv")) return {};
 const av=document.querySelector("ytlr-avatar-lockup")?.__instance?.props||{};
 const signIn=/^(sign in|anmelden|accedi|đăng nhập|guest|gast|ospite|khách)$/i.test((av.title||"").trim());
 const signedIn=!!av.title&&!signIn;
 const logo=document.querySelector("ytlr-logo-entity");
 const t=document.body?.innerText||"";
 const m=t.includes("yt.be/activate")?t.match(/\b[A-Z0-9]{3,5}(?:-[A-Z0-9]{3,5}){1,3}\b/):null;
 return {signedIn,signIn,premium:signedIn&&!!logo&&/premium/i.test(logo.outerHTML),code:m?m[0]:""};
})())`

type youtubeAccountProbe struct {
	SignedIn bool   `json:"signedIn"`
	SignIn   bool   `json:"signIn"`
	Premium  bool   `json:"premium"`
	Code     string `json:"code"`
}

func accountState(p youtubeAccountProbe, linking bool, until, now time.Time) YouTubeConnection {
	if p.SignedIn {
		return YouTubeConnection{State: "connected", Premium: p.Premium}
	}
	if linking && !until.IsZero() && !now.Before(until) {
		return YouTubeConnection{State: "expired"}
	}
	if linking {
		s := YouTubeConnection{State: "linking", ExpiresAt: until}
		if p.Code != "" {
			s.Code = p.Code
			s.VerificationURL = "https://yt.be/activate"
		}
		return s
	}
	if p.SignIn || p.Code != "" {
		return YouTubeConnection{State: "disconnected"}
	}
	return YouTubeConnection{State: "checking"}
}

func (a *YouTubeAccount) Run(ctx context.Context) {
	var page *engine.Page
	var linking bool
	var until, opened time.Time
	var code string
	var failures youtubeProbeHealth
	closePage := func() {
		if page != nil {
			_ = page.Close()
			page = nil
		}
	}
	defer closePage()
	open := func() {
		closePage()
		a.set(YouTubeConnection{State: "checking"})
		bounded, cancel := context.WithTimeout(ctx, 25*time.Second)
		defer cancel()
		p, err := a.host.Open(bounded, "youtube-account")
		if err != nil {
			a.set(
				YouTubeConnection{
					State: "error",
					Error: "Could not open the YouTube connection. Try again.",
				},
			)
			return
		}
		page = p
		for _, call := range []struct {
			method string
			params any
		}{
			{"Emulation.setUserAgentOverride", map[string]any{"userAgent": youtubeUserAgent()}},
			{"Page.enable", nil},
			{"Page.addScriptToEvaluateOnNewDocument", map[string]any{"source": "setInterval(()=>{for(const v of document.querySelectorAll('video,audio'))v.pause()},250)"}},
			{"Page.navigate", map[string]any{"url": "https://www.youtube.com/tv"}},
		} {
			if _, err = page.Request(bounded, call.method, call.params); err != nil {
				closePage()
				a.set(
					YouTubeConnection{State: "error", Error: "Could not open YouTube. Try again."},
				)
				return
			}
		}
		opened = time.Now()
		failures = youtubeProbeHealth{}
	}
	open()
	tick := time.NewTicker(2 * time.Second)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case action := <-a.commands:
			switch action {
			case "connect", "refresh":
				linking = action == "connect"
				until = time.Time{}
				code = ""
				open()
			case "cancel":
				linking = false
				until = time.Time{}
				code = ""
				open() // recheck: consent may have finished concurrently
			case "disconnect":
				// Stop playback before clearing only YouTube-origin local site data.
				// This is local sign-out, not Google-wide logout or remote grant revocation.
				a.set(YouTubeConnection{State: "checking"})
				_ = a.stop()
				linking = false
				until = time.Time{}
				code = ""
				if page == nil {
					open()
				}
				if page == nil {
					continue
				}
				bounded, cancel := context.WithTimeout(ctx, 10*time.Second)
				_, navErr := page.Request(
					bounded,
					"Page.navigate",
					map[string]any{"url": "about:blank"},
				)
				cancel()
				if navErr != nil {
					closePage()
					a.set(
						YouTubeConnection{
							State: "error",
							Error: "Local sign-out could not be completed. Try again.",
						},
					)
					continue
				}
				ok := true
				for _, origin := range []string{"https://www.youtube.com", "https://youtube.com", "https://music.youtube.com"} {
					bounded, cancel := context.WithTimeout(ctx, 10*time.Second)
					_, err := page.Request(
						bounded,
						"Storage.clearDataForOrigin",
						map[string]any{"origin": origin, "storageTypes": "all"},
					)
					cancel()
					if err != nil {
						ok = false
						break
					}
				}
				if !ok {
					closePage()
					a.set(
						YouTubeConnection{
							State: "error",
							Error: "Local sign-out could not be completed. Try again.",
						},
					)
					continue
				}
				open()
			}
		case now := <-tick.C:
			if page == nil {
				continue
			}
			select {
			case <-page.Done():
				closePage()
				a.set(
					YouTubeConnection{
						State: "error",
						Error: "YouTube connection interrupted. Try again.",
					},
				)
				continue
			default:
			}
			bounded, cancel := context.WithTimeout(ctx, 3*time.Second)
			raw, err := page.Request(
				bounded,
				"Runtime.evaluate",
				map[string]any{"expression": youtubeAccountJS, "returnByValue": true},
			)
			cancel()
			var result struct {
				Result struct {
					Value string `json:"value"`
				} `json:"result"`
			}
			var p youtubeAccountProbe
			if err != nil || json.Unmarshal(raw, &result) != nil ||
				json.Unmarshal([]byte(result.Result.Value), &p) != nil {
				if !failures.alive(now, errors.New("metadata unavailable")) {
					closePage()
					a.set(
						YouTubeConnection{
							State: "error",
							Error: "YouTube did not respond. Try again.",
						},
					)
				}
				continue
			}
			failures.alive(now, nil)
			// A code is an instruction to authorize, never evidence of a connection.
			if linking && p.Code != "" && p.Code != code {
				code = p.Code
				until = now.Add(10 * time.Minute)
			}
			if linking && until.IsZero() && now.Sub(opened) > 90*time.Second && !p.SignedIn {
				closePage()
				linking = false
				a.set(
					YouTubeConnection{
						State: "error",
						Error: "No activation code received. Try again.",
					},
				)
				continue
			}
			s := accountState(p, linking, until, now)
			if s.State == "connected" {
				linking = false
				until = time.Time{}
				code = ""
			}
			if s.State == "expired" {
				closePage()
				linking = false
			}
			if s.State == "checking" && now.Sub(opened) > 90*time.Second {
				closePage()
				s = YouTubeConnection{
					State: "error",
					Error: "YouTube did not finish loading. Try again.",
				}
			}
			a.set(s)
		}
	}
}

// Handler must be mounted behind the existing owner-authenticated web API.
// GET is metadata only; mutations are strictly POST and bodies are bounded.
func (a *YouTubeAccount) Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Referrer-Policy", "no-referrer")
		if r.URL.Path != "" && r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		switch r.Method {
		case http.MethodGet:
			_ = json.NewEncoder(w).Encode(a.Snapshot())
		case http.MethodPost:
			var cmd struct {
				Action string `json:"action"`
			}
			d := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1024))
			d.DisallowUnknownFields()
			if d.Decode(&cmd) != nil {
				http.Error(w, "Invalid YouTube connection action", http.StatusBadRequest)
				return
			}
			if err := a.Command(cmd.Action); err != nil {
				http.Error(w, err.Error(), http.StatusConflict)
				return
			}
			w.WriteHeader(http.StatusAccepted)
			_, _ = w.Write([]byte("{}"))
		default:
			w.Header().Set("Allow", "GET, POST")
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	})
}
