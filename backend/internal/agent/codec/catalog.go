// Package codec turns an agent's wire messages into normalized events and
// builds the messages the runner sends. Voice codecs work on WebSocket
// frames; text codecs read HTTP responses.
package codec

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"testforge/internal/agent/tmpl"
)

// FieldType is how an option is entered and validated.
type FieldType string

const (
	FieldText    FieldType = "text"
	FieldNumber  FieldType = "number"
	FieldBoolean FieldType = "boolean"
	FieldJSON    FieldType = "json"
)

// Field describes one codec option.
type Field struct {
	Key         string    `json:"key"`
	Label       string    `json:"label"`
	Type        FieldType `json:"type"`
	Required    bool      `json:"required"`
	Default     *string   `json:"default"`
	Placeholder *string   `json:"placeholder"`
	Help        *string   `json:"help"`
	// AllowAudio lets a JSON template reference {{audio}}; RequireAudio demands it.
	AllowAudio   bool `json:"-"`
	RequireAudio bool `json:"-"`
}

// Spec is a catalog entry.
type Spec struct {
	Codec       string  `json:"codec"`
	Label       string  `json:"label"`
	Description string  `json:"description"`
	Modality    string  `json:"modality"`
	Fields      []Field `json:"fields"`
}

func ptr(s string) *string { return &s }

var sampleRate = Field{Key: "sampleRate", Label: "Sample rate (Hz)", Type: FieldNumber, Required: true, Default: ptr("24000"),
	Help: ptr("PCM16 mono, base64 in JSON. Both directions use this rate.")}

// Catalog lists the codecs.
var Catalog = []Spec{
	{
		Codec: "vivi_live", Label: "Vivi Live", Modality: "voice",
		Description: "Vivi voice.* events. The agent speaks first; turns end on silence.",
		Fields:      []Field{sampleRate},
	},
	{
		Codec: "openai_realtime", Label: "OpenAI Realtime", Modality: "voice",
		Description: "OpenAI Realtime events: server VAD, response events, truncation on barge-in.",
		Fields: []Field{
			sampleRate,
			{Key: "requestResponseOnStart", Label: "Ask the agent to speak first", Type: FieldBoolean, Required: true, Default: ptr("false"),
				Help: ptr("Sends response.create once the session is ready.")},
			{Key: "playbackInterrupted", Label: "Vivi playback.interrupted extension", Type: FieldBoolean, Required: true, Default: ptr("false"),
				Help: ptr("On barge-in send playback.interrupted (Vivi relay) instead of conversation.item.truncate.")},
		},
	},
	{
		Codec: "json_audio", Label: "Generic JSON", Modality: "voice",
		Description: "Any JSON protocol: map event types and fields with dot paths.",
		Fields: []Field{
			sampleRate,
			{Key: "typeField", Label: "Event type field", Type: FieldText, Required: true, Default: ptr("type")},
			{Key: "readyType", Label: "Ready event", Type: FieldText, Default: ptr("ready"), Help: ptr("Leave empty if the agent sends no ready event.")},
			{Key: "audioType", Label: "Agent audio event", Type: FieldText, Required: true, Default: ptr("audio")},
			{Key: "audioPath", Label: "Agent audio field", Type: FieldText, Required: true, Default: ptr("audio"), Help: ptr("Dot path to the base64 PCM16 audio.")},
			{Key: "agentTranscriptType", Label: "Agent transcript event", Type: FieldText, Default: ptr("agent_transcript")},
			{Key: "agentTranscriptPath", Label: "Agent transcript field", Type: FieldText, Default: ptr("delta")},
			{Key: "userTranscriptType", Label: "User transcript event", Type: FieldText, Default: ptr("user_transcript")},
			{Key: "userTranscriptPath", Label: "User transcript field", Type: FieldText, Default: ptr("delta")},
			{Key: "errorType", Label: "Error event", Type: FieldText, Default: ptr("error")},
			{Key: "errorPath", Label: "Error message field", Type: FieldText, Default: ptr("message")},
			{Key: "sendAudio", Label: "Send audio message", Type: FieldJSON, Required: true, Default: ptr(`{"type":"audio","audio":"{{audio}}"}`),
				AllowAudio: true, RequireAudio: true, Help: ptr("JSON template for each chunk of caller audio; must contain {{audio}}.")},
			{Key: "closeMessage", Label: "Close message", Type: FieldJSON, Default: ptr(`{"type":"stop"}`),
				Help: ptr("Sent before closing. Leave empty to just close the socket.")},
		},
	},
	{
		Codec: "http_json", Label: "HTTP JSON", Modality: "text",
		Description: "One JSON response per message.",
		Fields: []Field{
			{Key: "replyPath", Label: "Reply field", Type: FieldText, Required: true, Default: ptr("message"), Help: ptr("Dot path to the reply text, e.g. data.reply.")},
		},
	},
	{
		Codec: "http_stream", Label: "HTTP stream", Modality: "text",
		Description: "Streamed lines of data: <json> (SSE-style), joined into one reply.",
		Fields: []Field{
			{Key: "deltaPath", Label: "Delta field", Type: FieldText, Required: true, Default: ptr("message"), Help: ptr("Dot path to the text delta, e.g. choices.0.delta.content.")},
			{Key: "deltaWhen", Label: "Delta when", Type: FieldText, Placeholder: ptr("event=on_chat_model_stream"), Help: ptr("Only lines where path=value count as deltas. Empty: every line.")},
			{Key: "skipWhen", Label: "Skip when", Type: FieldText, Placeholder: ptr("type=reasoning"), Help: ptr("Ignore lines where path=value.")},
			{Key: "endWhen", Label: "End when", Type: FieldText, Placeholder: ptr("event=on_chat_end"), Help: ptr("The reply is complete at this line. Empty: when the stream closes or sends [DONE].")},
		},
	},
}

