package codec

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"

	"testforge/internal/agent/tmpl"
)

// Kind classifies a normalized event.
type Kind string

const (
	KindReady      Kind = "ready"
	KindAudio      Kind = "audio"
	KindTranscript Kind = "transcript"
	KindTurnDone   Kind = "turn_done"
	KindBusy       Kind = "busy"
	KindError      Kind = "error"
	KindClosed     Kind = "closed"
	KindOther      Kind = "other"
)

// Roles in transcripts.
const (
	RoleAgent = "agent"
	RoleUser  = "user"
)

// Event is a protocol-independent view of one inbound message.
type Event struct {
	Kind       Kind
	Type       string // wire event type, for logs
	Audio      []byte // PCM16 (KindAudio)
	Role       string // KindTranscript
	Text       string // transcript text or error message
	Cumulative bool   // Text replaces the item's text instead of appending
	ItemID     string
	ResponseID string
}

// Frame is one WebSocket message.
type Frame struct {
	Binary bool
	Data   []byte
}

// Capabilities describe what a voice protocol supports.
type Capabilities struct {
	AgentSpeaksFirst bool
	ExplicitTurnEnd  bool
	TextTurns        bool
	Interruptions    bool
}

// ErrUnsupported is returned for operations a protocol does not have.
var ErrUnsupported = errors.New("not supported by this codec")

// Voice encodes and decodes a voice agent's WebSocket protocol.
type Voice interface {
	Capabilities() Capabilities
	SampleRate() int
	Decode(f Frame) []Event
	// EncodeAudio splits caller PCM16 into protocol frames.
	EncodeAudio(pcm []byte) ([]Frame, error)
	EncodeText(text string) ([]Frame, error)
	// OnReady returns frames to send once the session is ready.
	OnReady() []Frame
	// Interrupt tells the agent its audio was cut at audioEndMs (nil: not started).
	Interrupt(itemID, responseID string, audioEndMs *int) ([]Frame, error)
	// Close returns frames to send before closing and whether to wait for a KindClosed event.
	Close() ([]Frame, bool)
}

// NewVoice builds a voice codec. vals resolves {{run.*}}, {{var.*}} and
// {{session.*}} in JSON option templates.
func NewVoice(typ string, opts map[string]string, vals tmpl.Values) (Voice, error) {
	spec, ok := SpecFor(typ)
	if !ok || spec.Modality != "voice" {
		return nil, fmt.Errorf("%q is not a voice codec", typ)
	}
	o, err := Options(typ, opts)
	if err != nil {
		return nil, err
	}
	rate, _ := strconv.Atoi(o["sampleRate"])
	switch typ {
	case "vivi_live":
		return &viviLive{rate: rate}, nil
	case "openai_realtime":
		return &openAIRealtime{rate: rate, speakFirst: o["requestResponseOnStart"] == "true", viviInterrupt: o["playbackInterrupted"] == "true"}, nil
	default:
		return newJSONAudio(o, rate, vals)
	}
}

func jsonFrame(v any) Frame {
	b, _ := json.Marshal(v)
	return Frame{Data: b}
}

// chunkPCM splits PCM16 into chunks whose base64 stays under maxB64 characters.
func chunkPCM(pcm []byte, maxB64 int) ([][]byte, error) {
	if len(pcm) == 0 || len(pcm)%2 != 0 {
		return nil, errors.New("PCM16 audio must have a non-zero, even number of bytes")
	}
	maxBytes := maxB64 / 4 * 3
	maxBytes -= maxBytes % 2
	var chunks [][]byte
	for len(pcm) > 0 {
		n := min(len(pcm), maxBytes)
		chunks = append(chunks, pcm[:n])
		pcm = pcm[n:]
	}
	return chunks, nil
}

func decodeObject(f Frame) (map[string]any, string, bool) {
	if f.Binary {
		return nil, "binary", false
	}
	var m map[string]any
	if json.Unmarshal(f.Data, &m) != nil {
		return nil, "non-json", false
	}
	return m, "", true
}

func str(m map[string]any, path string) string {
	v, ok := Lookup(m, path)
	if !ok {
		return ""
	}
	s, _ := v.(string)
	return s
}

func audioEvent(typ, b64 string) Event {
	pcm, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		return Event{Kind: KindError, Type: typ, Text: "agent audio is not valid base64"}
	}
	return Event{Kind: KindAudio, Type: typ, Audio: pcm}
}

// --- Vivi Live ---------------------------------------------------------------

type viviLive struct{ rate int }

