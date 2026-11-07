package server_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"testforge/internal/platform/config"
	"testforge/internal/platform/database/dbtest"
	"testforge/internal/server"
)

const appOrigin = "http://localhost:3000"

type env struct {
	t    *testing.T
	srv  *httptest.Server
	pool *pgxpool.Pool
}

func newEnv(t *testing.T) *env {
	t.Helper()
	pool := dbtest.New(t)
	cfg := config.Config{AppOrigins: []string{appOrigin}, SessionTTL: time.Hour}
	srv := httptest.NewServer(server.New(pool, cfg))
	t.Cleanup(srv.Close)
	return &env{t: t, srv: srv, pool: pool}
}

// client is a browser-like client: it keeps cookies and sends the app Origin.
type client struct {
	e      *env
	http   *http.Client
	origin string
	bearer string
}

func (e *env) browser() *client {
	jar, _ := cookiejar.New(nil)
	return &client{e: e, http: &http.Client{Jar: jar}, origin: appOrigin}
}

func (e *env) apiClient(secret string) *client {
	return &client{e: e, http: &http.Client{}, bearer: secret}
}

type response struct {
	status int
	header http.Header
	body   map[string]any
}

func (c *client) do(method, path string, body any) response {
	c.e.t.Helper()
	var reader io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		reader = bytes.NewReader(b)
	}
	req, _ := http.NewRequest(method, c.e.srv.URL+path, reader)
	req.Header.Set("Content-Type", "application/json")
	if c.origin != "" {
		req.Header.Set("Origin", c.origin)
	}
	if c.bearer != "" {
		req.Header.Set("Authorization", "Bearer "+c.bearer)
	}
	res, err := c.http.Do(req)
	if err != nil {
		c.e.t.Fatalf("%s %s: %v", method, path, err)
	}
	defer res.Body.Close()
	out := response{status: res.StatusCode, header: res.Header}
	raw, _ := io.ReadAll(res.Body)
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &out.body); err != nil {
			c.e.t.Fatalf("%s %s: decode %q: %v", method, path, raw, err)
		}
	}
	return out
}

func expectStatus(t *testing.T, r response, want int) {
	t.Helper()
	if r.status != want {
		t.Fatalf("status = %d, want %d (body %v)", r.status, want, r.body)
	}
}

func expectErrorCode(t *testing.T, r response, want string) {
	t.Helper()
	errObj, _ := r.body["error"].(map[string]any)
	if errObj["code"] != want {
		t.Fatalf("error code = %v, want %s (body %v)", errObj["code"], want, r.body)
	}
}

func signup(t *testing.T, c *client, name, email string) map[string]any {
	t.Helper()
	r := c.do("POST", "/api/v1/auth/signup", map[string]any{"name": name, "email": email, "password": "correct horse"})
	expectStatus(t, r, http.StatusCreated)
	return r.body
}

func firstWorkspace(t *testing.T, me map[string]any) map[string]any {
	t.Helper()
	ws, _ := me["workspaces"].([]any)
	if len(ws) == 0 {
		t.Fatalf("no workspaces in %v", me)
	}
	return ws[0].(map[string]any)
}

func createKey(t *testing.T, c *client, slug string) map[string]any {
	t.Helper()
	r := c.do("POST", "/api/v1/workspaces/"+slug+"/api-keys", map[string]any{"name": "CI"})
	expectStatus(t, r, http.StatusCreated)
	return r.body
}

func TestHealthz(t *testing.T) {
	e := newEnv(t)
	r := e.browser().do("GET", "/healthz", nil)
	expectStatus(t, r, http.StatusOK)
	if r.body["status"] != "ok" {
		t.Fatalf("body = %v", r.body)
	}
}

