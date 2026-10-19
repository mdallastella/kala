package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"path"
	"strings"
	"time"

	"kala/internal/auth"
)

const heartbeat = 25 * time.Second

// events streams server-sent events to an authenticated client. Clients use
// fetch with an Authorization header rather than EventSource, so no token
// ends up in URLs or logs.
func (s *Server) events(w http.ResponseWriter, r *http.Request) error {
	rc := http.NewResponseController(w)
	// The server write timeout must not cut long-lived streams.
	_ = rc.SetWriteDeadline(time.Time{})
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)
	if _, err := fmt.Fprint(w, ": connected\n\n"); err != nil {
		return nil
	}
	if err := rc.Flush(); err != nil {
		return nil
	}
	ch, cancel := s.hub.Subscribe(uid(r))
	defer cancel()
	tick := time.NewTicker(heartbeat)
	defer tick.Stop()
	for {
		select {
		case <-r.Context().Done():
			return nil
		case <-tick.C:
			if _, err := fmt.Fprint(w, ": ping\n\n"); err != nil {
				return nil
			}
		case e := <-ch:
			data, err := json.Marshal(e.Data)
			if err != nil {
				s.log.Error("encode event", "err", err)
				continue
			}
			if _, err := fmt.Fprintf(w, "event: %s\ndata: %s\n\n", e.Type, data); err != nil {
				return nil
			}
		}
		if err := rc.Flush(); err != nil {
			return nil
		}
	}
}

// mcpAuth only admits personal access tokens to the MCP endpoint.
func (s *Server) mcpAuth(next http.Handler) http.Handler {
	return s.handle(func(w http.ResponseWriter, r *http.Request) error {
		id, err := s.authenticate(r, false)
		if err != nil {
			w.Header().Set("WWW-Authenticate", `Bearer realm="kala"`)
			return err
		}
		next.ServeHTTP(w, r.WithContext(auth.WithUser(r.Context(), id)))
		return nil
	})
}

// spa serves the embedded frontend, falling back to index.html for client
// routes.
func (s *Server) spa() http.Handler {
	if s.static == nil {
		return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			http.Error(w, "frontend not built: run `make web`", http.StatusNotFound)
		})
	}
	files := http.FileServerFS(s.static)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		p := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
		if p != "" {
			if st, err := fs.Stat(s.static, p); err == nil && !st.IsDir() {
				if strings.HasPrefix(p, "assets/") {
					w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
				}
				files.ServeHTTP(w, r)
				return
			} else if !errors.Is(err, fs.ErrNotExist) && err != nil {
				http.Error(w, "internal server error", http.StatusInternalServerError)
				return
			}
		}
		index, err := fs.ReadFile(s.static, "index.html")
		if err != nil {
			http.Error(w, "frontend not built: run `make web`", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-cache")
		_, _ = w.Write(index)
	})
}
