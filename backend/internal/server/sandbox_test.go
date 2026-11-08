package server_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"testforge/internal/run"
)

// toolLLM is a fake OpenAI Chat Completions server. Each step answers one
// request: text, tool calls, or an HTTP error. It records every request body.
type toolLLM struct {
	srv    *httptest.Server
	mu     sync.Mutex
	steps  []llmStep
	bodies []map[string]any
}

type llmStep struct {
	text   string
	calls  [][2]string // name, arguments
	status int
}

func newToolLLM(t *testing.T, steps ...llmStep) *toolLLM {
	t.Helper()
	f := &toolLLM{steps: steps}
	f.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		var body map[string]any
		_ = json.Unmarshal(raw, &body)
		f.mu.Lock()
		st := f.steps[min(len(f.bodies), len(f.steps)-1)]
		f.bodies = append(f.bodies, body)
		f.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		if st.status != 0 {
			w.WriteHeader(st.status)
			_, _ = w.Write([]byte(`{"error":{"message":"model overloaded for key sk-tool-1234"}}`))
			return
		}
		msg := map[string]any{"role": "assistant", "content": st.text}
		finish := "stop"
		if len(st.calls) > 0 {
			calls := []any{}
			for i, c := range st.calls {
				calls = append(calls, map[string]any{"id": "call_" + string(rune('a'+i)), "type": "function",
					"function": map[string]any{"name": c[0], "arguments": c[1]}})
			}
			msg["tool_calls"], msg["content"], finish = calls, nil, "tool_calls"
		}
		b, _ := json.Marshal(map[string]any{
			"choices": []any{map[string]any{"message": msg, "finish_reason": finish}},
			"usage":   map[string]any{"prompt_tokens": 50, "completion_tokens": 10},
		})
		_, _ = w.Write(b)
	}))
	t.Cleanup(f.srv.Close)
	return f
}

func (f *toolLLM) body(t *testing.T, i int) string {
	t.Helper()
	f.mu.Lock()
	defer f.mu.Unlock()
	if i >= len(f.bodies) {
		t.Fatalf("request %d not sent (%d requests)", i, len(f.bodies))
	}
	b, _ := json.Marshal(f.bodies[i])
	return string(b)
}

func (f *toolLLM) count() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.bodies)
}

type sandboxFixture struct {
	*scenarioFixture
	fake *toolLLM
	conn map[string]any
}

func newSandboxFixture(t *testing.T, steps ...llmStep) *sandboxFixture {
	t.Helper()
	f := newScenarioFixture(t)
	l := newToolLLM(t, steps...)
	conn := createConnection(t, f.owner, f.slug, map[string]any{"name": "OpenAI tools", "kind": "openai", "secret": "sk-tool-1234",
		"config": map[string]any{"baseUrl": l.srv.URL + "/v1"}})
	return &sandboxFixture{scenarioFixture: f, fake: l, conn: conn}
}

func orderTool() map[string]any {
	return map[string]any{
		"name": "get_order", "description": "Look up an order by id.",
		"parameters": map[string]any{"type": "object", "required": []any{"orderId"},
			"properties": map[string]any{"orderId": map[string]any{"type": "string"}}},
		"response":  `{"orderId":"{{args.orderId}}","status":"shipped","items":{{args.items}}}`,
		"latencyMs": 30, "errorRate": 0, "errorMessage": "",
	}
}

func (f *sandboxFixture) agentBody(name string, memory map[string]any, tools ...map[string]any) map[string]any {
	ts := []any{}
	for _, t := range tools {
		ts = append(ts, t)
	}
	return map[string]any{
		"name": name, "description": "Order support.", "modality": "text", "connectionId": f.conn["id"],
		"model": "gpt-5-mini", "systemPrompt": "You help customers track orders.", "temperature": 0.2,
		"memory": memory, "tools": ts,
	}
}

func (f *sandboxFixture) createSandbox(t *testing.T, body map[string]any) map[string]any {
	t.Helper()
	r := f.owner.do("POST", f.path("/sandbox-agents"), body)
	expectStatus(t, r, http.StatusCreated)
	return r.body
}

func noMemory() map[string]any { return map[string]any{"conversation": false, "longTerm": false} }