func TestSignupCreatesUserPersonalWorkspaceAndSession(t *testing.T) {
	e := newEnv(t)
	c := e.browser()

	r := c.do("POST", "/api/v1/auth/signup", map[string]any{"name": " Ana ", "email": "Ana@Example.com", "password": "correct horse"})
	expectStatus(t, r, http.StatusCreated)

	cookie := r.header.Get("Set-Cookie")
	for _, part := range []string{"tf_session=", "HttpOnly", "SameSite=Lax", "Path=/"} {
		if !strings.Contains(cookie, part) {
			t.Fatalf("Set-Cookie %q missing %q", cookie, part)
		}
	}

	user := r.body["user"].(map[string]any)
	if user["email"] != "ana@example.com" || user["name"] != "Ana" {
		t.Fatalf("user = %v", user)
	}
	if r.body["authMethod"] != "session" {
		t.Fatalf("authMethod = %v", r.body["authMethod"])
	}
	ws := firstWorkspace(t, r.body)
	if ws["role"] != "owner" || ws["name"] != "Ana's workspace" || ws["slug"] != "ana-s-workspace" {
		t.Fatalf("workspace = %v", ws)
	}

	var stored string
	if err := e.pool.QueryRow(context.Background(), "SELECT password_hash FROM users").Scan(&stored); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(stored, "$argon2id$") {
		t.Fatalf("password stored as %q, want argon2id hash", stored)
	}

	me := c.do("GET", "/api/v1/auth/me", nil)
	expectStatus(t, me, http.StatusOK)
	if me.body["user"].(map[string]any)["id"] != user["id"] {
		t.Fatalf("me = %v", me.body)
	}
}

func TestSignupValidation(t *testing.T) {
	e := newEnv(t)
	r := e.browser().do("POST", "/api/v1/auth/signup", map[string]any{"name": "  ", "email": "nope", "password": "short"})
	expectStatus(t, r, http.StatusUnprocessableEntity)
	expectErrorCode(t, r, "invalid")
	fields := r.body["error"].(map[string]any)["fields"].(map[string]any)
	for _, f := range []string{"name", "email", "password"} {
		if fields[f] == nil {
			t.Fatalf("missing field error %q in %v", f, fields)
		}
	}
}

func TestSignupDuplicateEmailIsConflict(t *testing.T) {
	e := newEnv(t)
	signup(t, e.browser(), "Ana", "ana@example.com")
	r := e.browser().do("POST", "/api/v1/auth/signup", map[string]any{"name": "Ana 2", "email": "ANA@example.com", "password": "correct horse"})
	expectStatus(t, r, http.StatusConflict)
	expectErrorCode(t, r, "conflict")
}

func TestLogin(t *testing.T) {
	e := newEnv(t)
	signup(t, e.browser(), "Ana", "ana@example.com")

	wrong := e.browser().do("POST", "/api/v1/auth/login", map[string]any{"email": "ana@example.com", "password": "wrong password"})
	expectStatus(t, wrong, http.StatusUnauthorized)
	unknown := e.browser().do("POST", "/api/v1/auth/login", map[string]any{"email": "bob@example.com", "password": "wrong password"})
	expectStatus(t, unknown, http.StatusUnauthorized)
	if wrong.body["error"].(map[string]any)["message"] != unknown.body["error"].(map[string]any)["message"] {
		t.Fatal("wrong password and unknown email must not be distinguishable")
	}

	c := e.browser()
	ok := c.do("POST", "/api/v1/auth/login", map[string]any{"email": "ANA@example.com", "password": "correct horse"})
	expectStatus(t, ok, http.StatusOK)
	if !strings.Contains(ok.header.Get("Set-Cookie"), "tf_session=") {
		t.Fatal("login did not set the session cookie")
	}
	expectStatus(t, c.do("GET", "/api/v1/auth/me", nil), http.StatusOK)
}

func TestLoginIsThrottledAfterRepeatedFailures(t *testing.T) {
	e := newEnv(t)
	signup(t, e.browser(), "Ana", "ana@example.com")
	c := e.browser()
	for i := 0; i < 10; i++ {
		expectStatus(t, c.do("POST", "/api/v1/auth/login", map[string]any{"email": "ana@example.com", "password": "wrong password"}), http.StatusUnauthorized)
	}
	r := c.do("POST", "/api/v1/auth/login", map[string]any{"email": "ana@example.com", "password": "correct horse"})
	expectStatus(t, r, http.StatusTooManyRequests)
	expectErrorCode(t, r, "rate_limited")
}

func TestLogoutEndsSession(t *testing.T) {
	e := newEnv(t)
	c := e.browser()
	signup(t, c, "Ana", "ana@example.com")

	r := c.do("POST", "/api/v1/auth/logout", nil)
	expectStatus(t, r, http.StatusNoContent)
	if !strings.Contains(r.header.Get("Set-Cookie"), "Max-Age=0") {
		t.Fatalf("logout did not clear the cookie: %q", r.header.Get("Set-Cookie"))
	}
	var n int
	_ = e.pool.QueryRow(context.Background(), "SELECT count(*) FROM sessions").Scan(&n)
	if n != 0 {
		t.Fatalf("sessions left = %d", n)
	}
	expectStatus(t, c.do("GET", "/api/v1/auth/me", nil), http.StatusUnauthorized)
}

