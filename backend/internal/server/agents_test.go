package server_test

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
)

const agentSecret = "vivi-key-0123456789"

func wsURL(srv *httptest.Server) string { return "ws" + strings.TrimPrefix(srv.URL, "http") }

// fakeViviLive behaves like a Vivi Live voice channel and records what it got.
type fakeViviLive struct {
	srv       *httptest.Server
	lastQuery chan string
	lastKey   chan string
	gotClose  chan bool
}

func newFakeViviLive(t *testing.T, echoSecret bool) *fakeViviLive {
	t.Helper()
	f := &fakeViviLive{lastQuery: make(chan string, 1), lastKey: make(chan string, 1), gotClose: make(chan bool, 1)}
	f.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.lastQuery <- r.URL.EscapedPath() + "?" + r.URL.RawQuery
		f.lastKey <- r.Header.Get("X-Api-Key")
		c, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		defer c.CloseNow()
		ctx := r.Context()
		send := func(s string) { _ = c.Write(ctx, websocket.MessageText, []byte(s)) }
		send(`{"type":"voice.protocol","protocol":"live-v1"}`)
		send(`{"type":"voice.ready","sessionId":"s1"}`)
		audio := base64.StdEncoding.EncodeToString(make([]byte, 4800)) // 100 ms at 24 kHz
		send(`{"type":"voice.audio.delta","delta":"` + audio + `"}`)
		send(`{"type":"voice.transcript.update","role":"assistant","messageId":"m1","text":"Ho","startMs":0}`)
		send(`{"type":"voice.transcript.update","role":"assistant","messageId":"m1","text":"Hola","startMs":0}`)
		if echoSecret {
			send(`{"type":"debug","note":"key was ` + agentSecret + `"}`)
		}
		for {
			_, data, err := c.Read(ctx)
			if err != nil {
				return
			}
			if strings.Contains(string(data), `"voice.close"`) {
				f.gotClose <- true
				send(`{"type":"voice.closed","usageSeconds":1}`)
				_ = c.Close(websocket.StatusNormalClosure, "")
				return
			}
		}
	}))
	t.Cleanup(f.srv.Close)
	return f
}

func viviLiveAgent(url string) map[string]any {
	return map[string]any{
		"name": "Booking voice", "description": "Books appointments.", "modality": "voice",
		"transport": map[string]any{
			"url": url + "/channelApi/{{var.channelId}}/realtime",
			"query": []any{
				map[string]any{"key": "voiceCapability", "value": "live-v1"},
				map[string]any{"key": "userId", "value": "{{var.userId}}"},
				map[string]any{"key": "threadId", "value": "{{run.thread_id}}"},
			},
			"headers":      []any{map[string]any{"key": "X-Api-Key", "value": "{{secret.api_key}}"}},
			"body":         nil,
			"initMessages": []any{},
			"preSession":   nil,
		},
		"codec":             map[string]any{"type": "vivi_live", "options": map[string]any{}},
		"variables":         []any{map[string]any{"key": "channelId", "value": "ch/1"}, map[string]any{"key": "userId", "value": "u1"}},
		"secrets":           []any{map[string]any{"name": "api_key", "value": agentSecret}},
		"traceConnectionId": nil,
		"traceSessionId":    "{{run.thread_id}}",
	}
}

func clone(m map[string]any) map[string]any {
	b, _ := json.Marshal(m)
	var out map[string]any
	_ = json.Unmarshal(b, &out)
	return out
}

func createAgent(t *testing.T, c *client, slug string, body map[string]any) map[string]any {
	t.Helper()
	r := c.do("POST", "/api/v1/workspaces/"+slug+"/agents", body)
	expectStatus(t, r, http.StatusCreated)
	return r.body
}