func TestSandboxAgentCRUDAndValidation(t *testing.T) {
	f := newSandboxFixture(t, llmStep{text: "hi"})
	a := f.createSandbox(t, f.agentBody("Orders bot", noMemory(), orderTool()))
	if a["id"] == nil || a["connectionId"] != f.conn["id"] || len(a["tools"].([]any)) != 1 || a["temperature"] != 0.2 {
		t.Fatalf("created = %v", a)
	}
	item := f.path("/sandbox-agents/" + a["id"].(string))

	r := f.owner.do("GET", f.path("/sandbox-agents"), nil)
	expectStatus(t, r, http.StatusOK)
	if items := r.body["items"].([]any); len(items) != 1 {
		t.Fatalf("list = %v", items)
	}

	// Name conflict is case-insensitive.
	r = f.owner.do("POST", f.path("/sandbox-agents"), f.agentBody("ORDERS BOT", noMemory()))
	expectStatus(t, r, http.StatusConflict)

	cases := []struct {
		name   string
		mutate func(b map[string]any)
		field  string
		msg    string
	}{
		{"voice", func(b map[string]any) { b["modality"] = "voice" }, "modality", "Voice sandbox agents are not available yet."},
		{"anthropic", func(b map[string]any) { b["connectionId"] = f.llm["id"] }, "connectionId", "Sandbox agents support OpenAI and Azure OpenAI connections."},
		{"foreign connection", func(b map[string]any) { b["connectionId"] = "00000000-0000-0000-0000-000000000000" }, "connectionId", "Choose a connection of this workspace."},
		{"empty prompt", func(b map[string]any) { b["systemPrompt"] = "  " }, "systemPrompt", "Required."},
		{"temperature", func(b map[string]any) { b["temperature"] = 3 }, "temperature", "Use a value from 0 to 2."},
		{"reserved tool", func(b map[string]any) {
			t := orderTool()
			t["name"] = "recall"
			b["tools"] = []any{t}
		}, "tools.0.name", `"recall" is reserved for long-term memory.`},
		{"bad tool name", func(b map[string]any) {
			t := orderTool()
			t["name"] = "get order"
			b["tools"] = []any{t}
		}, "tools.0.name", "Use 1 to 64 letters, digits, _ or -."},
		{"schema type", func(b map[string]any) {
			t := orderTool()
			t["parameters"] = map[string]any{"type": "string"}
			b["tools"] = []any{t}
		}, "tools.0.parameters", `The schema's "type" must be "object".`},
		{"duplicate tool", func(b map[string]any) { b["tools"] = []any{orderTool(), orderTool()} }, "tools.1.name", "Another tool already uses this name."},
		{"error rate", func(b map[string]any) {
			t := orderTool()
			t["errorRate"] = 1.5
			b["tools"] = []any{t}
		}, "tools.0.errorRate", "Use a value from 0 to 1."},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			b := f.agentBody("Other "+c.name, noMemory())
			c.mutate(b)
			r := f.owner.do("POST", f.path("/sandbox-agents"), b)
			expectStatus(t, r, http.StatusUnprocessableEntity)
			fields := r.body["error"].(map[string]any)["fields"].(map[string]any)
			if fields[c.field] != c.msg {
				t.Fatalf("fields = %v, want %s=%q", fields, c.field, c.msg)
			}
		})
	}

	upd := f.agentBody("Orders bot v2", map[string]any{"conversation": true, "longTerm": true})
	r = f.owner.do("PUT", item, upd)
	expectStatus(t, r, http.StatusOK)
	if r.body["name"] != "Orders bot v2" || len(r.body["tools"].([]any)) != 0 || r.body["memory"].(map[string]any)["longTerm"] != true {
		t.Fatalf("updated = %v", r.body)
	}

	viewer := addMember(t, f.e, f.slug, "viewer")
	expectStatus(t, viewer.do("GET", item, nil), http.StatusOK)
	expectStatus(t, viewer.do("PUT", item, upd), http.StatusForbidden)
	expectStatus(t, viewer.do("POST", item+"/chat", map[string]any{"message": "hi"}), http.StatusForbidden)

	// Other workspaces cannot see it.
	other := f.e.browser()
	otherWS := firstWorkspace(t, signup(t, other, "Eve", "eve@example.com"))
	expectStatus(t, other.do("GET", "/api/v1/workspaces/"+otherWS["slug"].(string)+"/sandbox-agents/"+a["id"].(string), nil), http.StatusNotFound)

	expectStatus(t, f.owner.do("DELETE", item, nil), http.StatusNoContent)
	expectStatus(t, f.owner.do("GET", item, nil), http.StatusNotFound)
}

