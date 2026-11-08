package server_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"testforge/internal/run"
)

// fakeLangfuse serves a session whose observation count grows, then settles.
func fakeLangfuse(t *testing.T, calls *atomic.Int32) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/public/projects":
			_, _ = w.Write([]byte(`{"data":[{"id":"proj-1"}]}`))
		case "/api/public/v2/observations":
			n := calls.Add(1)
			obs := []any{map[string]any{"id": "o1", "traceId": "t1", "type": "GENERATION", "name": "llm", "model": "gpt-agent",
				"startTime": "2026-11-07T10:00:00.000Z", "endTime": "2026-11-07T10:00:01.000Z", "usageDetails": map[string]any{"total": 50}}}
			if n > 1 {
				obs = append(obs, map[string]any{"id": "o2", "traceId": "t1", "type": "TOOL", "name": "hours_lookup",
					"startTime": "2026-11-07T10:00:00.200Z", "endTime": "2026-11-07T10:00:00.300Z"})
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"data": obs, "meta": map[string]any{}})
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

func judgeAnswer(criteria []any, bands ...int) string {
	var answers []any
	for i, c := range criteria {
		id := c.(map[string]any)["id"].(string)
		answers = append(answers, map[string]any{"criterionId": id, "observable": true, "band": bands[i], "failureTags": []any{},
			"reasoning": "The agent did it.", "evidence": []any{map[string]any{"source": "transcript", "ref": "A1", "quote": "¿En qué te ayudo?"}}})
	}
	b, _ := json.Marshal(map[string]any{"summary": "Good call.", "criteria": answers, "strengths": []any{"Polite"},
		"suggestions": []any{map[string]any{"title": "Be faster", "detail": "Cut the greeting.", "area": "latency", "priority": "medium", "evidenceRefs": []any{"T1"}}}})
	return string(b)
}

func judgedScenario(t *testing.T, f *runFixture, withTraces bool, judgeLLM *fakeLLM) map[string]any {
	t.Helper()
	agentBody := map[string]any{
		"name": "Judged chat", "description": "Clinic assistant.", "modality": "text",
		"transport": map[string]any{"url": "https://chat.example.com/c", "query": []any{}, "headers": []any{},
			"body": `{"message":"{{message}}"}`, "initMessages": []any{}, "preSession": nil},
		"codec": map[string]any{"type": "http_json", "options": map[string]any{}}, "variables": []any{}, "secrets": []any{},
		"traceConnectionId": nil, "traceSessionId": "",
	}
	criteria := []any{map[string]any{"id": nil, "text": "Gives the opening hours", "kind": "must", "source": "transcript", "weight": 70}}
	if withTraces {
		var calls atomic.Int32
		lf := fakeLangfuse(t, &calls)
		conn := createConnection(t, f.owner, f.slug, map[string]any{"name": "LF fake", "kind": "langfuse", "secret": "sk-lf-x",
			"config": map[string]any{"host": lf.URL, "publicKey": "pk-lf-x"}})
		agentBody["traceConnectionId"] = conn["id"]
		criteria = append(criteria, map[string]any{"id": nil, "text": "Looks up the hours with the tool", "kind": "should", "source": "trace", "weight": 30})
	} else {
		criteria[0].(map[string]any)["weight"] = 100
	}
	ag := createAgent(t, f.owner, f.slug, agentBody)
	judgeConn := createConnection(t, f.owner, f.slug, map[string]any{"name": "Judge", "kind": "openai", "secret": "sk-judge",
		"config": map[string]any{"baseUrl": judgeLLM.srv.URL + "/v1"}})
	body := f.scenario(ag, "Judged hours")
	body["criteria"] = criteria
	body["simulator"] = map[string]any{"mode": "text", "connectionId": f.llm["id"], "model": "sim"}
	body["judge"] = map[string]any{"connectionId": judgeConn["id"], "model": "judge-model"}
	return f.create(t, body)
}