func TestAgentCatalogs(t *testing.T) {
	e := newEnv(t)
	owner, _ := ownerWorkspace(t, e)

	codecs := owner.do("GET", "/api/v1/agent-codecs", nil)
	expectStatus(t, codecs, http.StatusOK)
	if n := len(codecs.body["items"].([]any)); n != 5 {
		t.Fatalf("codecs = %d", n)
	}

	presets := owner.do("GET", "/api/v1/agent-presets", nil)
	expectStatus(t, presets, http.StatusOK)
	byID := map[string]map[string]any{}
	for _, p := range presets.body["items"].([]any) {
		preset := p.(map[string]any)
		byID[preset["id"].(string)] = preset
	}
	live := byID["vivi_live"]
	if live == nil || live["draft"].(map[string]any)["codec"].(map[string]any)["type"] != "vivi_live" || live["modality"] != "voice" {
		t.Fatalf("vivi_live preset = %v", live)
	}
	text := byID["vivi_text"]
	if text == nil || text["draft"].(map[string]any)["codec"].(map[string]any)["type"] != "http_stream" || text["secretNames"].([]any)[0] != "api_key" {
		t.Fatalf("vivi_text preset = %v", text)
	}
	if byID["openai_realtime"] == nil || byID["vivi_realtime"] == nil || byID["generic_json"] == nil || byID["http_json"] == nil {
		t.Fatalf("missing presets: %v", presets.body)
	}
}

func TestAgentLifecycleAndSecrets(t *testing.T) {
	e := newEnv(t)
	owner, slug := ownerWorkspace(t, e)
	base := "/api/v1/workspaces/" + slug + "/agents"
	body := viviLiveAgent("wss://agents.example.com")

	agent := createAgent(t, owner, slug, body)
	secrets := agent["secrets"].([]any)
	if len(secrets) != 1 || secrets[0].(map[string]any)["hint"] != "•••• 6789" || secrets[0].(map[string]any)["value"] != nil {
		t.Fatalf("secrets = %v", secrets)
	}
	if agent["codec"].(map[string]any)["options"].(map[string]any)["sampleRate"] != "24000" {
		t.Fatalf("codec defaults not applied: %v", agent["codec"])
	}
	var raw string
	if err := e.pool.QueryRow(context.Background(), "SELECT secrets::text FROM agents").Scan(&raw); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(raw, agentSecret) || !strings.Contains(raw, "v1.") {
		t.Fatalf("stored secrets = %s", raw)
	}

	id := agent["id"].(string)
	item := base + "/" + id
	expectStatus(t, owner.do("GET", item, nil), http.StatusOK)
	if n := len(owner.do("GET", base, nil).body["items"].([]any)); n != 1 {
		t.Fatalf("list = %d", n)
	}
	dup := clone(body)
	dup["name"] = "BOOKING VOICE"
	expectStatus(t, owner.do("POST", base, dup), http.StatusConflict)

	// Rename, keep the secret with an empty value.
	upd := clone(body)
	upd["name"] = "Booking voice v2"
	upd["secrets"] = []any{map[string]any{"name": "api_key", "value": ""}}
	r := owner.do("PUT", item, upd)
	expectStatus(t, r, http.StatusOK)
	if r.body["name"] != "Booking voice v2" || r.body["secrets"].([]any)[0].(map[string]any)["hint"] != "•••• 6789" {
		t.Fatalf("updated = %v", r.body)
	}

	// Moving the transport to another URL requires the secret again.
	moved := clone(upd)
	moved["transport"].(map[string]any)["url"] = "wss://attacker.example.com/channelApi/{{var.channelId}}/realtime"
	r = owner.do("PUT", item, moved)
	expectStatus(t, r, http.StatusUnprocessableEntity)
	if r.body["error"].(map[string]any)["fields"].(map[string]any)["secrets"] == nil {
		t.Fatalf("want secrets error: %v", r.body)
	}
	moved["secrets"] = []any{map[string]any{"name": "api_key", "value": "new-key-9999"}}
	r = owner.do("PUT", item, moved)
	expectStatus(t, r, http.StatusOK)

	// A new secret name needs a value; omitted names are deleted.
	more := clone(moved)
	more["secrets"] = []any{map[string]any{"name": "api_key", "value": ""}, map[string]any{"name": "other", "value": ""}}
	expectStatus(t, owner.do("PUT", item, more), http.StatusUnprocessableEntity)
	none := clone(moved)
	none["transport"].(map[string]any)["headers"] = []any{}
	none["secrets"] = []any{}
	r = owner.do("PUT", item, none)
	expectStatus(t, r, http.StatusOK)
	if len(r.body["secrets"].([]any)) != 0 {
		t.Fatalf("secrets after removal = %v", r.body["secrets"])
	}

	// Modality cannot change.
	text := clone(none)
	text["modality"] = "text"
	expectStatus(t, owner.do("PUT", item, text), http.StatusUnprocessableEntity)

	expectStatus(t, owner.do("DELETE", item, nil), http.StatusNoContent)
	expectStatus(t, owner.do("GET", item, nil), http.StatusNotFound)
}