func (c *viviLive) Capabilities() Capabilities { return Capabilities{AgentSpeaksFirst: true} }
func (c *viviLive) SampleRate() int            { return c.rate }

func (c *viviLive) Decode(f Frame) []Event {
	m, other, ok := decodeObject(f)
	if !ok {
		return []Event{{Kind: KindOther, Type: other}}
	}
	typ := str(m, "type")
	switch typ {
	case "voice.ready":
		return []Event{{Kind: KindReady, Type: typ}}
	case "voice.audio.delta":
		return []Event{audioEvent(typ, str(m, "delta"))}
	case "voice.transcript.update":
		role := RoleAgent
		if str(m, "role") == "user" {
			role = RoleUser
		}
		return []Event{{Kind: KindTranscript, Type: typ, Role: role, ItemID: str(m, "messageId"), Text: str(m, "text"), Cumulative: true}}
	case "voice.closed":
		return []Event{{Kind: KindClosed, Type: typ}}
	case "voice.error", "error":
		return []Event{{Kind: KindError, Type: typ, Text: str(m, "message")}}
	}
	return []Event{{Kind: KindOther, Type: typ}}
}

func (c *viviLive) EncodeAudio(pcm []byte) ([]Frame, error) {
	chunks, err := chunkPCM(pcm, 131072)
	if err != nil {
		return nil, err
	}
	frames := make([]Frame, len(chunks))
	for i, ch := range chunks {
		frames[i] = jsonFrame(map[string]string{"type": "voice.input.audio", "audio": base64.StdEncoding.EncodeToString(ch)})
	}
	return frames, nil
}

func (c *viviLive) EncodeText(string) ([]Frame, error) { return nil, ErrUnsupported }
func (c *viviLive) OnReady() []Frame                   { return nil }
func (c *viviLive) Interrupt(string, string, *int) ([]Frame, error) {
	return nil, ErrUnsupported
}
func (c *viviLive) Close() ([]Frame, bool) {
	return []Frame{jsonFrame(map[string]string{"type": "voice.close"})}, true
}

// --- OpenAI Realtime ----------------------------------------------------------

type openAIRealtime struct {
	rate          int
	speakFirst    bool
	viviInterrupt bool
	ready         bool
}

func (c *openAIRealtime) Capabilities() Capabilities {
	return Capabilities{ExplicitTurnEnd: true, TextTurns: true, Interruptions: true}
}
func (c *openAIRealtime) SampleRate() int { return c.rate }

func (c *openAIRealtime) Decode(f Frame) []Event {
	m, other, ok := decodeObject(f)
	if !ok {
		return []Event{{Kind: KindOther, Type: other}}
	}
	typ := str(m, "type")
	switch typ {
	case "session.updated":
		if !c.ready {
			c.ready = true
			return []Event{{Kind: KindReady, Type: typ}}
		}
	case "response.output_audio.delta", "response.audio.delta":
		e := audioEvent(typ, str(m, "delta"))
		e.ItemID, e.ResponseID = str(m, "item_id"), str(m, "response_id")
		return []Event{e}
	case "response.output_audio_transcript.delta", "response.audio_transcript.delta":
		return []Event{{Kind: KindTranscript, Type: typ, Role: RoleAgent, ItemID: str(m, "item_id"), Text: str(m, "delta")}}
	case "conversation.item.input_audio_transcription.completed":
		return []Event{{Kind: KindTranscript, Type: typ, Role: RoleUser, ItemID: str(m, "item_id"), Text: str(m, "transcript"), Cumulative: true}}
	case "response.done":
		return []Event{{Kind: KindTurnDone, Type: typ, ResponseID: str(m, "response.id")}}
	case "response.function_call_arguments.delta", "response.function_call_arguments.done":
		return []Event{{Kind: KindBusy, Type: typ}}
	case "error", "rate_limit_exhausted":
		return []Event{{Kind: KindError, Type: typ, Text: str(m, "error.message")}}
	}
	return []Event{{Kind: KindOther, Type: typ}}
}

func (c *openAIRealtime) EncodeAudio(pcm []byte) ([]Frame, error) {
	chunks, err := chunkPCM(pcm, 1<<20)
	if err != nil {
		return nil, err
	}
	frames := make([]Frame, len(chunks))
	for i, ch := range chunks {
		frames[i] = jsonFrame(map[string]string{"type": "input_audio_buffer.append", "audio": base64.StdEncoding.EncodeToString(ch)})
	}
	return frames, nil
}