func waitJudgement(t *testing.T, f *runFixture, runID string, want ...string) (map[string]any, []string) {
	t.Helper()
	var seen []string
	deadline := time.Now().Add(20 * time.Second)
	for {
		r := f.owner.do("GET", f.path("/runs/"+runID+"/judgement"), nil)
		if r.status == http.StatusOK {
			st := r.body["status"].(string)
			if len(seen) == 0 || seen[len(seen)-1] != st {
				seen = append(seen, st)
			}
			for _, w := range want {
				if st == w {
					return r.body, seen
				}
			}
		}
		if time.Now().After(deadline) {
			t.Fatalf("judgement statuses %v, want %v (last %v)", seen, want, r.body)
		}
		time.Sleep(100 * time.Millisecond)
	}
}

func TestJudgementWithTracesAndReports(t *testing.T) {
	f := newRunFixture(t)
	judgeLLM := newFakeLLM(t, "placeholder")
	sc := judgedScenario(t, f, true, judgeLLM)
	judgeLLM.decisions = []string{judgeAnswer(sc["criteria"].([]any), 5, 3), judgeAnswer(sc["criteria"].([]any), 4, 3), judgeAnswer(sc["criteria"].([]any), 5, 2)}

	startWorkerWith(t, f.e, &fakeEngine{}, 2, &run.JudgeOptions{Concurrency: 1, TraceWait: 10 * time.Second, TracePoll: 300 * time.Millisecond, AllowPrivate: true})
	batch := f.launch(t, map[string]any{"scenarioIds": []any{sc["id"]}})
	id := f.runIDs(t, batch["id"].(string))[0]
	runBody := f.waitStatus(t, id, "completed")
	j, seen := waitJudgement(t, f, id, "completed", "failed")

	if j["status"] != "completed" || j["model"] != "judge-model" {
		t.Fatalf("judgement = %v", j)
	}
	if !strings.Contains(strings.Join(seen, ","), "waiting_traces") {
		t.Fatalf("statuses %v never waited for traces", seen)
	}
	tr := j["traces"].(map[string]any)
	if tr["status"] != "available" || tr["observations"].(float64) != 2 || tr["toolCalls"].(float64) != 1 ||
		tr["url"] != tr["url"].(string) || !strings.HasSuffix(tr["url"].(string), "/project/proj-1/sessions/"+runBody["threadId"].(string)) {
		t.Fatalf("traces = %v", tr)
	}
	res := j["result"].(map[string]any)
	// c1 bands 5,4,5 → 5 (100); c2 bands 3,3,2 → 3 (70); score = 100*0.7 + 70*0.3 = 91.
	if res["score"].(float64) != 91 || res["passed"] != true || res["samples"].(float64) != 3 || len(res["suggestions"].([]any)) != 1 {
		t.Fatalf("result = %v", res)
	}
	// The judge saw the trace digest and the transcript, escaped in tagged blocks.
	msgs := judgeLLM.lastBody["messages"].([]any)
	prompt := msgs[len(msgs)-1].(map[string]any)["content"].(string)
	for _, want := range []string{"<traces>", "hours_lookup", "<transcript>", "Gives the opening hours"} {
		if !strings.Contains(prompt, want) {
			t.Errorf("judge request misses %q", want)
		}
	}

	sum := f.owner.do("GET", f.path("/runs?batchId="+batch["id"].(string)), nil).body["items"].([]any)[0].(map[string]any)
	if sum["judgementStatus"] != "completed" || sum["score"].(float64) != 91 || sum["passed"] != true {
		t.Fatalf("summary = %v", sum)
	}

	rep := f.owner.do("GET", f.path("/reports/overview?days=7"), nil)
	expectStatus(t, rep, http.StatusOK)
	o := rep.body
	if o["judged"].(float64) != 1 || o["avgScore"].(float64) != 91 || o["passRate"].(float64) != 1 || len(o["trend"].([]any)) != 7 {
		t.Fatalf("overview = %v", o)
	}
	weak := o["weakestCriteria"].([]any)
	if len(weak) != 2 || weak[0].(map[string]any)["text"] != "Looks up the hours with the tool" {
		t.Fatalf("weakest = %v", weak)
	}
	if rows := o["scenarios"].([]any); len(rows) != 1 || rows[0].(map[string]any)["scenarioName"] != "Judged hours" {
		t.Fatalf("scenarios = %v", rows)
	}
	expectStatus(t, f.owner.do("GET", f.path("/reports/overview?days=5"), nil), http.StatusUnprocessableEntity)

	// Re-judge queues attempt 2.
	again := f.owner.do("POST", f.path("/runs/"+id+"/judgement"), nil)
	expectStatus(t, again, http.StatusAccepted)
	if again.body["attempt"].(float64) != 2 || again.body["status"] != "queued" {
		t.Fatalf("rejudge = %v", again.body)
	}
	expectStatus(t, f.owner.do("POST", f.path("/runs/"+id+"/judgement"), nil), http.StatusConflict)
	j2, _ := waitJudgement(t, f, id, "completed", "failed")
	if j2["attempt"].(float64) != 2 || j2["status"] != "completed" {
		t.Fatalf("second judgement = %v", j2)
	}
	expectStatus(t, addMember(t, f.e, f.slug, "viewer").do("POST", f.path("/runs/"+id+"/judgement"), nil), http.StatusForbidden)
}