func TestAgentValidation(t *testing.T) {
	e := newEnv(t)
	owner, slug := ownerWorkspace(t, e)
	base := "/api/v1/workspaces/" + slug + "/agents"
	lf := createConnection(t, owner, slug, map[string]any{"name": "LF", "kind": "langfuse", "secret": "sk-lf-1", "config": map[string]any{"publicKey": "pk-lf-1"}})
	llm := createConnection(t, owner, slug, map[string]any{"name": "Claude", "kind": "anthropic", "secret": "sk-ant-1", "config": map[string]any{}})

	tr := func(b map[string]any) map[string]any { return b["transport"].(map[string]any) }
	cases := []struct {
		name   string
		mutate func(b map[string]any)
		field  string
	}{
		{"https for voice", func(b map[string]any) { tr(b)["url"] = "https://x.example.com/ws" }, "transport.url"},
		{"query in url", func(b map[string]any) { tr(b)["url"] = "wss://x.example.com/ws?a=1" }, "transport.url"},
		{"templated host", func(b map[string]any) { tr(b)["url"] = "wss://{{var.channelId}}.example.com/ws" }, "transport.url"},
		{"unknown var", func(b map[string]any) { tr(b)["headers"] = []any{map[string]any{"key": "X", "value": "{{var.nope}}"}} }, "transport.headers.0.value"},
		{"unknown secret", func(b map[string]any) { b["secrets"] = []any{} }, "transport.headers.0.value"},
		{"bad template", func(b map[string]any) { tr(b)["query"] = []any{map[string]any{"key": "a", "value": "{{env.HOME}}"}} }, "transport.query.0.value"},
		{"forbidden header", func(b map[string]any) { tr(b)["headers"] = []any{map[string]any{"key": "Host", "value": "x"}} }, "transport.headers.0.key"},
		{"session without pre-session", func(b map[string]any) { b["traceSessionId"] = "{{session.sid}}" }, "traceSessionId"},
		{"message outside body", func(b map[string]any) { tr(b)["initMessages"] = []any{`{"m":"{{message}}"}`} }, "transport.initMessages.0"},
		{"invalid init JSON", func(b map[string]any) { tr(b)["initMessages"] = []any{`{"m":}`} }, "transport.initMessages.0"},
		{"text codec on voice", func(b map[string]any) { b["codec"] = map[string]any{"type": "http_json", "options": map[string]any{}} }, "codec.type"},
		{"bad option", func(b map[string]any) {
			b["codec"] = map[string]any{"type": "vivi_live", "options": map[string]any{"sampleRate": "fast"}}
		}, "codec.options.sampleRate"},
		{"duplicate variable", func(b map[string]any) {
			b["variables"] = append(b["variables"].([]any), map[string]any{"key": "channelId", "value": "x"})
		}, "variables.2.key"},
		{"non-traces connection", func(b map[string]any) { b["traceConnectionId"] = llm["id"] }, "traceConnectionId"},
		{"missing secret value", func(b map[string]any) { b["secrets"] = []any{map[string]any{"name": "api_key", "value": ""}} }, "secrets.0.value"},
		{"empty name", func(b map[string]any) { b["name"] = " " }, "name"},
	}
	for _, tc := range cases {
		b := viviLiveAgent("wss://agents.example.com")
		tc.mutate(b)
		r := owner.do("POST", base, b)
		if r.status != http.StatusUnprocessableEntity {
			t.Errorf("%s: status %d, body %v", tc.name, r.status, r.body)
			continue
		}
		fields, _ := r.body["error"].(map[string]any)["fields"].(map[string]any)
		if fields[tc.field] == nil {
			t.Errorf("%s: missing %q in %v", tc.name, tc.field, fields)
		}
	}

	text := map[string]any{
		"name": "Chat", "description": "", "modality": "text",
		"transport": map[string]any{"url": "https://chat.example.com/stream", "query": []any{}, "headers": []any{},
			"body": `{"threadId":"{{run.thread_id}}"}`, "initMessages": []any{}, "preSession": nil},
		"codec": map[string]any{"type": "http_json", "options": map[string]any{}}, "variables": []any{}, "secrets": []any{},
		"traceConnectionId": lf["id"], "traceSessionId": "",
	}
	r := owner.do("POST", base, text)
	expectStatus(t, r, http.StatusUnprocessableEntity)
	if r.body["error"].(map[string]any)["fields"].(map[string]any)["transport.body"] == nil {
		t.Fatalf("body without {{message}}: %v", r.body)
	}
	text["transport"].(map[string]any)["body"] = `{"message":"{{message}}","threadId":"{{run.thread_id}}"}`
	ok := createAgent(t, owner, slug, text)
	if ok["traceSessionId"] != "{{run.thread_id}}" || ok["traceConnectionId"] != lf["id"] {
		t.Fatalf("text agent = %v", ok)
	}
}

