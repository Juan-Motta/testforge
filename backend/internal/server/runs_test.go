package server_test

import (
	"context"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"testforge/internal/agent"
	"testforge/internal/connection"
	"testforge/internal/platform/sealbox"
	"testforge/internal/run"
)

// fakeEngine records concurrency and either finishes at once or blocks until cancelled.
type fakeEngine struct {
	block   bool
	mu      sync.Mutex
	active  int
	maxSeen int
	jobs    []run.Job
}

func (f *fakeEngine) Run(ctx context.Context, job run.Job, rec run.Recorder) run.Outcome {
	f.mu.Lock()
	f.active++
	f.maxSeen = max(f.maxSeen, f.active)
	f.jobs = append(f.jobs, job)
	f.mu.Unlock()
	defer func() { f.mu.Lock(); f.active--; f.mu.Unlock() }()

	rec.Log("info", "fake engine started")
	turn := run.Turn{Role: "user", Text: "Hola", StartMs: 0, EndMs: 500}
	rec.Transcript(0, turn, true)
	if f.block {
		<-ctx.Done()
		return run.Outcome{Status: run.StatusFailed, EndReason: run.EndError, Error: "stopped"}
	}
	latency := 420
	out := run.Outcome{
		Status: run.StatusCompleted, EndReason: run.EndUserHungUp, TraceSessionID: job.ThreadID,
		Transcript: []run.Turn{turn, {Role: "agent", Text: "¿En qué te ayudo?", StartMs: 920, EndMs: 2000, LatencyMs: &latency}},
		Metrics:    run.Metrics{DurationMs: 2000, UserTurns: 1, AgentTurns: 1, LatencyP50Ms: &latency, LatencyP95Ms: &latency},
	}
	if job.Voice != nil {
		out.Audio = map[string][]byte{"conversation": []byte("RIFF....WAVEfake")}
	}
	return out
}

func startWorker(t *testing.T, e *env, engine run.Engine, concurrency int) {
	t.Helper()
	box, err := sealbox.New(e.cfg.SecretKey)
	if err != nil {
		t.Fatal(err)
	}
	conns := connection.NewService(e.pool, box, true)
	agents := agent.NewService(e.pool, box, true)
	w := run.NewWorker(e.pool, conns, agents, func(string) run.Engine { return engine }, run.LocalStorage{Dir: e.cfg.ArtifactsDir}, concurrency)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { w.Run(ctx); close(done) }()
	t.Cleanup(func() { cancel(); <-done })
}

type runFixture struct {
	*scenarioFixture
	textScenario  map[string]any
	voiceScenario map[string]any
}

func newRunFixture(t *testing.T) *runFixture {
	t.Helper()
	f := newScenarioFixture(t)
	sim := map[string]any{"mode": "", "connectionId": f.llm["id"], "model": "claude-sonnet-5-5"}
	text := f.scenario(f.text, "Opening hours")
	text["simulator"] = sim
	voice := f.scenario(f.realtime, "Reschedule")
	voice["simulator"] = sim
	tts := createConnection(t, f.owner, f.slug, map[string]any{"name": "Voices", "kind": "openai", "secret": "sk-openai-1", "config": map[string]any{}})
	vp := personaBody("Voiced caller")
	vp["voice"] = map[string]any{"connectionId": tts["id"], "voice": "alloy"}
	r := f.owner.do("POST", f.path("/personas"), vp)
	expectStatus(t, r, http.StatusCreated)
	voice["personaId"] = r.body["id"]
	return &runFixture{scenarioFixture: f, textScenario: f.create(t, text), voiceScenario: f.create(t, voice)}
}

func (f *runFixture) launch(t *testing.T, body map[string]any) map[string]any {
	t.Helper()
	r := f.owner.do("POST", f.path("/runs"), body)
	expectStatus(t, r, http.StatusCreated)
	return r.body
}

func (f *runFixture) waitStatus(t *testing.T, runID string, want ...string) map[string]any {
	t.Helper()
	deadline := time.Now().Add(15 * time.Second)
	for {
		r := f.owner.do("GET", f.path("/runs/"+runID), nil)
		for _, w := range want {
			if r.body["status"] == w {
				return r.body
			}
		}
		if time.Now().After(deadline) {
			t.Fatalf("run %s status %v, want %v", runID, r.body["status"], want)
		}
		time.Sleep(100 * time.Millisecond)
	}
}