func (c *openAIRealtime) EncodeText(text string) ([]Frame, error) {
	return []Frame{
		jsonFrame(map[string]any{"type": "conversation.item.create", "item": map[string]any{
			"type": "message", "role": "user", "content": []any{map[string]string{"type": "input_text", "text": text}},
		}}),
		jsonFrame(map[string]string{"type": "response.create"}),
	}, nil
}

func (c *openAIRealtime) OnReady() []Frame {
	if c.speakFirst {
		return []Frame{jsonFrame(map[string]string{"type": "response.create"})}
	}
	return nil
}

func (c *openAIRealtime) Interrupt(itemID, responseID string, audioEndMs *int) ([]Frame, error) {
	if c.viviInterrupt {
		return []Frame{jsonFrame(map[string]any{
			"type": "playback.interrupted", "itemId": itemID, "responseId": responseID, "contentIndex": 0, "audioEndMs": audioEndMs,
		})}, nil
	}
	end := 0
	if audioEndMs != nil {
		end = *audioEndMs
	}
	return []Frame{
		jsonFrame(map[string]any{"type": "conversation.item.truncate", "item_id": itemID, "content_index": 0, "audio_end_ms": end}),
	}, nil
}

func (c *openAIRealtime) Close() ([]Frame, bool) { return nil, false }

// --- Generic JSON -------------------------------------------------------------

type jsonAudio struct {
	rate      int
	o         map[string]string
	sendAudio tmpl.Template
	vals      tmpl.Values
	closeMsg  string
}

func newJSONAudio(o map[string]string, rate int, vals tmpl.Values) (*jsonAudio, error) {
	send, _ := tmpl.Parse(o["sendAudio"])
	c := &jsonAudio{rate: rate, o: o, sendAudio: send, vals: tmpl.Values{}}
	for k, v := range vals {
		c.vals[k] = v
	}
	c.vals["audio"] = "" // checked by Render below
	if _, err := send.Render(c.vals, tmpl.JSON); err != nil {
		return nil, err
	}
	if o["closeMessage"] != "" {
		t, _ := tmpl.Parse(o["closeMessage"])
		msg, err := t.Render(c.vals, tmpl.JSON)
		if err != nil {
			return nil, err
		}
		c.closeMsg = msg
	}
	return c, nil
}

func (c *jsonAudio) Capabilities() Capabilities { return Capabilities{} }
func (c *jsonAudio) SampleRate() int            { return c.rate }

func (c *jsonAudio) Decode(f Frame) []Event {
	m, other, ok := decodeObject(f)
	if !ok {
		return []Event{{Kind: KindOther, Type: other}}
	}
	typ := str(m, c.o["typeField"])
	switch {
	case typ == "":
	case typ == c.o["readyType"]:
		return []Event{{Kind: KindReady, Type: typ}}
	case typ == c.o["audioType"]:
		return []Event{audioEvent(typ, str(m, c.o["audioPath"]))}
	case typ == c.o["agentTranscriptType"]:
		return []Event{{Kind: KindTranscript, Type: typ, Role: RoleAgent, Text: str(m, c.o["agentTranscriptPath"])}}
	case typ == c.o["userTranscriptType"]:
		return []Event{{Kind: KindTranscript, Type: typ, Role: RoleUser, Text: str(m, c.o["userTranscriptPath"])}}
	case typ == c.o["errorType"]:
		return []Event{{Kind: KindError, Type: typ, Text: str(m, c.o["errorPath"])}}
	}
	return []Event{{Kind: KindOther, Type: typ}}
}

func (c *jsonAudio) EncodeAudio(pcm []byte) ([]Frame, error) {
	chunks, err := chunkPCM(pcm, 1<<20)
	if err != nil {
		return nil, err
	}
	frames := make([]Frame, len(chunks))
	for i, ch := range chunks {
		c.vals["audio"] = base64.StdEncoding.EncodeToString(ch)
		out, err := c.sendAudio.Render(c.vals, tmpl.JSON)
		if err != nil {
			return nil, err
		}
		frames[i] = Frame{Data: []byte(out)}
	}
	return frames, nil
}

func (c *jsonAudio) EncodeText(string) ([]Frame, error) { return nil, ErrUnsupported }
func (c *jsonAudio) OnReady() []Frame                   { return nil }
func (c *jsonAudio) Interrupt(string, string, *int) ([]Frame, error) {
	return nil, ErrUnsupported
}
func (c *jsonAudio) Close() ([]Frame, bool) {
	if c.closeMsg == "" {
		return nil, false
	}
	return []Frame{{Data: []byte(c.closeMsg)}}, false
}