func TestAgentRolesAndTenancy(t *testing.T) {
	e := newEnv(t)
	owner, slug := ownerWorkspace(t, e)
	fake := newFakeViviLive(t, false)
	agent := createAgent(t, owner, slug, viviLiveAgent(wsURL(fake.srv)))
	base := "/api/v1/workspaces/" + slug + "/agents"
	item := base + "/" + agent["id"].(string)
	probe := map[string]any{"durationSeconds": 1}

	member := addMember(t, e, slug, "member")
	expectStatus(t, member.do("GET", base, nil), http.StatusOK)
	expectStatus(t, member.do("POST", base, viviLiveAgent("wss://x.example.com")), http.StatusForbidden)
	expectStatus(t, member.do("DELETE", item, nil), http.StatusForbidden)
	expectStatus(t, member.do("POST", item+"/probe", probe), http.StatusOK)

	viewer := addMember(t, e, slug, "viewer")
	expectStatus(t, viewer.do("GET", item, nil), http.StatusOK)
	expectStatus(t, viewer.do("POST", item+"/probe", probe), http.StatusForbidden)

	bob := e.browser()
	bobSlug := firstWorkspace(t, signup(t, bob, "Bob", "bob@example.com"))["slug"].(string)
	foreign := "/api/v1/workspaces/" + bobSlug + "/agents/" + agent["id"].(string)
	expectStatus(t, bob.do("GET", foreign, nil), http.StatusNotFound)
	expectStatus(t, bob.do("POST", foreign+"/probe", probe), http.StatusNotFound)
	expectStatus(t, bob.do("DELETE", foreign, nil), http.StatusNotFound)
}

func eventKinds(r response) string {
	var kinds []string
	for _, ev := range r.body["events"].([]any) {
		kinds = append(kinds, ev.(map[string]any)["kind"].(string))
	}
	return strings.Join(kinds, ",")
}

func TestProbeViviLiveVoiceAgent(t *testing.T) {
	e := newEnv(t)
	owner, slug := ownerWorkspace(t, e)
	fake := newFakeViviLive(t, true)
	agent := createAgent(t, owner, slug, viviLiveAgent(wsURL(fake.srv)))

	r := owner.do("POST", "/api/v1/workspaces/"+slug+"/agents/"+agent["id"].(string)+"/probe", map[string]any{"durationSeconds": 1})
	expectStatus(t, r, http.StatusOK)
	if r.body["ok"] != true || r.body["audioMs"].(float64) != 100 || r.body["transcript"] != "Hola" {
		t.Fatalf("probe = %v", r.body)
	}
	thread := r.body["threadId"].(string)
	if len(thread) != 36 || r.body["traceSessionId"] != thread {
		t.Fatalf("thread %q trace %v", thread, r.body["traceSessionId"])
	}
	kinds := eventKinds(r)
	for _, k := range []string{"connect", "ready", "audio", "transcript", "send", "close"} {
		if !strings.Contains(kinds, k) {
			t.Fatalf("events %s missing %s", kinds, k)
		}
	}
	raw, _ := json.Marshal(r.body)
	if strings.Contains(string(raw), agentSecret) {
		t.Fatalf("probe result leaks the secret: %s", raw)
	}
	if q := <-fake.lastQuery; q != "/channelApi/ch%2F1/realtime?threadId="+thread+"&userId=u1&voiceCapability=live-v1" {
		t.Fatalf("agent saw %q", q)
	}
	if k := <-fake.lastKey; k != agentSecret {
		t.Fatalf("agent got key %q", k)
	}
	select {
	case <-fake.gotClose:
	case <-time.After(2 * time.Second):
		t.Fatal("probe did not send voice.close")
	}
}