func TestSandboxChatRunsToolsAndKeepsConversation(t *testing.T) {
	f := newSandboxFixture(t,
		llmStep{calls: [][2]string{{"get_order", `{"orderId":"A1","items":["mug",2]}`}}},
		llmStep{text: "Your order A1 has shipped."},
		llmStep{text: "You asked about order A1."},
	)
	broken := orderTool()
	broken["name"], broken["errorRate"], broken["errorMessage"] = "refund", 1, "Refund service down."
	a := f.createSandbox(t, f.agentBody("Orders bot", map[string]any{"conversation": true, "longTerm": false}, orderTool(), broken))
	chat := f.path("/sandbox-agents/" + a["id"].(string) + "/chat")

	r := f.owner.do("POST", chat, map[string]any{"message": "Where is order A1?"})
	expectStatus(t, r, http.StatusOK)
	if r.body["message"] != "Your order A1 has shipped." {
		t.Fatalf("reply = %v", r.body)
	}
	thread, _ := r.body["threadId"].(string)
	if !strings.HasPrefix(thread, "sbx-") {
		t.Fatalf("threadId = %q", thread)
	}
	calls := r.body["toolCalls"].([]any)
	if len(calls) != 1 {
		t.Fatalf("toolCalls = %v", calls)
	}
	c := calls[0].(map[string]any)
	if c["name"] != "get_order" || c["error"] != false || c["result"] != `{"orderId":"A1","status":"shipped","items":["mug",2]}` || c["latencyMs"].(float64) < 30 {
		t.Fatalf("tool call = %v", c)
	}
	if u := r.body["usage"].(map[string]any); u["inputTokens"].(float64) != 100 || u["outputTokens"].(float64) != 20 {
		t.Fatalf("usage = %v", u)
	}
	first := f.fake.body(t, 0)
	for _, want := range []string{`"model":"gpt-5-mini"`, `"temperature":0.2`, `"name":"get_order"`, `"name":"refund"`, "You help customers track orders."} {
		if !strings.Contains(first, want) {
			t.Errorf("first request misses %s: %s", want, first)
		}
	}
	if strings.Contains(first, `"name":"remember"`) {
		t.Errorf("memory tools offered without long-term memory: %s", first)
	}
	second := f.fake.body(t, 1)
	if !strings.Contains(second, `"tool_call_id":"call_a"`) || !strings.Contains(second, `\"status\":\"shipped\"`) {
		t.Errorf("tool result not sent back: %s", second)
	}

	// Same thread: the stored conversation, tool exchange included, comes back.
	r = f.owner.do("POST", chat, map[string]any{"message": "What did I ask?", "threadId": thread})
	expectStatus(t, r, http.StatusOK)
	third := f.fake.body(t, 2)
	for _, want := range []string{"Where is order A1?", "Your order A1 has shipped.", `"tool_call_id":"call_a"`, "What did I ask?"} {
		if !strings.Contains(third, want) {
			t.Errorf("history misses %q: %s", want, third)
		}
	}
	// A new thread starts empty.
	r = f.owner.do("POST", chat, map[string]any{"message": "Hello"})
	expectStatus(t, r, http.StatusOK)
	if fourth := f.fake.body(t, 3); strings.Contains(fourth, "Where is order A1?") || r.body["threadId"] == thread {
		t.Errorf("new thread carried history: %s", fourth)
	}

	expectStatus(t, f.owner.do("POST", chat, map[string]any{"message": " "}), http.StatusUnprocessableEntity)
}