// SpecFor returns the catalog entry of a codec.
func SpecFor(codec string) (Spec, bool) {
	for _, s := range Catalog {
		if s.Codec == codec {
			return s, true
		}
	}
	return Spec{}, false
}

// OptionError carries per-option validation messages keyed by option name.
type OptionError struct {
	Fields map[string]string
}

func (e *OptionError) Error() string {
	keys := make([]string, 0, len(e.Fields))
	for k := range e.Fields {
		keys = append(keys, k+": "+e.Fields[k])
	}
	sort.Strings(keys)
	return "invalid codec options: " + strings.Join(keys, "; ")
}

// Options validates options against the codec spec and applies defaults.
func Options(codec string, in map[string]string) (map[string]string, error) {
	spec, ok := SpecFor(codec)
	if !ok {
		return nil, fmt.Errorf("unknown codec %q", codec)
	}
	out := map[string]string{}
	bad := map[string]string{}
	known := map[string]bool{}
	for _, f := range spec.Fields {
		known[f.Key] = true
		v, set := in[f.Key]
		v = strings.TrimSpace(v)
		if !set && f.Default != nil {
			v = *f.Default
		}
		if v == "" {
			if f.Required {
				bad[f.Key] = "Required."
			}
			out[f.Key] = ""
			continue
		}
		if msg := checkValue(f, v); msg != "" {
			bad[f.Key] = msg
			continue
		}
		out[f.Key] = v
	}
	for k := range in {
		if !known[k] {
			bad[k] = "Unknown option for this codec."
		}
	}
	if len(bad) > 0 {
		return nil, &OptionError{Fields: bad}
	}
	return out, nil
}

func checkValue(f Field, v string) string {
	switch f.Type {
	case FieldNumber:
		n, err := strconv.Atoi(v)
		if err != nil || n < 8000 || n > 48000 {
			return "Use a whole number between 8000 and 48000."
		}
	case FieldBoolean:
		if v != "true" && v != "false" {
			return "Use true or false."
		}
	case FieldJSON:
		t, err := tmpl.Parse(v)
		if err != nil {
			return err.Error()
		}
		hasAudio := false
		for _, r := range t.Refs() {
			switch r.NS {
			case tmpl.NSAudio:
				if !f.AllowAudio {
					return "{{audio}} is not available here."
				}
				hasAudio = true
			case tmpl.NSRun, tmpl.NSVar, tmpl.NSSession:
			default:
				return fmt.Sprintf("{{%s}} is not available here.", r)
			}
		}
		if f.RequireAudio && !hasAudio {
			return "Must contain {{audio}}."
		}
		if !ValidJSONTemplate(t) {
			return "Must be valid JSON (templates go inside strings)."
		}
	case FieldText:
		if len(v) > 500 {
			return "Use at most 500 characters."
		}
	}
	return ""
}

// ValidJSONTemplate reports whether t renders to valid JSON for any values.
func ValidJSONTemplate(t tmpl.Template) bool {
	vals := tmpl.Values{}
	for _, r := range t.Refs() {
		vals[r.String()] = `x"\` // escaping must keep the document valid
	}
	out, err := t.Render(vals, tmpl.JSON)
	return err == nil && json.Valid([]byte(out))
}