func (f *runFixture) runIDs(t *testing.T, batchID string) []string {
	t.Helper()
	r := f.owner.do("GET", f.path("/runs?batchId="+batchID), nil)
	expectStatus(t, r, http.StatusOK)
	var ids []string
	for _, it := range r.body["items"].([]any) {
		ids = append(ids, it.(map[string]any)["id"].(string))
	}
	return ids
}

func TestLaunchValidation(t *testing.T) {
	f := newRunFixture(t)
	noSim := f.create(t, f.scenario(f.text, "No simulator"))
	rt := f.scenario(f.realtime, "Realtime mode")
	rt["simulator"] = map[string]any{"mode": "realtime", "connectionId": nil, "model": ""}
	realtime := f.create(t, rt)
	silent := f.scenario(f.realtime, "Silent persona")
	silent["simulator"] = map[string]any{"mode": "pipeline", "connectionId": f.llm["id"], "model": "m"}
	noVoice := f.create(t, silent)

	r := f.owner.do("POST", f.path("/runs"), map[string]any{
		"scenarioIds": []any{f.textScenario["id"], noSim["id"], realtime["id"], noVoice["id"], "00000000-0000-0000-0000-000000000000"},
		"repetitions": 11,
	})
	fields := fieldErrors(t, r)
	for k, want := range map[string]string{
		"scenarioIds.1": "simulator connection", "scenarioIds.2": "realtime", "scenarioIds.3": "voice",
		"scenarioIds.4": "not found", "repetitions": "1 to 10",
	} {
		if msg, _ := fields[k].(string); !strings.Contains(msg, want) {
			t.Errorf("%s = %q, want it to mention %q", k, msg, want)
		}
	}
	if fields["scenarioIds.0"] != nil {
		t.Errorf("valid scenario flagged: %v", fields["scenarioIds.0"])
	}
	expectStatus(t, addMember(t, f.e, f.slug, "viewer").do("POST", f.path("/runs"), map[string]any{"scenarioIds": []any{f.textScenario["id"]}}), http.StatusForbidden)
}