func TestUnsafeCookieRequestsRequireAppOrigin(t *testing.T) {
	e := newEnv(t)
	c := e.browser()
	me := signup(t, c, "Ana", "ana@example.com")

	c.origin = "https://evil.example"
	r := c.do("POST", "/api/v1/workspaces", map[string]any{"name": "Team"})
	expectStatus(t, r, http.StatusForbidden)
	expectErrorCode(t, r, "forbidden")

	c.origin = ""
	expectStatus(t, c.do("POST", "/api/v1/workspaces", map[string]any{"name": "Team"}), http.StatusForbidden)
	expectStatus(t, e.apiClient("").do("POST", "/api/v1/auth/login", map[string]any{"email": "ana@example.com", "password": "correct horse"}), http.StatusForbidden)

	// Safe methods do not need an Origin.
	expectStatus(t, c.do("GET", "/api/v1/workspaces/"+firstWorkspace(t, me)["slug"].(string), nil), http.StatusOK)
}

func TestWorkspaces(t *testing.T) {
	e := newEnv(t)
	ana := e.browser()
	signup(t, ana, "Ana", "ana@example.com")
	bob := e.browser()
	bobWs := firstWorkspace(t, signup(t, bob, "Bob", "bob@example.com"))

	created := ana.do("POST", "/api/v1/workspaces", map[string]any{"name": "Voice Team"})
	expectStatus(t, created, http.StatusCreated)
	if created.body["slug"] != "voice-team" || created.body["role"] != "owner" {
		t.Fatalf("created = %v", created.body)
	}

	again := ana.do("POST", "/api/v1/workspaces", map[string]any{"name": "Voice Team"})
	expectStatus(t, again, http.StatusCreated)
	if slug := again.body["slug"].(string); slug == "voice-team" || !strings.HasPrefix(slug, "voice-team-") {
		t.Fatalf("duplicate name slug = %q, want voice-team-<suffix>", slug)
	}

	expectStatus(t, ana.do("POST", "/api/v1/workspaces", map[string]any{"name": " "}), http.StatusUnprocessableEntity)

	list := ana.do("GET", "/api/v1/workspaces", nil)
	expectStatus(t, list, http.StatusOK)
	if n := len(list.body["items"].([]any)); n != 3 {
		t.Fatalf("workspaces = %d, want 3", n)
	}

	get := ana.do("GET", "/api/v1/workspaces/voice-team", nil)
	expectStatus(t, get, http.StatusOK)
	if get.body["name"] != "Voice Team" {
		t.Fatalf("get = %v", get.body)
	}

	other := ana.do("GET", "/api/v1/workspaces/"+bobWs["slug"].(string), nil)
	expectStatus(t, other, http.StatusNotFound)
	expectErrorCode(t, other, "not_found")

	expectStatus(t, e.browser().do("GET", "/api/v1/workspaces", nil), http.StatusUnauthorized)
}