func TestSandboxFaultInjectionAndLimits(t *testing.T) {
	broken := orderTool()
	broken["name"], broken["errorRate"], broken["errorMessage"], broken["latencyMs"] = "refund", 1, "Refund service down.", 0

	t.Run("failing tool", func(t *testing.T) {
		f := newSandboxFixture(t, llmStep{calls: [][2]string{{"refund", `{}`}, {"nope", `{}`}, {"get_order", `not json`}}}, llmStep{text: "Sorry."})
		a := f.createSandbox(t, f.agentBody("Bot", noMemory(), broken, orderTool()))
		r := f.owner.do("POST", f.path("/sandbox-agents/"+a["id"].(string)+"/chat"), map[string]any{"message": "refund"})
		expectStatus(t, r, http.StatusOK)
		calls := r.body["toolCalls"].([]any)
		want := []string{`{"error":"Refund service down."}`, `{"error":"Unknown tool \"nope\"."}`, `{"error":"The arguments are not valid JSON."}`}
		for i, w := range want {
			c := calls[i].(map[string]any)
			if c["result"] != w || c["error"] != true {
				t.Errorf("call %d = %v, want %s", i, c, w)
			}
		}
		// Without conversation memory nothing is stored.
		var n int
		_ = f.e.pool.QueryRow(t.Context(), `SELECT count(*) FROM sandbox_messages`).Scan(&n)
		if n != 0 {
			t.Errorf("stored %d messages without conversation memory", n)
		}
	})

	t.Run("too many rounds", func(t *testing.T) {
		f := newSandboxFixture(t, llmStep{calls: [][2]string{{"get_order", `{"orderId":"A1"}`}}})
		a := f.createSandbox(t, f.agentBody("Loop", noMemory(), orderTool()))
		r := f.owner.do("POST", f.path("/sandbox-agents/"+a["id"].(string)+"/chat"), map[string]any{"message": "loop"})
		expectStatus(t, r, http.StatusBadGateway)
		expectErrorCode(t, r, "upstream_error")
		if msg := r.body["error"].(map[string]any)["message"]; msg != "The agent called tools too many times without answering." || f.fake.count() != 8 {
			t.Fatalf("message = %v, requests = %d", msg, f.fake.count())
		}
	})

	t.Run("provider error", func(t *testing.T) {
		f := newSandboxFixture(t, llmStep{status: http.StatusServiceUnavailable})
		a := f.createSandbox(t, f.agentBody("Down", noMemory()))
		r := f.owner.do("POST", f.path("/sandbox-agents/"+a["id"].(string)+"/chat"), map[string]any{"message": "hi"})
		expectStatus(t, r, http.StatusBadGateway)
		msg := r.body["error"].(map[string]any)["message"].(string)
		if !strings.Contains(msg, "HTTP 503") || strings.Contains(msg, "sk-tool-1234") {
			t.Fatalf("message = %q", msg)
		}
	})
}

func TestSandboxLongTermMemory(t *testing.T) {
	f := newSandboxFixture(t,
		llmStep{calls: [][2]string{{"remember", `{"fact":"Prefers email updates"}`}}},
		llmStep{text: "Noted."},
		llmStep{calls: [][2]string{{"recall", `{"query":"email preferences"}`}}},
		llmStep{text: "You prefer email."},
		llmStep{calls: [][2]string{{"recall", `{"query":""}`}}},
		llmStep{text: "I know nothing about you."},
	)
	a := f.createSandbox(t, f.agentBody("Memo", map[string]any{"conversation": false, "longTerm": true}))
	id := a["id"].(string)
	chat := f.path("/sandbox-agents/" + id + "/chat")

	r := f.owner.do("POST", chat, map[string]any{"message": "Email me updates", "userId": "ana"})
	expectStatus(t, r, http.StatusOK)
	if c := r.body["toolCalls"].([]any)[0].(map[string]any); c["result"] != `{"saved":true}` {
		t.Fatalf("remember = %v", c)
	}
	if !strings.Contains(f.fake.body(t, 0), `"name":"remember"`) {
		t.Fatalf("memory tools not offered: %s", f.fake.body(t, 0))
	}
	// Another thread, same user: recall finds it.
	r = f.owner.do("POST", chat, map[string]any{"message": "How do I like updates?", "userId": "ana"})
	expectStatus(t, r, http.StatusOK)
	if c := r.body["toolCalls"].([]any)[0].(map[string]any); c["result"] != `{"memories":["Prefers email updates"]}` {
		t.Fatalf("recall = %v", c)
	}
	// Another user sees nothing.
	r = f.owner.do("POST", chat, map[string]any{"message": "What do you know?", "userId": "bob"})
	expectStatus(t, r, http.StatusOK)
	if c := r.body["toolCalls"].([]any)[0].(map[string]any); c["result"] != `{"memories":[]}` {
		t.Fatalf("recall for bob = %v", c)
	}

	r = f.owner.do("GET", f.path("/sandbox-agents/"+id+"/memories?userId=ana"), nil)
	expectStatus(t, r, http.StatusOK)
	items := r.body["items"].([]any)
	if len(items) != 1 || items[0].(map[string]any)["content"] != "Prefers email updates" || items[0].(map[string]any)["userId"] != "ana" {
		t.Fatalf("memories = %v", items)
	}
	r = f.owner.do("GET", f.path("/sandbox-agents/"+id+"/memories?userId=bob"), nil)
	if len(r.body["items"].([]any)) != 0 {
		t.Fatalf("bob memories = %v", r.body)
	}
	mem := f.path("/sandbox-agents/" + id + "/memories/" + items[0].(map[string]any)["id"].(string))
	expectStatus(t, f.owner.do("DELETE", mem, nil), http.StatusNoContent)
	expectStatus(t, f.owner.do("DELETE", mem, nil), http.StatusNotFound)
}

