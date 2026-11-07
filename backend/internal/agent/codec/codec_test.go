package codec

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"testforge/internal/agent/tmpl"
)

func frame(s string) Frame { return Frame{Data: []byte(s)} }

func b64(b []byte) string { return base64.StdEncoding.EncodeToString(b) }

func decodeJSON(t *testing.T, f Frame) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(f.Data, &m); err != nil {
		t.Fatalf("frame %q is not JSON: %v", f.Data, err)
	}
	return m
}

func newVoice(t *testing.T, typ string, opts map[string]string) Voice {
	t.Helper()
	v, err := NewVoice(typ, opts, tmpl.Values{"run.thread_id": "t-1"})
	if err != nil {
		t.Fatal(err)
	}
	return v
}

func TestViviLive(t *testing.T) {
	v := newVoice(t, "vivi_live", nil)
	if !v.Capabilities().AgentSpeaksFirst || v.SampleRate() != 24000 {
		t.Fatalf("caps = %+v rate %d", v.Capabilities(), v.SampleRate())
	}
	pcm := []byte{1, 2, 3, 4}
	events := v.Decode(frame(`{"type":"voice.ready","sessionId":"s"}`))
	events = append(events, v.Decode(frame(`{"type":"voice.audio.delta","delta":"`+b64(pcm)+`"}`))...)
	events = append(events, v.Decode(frame(`{"type":"voice.transcript.update","role":"assistant","messageId":"m1","text":"Hola","startMs":0}`))...)
	events = append(events, v.Decode(frame(`{"type":"voice.task.status"}`))...)
	events = append(events, v.Decode(frame(`{"type":"voice.closed","usageSeconds":3}`))...)
	kinds := []Kind{KindReady, KindAudio, KindTranscript, KindOther, KindClosed}
	for i, k := range kinds {
		if events[i].Kind != k {
			t.Fatalf("event %d = %+v, want kind %v", i, events[i], k)
		}
	}
	if !bytes.Equal(events[1].Audio, pcm) || events[2].Role != RoleAgent || events[2].Text != "Hola" || !events[2].Cumulative || events[2].ItemID != "m1" {
		t.Fatalf("events = %+v", events)
	}

	frames, err := v.EncodeAudio(make([]byte, 200000))
	if err != nil || len(frames) != 3 {
		t.Fatalf("EncodeAudio frames = %d, %v", len(frames), err)
	}
	for _, f := range frames {
		m := decodeJSON(t, f)
		if m["type"] != "voice.input.audio" || len(m["audio"].(string)) > 131072 {
			t.Fatalf("audio frame type %v len %d", m["type"], len(m["audio"].(string)))
		}
	}
	if _, err := v.EncodeAudio([]byte{1, 2, 3}); err == nil {
		t.Fatal("odd-length PCM accepted")
	}
	if _, err := v.EncodeText("hi"); err == nil {
		t.Fatal("vivi_live must not support text turns")
	}
	closeFrames, awaitAck := v.Close()
	if !awaitAck || decodeJSON(t, closeFrames[0])["type"] != "voice.close" {
		t.Fatalf("close = %s, %v", closeFrames[0].Data, awaitAck)
	}
}

