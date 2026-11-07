package server_test

import (
	"context"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/coder/websocket"

	"testforge/internal/run"
)

func tone(ms, rate int) []byte {
	n := rate * ms / 1000
	b := make([]byte, n*2)
	for i := range n {
		v := int16(2000)
		if (i/20)%2 == 0 {
			v = -2000
		}
		binary.LittleEndian.PutUint16(b[i*2:], uint16(v))
	}
	return b
}

func loud(pcm []byte) bool {
	for i := 0; i+1 < len(pcm); i += 2 {
		if v := int16(binary.LittleEndian.Uint16(pcm[i:])); v > 300 || v < -300 {
			return true
		}
	}
	return false
}

// speechDetector reports when the caller finishes an utterance (speech, then 300 ms of silence).
type speechDetector struct{ speaking, quiet int }

func (d *speechDetector) feed(pcm []byte) bool {
	if loud(pcm) {
		d.speaking++
		d.quiet = 0
		return false
	}
	if d.speaking > 0 {
		d.quiet++
		if d.quiet >= 3 {
			d.speaking, d.quiet = 0, 0
			return true
		}
	}
	return false
}

func fakeTTS(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(tone(300, 24000))
	}))
	t.Cleanup(srv.Close)
	return srv
}

type wsRecorder struct {
	mu     sync.Mutex
	frames []string
}

func (r *wsRecorder) add(s string) { r.mu.Lock(); r.frames = append(r.frames, s); r.mu.Unlock() }
func (r *wsRecorder) find(sub string) string {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, f := range r.frames {
		if strings.Contains(f, sub) {
			return f
		}
	}
	return ""
}

