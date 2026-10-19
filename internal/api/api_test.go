package api_test

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"kala/internal/api"
	"kala/internal/auth"
	"kala/internal/config"
	"kala/internal/events"
	"kala/internal/mcp"
	"kala/internal/service"
	"kala/internal/store"
)

func newServer(t *testing.T) *httptest.Server {
	t.Helper()
	db, err := store.Open(context.Background(), filepath.Join(t.TempDir(), "api.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	hub := events.NewHub()
	svc := service.New(db, hub, nil)
	h := api.New(api.Options{
		Service: svc,
		Signer:  auth.NewSigner([]byte("test-secret-test-secret-test-secret"), api.AccessTTL),
		Hub:     hub,
		Config:  config.Config{Dev: true},
		Logger:  slog.New(slog.DiscardHandler),
		MCP:     mcp.Handler(svc),
	})
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	return srv
}

type client struct {
	t     *testing.T
	base  string
	http  *http.Client
	token string
}

func newClient(t *testing.T, base string) *client {
	jar, _ := cookiejar.New(nil)
	return &client{t: t, base: base, http: &http.Client{Jar: jar}}
}

func (c *client) do(method, path string, body any, out any) int {
	c.t.Helper()
	var r io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		r = bytes.NewReader(b)
	}
	req, _ := http.NewRequestWithContext(context.Background(), method, c.base+path, r)
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	req.Header.Set("Content-Type", "application/json")
	res, err := c.http.Do(req)
	if err != nil {
		c.t.Fatal(err)
	}
	defer func() { _ = res.Body.Close() }()
	if out != nil && res.StatusCode < 300 {
		if err := json.NewDecoder(res.Body).Decode(out); err != nil {
			c.t.Fatalf("%s %s: decode: %v", method, path, err)
		}
	}
	return res.StatusCode
}

type sessionResp struct {
	AccessToken string `json:"access_token"`
}

func (c *client) register(email string) {
	var s sessionResp
	if code := c.do("POST", "/api/v1/auth/register", map[string]string{"email": email, "password": "password123", "timezone": "Europe/Rome"}, &s); code != 200 {
		c.t.Fatalf("register: %d", code)
	}
	c.token = s.AccessToken
}

func TestAuthFlow(t *testing.T) {
	srv := newServer(t)
	c := newClient(t, srv.URL)

	if code := c.do("GET", "/api/v1/tasks", nil, nil); code != 401 {
		t.Fatalf("unauthenticated: %d", code)
	}
	c.register("a@example.com")
	if code := c.do("GET", "/api/v1/me", nil, nil); code != 200 {
		t.Fatalf("me: %d", code)
	}
	// Signup is closed after the first user.
	other := newClient(t, srv.URL)
	if code := other.do("POST", "/api/v1/auth/register", map[string]string{"email": "b@example.com", "password": "password123"}, nil); code != 403 {
		t.Fatalf("second signup: %d", code)
	}
	// Refresh with the cookie issues a new access token.
	c.token = ""
	var s sessionResp
	if code := c.do("POST", "/api/v1/auth/refresh", nil, &s); code != 200 || s.AccessToken == "" {
		t.Fatalf("refresh: %d", code)
	}
	c.token = s.AccessToken
	if code := c.do("POST", "/api/v1/auth/logout", nil, nil); code != 204 {
		t.Fatalf("logout: %d", code)
	}
	if code := c.do("POST", "/api/v1/auth/refresh", nil, nil); code != 401 {
		t.Fatalf("refresh after logout: %d", code)
	}
	if code := c.do("POST", "/api/v1/auth/login", map[string]string{"email": "a@example.com", "password": "wrong-password"}, nil); code != 401 {
		t.Fatalf("bad login: %d", code)
	}
}

func TestTasksAndPAT(t *testing.T) {
	srv := newServer(t)
	c := newClient(t, srv.URL)
	c.register("a@example.com")

	var task service.Task
	if code := c.do("POST", "/api/v1/tasks", map[string]string{"quick": "Ship it @kala !high due:today"}, &task); code != 201 {
		t.Fatalf("create: %d", code)
	}
	if code := c.do("PATCH", "/api/v1/tasks/999", map[string]string{"title": "x"}, nil); code != 404 {
		t.Fatalf("patch missing: %d", code)
	}
	if code := c.do("PATCH", "/api/v1/tasks/1", map[string]any{"bogus": 1}, nil); code != 400 {
		t.Fatalf("unknown field: %d", code)
	}
	if code := c.do("POST", "/api/v1/tasks/1/timer/start", nil, nil); code != 200 {
		t.Fatalf("start: %d", code)
	}
	if code := c.do("POST", "/api/v1/tasks/1/timer/start", nil, nil); code != 409 {
		t.Fatalf("double start: %d", code)
	}

	var pat service.AccessToken
	if code := c.do("POST", "/api/v1/tokens", map[string]string{"name": "cli"}, &pat); code != 201 || !strings.HasPrefix(pat.Token, auth.PATPrefix) {
		t.Fatalf("token: %d %+v", code, pat)
	}
	p := newClient(t, srv.URL)
	p.token = pat.Token
	var list []service.Task
	if code := p.do("GET", "/api/v1/tasks?q=project:kala", nil, &list); code != 200 || len(list) != 1 {
		t.Fatalf("list with PAT: %d %v", code, list)
	}
	if code := c.do("DELETE", "/api/v1/tokens/1", nil, nil); code != 204 {
		t.Fatalf("revoke: %d", code)
	}
	if code := p.do("GET", "/api/v1/tasks", nil, nil); code != 401 {
		t.Fatalf("revoked PAT: %d", code)
	}
}

func TestEventsStream(t *testing.T) {
	srv := newServer(t)
	c := newClient(t, srv.URL)
	c.register("a@example.com")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, "GET", srv.URL+"/api/v1/events", nil)
	req.Header.Set("Authorization", "Bearer "+c.token)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode != 200 {
		t.Fatalf("events: %d", res.StatusCode)
	}
	sc := bufio.NewScanner(res.Body)
	sc.Scan() // ": connected"

	// Create a task from another goroutine; it must not call t.Fatal.
	go func() {
		req, _ := http.NewRequestWithContext(ctx, "POST", srv.URL+"/api/v1/tasks", strings.NewReader(`{"title":"hello"}`))
		req.Header.Set("Authorization", "Bearer "+c.token)
		if res, err := http.DefaultClient.Do(req); err == nil {
			_ = res.Body.Close()
		}
	}()
	for sc.Scan() {
		if line := sc.Text(); strings.HasPrefix(line, "event: changed") {
			sc.Scan()
			if !strings.Contains(sc.Text(), "tasks") {
				t.Fatalf("data = %q", sc.Text())
			}
			return
		}
	}
	t.Fatalf("no changed event: %v", sc.Err())
}