func TestSandboxGenerate(t *testing.T) {
	agentDraft := `{"name":"Clinic receptionist","description":"Books appointments.","systemPrompt":"Eres recepcionista.",
		"memory":{"conversation":true,"longTerm":false},
		"tools":[{"name":"book_slot","description":"Books a slot.","parametersJson":"{\"type\":\"object\",\"properties\":{\"date\":{\"type\":\"string\"}},\"required\":[\"date\"]}",
		"response":"{\"booked\":\"{{args.date}}\"}","latencyMs":0,"errorRate":0,"errorMessage":""}]}`
	toolDraft := `{"name":"cancel_slot","description":"Cancels a slot.","parametersJson":"{\"type\":\"object\",\"properties\":{}}",
		"response":"{\"cancelled\":true}","latencyMs":200,"errorRate":0.1,"errorMessage":"Busy"}`
	badDraft := `{"name":"x y","description":"","parametersJson":"{not json","response":"","latencyMs":0,"errorRate":0,"errorMessage":""}`
	f := newSandboxFixture(t, llmStep{text: agentDraft}, llmStep{text: toolDraft}, llmStep{text: badDraft})
	gen := f.path("/sandbox/generate")

	r := f.owner.do("POST", gen, map[string]any{"kind": "agent", "connectionId": f.conn["id"], "model": "gpt-5", "description": "Una recepcionista de clínica"})
	expectStatus(t, r, http.StatusOK)
	ag := r.body["agent"].(map[string]any)
	tool := ag["tools"].([]any)[0].(map[string]any)
	if ag["name"] != "Clinic receptionist" || r.body["tool"] != nil || tool["parameters"].(map[string]any)["type"] != "object" {
		t.Fatalf("agent draft = %v", r.body)
	}
	if req := f.fake.body(t, 0); !strings.Contains(req, `"strict":true`) || !strings.Contains(req, "Una recepcionista de clínica") {
		t.Errorf("generate request = %s", req)
	}
	// The draft saves as is.
	body := f.agentBody("ignored", noMemory())
	for _, k := range []string{"name", "description", "systemPrompt", "memory", "tools"} {
		body[k] = ag[k]
	}
	f.createSandbox(t, body)

	r = f.owner.do("POST", gen, map[string]any{"kind": "tool", "connectionId": f.conn["id"], "model": "gpt-5", "description": "Cancel a slot",
		"agent": map[string]any{"name": "Clinic", "description": "", "systemPrompt": "Eres recepcionista.", "toolNames": []any{"book_slot"}}})
	expectStatus(t, r, http.StatusOK)
	if tl := r.body["tool"].(map[string]any); tl["name"] != "cancel_slot" || tl["errorRate"] != 0.1 || r.body["agent"] != nil {
		t.Fatalf("tool draft = %v", r.body)
	}
	if req := f.fake.body(t, 1); !strings.Contains(req, "book_slot") {
		t.Errorf("tool request without agent context: %s", req)
	}

	r = f.owner.do("POST", gen, map[string]any{"kind": "tool", "connectionId": f.conn["id"], "model": "gpt-5", "description": "Broken"})
	expectStatus(t, r, http.StatusBadGateway)
	msg := r.body["error"].(map[string]any)["message"].(string)
	if !strings.HasPrefix(msg, "The model returned an invalid draft: ") || !strings.Contains(msg, "parameters: Not valid JSON.") || !strings.Contains(msg, "name: ") {
		t.Fatalf("message = %q", msg)
	}

	r = f.owner.do("POST", gen, map[string]any{"kind": "persona", "connectionId": f.speech["id"], "model": "", "description": ""})
	expectStatus(t, r, http.StatusUnprocessableEntity)
	fields := r.body["error"].(map[string]any)["fields"].(map[string]any)
	if fields["kind"] == nil || fields["model"] == nil || fields["description"] == nil {
		t.Fatalf("fields = %v", fields)
	}
	r = f.owner.do("POST", gen, map[string]any{"kind": "agent", "connectionId": f.speech["id"], "model": "m", "description": "d"})
	expectStatus(t, r, http.StatusUnprocessableEntity)
	if r.body["error"].(map[string]any)["fields"].(map[string]any)["connectionId"] != "Choose a connection with the llm capability." {
		t.Fatalf("speech connection = %v", r.body)
	}
}