func TestJudgementSkippedWithoutJudgeAndGatesTraces(t *testing.T) {
	f := newRunFixture(t)
	startWorkerWith(t, f.e, &fakeEngine{}, 2, &run.JudgeOptions{Concurrency: 1, TraceWait: time.Second, TracePoll: 200 * time.Millisecond, AllowPrivate: true})

	batch := f.launch(t, map[string]any{"scenarioIds": []any{f.textScenario["id"]}})
	id := f.runIDs(t, batch["id"].(string))[0]
	f.waitStatus(t, id, "completed")
	j, _ := waitJudgement(t, f, id, "skipped")
	if !strings.Contains(j["error"].(string), "judge connection") {
		t.Fatalf("skipped judgement = %v", j)
	}

	// Without a trace connection, judging still works and traces are reported as not configured.
	judgeLLM := newFakeLLM(t, "placeholder")
	sc := judgedScenario(t, f, false, judgeLLM)
	judgeLLM.decisions = []string{judgeAnswer(sc["criteria"].([]any), 2)}
	b2 := f.launch(t, map[string]any{"scenarioIds": []any{sc["id"]}})
	id2 := f.runIDs(t, b2["id"].(string))[0]
	f.waitStatus(t, id2, "completed")
	j2, _ := waitJudgement(t, f, id2, "completed", "failed")
	res := j2["result"].(map[string]any)
	if j2["traces"].(map[string]any)["status"] != "not_configured" || res["passed"] != false || res["mustFailed"] != true || res["score"].(float64) != 50 {
		t.Fatalf("judgement = %v", j2)
	}

	// The scenario's current judge settings win: removing the judge makes a re-judge skip.
	f.e.pool.Exec(t.Context(), `UPDATE scenarios SET judge_connection_id = NULL, judge_model = '' WHERE id = $1`, sc["id"])
	expectStatus(t, f.owner.do("POST", f.path("/runs/"+id2+"/judgement"), nil), http.StatusAccepted)
	if j, _ := waitJudgement(t, f, id2, "skipped", "completed", "failed"); j["status"] != "skipped" {
		t.Fatalf("re-judge without a judge = %v", j)
	}

	// A run without a judgement row (completed before judging existed) uses the
	// scenario's current judge when re-judged.
	judgeConnID := sc["judge"].(map[string]any)["connectionId"]
	judgeLLM.decisions = []string{judgeAnswer(f.textScenario["criteria"].([]any), 5, 5)}
	f.e.pool.Exec(t.Context(), `DELETE FROM judgements WHERE run_id = $1`, id)
	f.e.pool.Exec(t.Context(), `UPDATE scenarios SET judge_connection_id = $2, judge_model = 'judge-model' WHERE id = $1`, f.textScenario["id"], judgeConnID)
	expectStatus(t, f.owner.do("POST", f.path("/runs/"+id+"/judgement"), nil), http.StatusAccepted)
	if j, _ := waitJudgement(t, f, id, "skipped", "completed", "failed"); j["status"] != "completed" {
		t.Fatalf("re-judge of a pre-judging run = %v", j)
	}

	// A run that did not complete cannot be judged.
	f.e.pool.Exec(t.Context(), `UPDATE runs SET status = 'failed' WHERE id = $1`, id2)
	expectStatus(t, f.owner.do("POST", f.path("/runs/"+id2+"/judgement"), nil), http.StatusConflict)
}