func TestRunsExecuteAndStream(t *testing.T) {
	f := newRunFixture(t)
	batch := f.launch(t, map[string]any{"scenarioIds": []any{f.textScenario["id"], f.voiceScenario["id"]}, "repetitions": 2, "label": "Nightly"})
	if batch["total"].(float64) != 4 || batch["counts"].(map[string]any)["queued"].(float64) != 4 || batch["label"] != "Nightly" {
		t.Fatalf("batch = %v", batch)
	}
	list := f.owner.do("GET", f.path("/runs?batchId="+batch["id"].(string)), nil).body["items"].([]any)
	first := list[0].(map[string]any)
	if len(list) != 4 || first["scenarioName"] == "" || first["agentName"] == "" || first["personaName"] == "" || first["status"] != "queued" {
		t.Fatalf("runs = %v", list)
	}

	engine := &fakeEngine{}
	startWorker(t, f.e, engine, 3)
	var voiceRun map[string]any
	for _, id := range f.runIDs(t, batch["id"].(string)) {
		r := f.waitStatus(t, id, "completed")
		if r["endReason"] != "user_hung_up" || len(r["transcript"].([]any)) != 2 || r["metrics"].(map[string]any)["latencyP50Ms"].(float64) != 420 {
			t.Fatalf("run = %v", r)
		}
		if r["threadId"] == nil || r["traceSessionId"] != r["threadId"] {
			t.Fatalf("thread %v trace %v", r["threadId"], r["traceSessionId"])
		}
		if r["modality"] == "voice" {
			voiceRun = r
		}
	}
	// The worker resolved credentials for the engine.
	for _, job := range engine.jobs {
		if job.Simulator.Secret != "sk-ant-1" || job.SimulatorModel != "claude-sonnet-5-5" {
			t.Fatalf("job simulator = %+v", job.Simulator)
		}
		if job.Snapshot.Agent.Modality == "voice" && (job.Voice == nil || job.Voice.Voice != "alloy" || job.Voice.Secret != "sk-openai-1") {
			t.Fatalf("job voice = %+v", job.Voice)
		}
	}

	// Audio for voice runs; none for text runs.
	id := voiceRun["id"].(string)
	if tracks := voiceRun["audioTracks"].([]any); len(tracks) != 1 || tracks[0] != "conversation" {
		t.Fatalf("tracks = %v", tracks)
	}
	res, err := f.owner.http.Get(f.e.srv.URL + f.path("/runs/"+id+"/audio/conversation"))
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(res.Body)
	res.Body.Close()
	if res.StatusCode != 200 || res.Header.Get("Content-Type") != "audio/wav" || string(body) != "RIFF....WAVEfake" {
		t.Fatalf("audio = %d %s %q", res.StatusCode, res.Header.Get("Content-Type"), body)
	}
	expectStatus(t, f.owner.do("GET", f.path("/runs/"+id+"/audio/agent"), nil), http.StatusNotFound)

	// The event stream replays status, transcript, metrics and ends.
	req, _ := http.NewRequest("GET", f.e.srv.URL+f.path("/runs/"+id+"/events"), nil)
	res, err = f.owner.http.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	stream, _ := io.ReadAll(res.Body)
	res.Body.Close()
	if cc := res.Header.Get("Cache-Control"); cc != "no-cache, no-transform" {
		t.Fatalf("events Cache-Control = %q", cc)
	}
	for _, part := range []string{"event: status", `"status":"running"`, "event: transcript", "event: log", "event: metrics", `"status":"completed"`, "event: end"} {
		if !strings.Contains(string(stream), part) {
			t.Fatalf("stream missing %q:\n%s", part, stream)
		}
	}
	// Resuming after the last event returns nothing more.
	lastID := strings.TrimPrefix(strings.Split(strings.Split(string(stream), "\nid: ")[len(strings.Split(string(stream), "\nid: "))-1], "\n")[0], "id: ")
	req.Header.Set("Last-Event-ID", lastID)
	res, _ = f.owner.http.Do(req)
	rest, _ := io.ReadAll(res.Body)
	res.Body.Close()
	if strings.Contains(string(rest), "event:") {
		t.Fatalf("resume returned events: %s", rest)
	}

	batches := f.owner.do("GET", f.path("/run-batches"), nil).body["items"].([]any)
	if batches[0].(map[string]any)["counts"].(map[string]any)["completed"].(float64) != 4 {
		t.Fatalf("batches = %v", batches)
	}

	rerun := f.owner.do("POST", f.path("/runs/"+id+"/rerun"), nil)
	expectStatus(t, rerun, http.StatusCreated)
	if rerun.body["attempt"].(float64) != 3 || rerun.body["batchId"] != batch["id"] {
		t.Fatalf("rerun = %v", rerun.body)
	}
	f.waitStatus(t, rerun.body["id"].(string), "completed")
}

func TestBatchConcurrencyAndCancel(t *testing.T) {
	f := newRunFixture(t)
	batch := f.launch(t, map[string]any{"scenarioIds": []any{f.textScenario["id"]}, "repetitions": 3, "concurrency": 1})
	engine := &fakeEngine{block: true}
	startWorker(t, f.e, engine, 3)

	ids := f.runIDs(t, batch["id"].(string))
	time.Sleep(1500 * time.Millisecond)
	engine.mu.Lock()
	maxSeen := engine.maxSeen
	engine.mu.Unlock()
	if maxSeen != 1 {
		t.Fatalf("max concurrent runs = %d, want 1", maxSeen)
	}

	running := ""
	for _, id := range ids {
		if f.owner.do("GET", f.path("/runs/"+id), nil).body["status"] == "running" {
			running = id
		}
	}
	if running == "" {
		t.Fatal("no run is running")
	}
	expectStatus(t, f.owner.do("POST", f.path("/runs/"+running+"/cancel"), nil), http.StatusOK)
	r := f.waitStatus(t, running, "cancelled")
	if r["endReason"] != "cancelled" || r["finishedAt"] == nil {
		t.Fatalf("cancelled run = %v", r)
	}

	b := f.owner.do("POST", f.path("/run-batches/"+batch["id"].(string)+"/cancel"), nil)
	expectStatus(t, b, http.StatusOK)
	for _, id := range ids {
		f.waitStatus(t, id, "cancelled")
	}
}