func fakeViviConversation(t *testing.T, got *wsRecorder) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		defer c.CloseNow()
		ctx := r.Context()
		send := func(v any) { b, _ := json.Marshal(v); _ = c.Write(ctx, websocket.MessageText, b) }
		speak := func(id, text string) {
			send(map[string]any{"type": "voice.audio.delta", "delta": base64.StdEncoding.EncodeToString(tone(400, 24000))})
			send(map[string]any{"type": "voice.transcript.update", "role": "assistant", "messageId": id, "text": text})
		}
		send(map[string]any{"type": "voice.ready"})
		speak("m1", "Hola, ¿en qué te ayudo?")
		det := &speechDetector{}
		replies := 0
		for {
			_, data, err := c.Read(ctx)
			if err != nil {
				return
			}
			var m map[string]any
			_ = json.Unmarshal(data, &m)
			switch m["type"] {
			case "voice.input.audio":
				pcm, _ := base64.StdEncoding.DecodeString(m["audio"].(string))
				if det.feed(pcm) {
					replies++
					speak("m"+string(rune('1'+replies)), "Claro, la cambio al viernes.")
				}
			case "voice.close":
				got.add("voice.close")
				send(map[string]any{"type": "voice.closed"})
				_ = c.Close(websocket.StatusNormalClosure, "")
				return
			}
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

func voiceRunFixture(t *testing.T, agentBody map[string]any, decisions []string, interruptions []any, llmDelay ...time.Duration) (*runFixture, map[string]any) {
	t.Helper()
	f := newRunFixture(t)
	llm := newFakeLLM(t, decisions...)
	if len(llmDelay) > 0 {
		llm.delay = llmDelay[0]
	}
	speech := fakeTTS(t)
	ag := createAgent(t, f.owner, f.slug, agentBody)
	sim := createConnection(t, f.owner, f.slug, map[string]any{"name": "Sim", "kind": "openai", "secret": "sk-sim", "config": map[string]any{"baseUrl": llm.srv.URL + "/v1"}})
	ttsConn := createConnection(t, f.owner, f.slug, map[string]any{"name": "TTS", "kind": "openai", "secret": "sk-tts", "config": map[string]any{"baseUrl": speech.URL + "/v1"}})
	p := personaBody("Voice tester")
	p["voice"] = map[string]any{"connectionId": ttsConn["id"], "voice": "alloy"}
	r := f.owner.do("POST", f.path("/personas"), p)
	expectStatus(t, r, http.StatusCreated)
	body := f.scenario(ag, "Voice reschedule")
	body["personaId"] = r.body["id"]
	body["simulator"] = map[string]any{"mode": "pipeline", "connectionId": sim["id"], "model": "sim"}
	body["interruptions"] = interruptions
	body["endConditions"] = map[string]any{"maxDurationSeconds": 30, "maxSilenceSeconds": 8, "maxTurns": 20, "hangUpWhen": "goal_or_goodbye"}
	return f, f.create(t, body)
}

func fastVoice() *run.VoiceEngine {
	return &run.VoiceEngine{AllowPrivate: true, TurnGap: 500 * time.Millisecond, Settle: 400 * time.Millisecond,
		PlaybackGrace: 200 * time.Millisecond, TrailingSilence: 400 * time.Millisecond, ReadyWait: 300 * time.Millisecond}
}

func TestVoiceEngineViviLiveConversation(t *testing.T) {
	got := &wsRecorder{}
	agentSrv := fakeViviConversation(t, got)
	body := viviLiveAgent(wsURL(agentSrv))
	body["name"] = "Vivi live E2E"
	f, sc := voiceRunFixture(t, body, []string{decision("Quiero mover mi cita al viernes.", false), decision("Perfecto, gracias. Adiós.", true)}, []any{})
	startWorker(t, f.e, fastVoice(), 1)
	batch := f.launch(t, map[string]any{"scenarioIds": []any{sc["id"]}})
	r := f.waitStatus(t, f.runIDs(t, batch["id"].(string))[0], "completed", "failed")

	if r["status"] != "completed" || r["endReason"] != "user_hung_up" {
		t.Fatalf("run = %v", r)
	}
	var roles []string
	for _, turn := range r["transcript"].([]any) {
		roles = append(roles, turn.(map[string]any)["role"].(string))
	}
	if strings.Join(roles, ",") != "agent,user,agent,user" {
		t.Fatalf("roles = %v, transcript %v", roles, r["transcript"])
	}
	tr := r["transcript"].([]any)
	if tr[0].(map[string]any)["text"] != "Hola, ¿en qué te ayudo?" || tr[2].(map[string]any)["latencyMs"] == nil {
		t.Fatalf("transcript = %v", tr)
	}
	if tr[1].(map[string]any)["text"] != "Quiero mover mi cita al viernes." || tr[1].(map[string]any)["endMs"].(float64) <= tr[1].(map[string]any)["startMs"].(float64) {
		t.Fatalf("user turn = %v", tr[1])
	}
	m := r["metrics"].(map[string]any)
	if m["agentAudioMs"].(float64) != 800 || m["latencyP50Ms"] == nil {
		t.Fatalf("metrics = %v", m)
	}
	if tracks := r["audioTracks"].([]any); len(tracks) != 3 {
		t.Fatalf("tracks = %v", tracks)
	}
	if got.find("voice.close") == "" {
		t.Fatal("the agent did not receive voice.close")
	}
}

func TestVoiceEngineInterruptsRealtimeAgent(t *testing.T) {
	got := &wsRecorder{}
	agentSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		defer c.CloseNow()
		ctx := r.Context()
		send := func(v any) { b, _ := json.Marshal(v); _ = c.Write(ctx, websocket.MessageText, b) }
		send(map[string]any{"type": "session.updated"})
		det := &speechDetector{}
		n := 0
		for {
			_, data, err := c.Read(ctx)
			if err != nil {
				return
			}
			var m map[string]any
			_ = json.Unmarshal(data, &m)
			switch m["type"] {
			case "input_audio_buffer.append":
				pcm, _ := base64.StdEncoding.DecodeString(m["audio"].(string))
				if det.feed(pcm) {
					n++
					item, resp := "it"+string(rune('0'+n)), "r"+string(rune('0'+n))
					send(map[string]any{"type": "response.created", "response": map[string]any{"id": resp}})
					for range 20 { // 2 s of audio, delivered at once
						send(map[string]any{"type": "response.output_audio.delta", "item_id": item, "response_id": resp,
							"delta": base64.StdEncoding.EncodeToString(tone(100, 24000))})
					}
					send(map[string]any{"type": "response.output_audio_transcript.delta", "item_id": item, "delta": "Tenemos el lunes, el martes y"})
					send(map[string]any{"type": "response.done", "response": map[string]any{"id": resp}})
				}
			case "conversation.item.truncate":
				got.add(string(data))
			}
		}
	}))
	t.Cleanup(agentSrv.Close)

	body := viviLiveAgent(wsURL(agentSrv))
	body["name"] = "Realtime E2E"
	body["codec"] = map[string]any{"type": "openai_realtime", "options": map[string]any{}}
	f, sc := voiceRunFixture(t, body, []string{decision("Hola, quiero una cita.", false), decision("Ok, gracias. Adiós.", true)},
		[]any{map[string]any{"agentTurn": 1, "afterMs": 600}})
	startWorker(t, f.e, fastVoice(), 1)
	batch := f.launch(t, map[string]any{"scenarioIds": []any{sc["id"]}})
	r := f.waitStatus(t, f.runIDs(t, batch["id"].(string))[0], "completed", "failed")
	if r["status"] != "completed" {
		t.Fatalf("run = %v", r)
	}
	tr := r["transcript"].([]any)
	if len(tr) < 3 || tr[1].(map[string]any)["role"] != "agent" || tr[1].(map[string]any)["interrupted"] != true {
		t.Fatalf("transcript = %v", tr)
	}
	frame := got.find("conversation.item.truncate")
	if frame == "" {
		t.Fatal("no truncate frame sent")
	}
	var m map[string]any
	_ = json.Unmarshal([]byte(frame), &m)
	end := m["audio_end_ms"].(float64)
	if m["item_id"] != "it1" || end < 500 || end > 1500 {
		t.Fatalf("truncate = %s", frame)
	}
	_ = context.Background()
}