func TestApiKeyLifecycle(t *testing.T) {
	e := newEnv(t)
	ana := e.browser()
	me := signup(t, ana, "Ana", "ana@example.com")
	slug := firstWorkspace(t, me)["slug"].(string)
	team := ana.do("POST", "/api/v1/workspaces", map[string]any{"name": "Team"})
	expectStatus(t, team, http.StatusCreated)

	key := createKey(t, ana, slug)
	secret, _ := key["secret"].(string)
	if !strings.HasPrefix(secret, "tf_") || len(secret) < 40 {
		t.Fatalf("secret = %q", secret)
	}
	if key["prefix"] != secret[:11] {
		t.Fatalf("prefix = %v, want %q", key["prefix"], secret[:11])
	}
	if key["createdBy"].(map[string]any)["name"] != "Ana" || key["lastUsedAt"] != nil {
		t.Fatalf("key = %v", key)
	}

	list := ana.do("GET", "/api/v1/workspaces/"+slug+"/api-keys", nil)
	expectStatus(t, list, http.StatusOK)
	items := list.body["items"].([]any)
	if len(items) != 1 || items[0].(map[string]any)["secret"] != nil {
		t.Fatalf("list must contain the key without its secret: %v", items)
	}

	api := e.apiClient(secret)
	me2 := api.do("GET", "/api/v1/auth/me", nil)
	expectStatus(t, me2, http.StatusOK)
	if me2.body["authMethod"] != "api_key" {
		t.Fatalf("authMethod = %v", me2.body["authMethod"])
	}
	ws := me2.body["workspaces"].([]any)
	if len(ws) != 1 || ws[0].(map[string]any)["slug"] != slug || ws[0].(map[string]any)["role"] != "member" {
		t.Fatalf("api key workspaces = %v", ws)
	}

	// Scoped to its own workspace, even one its creator can reach.
	expectStatus(t, api.do("GET", "/api/v1/workspaces/"+team.body["slug"].(string), nil), http.StatusNotFound)
	expectStatus(t, api.do("GET", "/api/v1/workspaces/"+slug, nil), http.StatusOK)

	// Management is session only.
	expectStatus(t, api.do("POST", "/api/v1/workspaces/"+slug+"/api-keys", map[string]any{"name": "x"}), http.StatusForbidden)
	expectStatus(t, api.do("GET", "/api/v1/workspaces/"+slug+"/api-keys", nil), http.StatusForbidden)
	expectStatus(t, api.do("POST", "/api/v1/workspaces", map[string]any{"name": "x"}), http.StatusForbidden)

	used := ana.do("GET", "/api/v1/workspaces/"+slug+"/api-keys", nil).body["items"].([]any)[0].(map[string]any)
	if used["lastUsedAt"] == nil {
		t.Fatal("lastUsedAt not recorded after use")
	}

	path := "/api/v1/workspaces/" + slug + "/api-keys/" + key["id"].(string)
	expectStatus(t, ana.do("DELETE", path, nil), http.StatusNoContent)
	expectStatus(t, ana.do("DELETE", path, nil), http.StatusNoContent)
	revoked := ana.do("GET", "/api/v1/workspaces/"+slug+"/api-keys", nil).body["items"].([]any)[0].(map[string]any)
	if revoked["revokedAt"] == nil {
		t.Fatal("revokedAt not set")
	}
	expectStatus(t, api.do("GET", "/api/v1/auth/me", nil), http.StatusUnauthorized)

	expectStatus(t, ana.do("DELETE", "/api/v1/workspaces/"+slug+"/api-keys/00000000-0000-0000-0000-000000000000", nil), http.StatusNotFound)
}

func TestApiKeyExpiryAndInvalidBearer(t *testing.T) {
	e := newEnv(t)
	ana := e.browser()
	slug := firstWorkspace(t, signup(t, ana, "Ana", "ana@example.com"))["slug"].(string)

	r := ana.do("POST", "/api/v1/workspaces/"+slug+"/api-keys", map[string]any{"name": "short", "expiresInDays": 1})
	expectStatus(t, r, http.StatusCreated)
	if r.body["expiresAt"] == nil {
		t.Fatal("expiresAt not set")
	}
	expectStatus(t, ana.do("POST", "/api/v1/workspaces/"+slug+"/api-keys", map[string]any{"name": "bad", "expiresInDays": 0}), http.StatusUnprocessableEntity)

	if _, err := e.pool.Exec(context.Background(), "UPDATE api_keys SET expires_at = now() - interval '1 second'"); err != nil {
		t.Fatal(err)
	}
	expectStatus(t, e.apiClient(r.body["secret"].(string)).do("GET", "/api/v1/auth/me", nil), http.StatusUnauthorized)

	// An invalid bearer is rejected even when a valid session cookie is present.
	ana.bearer = "tf_not_a_real_key"
	expectStatus(t, ana.do("GET", "/api/v1/auth/me", nil), http.StatusUnauthorized)
}

func TestApiKeyManagementRequiresAdmin(t *testing.T) {
	e := newEnv(t)
	owner := e.browser()
	slug := firstWorkspace(t, signup(t, owner, "Ana", "ana@example.com"))["slug"].(string)
	key := createKey(t, owner, slug)

	for _, role := range []string{"member", "viewer"} {
		c := e.browser()
		u := signup(t, c, "U "+role, role+"@example.com")["user"].(map[string]any)
		_, err := e.pool.Exec(context.Background(),
			`INSERT INTO workspace_members (workspace_id, user_id, role)
			 SELECT id, $1, $2 FROM workspaces WHERE slug = $3`, u["id"], role, slug)
		if err != nil {
			t.Fatal(err)
		}
		expectStatus(t, c.do("GET", "/api/v1/workspaces/"+slug+"/api-keys", nil), http.StatusOK)
		r := c.do("POST", "/api/v1/workspaces/"+slug+"/api-keys", map[string]any{"name": "x"})
		expectStatus(t, r, http.StatusForbidden)
		expectStatus(t, c.do("DELETE", "/api/v1/workspaces/"+slug+"/api-keys/"+key["id"].(string), nil), http.StatusForbidden)
	}
}