func TestOpenAIRealtime(t *testing.T) {
	v := newVoice(t, "openai_realtime", map[string]string{"playbackInterrupted": "true", "requestResponseOnStart": "true"})
	if v.Decode(frame(`{"type":"session.created","session":{"id":"s"}}`))[0].Kind != KindOther {
		t.Fatal("session.created must not be ready")
	}
	if v.Decode(frame(`{"type":"session.updated"}`))[0].Kind != KindReady {
		t.Fatal("first session.updated is ready")
	}
	if v.Decode(frame(`{"type":"session.updated"}`))[0].Kind != KindOther {
		t.Fatal("only the first session.updated is ready")
	}
	audio := v.Decode(frame(`{"type":"response.output_audio.delta","delta":"` + b64([]byte{9, 9}) + `","item_id":"i1","response_id":"r1"}`))[0]
	if audio.Kind != KindAudio || audio.ItemID != "i1" || audio.ResponseID != "r1" {
		t.Fatalf("audio = %+v", audio)
	}
	tr := v.Decode(frame(`{"type":"response.output_audio_transcript.delta","item_id":"i1","delta":"Ho"}`))[0]
	if tr.Kind != KindTranscript || tr.Cumulative || tr.Text != "Ho" || tr.Role != RoleAgent {
		t.Fatalf("transcript = %+v", tr)
	}
	user := v.Decode(frame(`{"type":"conversation.item.input_audio_transcription.completed","item_id":"u1","transcript":"hola"}`))[0]
	if user.Role != RoleUser || !user.Cumulative {
		t.Fatalf("user transcript = %+v", user)
	}
	if v.Decode(frame(`{"type":"response.done","response":{"id":"r1"}}`))[0].Kind != KindTurnDone {
		t.Fatal("response.done is turn done")
	}
	if v.Decode(frame(`{"type":"response.function_call_arguments.delta"}`))[0].Kind != KindBusy {
		t.Fatal("tool call is busy")
	}
	if e := v.Decode(frame(`{"type":"error","error":{"message":"bad"}}`))[0]; e.Kind != KindError || e.Text != "bad" {
		t.Fatalf("error = %+v", e)
	}

	if f := v.OnReady(); len(f) != 1 || decodeJSON(t, f[0])["type"] != "response.create" {
		t.Fatalf("OnReady = %v", f)
	}
	text, _ := v.EncodeText("hi")
	if len(text) != 2 || decodeJSON(t, text[0])["type"] != "conversation.item.create" || decodeJSON(t, text[1])["type"] != "response.create" {
		t.Fatalf("EncodeText = %v", text)
	}
	end := 1200
	intr, _ := v.Interrupt("i1", "r1", &end)
	m := decodeJSON(t, intr[0])
	if m["type"] != "playback.interrupted" || m["itemId"] != "i1" || m["audioEndMs"].(float64) != 1200 {
		t.Fatalf("interrupt = %v", m)
	}

	plain := newVoice(t, "openai_realtime", nil)
	if len(plain.OnReady()) != 0 {
		t.Fatal("response.create sent without the option")
	}
	intr, _ = plain.Interrupt("i1", "r1", &end)
	if decodeJSON(t, intr[0])["type"] != "conversation.item.truncate" {
		t.Fatalf("standard interrupt = %s", intr[0].Data)
	}
	if frames, awaitAck := plain.Close(); len(frames) != 0 || awaitAck {
		t.Fatal("openai close sends nothing")
	}
}

func TestJSONAudio(t *testing.T) {
	v := newVoice(t, "json_audio", map[string]string{
		"typeField": "event", "audioType": "media", "audioPath": "payload.pcm", "sampleRate": "16000",
		"sendAudio": `{"event":"media","session":"{{run.thread_id}}","payload":{"pcm":"{{audio}}"}}`,
	})
	if v.SampleRate() != 16000 {
		t.Fatalf("rate = %d", v.SampleRate())
	}
	if e := v.Decode(frame(`{"event":"ready"}`))[0]; e.Kind != KindReady {
		t.Fatalf("ready = %+v", e)
	}
	if e := v.Decode(frame(`{"event":"media","payload":{"pcm":"` + b64([]byte{7, 7}) + `"}}`))[0]; e.Kind != KindAudio || !bytes.Equal(e.Audio, []byte{7, 7}) {
		t.Fatalf("audio = %+v", e)
	}
	if e := v.Decode(frame(`{"event":"agent_transcript","delta":"Hi"}`))[0]; e.Kind != KindTranscript || e.Text != "Hi" {
		t.Fatalf("transcript = %+v", e)
	}
	if e := v.Decode(Frame{Binary: true, Data: []byte{1}})[0]; e.Kind != KindOther {
		t.Fatalf("binary = %+v", e)
	}
	if e := v.Decode(frame(`not json`))[0]; e.Kind != KindOther {
		t.Fatalf("non-JSON = %+v", e)
	}
	frames, err := v.EncodeAudio([]byte{1, 0})
	if err != nil {
		t.Fatal(err)
	}
	m := decodeJSON(t, frames[0])
	if m["session"] != "t-1" || m["payload"].(map[string]any)["pcm"] != b64([]byte{1, 0}) {
		t.Fatalf("send = %v", m)
	}
	closeFrames, _ := v.Close()
	if decodeJSON(t, closeFrames[0])["type"] != "stop" {
		t.Fatalf("close = %s", closeFrames[0].Data)
	}

	if _, err := NewVoice("json_audio", map[string]string{"sendAudio": `{"a":"x"}`}, nil); err == nil {
		t.Fatal("sendAudio without {{audio}} accepted")
	}
	if _, err := NewVoice("http_json", nil, nil); err == nil {
		t.Fatal("text codec accepted as voice")
	}
}

