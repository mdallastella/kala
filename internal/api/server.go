// Package api exposes the REST API, the SSE stream, the MCP endpoint and the
// single-page frontend.
package api

import (
	"encoding/json"
	"errors"
	"io/fs"
	"log/slog"
	"net"
	"net/http"
	"runtime/debug"
	"strconv"
	"strings"
	"time"

	"kala/internal/auth"
	"kala/internal/config"
	"kala/internal/events"
	"kala/internal/service"
)

// AccessTTL is the lifetime of access tokens.
const AccessTTL = 15 * time.Minute

const refreshCookie = "kala_refresh"

// Server holds the HTTP dependencies.
type Server struct {
	svc     *service.Service
	signer  *auth.Signer
	hub     *events.Hub
	cfg     config.Config
	log     *slog.Logger
	static  fs.FS
	mcp     http.Handler
	limiter *rateLimiter
}

// Options configures New.
type Options struct {
	Service *service.Service
	Signer  *auth.Signer
	Hub     *events.Hub
	Config  config.Config
	Logger  *slog.Logger
	// Static is the built frontend (may be nil).
	Static fs.FS
	// MCP is the MCP handler mounted at /mcp; requests reach it authenticated
	// by a personal access token.
	MCP http.Handler
}

// New builds the HTTP handler.
func New(o Options) http.Handler {
	s := &Server{svc: o.Service, signer: o.Signer, hub: o.Hub, cfg: o.Config, log: o.Logger, static: o.Static, mcp: o.MCP,
		limiter: newRateLimiter(10, 5*time.Minute)}
	mux := http.NewServeMux()
	s.routes(mux)
	return s.recoverer(s.logRequests(mux))
}

// handlerFunc is an HTTP handler that may fail.
type handlerFunc func(w http.ResponseWriter, r *http.Request) error

func (s *Server) handle(h handlerFunc) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := h(w, r); err != nil {
			s.writeError(w, r, err)
		}
	})
}

// authed requires a valid access token or PAT.
func (s *Server) authed(h handlerFunc) http.Handler {
	return s.handle(func(w http.ResponseWriter, r *http.Request) error {
		uid, err := s.authenticate(r, true)
		if err != nil {
			return err
		}
		return h(w, r.WithContext(auth.WithUser(r.Context(), uid)))
	})
}

func bearer(r *http.Request) string {
	h := r.Header.Get("Authorization")
	if len(h) > 7 && strings.EqualFold(h[:7], "bearer ") {
		return strings.TrimSpace(h[7:])
	}
	return ""
}

func (s *Server) authenticate(r *http.Request, allowJWT bool) (int64, error) {
	tok := bearer(r)
	if tok == "" {
		return 0, &service.Error{Code: service.CodeAuth, Message: "missing bearer token"}
	}
	if strings.HasPrefix(tok, auth.PATPrefix) {
		return s.svc.LookupAccessToken(r.Context(), tok)
	}
	if !allowJWT {
		return 0, &service.Error{Code: service.CodeAuth, Message: "a personal access token is required"}
	}
	uid, err := s.signer.Verify(tok)
	if err != nil {
		return 0, &service.Error{Code: service.CodeAuth, Message: "invalid or expired access token"}
	}
	return uid, nil
}

func uid(r *http.Request) int64 { return auth.UserID(r.Context()) }

const maxBody = 1 << 20

func decode(w http.ResponseWriter, r *http.Request, v any, limit int64) error {
	if limit == 0 {
		limit = maxBody
	}
	r.Body = http.MaxBytesReader(w, r.Body, limit)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		if _, ok := errors.AsType[*http.MaxBytesError](err); ok {
			return err
		}
		return &service.Error{Code: service.CodeInvalid, Message: "invalid JSON body: " + err.Error()}
	}
	return nil
}

func writeJSON(w http.ResponseWriter, status int, v any) error {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if v == nil {
		return nil
	}
	return json.NewEncoder(w).Encode(v)
}

var statusOf = map[string]int{
	service.CodeInvalid:  http.StatusBadRequest,
	service.CodeNotFound: http.StatusNotFound,
	service.CodeConflict: http.StatusConflict,
	service.CodeAuth:     http.StatusUnauthorized,
	service.CodeDenied:   http.StatusForbidden,
	"rate_limited":       http.StatusTooManyRequests,
	"too_large":          http.StatusRequestEntityTooLarge,
}

func (s *Server) writeError(w http.ResponseWriter, r *http.Request, err error) {
	de, ok := errors.AsType[*service.Error](err)
	if _, tooLarge := errors.AsType[*http.MaxBytesError](err); tooLarge {
		de, ok = &service.Error{Code: "too_large", Message: "request body too large"}, true
	}
	if !ok {
		s.log.Error("request failed", "method", r.Method, "path", r.URL.Path, "err", err)
		de = &service.Error{Code: "internal", Message: "internal server error"}
	}
	status, found := statusOf[de.Code]
	if !found {
		status = http.StatusInternalServerError
	}
	_ = writeJSON(w, status, map[string]any{"error": de})
}

func pathID(r *http.Request, name string) (int64, error) {
	id, err := strconv.ParseInt(r.PathValue(name), 10, 64)
	if err != nil || id <= 0 {
		return 0, &service.Error{Code: service.CodeInvalid, Message: "invalid id"}
	}
	return id, nil
}

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(code int) {
	if w.status == 0 {
		w.status = code
	}
	w.ResponseWriter.WriteHeader(code)
}

// Unwrap lets http.ResponseController reach Flush and deadlines.
func (w *statusWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }

func (s *Server) logRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		sw := &statusWriter{ResponseWriter: w}
		next.ServeHTTP(sw, r)
		if sw.status == 0 {
			sw.status = http.StatusOK
		}
		if strings.HasPrefix(r.URL.Path, "/api/") || strings.HasPrefix(r.URL.Path, "/mcp") {
			s.log.Info("http", "method", r.Method, "path", r.URL.Path, "status", sw.status, "dur", time.Since(start).Round(time.Microsecond))
		}
	})
}

func (s *Server) recoverer(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if v := recover(); v != nil {
				if err, ok := v.(error); ok && errors.Is(err, http.ErrAbortHandler) {
					panic(v)
				}
				s.log.Error("panic", "path", r.URL.Path, "value", v, "stack", string(debug.Stack()))
				http.Error(w, "internal server error", http.StatusInternalServerError)
			}
		}()
		next.ServeHTTP(w, r)
	})
}

func (s *Server) clientIP(r *http.Request) string {
	if s.cfg.TrustProxy {
		if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
			return strings.TrimSpace(strings.Split(xff, ",")[0])
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