func TestProbeJSONAudioWithPreSession(t *testing.T) {
	e := newEnv(t)
	owner, slug := ownerWorkspace(t, e)
	gotSession := make(chan string, 1)
	gotPreBody := make(chan string, 1)
	mux := http.NewServeMux()
	mux.HandleFunc("POST /sessions", func(w http.ResponseWriter, r *http.Request) {
		b := make([]byte, 512)
		n, _ := r.Body.Read(b)
		gotPreBody <- string(b[:n])
		_, _ = w.Write([]byte(`{"data":{"session_id":"sess-42"}}`))
	})
	mux.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		gotSession <- r.URL.Query().Get("session_id")
		c, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		defer c.CloseNow()
		_ = c.Write(r.Context(), websocket.MessageText, []byte(`{"type":"ready"}`))
		_ = c.Write(r.Context(), websocket.MessageText, []byte(`{"type":"agent_transcript","delta":"Hi "}`))
		_ = c.Write(r.Context(), websocket.MessageText, []byte(`{"type":"agent_transcript","delta":"there"}`))
		for {
			if _, _, err := c.Read(r.Context()); err != nil {
				return
			}
		}
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	body := map[string]any{
		"name": "Relay", "description": "", "modality": "voice",
		"transport": map[string]any{
			"url":          wsURL(srv) + "/ws",
			"query":        []any{map[string]any{"key": "session_id", "value": "{{session.sid}}"}},
			"headers":      []any{},
			"body":         nil,
			"initMessages": []any{`{"type":"hello","thread":"{{run.thread_id}}"}`},
			"preSession": map[string]any{
				"method": "POST", "url": srv.URL + "/sessions", "headers": []any{},
				"body": `{"thread":"{{run.thread_id}}"}`, "captures": []any{map[string]any{"name": "sid", "path": "data.session_id"}},
			},
		},
		"codec":     map[string]any{"type": "json_audio", "options": map[string]any{}},
		"variables": []any{}, "secrets": []any{}, "traceConnectionId": nil, "traceSessionId": "{{session.sid}}",
	}
	agent := createAgent(t, owner, slug, body)
	r := owner.do("POST", "/api/v1/workspaces/"+slug+"/agents/"+agent["id"].(string)+"/probe", map[string]any{"durationSeconds": 1})
	expectStatus(t, r, http.StatusOK)
	if r.body["ok"] != true || r.body["traceSessionId"] != "sess-42" || r.body["transcript"] != "Hi there" {
		t.Fatalf("probe = %v", r.body)
	}
	if s := <-gotSession; s != "sess-42" {
		t.Fatalf("ws session_id = %q", s)
	}
	if pre := <-gotPreBody; pre != fmt.Sprintf(`{"thread":"%s"}`, r.body["threadId"]) {
		t.Fatalf("pre-session body = %q", pre)
	}
}