func TestReaperInterruptsLostRuns(t *testing.T) {
	f := newRunFixture(t)
	batch := f.launch(t, map[string]any{"scenarioIds": []any{f.textScenario["id"]}})
	id := f.runIDs(t, batch["id"].(string))[0]
	if _, err := f.e.pool.Exec(context.Background(),
		`UPDATE runs SET status = 'running', lease_owner = 'gone', lease_until = now() - interval '1 second', started_at = now() WHERE id = $1`, id); err != nil {
		t.Fatal(err)
	}
	startWorker(t, f.e, &fakeEngine{}, 1)
	r := f.waitStatus(t, id, "interrupted")
	if r["endReason"] != "worker_lost" {
		t.Fatalf("reaped run = %v", r)
	}
}

func TestRunsTenancyAndFilters(t *testing.T) {
	f := newRunFixture(t)
	batch := f.launch(t, map[string]any{"scenarioIds": []any{f.textScenario["id"], f.voiceScenario["id"]}})
	id := f.runIDs(t, batch["id"].(string))[0]

	if n := len(f.owner.do("GET", f.path("/runs?agentId="+f.text["id"].(string)), nil).body["items"].([]any)); n != 1 {
		t.Fatalf("agent filter = %d", n)
	}
	page := f.owner.do("GET", f.path("/runs?limit=1"), nil)
	if len(page.body["items"].([]any)) != 1 || page.body["nextCursor"] == nil {
		t.Fatalf("page = %v", page.body)
	}
	next := f.owner.do("GET", f.path("/runs?limit=1&before="+page.body["nextCursor"].(string)), nil)
	if len(next.body["items"].([]any)) != 1 || next.body["items"].([]any)[0].(map[string]any)["id"] == page.body["items"].([]any)[0].(map[string]any)["id"] {
		t.Fatalf("next page = %v", next.body)
	}
	expectStatus(t, f.owner.do("GET", f.path("/runs?status=weird"), nil), http.StatusUnprocessableEntity)

	bob := f.e.browser()
	bobSlug := firstWorkspace(t, signup(t, bob, "Bob", "bob@example.com"))["slug"].(string)
	foreign := "/api/v1/workspaces/" + bobSlug + "/runs/" + id
	expectStatus(t, bob.do("GET", foreign, nil), http.StatusNotFound)
	expectStatus(t, bob.do("GET", foreign+"/events", nil), http.StatusNotFound)
	expectStatus(t, bob.do("POST", foreign+"/cancel", nil), http.StatusNotFound)
	expectStatus(t, bob.do("POST", "/api/v1/workspaces/"+bobSlug+"/runs", map[string]any{"scenarioIds": []any{f.textScenario["id"]}}), http.StatusUnprocessableEntity)
}

func TestLaterBatchIsNotStarvedByFullBatch(t *testing.T) {
	f := newRunFixture(t)
	f.launch(t, map[string]any{"scenarioIds": []any{f.textScenario["id"]}, "repetitions": 10, "concurrency": 1})
	f.launch(t, map[string]any{"scenarioIds": []any{f.textScenario["id"]}, "repetitions": 10, "concurrency": 1})
	f.launch(t, map[string]any{"scenarioIds": []any{f.textScenario["id"]}, "repetitions": 5, "concurrency": 1})
	late := f.launch(t, map[string]any{"scenarioIds": []any{f.voiceScenario["id"]}})
	startWorker(t, f.e, &fakeEngine{block: true}, 4)
	f.waitStatus(t, f.runIDs(t, late["id"].(string))[0], "running")
}

func TestRunEventsRejectsInvalidID(t *testing.T) {
	f := newRunFixture(t)
	expectStatus(t, f.owner.do("GET", f.path("/runs/not-a-uuid/events"), nil), http.StatusNotFound)
}