func TestVoiceEngineRedactsAgentTextAndMeasuresLatency(t *testing.T) {
	agentSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		defer c.CloseNow()
		ctx := r.Context()
		key := r.Header.Get("X-Api-Key")
		send := func(v any) { b, _ := json.Marshal(v); _ = c.Write(ctx, websocket.MessageText, b) }
		speak := func(id, text string) {
			send(map[string]any{"type": "voice.audio.delta", "delta": base64.StdEncoding.EncodeToString(tone(300, 24000))})
			send(map[string]any{"type": "voice.transcript.update", "role": "assistant", "messageId": id, "text": text})
		}
		send(map[string]any{"type": "voice.ready"})
		speak("m1", "Hola.")
		det := &speechDetector{}
		answered := false
		for {
			_, data, err := c.Read(ctx)
			if err != nil {
				return
			}
			var m map[string]any
			_ = json.Unmarshal(data, &m)
			switch m["type"] {
			case "voice.input.audio":
				pcm, _ := base64.StdEncoding.DecodeString(m["audio"].(string))
				if det.feed(pcm) && !answered {
					answered = true
					speak("m2", "Un momento…")
					go func() {
						time.Sleep(1000 * time.Millisecond) // a second agent turn with no user turn between
						speak("m3", "Your key is "+key)
						send(map[string]any{"type": "voice.error", "message": "bad key " + key})
					}()
				}
			case "voice.close":
				send(map[string]any{"type": "voice.closed"})
				return
			}
		}
	}))
	t.Cleanup(agentSrv.Close)
	body := viviLiveAgent(wsURL(agentSrv))
	body["name"] = "Echo voice"
	f, sc := voiceRunFixture(t, body, []string{decision("¿Me ayudas?", false), decision("Gracias, adiós.", true)}, []any{}, 1500*time.Millisecond)
	startWorker(t, f.e, fastVoice(), 1)
	batch := f.launch(t, map[string]any{"scenarioIds": []any{sc["id"]}})
	id := f.runIDs(t, batch["id"].(string))[0]
	r := f.waitStatus(t, id, "completed", "failed")

	raw, _ := json.Marshal(r)
	if strings.Contains(string(raw), agentSecret) {
		t.Fatalf("run leaks the agent secret: %s", raw)
	}
	var leaked int
	if err := f.e.pool.QueryRow(context.Background(), `SELECT count(*) FROM run_events WHERE data::text LIKE '%' || $1 || '%'`, agentSecret).Scan(&leaked); err != nil {
		t.Fatal(err)
	}
	if leaked != 0 {
		t.Fatalf("%d run events contain the secret", leaked)
	}
	var agentTurns []map[string]any
	for _, turn := range r["transcript"].([]any) {
		if m := turn.(map[string]any); m["role"] == "agent" {
			agentTurns = append(agentTurns, m)
		}
	}
	if len(agentTurns) < 3 {
		t.Fatalf("agent turns = %v", agentTurns)
	}
	if agentTurns[1]["latencyMs"] == nil || agentTurns[2]["latencyMs"] != nil {
		t.Fatalf("latencies = %v / %v (a consecutive agent turn has no user turn to measure from)", agentTurns[1]["latencyMs"], agentTurns[2]["latencyMs"])
	}
}
