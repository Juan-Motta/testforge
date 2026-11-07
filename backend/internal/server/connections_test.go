package server_test

import (
	"context"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

const testSecret = "sk-test-0123456789abcd"

func addMember(t *testing.T, e *env, slug, role string) *client {
	t.Helper()
	c := e.browser()
	u := signup(t, c, "U "+role, role+"-"+slug+"@example.com")["user"].(map[string]any)
	if _, err := e.pool.Exec(context.Background(),
		`INSERT INTO workspace_members (workspace_id, user_id, role) SELECT id, $1, $2 FROM workspaces WHERE slug = $3`,
		u["id"], role, slug); err != nil {
		t.Fatal(err)
	}
	return c
}

// fakeProvider answers like a provider's "list models"/"projects" endpoint and
// records the credentials it received.
type fakeProvider struct {
	srv      *httptest.Server
	lastAuth atomic.Value
	reject   atomic.Bool
}

func newFakeProvider(t *testing.T, tls bool) *fakeProvider {
	t.Helper()
	f := &fakeProvider{}
	f.lastAuth.Store("")
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization") + r.Header.Get("X-Api-Key") + r.Header.Get("Api-Key")
		f.lastAuth.Store(r.Method + " " + r.URL.RequestURI() + " " + auth + " " + r.Header.Get("Anthropic-Version"))
		if f.reject.Load() {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"error":"invalid key ` + testSecret + `"}`))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.HasSuffix(r.URL.Path, "/api/public/projects"):
			_, _ = w.Write([]byte(`{"data":[{"id":"p1","name":"Production"}]}`))
		default:
			_, _ = w.Write([]byte(`{"data":[{"id":"a"},{"id":"b"}]}`))
		}
	})
	if tls {
		f.srv = httptest.NewTLSServer(h)
	} else {
		f.srv = httptest.NewServer(h)
	}
	t.Cleanup(f.srv.Close)
	return f
}

func createConnection(t *testing.T, c *client, slug string, body map[string]any) map[string]any {
	t.Helper()
	r := c.do("POST", "/api/v1/workspaces/"+slug+"/connections", body)
	expectStatus(t, r, http.StatusCreated)
	return r.body
}

func ownerWorkspace(t *testing.T, e *env) (*client, string) {
	t.Helper()
	owner := e.browser()
	return owner, firstWorkspace(t, signup(t, owner, "Ana", "ana@example.com"))["slug"].(string)
}

func TestConnectionKindsCatalog(t *testing.T) {
	e := newEnv(t)
	owner, _ := ownerWorkspace(t, e)
	r := owner.do("GET", "/api/v1/connection-kinds", nil)
	expectStatus(t, r, http.StatusOK)

	kinds := map[string]map[string]any{}
	for _, k := range r.body["items"].([]any) {
		kind := k.(map[string]any)
		kinds[kind["kind"].(string)] = kind
	}
	for _, want := range []string{"openai", "anthropic", "azure_openai", "azure_speech", "langfuse"} {
		if kinds[want] == nil {
			t.Fatalf("catalog misses %q", want)
		}
	}
	lf := kinds["langfuse"]
	if lf["secretLabel"] != "Secret key" || lf["capabilities"].([]any)[0] != "traces" {
		t.Fatalf("langfuse = %v", lf)
	}
	openai := kinds["openai"]["fields"].([]any)[0].(map[string]any)
	if openai["key"] != "baseUrl" || openai["type"] != "url" || openai["default"] != "https://api.openai.com/v1" {
		t.Fatalf("openai baseUrl field = %v", openai)
	}
	expectStatus(t, e.browser().do("GET", "/api/v1/connection-kinds", nil), http.StatusUnauthorized)
}

func TestConnectionLifecycleKeepsSecretEncryptedAndHidden(t *testing.T) {
	e := newEnv(t)
	owner, slug := ownerWorkspace(t, e)
	provider := newFakeProvider(t, false)

	conn := createConnection(t, owner, slug, map[string]any{
		"name": "OpenAI prod", "kind": "openai", "secret": testSecret,
		"config": map[string]any{"baseUrl": provider.srv.URL + "/v1/"},
	})
	if conn["secret"] != nil || conn["secretHint"] != "•••• abcd" || conn["lastTest"] != nil {
		t.Fatalf("created = %v", conn)
	}
	cfg := conn["config"].(map[string]any)
	if cfg["baseUrl"] != provider.srv.URL+"/v1" || cfg["organization"] != nil {
		t.Fatalf("config = %v", cfg)
	}
	caps := conn["capabilities"].([]any)
	if len(caps) != 4 || caps[0] != "llm" {
		t.Fatalf("capabilities = %v", caps)
	}
	if conn["createdBy"].(map[string]any)["name"] != "Ana" {
		t.Fatalf("createdBy = %v", conn["createdBy"])
	}

	var stored string
	if err := e.pool.QueryRow(context.Background(), "SELECT secret_ciphertext FROM connections").Scan(&stored); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(stored, "v1.") || strings.Contains(stored, testSecret) {
		t.Fatalf("secret stored as %q", stored)
	}

	base := "/api/v1/workspaces/" + slug + "/connections"
	id := conn["id"].(string)
	list := owner.do("GET", base, nil)
	expectStatus(t, list, http.StatusOK)
	if items := list.body["items"].([]any); len(items) != 1 || items[0].(map[string]any)["secret"] != nil {
		t.Fatalf("list = %v", items)
	}
	expectStatus(t, owner.do("GET", base+"/"+id, nil), http.StatusOK)

	dup := owner.do("POST", base, map[string]any{"name": "openai PROD", "kind": "anthropic", "secret": "x", "config": map[string]any{}})
	expectStatus(t, dup, http.StatusConflict)

	// Rename and change config without resending the secret: the stored one is kept.
	upd := owner.do("PATCH", base+"/"+id, map[string]any{"name": "OpenAI staging", "config": map[string]any{"baseUrl": provider.srv.URL, "organization": "org-1"}})
	expectStatus(t, upd, http.StatusOK)
	if upd.body["name"] != "OpenAI staging" || upd.body["config"].(map[string]any)["organization"] != "org-1" || upd.body["secretHint"] != "•••• abcd" {
		t.Fatalf("updated = %v", upd.body)
	}
	test := owner.do("POST", base+"/"+id+"/test", nil)
	expectStatus(t, test, http.StatusOK)
	if test.body["ok"] != true || !strings.Contains(test.body["message"].(string), "2 models") {
		t.Fatalf("test = %v", test.body)
	}
	if got := provider.lastAuth.Load().(string); got != "GET /models Bearer "+testSecret+" " {
		t.Fatalf("provider saw %q", got)
	}
	got := owner.do("GET", base+"/"+id, nil).body
	if got["lastTest"].(map[string]any)["ok"] != true {
		t.Fatalf("lastTest not recorded: %v", got["lastTest"])
	}

	// A new secret replaces the old one and clears the last test.
	upd = owner.do("PATCH", base+"/"+id, map[string]any{"secret": "sk-new-secret-9999"})
	expectStatus(t, upd, http.StatusOK)
	if upd.body["secretHint"] != "•••• 9999" || upd.body["lastTest"] != nil {
		t.Fatalf("after secret change = %v", upd.body)
	}
	owner.do("POST", base+"/"+id+"/test", nil)
	if got := provider.lastAuth.Load().(string); !strings.Contains(got, "Bearer sk-new-secret-9999") {
		t.Fatalf("provider saw %q", got)
	}

	expectStatus(t, owner.do("DELETE", base+"/"+id, nil), http.StatusNoContent)
	expectStatus(t, owner.do("GET", base+"/"+id, nil), http.StatusNotFound)
	expectStatus(t, owner.do("GET", base+"/not-a-uuid", nil), http.StatusNotFound)
}

func TestConnectionValidation(t *testing.T) {
	e := newEnv(t)
	owner, slug := ownerWorkspace(t, e)
	base := "/api/v1/workspaces/" + slug + "/connections"

	cases := []struct {
		body  map[string]any
		field string
	}{
		{map[string]any{"name": "x", "kind": "nope", "secret": "s", "config": map[string]any{}}, "kind"},
		{map[string]any{"name": " ", "kind": "openai", "secret": "s", "config": map[string]any{}}, "name"},
		{map[string]any{"name": "x", "kind": "openai", "secret": "", "config": map[string]any{}}, "secret"},
		{map[string]any{"name": "x", "kind": "langfuse", "secret": "s", "config": map[string]any{}}, "config.publicKey"},
		{map[string]any{"name": "x", "kind": "langfuse", "secret": "s", "config": map[string]any{"publicKey": "nope"}}, "config.publicKey"},
		{map[string]any{"name": "x", "kind": "openai", "secret": "s", "config": map[string]any{"foo": "bar"}}, "config.foo"},
		{map[string]any{"name": "x", "kind": "openai", "secret": "s", "config": map[string]any{"baseUrl": "ftp://x"}}, "config.baseUrl"},
		{map[string]any{"name": "x", "kind": "azure_speech", "secret": "s", "config": map[string]any{"region": "East US"}}, "config.region"},
		{map[string]any{"name": "x", "kind": "azure_openai", "secret": "s", "config": map[string]any{}}, "config.endpoint"},
	}
	for _, tc := range cases {
		r := owner.do("POST", base, tc.body)
		expectStatus(t, r, http.StatusUnprocessableEntity)
		fields, _ := r.body["error"].(map[string]any)["fields"].(map[string]any)
		if fields[tc.field] == nil {
			t.Errorf("body %v: missing field error %q in %v", tc.body, tc.field, fields)
		}
	}

	conn := createConnection(t, owner, slug, map[string]any{"name": "Anthropic", "kind": "anthropic", "secret": "sk-ant-1234", "config": map[string]any{}})
	r := owner.do("PATCH", base+"/"+conn["id"].(string), map[string]any{"config": map[string]any{"baseUrl": "not a url"}})
	expectStatus(t, r, http.StatusUnprocessableEntity)
}

func TestHTTPProviderURLsRequirePrivateNetworkMode(t *testing.T) {
	e := newEnvWith(t, func(c *configOverride) { c.allowPrivate = false })
	owner, slug := ownerWorkspace(t, e)
	base := "/api/v1/workspaces/" + slug + "/connections"

	r := owner.do("POST", base, map[string]any{"name": "Local", "kind": "langfuse", "secret": "sk-lf-1", "config": map[string]any{"host": "http://localhost:3000", "publicKey": "pk-lf-1"}})
	expectStatus(t, r, http.StatusUnprocessableEntity)

	// https to a loopback address passes validation but is refused at dial time.
	provider := newFakeProvider(t, true)
	conn := createConnection(t, owner, slug, map[string]any{"name": "Sneaky", "kind": "openai", "secret": testSecret, "config": map[string]any{"baseUrl": provider.srv.URL}})
	test := owner.do("POST", base+"/"+conn["id"].(string)+"/test", nil)
	expectStatus(t, test, http.StatusOK)
	msg := test.body["message"].(string)
	if test.body["ok"] != false || !strings.Contains(msg, "private network") {
		t.Fatalf("test = %v", test.body)
	}
	if provider.lastAuth.Load().(string) != "" {
		t.Fatal("request reached the private address")
	}
}

func TestConnectionTestPerProviderAndFailures(t *testing.T) {
	e := newEnv(t)
	owner, slug := ownerWorkspace(t, e)
	base := "/api/v1/workspaces/" + slug + "/connections"
	provider := newFakeProvider(t, false)

	cases := []struct {
		kind, wantAuth, wantMsg string
		config                  map[string]any
	}{
		{"anthropic", "GET /v1/models " + testSecret + " 2023-06-01", "2 models", map[string]any{"baseUrl": provider.srv.URL}},
		{"azure_openai", "GET /openai/models?api-version=2024-10-21 " + testSecret + " ", "2 models", map[string]any{"endpoint": provider.srv.URL}},
		{"langfuse", "GET /api/public/projects Basic " + base64.StdEncoding.EncodeToString([]byte("pk-lf-abc:"+testSecret)) + " ", "Production", map[string]any{"host": provider.srv.URL, "publicKey": "pk-lf-abc"}},
	}
	for _, tc := range cases {
		conn := createConnection(t, owner, slug, map[string]any{"name": tc.kind, "kind": tc.kind, "secret": testSecret, "config": tc.config})
		r := owner.do("POST", base+"/"+conn["id"].(string)+"/test", nil)
		expectStatus(t, r, http.StatusOK)
		if r.body["ok"] != true || !strings.Contains(r.body["message"].(string), tc.wantMsg) {
			t.Errorf("%s test = %v", tc.kind, r.body)
		}
		if got := provider.lastAuth.Load().(string); got != tc.wantAuth {
			t.Errorf("%s provider saw %q, want %q", tc.kind, got, tc.wantAuth)
		}
	}

	provider.reject.Store(true)
	conn := createConnection(t, owner, slug, map[string]any{"name": "Rejected", "kind": "openai", "secret": testSecret, "config": map[string]any{"baseUrl": provider.srv.URL}})
	r := owner.do("POST", base+"/"+conn["id"].(string)+"/test", nil)
	msg := r.body["message"].(string)
	if r.body["ok"] != false || !strings.Contains(msg, "401") || strings.Contains(msg, testSecret) {
		t.Fatalf("rejected test = %v", r.body)
	}

	provider.srv.Close()
	r = owner.do("POST", base+"/"+conn["id"].(string)+"/test", nil)
	if r.body["ok"] != false || !strings.Contains(r.body["message"].(string), "Could not reach") {
		t.Fatalf("unreachable test = %v", r.body)
	}
}

func TestConnectionRolesAndTenancy(t *testing.T) {
	e := newEnv(t)
	owner, slug := ownerWorkspace(t, e)
	provider := newFakeProvider(t, false)
	base := "/api/v1/workspaces/" + slug + "/connections"
	conn := createConnection(t, owner, slug, map[string]any{"name": "OpenAI", "kind": "openai", "secret": testSecret, "config": map[string]any{"baseUrl": provider.srv.URL}})
	item := base + "/" + conn["id"].(string)
	body := map[string]any{"name": "x", "kind": "anthropic", "secret": "s", "config": map[string]any{}}

	admin := addMember(t, e, slug, "admin")
	expectStatus(t, admin.do("PATCH", item, map[string]any{"name": "Renamed"}), http.StatusOK)

	member := addMember(t, e, slug, "member")
	expectStatus(t, member.do("GET", base, nil), http.StatusOK)
	expectStatus(t, member.do("POST", item+"/test", nil), http.StatusOK)
	expectStatus(t, member.do("POST", base, body), http.StatusForbidden)
	expectStatus(t, member.do("PATCH", item, map[string]any{"name": "y"}), http.StatusForbidden)
	expectStatus(t, member.do("DELETE", item, nil), http.StatusForbidden)

	viewer := addMember(t, e, slug, "viewer")
	expectStatus(t, viewer.do("GET", item, nil), http.StatusOK)
	expectStatus(t, viewer.do("POST", item+"/test", nil), http.StatusForbidden)

	api := e.apiClient(createKey(t, owner, slug)["secret"].(string))
	expectStatus(t, api.do("GET", base, nil), http.StatusOK)
	expectStatus(t, api.do("POST", item+"/test", nil), http.StatusOK)
	expectStatus(t, api.do("POST", base, body), http.StatusForbidden)

	// Another workspace cannot reach this connection, even through its own slug.
	bob := e.browser()
	bobSlug := firstWorkspace(t, signup(t, bob, "Bob", "bob@example.com"))["slug"].(string)
	foreign := "/api/v1/workspaces/" + bobSlug + "/connections/" + conn["id"].(string)
	for _, m := range []string{"GET", "PATCH", "DELETE"} {
		expectStatus(t, bob.do(m, foreign, map[string]any{"name": "z"}), http.StatusNotFound)
	}
	expectStatus(t, bob.do("POST", foreign+"/test", nil), http.StatusNotFound)
	expectStatus(t, bob.do("GET", item, nil), http.StatusNotFound)
}

func TestDeletingCreatorKeepsConnection(t *testing.T) {
	e := newEnv(t)
	owner, slug := ownerWorkspace(t, e)
	admin := addMember(t, e, slug, "admin")
	conn := createConnection(t, admin, slug, map[string]any{"name": "Anthropic", "kind": "anthropic", "secret": "sk-ant-1234", "config": map[string]any{}})
	if _, err := e.pool.Exec(context.Background(), "DELETE FROM users WHERE email LIKE 'admin-%'"); err != nil {
		t.Fatal(err)
	}
	r := owner.do("GET", "/api/v1/workspaces/"+slug+"/connections/"+conn["id"].(string), nil)
	expectStatus(t, r, http.StatusOK)
	if _, present := r.body["createdBy"]; !present || r.body["createdBy"] != nil {
		t.Fatalf("createdBy = %v", r.body["createdBy"])
	}
}