type bearerTransport struct{ token string }

func (b bearerTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	r = r.Clone(r.Context())
	r.Header.Set("Authorization", "Bearer "+b.token)
	return http.DefaultTransport.RoundTrip(r)
}

func TestMCP(t *testing.T) {
	srv := newServer(t)
	c := newClient(t, srv.URL)
	c.register("a@example.com")

	// JWTs are not accepted on /mcp.
	if code := c.do("POST", "/mcp", map[string]any{}, nil); code != 401 {
		t.Fatalf("mcp with JWT: %d", code)
	}
	var pat service.AccessToken
	c.do("POST", "/api/v1/tokens", map[string]string{"name": "claude"}, &pat)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cl := sdk.NewClient(&sdk.Implementation{Name: "test", Version: "1"}, nil)
	sess, err := cl.Connect(ctx, &sdk.StreamableClientTransport{
		Endpoint:   srv.URL + "/mcp",
		HTTPClient: &http.Client{Transport: bearerTransport{pat.Token}},
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = sess.Close() }()

	tools, err := sess.ListTools(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(tools.Tools) < 20 {
		t.Fatalf("only %d tools", len(tools.Tools))
	}
	res, err := sess.CallTool(ctx, &sdk.CallToolParams{Name: "create_task", Arguments: map[string]any{"quick": "From Claude @kala"}})
	if err != nil || res.IsError {
		t.Fatalf("create_task: %v %+v", err, res)
	}
	res, err = sess.CallTool(ctx, &sdk.CallToolParams{Name: "search_tasks", Arguments: map[string]any{"query": "project:kala"}})
	if err != nil || res.IsError {
		t.Fatalf("search_tasks: %v %+v", err, res)
	}
	text := res.Content[0].(*sdk.TextContent).Text
	if !strings.Contains(text, "From Claude") {
		t.Fatalf("search result: %s", text)
	}
	res, err = sess.CallTool(ctx, &sdk.CallToolParams{Name: "get_task", Arguments: map[string]any{"id": 999}})
	if err != nil || !res.IsError {
		t.Fatalf("missing task should be a tool error: %v %+v", err, res)
	}
}