func TestSandboxLinkedAgentRunsInScenario(t *testing.T) {
	f := newSandboxFixture(t,
		llmStep{calls: [][2]string{{"get_order", `{"orderId":"Z9"}`}}},
		llmStep{text: "Order Z9 shipped yesterday."},
	)
	a := f.createSandbox(t, f.agentBody("Orders bot", map[string]any{"conversation": true, "longTerm": false}, orderTool()))
	link := f.path("/sandbox-agents/" + a["id"].(string) + "/link")

	member := addMember(t, f.e, f.slug, "member")
	expectStatus(t, member.do("POST", link, nil), http.StatusForbidden)
	key := createKey(t, f.owner, f.slug)
	expectStatus(t, f.e.apiClient(key["secret"].(string)).do("POST", link, nil), http.StatusForbidden)

	r := f.owner.do("POST", link, nil)
	expectStatus(t, r, http.StatusCreated)
	ag := r.body
	tr := ag["transport"].(map[string]any)
	wantURL := f.e.cfg.PublicAPIURL + "/api/v1/workspaces/" + f.slug + "/sandbox-agents/" + a["id"].(string) + "/chat"
	if ag["name"] != "Orders bot (sandbox)" || tr["url"] != wantURL || ag["codec"].(map[string]any)["type"] != "http_json" {
		t.Fatalf("linked agent = %v", ag)
	}
	if s := ag["secrets"].([]any); len(s) != 1 || s[0].(map[string]any)["name"] != "api_key" {
		t.Fatalf("secrets = %v", s)
	}
	keys := f.owner.do("GET", f.path("/api-keys"), nil).body["items"].([]any)
	found := false
	for _, k := range keys {
		found = found || k.(map[string]any)["name"] == "Sandbox: Orders bot"
	}
	if !found {
		t.Fatalf("no sandbox API key in %v", keys)
	}
	// Linking again picks the next free name.
	r = f.owner.do("POST", link, nil)
	expectStatus(t, r, http.StatusCreated)
	if r.body["name"] != "Orders bot (sandbox) 2" {
		t.Fatalf("second link name = %v", r.body["name"])
	}

	// A scenario on the linked agent runs end to end through the chat endpoint.
	sim := newFakeLLM(t, decision("¿Dónde está mi pedido Z9?", false), decision("Gracias, adiós.", true))
	simConn := createConnection(t, f.owner, f.slug, map[string]any{"name": "Sim", "kind": "openai", "secret": "sk-sim-1234",
		"config": map[string]any{"baseUrl": sim.srv.URL + "/v1"}})
	body := f.scenario(ag, "Track order")
	body["simulator"] = map[string]any{"mode": "text", "connectionId": simConn["id"], "model": "sim-model"}
	body["endConditions"] = map[string]any{"maxDurationSeconds": 60, "maxSilenceSeconds": 15, "maxTurns": 6, "hangUpWhen": "goal_or_goodbye"}
	sc := f.create(t, body)
	rf := &runFixture{scenarioFixture: f.scenarioFixture}
	startWorker(t, f.e, &run.TextEngine{AllowPrivate: true}, 1)
	batch := rf.launch(t, map[string]any{"scenarioIds": []any{sc["id"]}})
	got := rf.waitStatus(t, rf.runIDs(t, batch["id"].(string))[0], "completed", "failed")
	if got["status"] != "completed" {
		t.Fatalf("run = %v", got)
	}
	turns := got["transcript"].([]any)
	if a := turns[1].(map[string]any); a["role"] != "agent" || a["text"] != "Order Z9 shipped yesterday." {
		t.Fatalf("agent turn = %v (transcript %v)", a, turns)
	}
	// The run's thread became the sandbox conversation thread, with userId from the variable.
	var thread string
	if err := f.e.pool.QueryRow(t.Context(), `SELECT thread_id FROM sandbox_messages LIMIT 1`).Scan(&thread); err != nil || thread == "" {
		t.Fatalf("no stored conversation: %v", err)
	}
}