func TestProbeTextStreamAgent(t *testing.T) {
	e := newEnv(t)
	owner, slug := ownerWorkspace(t, e)
	type seen struct{ key, body string }
	got := make(chan seen, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b := make([]byte, 1024)
		n, _ := r.Body.Read(b)
		got <- seen{r.Header.Get("Vivi-Api-Key"), string(b[:n])}
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, "data: {\"event\":\"on_chat_model_stream\",\"message\":\"Hola\"}\n\n")
		fmt.Fprint(w, "data: {\"event\":\"on_tool_start\",\"metadata\":{\"toolName\":\"calendar\"}}\n\n")
		fmt.Fprint(w, "data: {\"event\":\"on_chat_model_stream\",\"message\":\"!\"}\n\n")
		fmt.Fprint(w, "data: {\"event\":\"on_chat_end\"}\n\n")
	}))
	t.Cleanup(srv.Close)

	body := map[string]any{
		"name": "Vivi text", "description": "", "modality": "text",
		"transport": map[string]any{
			"url": srv.URL + "/channelsApi/{{var.channelId}}/stream", "query": []any{},
			"headers":      []any{map[string]any{"key": "vivi-api-key", "value": "{{secret.api_key}}"}},
			"body":         `{"message":"{{message}}","threadId":"{{run.thread_id}}"}`,
			"initMessages": []any{}, "preSession": nil,
		},
		"codec": map[string]any{"type": "http_stream", "options": map[string]any{
			"deltaWhen": "event=on_chat_model_stream", "deltaPath": "message", "endWhen": "event=on_chat_end"}},
		"variables":         []any{map[string]any{"key": "channelId", "value": "c1"}},
		"secrets":           []any{map[string]any{"name": "api_key", "value": agentSecret}},
		"traceConnectionId": nil, "traceSessionId": "{{run.thread_id}}",
	}
	agent := createAgent(t, owner, slug, body)
	r := owner.do("POST", "/api/v1/workspaces/"+slug+"/agents/"+agent["id"].(string)+"/probe", map[string]any{"message": `Say "hi"`})
	expectStatus(t, r, http.StatusOK)
	if r.body["ok"] != true || r.body["reply"] != "Hola!" {
		t.Fatalf("probe = %v", r.body)
	}
	s := <-got
	var sent map[string]string
	if err := json.Unmarshal([]byte(s.body), &sent); err != nil || sent["message"] != `Say "hi"` || sent["threadId"] != r.body["threadId"] {
		t.Fatalf("agent got body %q (%v)", s.body, err)
	}
	if s.key != agentSecret {
		t.Fatalf("agent got key %q", s.key)
	}
	if !strings.Contains(eventKinds(r), "reply") {
		t.Fatalf("events = %s", eventKinds(r))
	}
	expectStatus(t, owner.do("POST", "/api/v1/workspaces/"+slug+"/agents/"+agent["id"].(string)+"/probe", map[string]any{"durationSeconds": 99}), http.StatusUnprocessableEntity)
}

func TestProbeFailuresAndPrivateNetwork(t *testing.T) {
	e := newEnv(t)
	owner, slug := ownerWorkspace(t, e)
	fake := newFakeViviLive(t, false)
	agent := createAgent(t, owner, slug, viviLiveAgent(wsURL(fake.srv)))
	fake.srv.Close()
	r := owner.do("POST", "/api/v1/workspaces/"+slug+"/agents/"+agent["id"].(string)+"/probe", map[string]any{"durationSeconds": 1})
	expectStatus(t, r, http.StatusOK)
	if r.body["ok"] != false || !strings.Contains(eventKinds(r), "error") {
		t.Fatalf("probe = %v", r.body)
	}

	locked := newEnvWith(t, func(c *configOverride) { c.allowPrivate = false })
	lowner, lslug := ownerWorkspace(t, locked)
	expectStatus(t, lowner.do("POST", "/api/v1/workspaces/"+lslug+"/agents", viviLiveAgent("ws://127.0.0.1:9")), http.StatusUnprocessableEntity)
	a := createAgent(t, lowner, lslug, viviLiveAgent("wss://127.0.0.1:9"))
	r = lowner.do("POST", "/api/v1/workspaces/"+lslug+"/agents/"+a["id"].(string)+"/probe", map[string]any{"durationSeconds": 1})
	if r.body["ok"] != false || !strings.Contains(r.body["summary"].(string), "private network") {
		t.Fatalf("locked probe = %v", r.body)
	}
}