func readReply(t *testing.T, typ string, opts map[string]string, body string) (string, []string, error) {
	t.Helper()
	c, err := NewText(typ, opts)
	if err != nil {
		t.Fatal(err)
	}
	var seen []string
	reply, err := c.ReadReply(&http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body))},
		func(kind Kind, detail string) { seen = append(seen, detail) })
	return reply, seen, err
}

func TestHTTPJSON(t *testing.T) {
	reply, _, err := readReply(t, "http_json", map[string]string{"replyPath": "data.text"}, `{"data":{"text":"Hola"}}`)
	if err != nil || reply != "Hola" {
		t.Fatalf("reply = %q, %v", reply, err)
	}
	if _, _, err := readReply(t, "http_json", nil, `{"other":1}`); err == nil {
		t.Fatal("missing reply path accepted")
	}
}

func TestHTTPStream(t *testing.T) {
	opts := map[string]string{"deltaWhen": "event=on_chat_model_stream", "deltaPath": "message", "skipWhen": "type=reasoning", "endWhen": "event=on_chat_end"}
	body := strings.Join([]string{
		`data: {"event":"on_chat_model_stream","message":"Ho"}`,
		``,
		`data: {"event":"on_chat_model_stream","type":"reasoning","message":"thinking"}`,
		`data: {"event":"on_tool_start","metadata":{"toolName":"calendar"}}`,
		`data: {"event":"on_chat_model_stream","message":"la"}`,
		`data: {"event":"on_chat_end"}`,
		`data: {"event":"on_chat_model_stream","message":" ignored after end"}`,
	}, "\n")
	reply, seen, err := readReply(t, "http_stream", opts, body)
	if err != nil || reply != "Hola" {
		t.Fatalf("reply = %q, %v", reply, err)
	}
	if !strings.Contains(strings.Join(seen, "|"), "on_tool_start") {
		t.Fatalf("events = %v", seen)
	}
	if _, _, err := readReply(t, "http_stream", opts, `data: {"event":"on_chat_model_stream","message":"cut"}`); err == nil {
		t.Fatal("stream without end event accepted")
	}
	reply, _, err = readReply(t, "http_stream", map[string]string{"deltaPath": "choices.0.delta.content"},
		"data: {\"choices\":[{\"delta\":{\"content\":\"A\"}}]}\ndata: {\"choices\":[{\"delta\":{\"content\":\"B\"}}]}\ndata: [DONE]\n")
	if err != nil || reply != "AB" {
		t.Fatalf("openai-style stream = %q, %v", reply, err)
	}
}

func TestCatalogDefaultsAndValidation(t *testing.T) {
	for _, spec := range Catalog {
		if spec.Label == "" || (spec.Modality != "voice" && spec.Modality != "text") {
			t.Fatalf("bad spec %+v", spec)
		}
	}
	if _, err := Options("vivi_live", map[string]string{"sampleRate": "abc"}); err == nil {
		t.Fatal("non-numeric sample rate accepted")
	}
	if _, err := Options("openai_realtime", map[string]string{"playbackInterrupted": "yes"}); err == nil {
		t.Fatal("non-boolean accepted")
	}
	if _, err := Options("http_json", map[string]string{"nope": "x"}); err == nil {
		t.Fatal("unknown option accepted")
	}
}
