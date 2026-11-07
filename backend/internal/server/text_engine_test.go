package server_test

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"testforge/internal/run"
)

// fakeLLM answers OpenAI chat completions and Anthropic messages with scripted decisions.
type fakeLLM struct {
	srv       *httptest.Server
	mu        sync.Mutex
	decisions []string
	calls     int
	lastBody  map[string]any
}

func newFakeLLM(t *testing.T, decisions ...string) *fakeLLM {
	t.Helper()
	f := &fakeLLM{decisions: decisions}
	f.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		f.mu.Lock()
		_ = json.Unmarshal(raw, &f.lastBody)
		d := f.decisions[min(f.calls, len(f.decisions)-1)]
		f.calls++
		f.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		if strings.HasSuffix(r.URL.Path, "/v1/messages") {
			b, _ := json.Marshal(map[string]any{
				"id": "msg_1", "type": "message", "role": "assistant", "model": "claude-sonnet-5-5",
				"content": []any{map[string]any{"type": "text", "text": d}}, "stop_reason": "end_turn",
				"usage": map[string]any{"input_tokens": 100, "output_tokens": 20},
			})
			_, _ = w.Write(b)
			return
		}
		b, _ := json.Marshal(map[string]any{
			"choices": []any{map[string]any{"message": map[string]any{"content": d}, "finish_reason": "stop"}},
			"usage":   map[string]any{"prompt_tokens": 100, "completion_tokens": 20},
		})
		_, _ = w.Write(b)
	}))
	t.Cleanup(f.srv.Close)
	return f
}

func decision(say string, hangUp bool) string {
	b, _ := json.Marshal(map[string]any{"say": say, "hangUp": hangUp, "reason": "test"})
	return string(b)
}

func fakeChatAgent(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]string
		_ = json.NewDecoder(r.Body).Decode(&body)
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"reply":"Echo: %s"}`, strings.ReplaceAll(body["message"], `"`, `'`))
	}))
	t.Cleanup(srv.Close)
	return srv
}

func textRunFixture(t *testing.T, llmKind string, llm *fakeLLM, maxTurns int) (*runFixture, map[string]any) {
	t.Helper()
	f := newRunFixture(t)
	agentSrv := fakeChatAgent(t)
	ag := createAgent(t, f.owner, f.slug, map[string]any{
		"name": "Echo chat", "description": "Clinic assistant.", "modality": "text",
		"transport": map[string]any{"url": agentSrv.URL + "/chat", "query": []any{}, "headers": []any{},
			"body": `{"message":"{{message}}","thread":"{{run.thread_id}}"}`, "initMessages": []any{}, "preSession": nil},
		"codec": map[string]any{"type": "http_json", "options": map[string]any{"replyPath": "reply"}}, "variables": []any{}, "secrets": []any{},
		"traceConnectionId": nil, "traceSessionId": "",
	})
	cfg := map[string]any{"baseUrl": llm.srv.URL}
	if llmKind == "openai" {
		cfg = map[string]any{"baseUrl": llm.srv.URL + "/v1"}
	}
	conn := createConnection(t, f.owner, f.slug, map[string]any{"name": "Sim " + llmKind, "kind": llmKind, "secret": "sk-sim-1234", "config": cfg})
	body := f.scenario(ag, "Ask opening hours")
	body["simulator"] = map[string]any{"mode": "text", "connectionId": conn["id"], "model": "sim-model"}
	body["endConditions"] = map[string]any{"maxDurationSeconds": 60, "maxSilenceSeconds": 15, "maxTurns": maxTurns, "hangUpWhen": "goal_or_goodbye"}
	return f, f.create(t, body)
}

func TestTextEngineConversation(t *testing.T) {
	for _, kind := range []string{"openai", "anthropic"} {
		t.Run(kind, func(t *testing.T) {
			llm := newFakeLLM(t, decision("¿A qué hora abren el domingo?", false), decision("Perfecto, gracias. Adiós.", true))
			f, sc := textRunFixture(t, kind, llm, 10)
			startWorker(t, f.e, &run.TextEngine{AllowPrivate: true}, 1)
			batch := f.launch(t, map[string]any{"scenarioIds": []any{sc["id"]}})
			r := f.waitStatus(t, f.runIDs(t, batch["id"].(string))[0], "completed", "failed")

			if r["status"] != "completed" || r["endReason"] != "user_hung_up" {
				t.Fatalf("run = %v", r)
			}
			tr := r["transcript"].([]any)
			if len(tr) != 3 {
				t.Fatalf("transcript = %v", tr)
			}
			if a := tr[1].(map[string]any); a["role"] != "agent" || a["text"] != "Echo: ¿A qué hora abren el domingo?" || a["latencyMs"] == nil {
				t.Fatalf("agent turn = %v", a)
			}
			m := r["metrics"].(map[string]any)
			if m["userTurns"].(float64) != 2 || m["agentTurns"].(float64) != 1 || m["llmInputTokens"].(float64) != 200 {
				t.Fatalf("metrics = %v", m)
			}

			// The simulator request carries the persona, the hidden facts and a strict JSON schema.
			raw, _ := json.Marshal(llm.lastBody)
			for _, want := range []string{"Impatient caller", "patientId: 4471", "hangUp", "Echo: "} {
				if !strings.Contains(string(raw), want) {
					t.Errorf("%s request misses %q: %s", kind, want, raw)
				}
			}
			if kind == "openai" && !strings.Contains(string(raw), `"strict":true`) {
				t.Errorf("openai request without strict schema: %s", raw)
			}
			if kind == "anthropic" && !strings.Contains(string(raw), `"output_config"`) {
				t.Errorf("anthropic request without output_config: %s", raw)
			}
		})
	}
}

func TestTextEngineLimitsAndFailures(t *testing.T) {
	llm := newFakeLLM(t, decision("Hola", false))
	f, sc := textRunFixture(t, "openai", llm, 2)
	startWorker(t, f.e, &run.TextEngine{AllowPrivate: true}, 1)
	batch := f.launch(t, map[string]any{"scenarioIds": []any{sc["id"]}})
	r := f.waitStatus(t, f.runIDs(t, batch["id"].(string))[0], "completed", "failed")
	if r["endReason"] != "max_turns" || len(r["transcript"].([]any)) != 4 {
		t.Fatalf("max turns run = %v", r)
	}

	broken := newFakeLLM(t, `not json`)
	f2, sc2 := textRunFixture(t, "openai", broken, 5)
	startWorker(t, f2.e, &run.TextEngine{AllowPrivate: true}, 1)
	batch2 := f2.launch(t, map[string]any{"scenarioIds": []any{sc2["id"]}})
	r2 := f2.waitStatus(t, f2.runIDs(t, batch2["id"].(string))[0], "completed", "failed")
	if r2["status"] != "failed" || !strings.Contains(r2["error"].(string), "Simulator error") {
		t.Fatalf("broken simulator run = %v", r2)
	}
}