func TestTextProbeHonorsDuration(t *testing.T) {
	e := newEnv(t)
	owner, slug := ownerWorkspace(t, e)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-time.After(5 * time.Second):
		case <-r.Context().Done():
		}
	}))
	t.Cleanup(srv.Close)
	body := map[string]any{
		"name": "Slow", "description": "", "modality": "text",
		"transport": map[string]any{"url": srv.URL + "/chat", "query": []any{}, "headers": []any{},
			"body": `{"message":"{{message}}"}`, "initMessages": []any{}, "preSession": nil},
		"codec": map[string]any{"type": "http_json", "options": map[string]any{}}, "variables": []any{}, "secrets": []any{},
		"traceConnectionId": nil, "traceSessionId": "",
	}
	agent := createAgent(t, owner, slug, body)
	start := time.Now()
	r := owner.do("POST", "/api/v1/workspaces/"+slug+"/agents/"+agent["id"].(string)+"/probe", map[string]any{"durationSeconds": 1})
	if r.body["ok"] != false || time.Since(start) > 3*time.Second {
		t.Fatalf("probe took %s: %v", time.Since(start), r.body)
	}
}

// echoAgent is a Vivi Live agent that repeats the key it received in transcripts,
// errors and its URL-escaped form, to check every output is redacted.
func TestProbeRedactsEchoedSecretsEverywhere(t *testing.T) {
	e := newEnv(t)
	owner, slug := ownerWorkspace(t, e)
	secret := "eyJhbGciOiJIUzI1NiJ9/QQQQQQQQQQ+QQQQQQQQQQQQQQQQQQQQQQQQQQQQQQ=="
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		defer c.CloseNow()
		key := r.Header.Get("X-Key")
		send := func(v any) { b, _ := json.Marshal(v); _ = c.Write(r.Context(), websocket.MessageText, b) }
		send(map[string]any{"type": "voice.ready"})
		send(map[string]any{"type": "voice.transcript.update", "role": "assistant", "messageId": "m1", "text": "your key is " + key})
		send(map[string]any{"type": "voice.error", "message": strings.Repeat("x", 250) + " key " + key})
		send(map[string]any{"type": "debug", "path": r.URL.EscapedPath()})
		for {
			if _, _, err := c.Read(r.Context()); err != nil {
				return
			}
		}
	}))
	t.Cleanup(srv.Close)

	body := viviLiveAgent(wsURL(srv))
	body["transport"].(map[string]any)["url"] = wsURL(srv) + "/live/{{secret.api_key}}"
	body["transport"].(map[string]any)["headers"] = []any{map[string]any{"key": "X-Key", "value": "{{secret.api_key}}"}}
	body["secrets"] = []any{map[string]any{"name": "api_key", "value": secret}}
	agent := createAgent(t, owner, slug, body)
	r := owner.do("POST", "/api/v1/workspaces/"+slug+"/agents/"+agent["id"].(string)+"/probe", map[string]any{"durationSeconds": 1})
	raw, _ := json.Marshal(r.body)
	for _, leak := range []string{secret[:16], strings.ReplaceAll(secret[:24], "/", "%2F")} {
		if strings.Contains(string(raw), leak) {
			t.Fatalf("probe result contains %q: %s", leak, raw)
		}
	}
}

func TestSecretReachingANewHostRequiresReentry(t *testing.T) {
	e := newEnv(t)
	owner, slug := ownerWorkspace(t, e)
	body := viviLiveAgent("wss://agents.example.com")
	item := "/api/v1/workspaces/" + slug + "/agents/" + createAgent(t, owner, slug, body)["id"].(string)
	keep := []any{map[string]any{"name": "api_key", "value": ""}}

	// Same host, new path: the secret still only reaches agents.example.com.
	samePath := clone(body)
	samePath["secrets"] = keep
	samePath["transport"].(map[string]any)["url"] = "wss://agents.example.com/v2/{{var.channelId}}"
	expectStatus(t, owner.do("PUT", item, samePath), http.StatusOK)

	// Moving the secret reference into a pre-session on another host.
	moved := clone(samePath)
	moved["transport"].(map[string]any)["headers"] = []any{}
	moved["transport"].(map[string]any)["preSession"] = map[string]any{
		"method": "POST", "url": "https://collector.example.net/s", "body": nil, "captures": []any{},
		"headers": []any{map[string]any{"key": "X-Key", "value": "{{secret.api_key}}"}},
	}
	r := owner.do("PUT", item, moved)
	expectStatus(t, r, http.StatusUnprocessableEntity)
	if r.body["error"].(map[string]any)["fields"].(map[string]any)["secrets"] == nil {
		t.Fatalf("want secrets error, got %v", r.body)
	}
}
