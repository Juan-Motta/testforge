package agent

// Draft is what a preset fills the editor with.
type Draft struct {
	Transport      Transport `json:"transport"`
	Codec          Codec     `json:"codec"`
	Variables      []KV      `json:"variables"`
	TraceSessionID string    `json:"traceSessionId"`
}

// Preset is a ready-made agent draft.
type Preset struct {
	ID          string   `json:"id"`
	Label       string   `json:"label"`
	Description string   `json:"description"`
	Modality    string   `json:"modality"`
	Draft       Draft    `json:"draft"`
	SecretNames []string `json:"secretNames"`
}

func str(s string) *string { return &s }

func viviVoiceTransport() Transport {
	return Transport{
		URL: "wss://your-vivi-host/channelApi/{{var.channelId}}/realtime",
		Query: []KV{
			{Key: "voiceCapability", Value: "live-v1"},
			{Key: "userId", Value: "{{var.userId}}"},
			{Key: "threadId", Value: "{{run.thread_id}}"},
		},
		Headers: []KV{}, InitMessages: []string{},
	}
}

// Presets lists the built-in drafts. Each only fills editable fields.
var Presets = []Preset{
	{
		ID: "vivi_live", Label: "Vivi Live", Modality: ModalityVoice,
		Description: "Vivi voice channel with the live protocol. The agent greets first.",
		Draft: Draft{
			Transport: viviVoiceTransport(),
			Codec:     Codec{Type: "vivi_live", Options: map[string]string{}},
			Variables: []KV{{Key: "channelId"}, {Key: "userId"}}, TraceSessionID: defaultTraceSessionID,
		},
		SecretNames: []string{},
	},
	{
		ID: "vivi_realtime", Label: "Vivi Realtime", Modality: ModalityVoice,
		Description: "Vivi voice channel speaking the OpenAI Realtime protocol, with barge-in.",
		Draft: Draft{
			Transport: viviVoiceTransport(),
			Codec:     Codec{Type: "openai_realtime", Options: map[string]string{"playbackInterrupted": "true"}},
			Variables: []KV{{Key: "channelId"}, {Key: "userId"}}, TraceSessionID: defaultTraceSessionID,
		},
		SecretNames: []string{},
	},
	{
		ID: "openai_realtime", Label: "OpenAI Realtime", Modality: ModalityVoice,
		Description: "A model on the OpenAI Realtime API, or any server that speaks its protocol.",
		Draft: Draft{
			Transport: Transport{
				URL:          "wss://api.openai.com/v1/realtime",
				Query:        []KV{{Key: "model", Value: "gpt-realtime"}},
				Headers:      []KV{{Key: "Authorization", Value: "Bearer {{secret.api_key}}"}},
				InitMessages: []string{},
			},
			Codec:     Codec{Type: "openai_realtime", Options: map[string]string{"requestResponseOnStart": "true"}},
			Variables: []KV{}, TraceSessionID: defaultTraceSessionID,
		},
		SecretNames: []string{"api_key"},
	},
	{
		ID: "vivi_text", Label: "Vivi text", Modality: ModalityText,
		Description: "Vivi chat channel over HTTP streaming.",
		Draft: Draft{
			Transport: Transport{
				URL:   "https://your-vivi-host/channelsApi/{{var.channelId}}/stream",
				Query: []KV{},
				Headers: []KV{
					{Key: "vivi-api-key", Value: "{{secret.api_key}}"},
				},
				Body:         str(`{"message":"{{message}}","threadId":"{{run.thread_id}}"}`),
				InitMessages: []string{},
			},
			Codec: Codec{Type: "http_stream", Options: map[string]string{
				"deltaWhen": "event=on_chat_model_stream", "deltaPath": "message",
				"skipWhen": "type=reasoning", "endWhen": "event=on_chat_end",
			}},
			Variables: []KV{{Key: "channelId"}}, TraceSessionID: defaultTraceSessionID,
		},
		SecretNames: []string{"api_key"},
	},
	{
		ID: "generic_json", Label: "Custom WebSocket", Modality: ModalityVoice,
		Description: "Any JSON-over-WebSocket voice protocol. Map its events in the codec.",
		Draft: Draft{
			Transport: Transport{
				URL:          "wss://agent.example.com/ws",
				Query:        []KV{{Key: "session_id", Value: "{{run.thread_id}}"}},
				Headers:      []KV{},
				InitMessages: []string{},
			},
			Codec:     Codec{Type: "json_audio", Options: map[string]string{}},
			Variables: []KV{}, TraceSessionID: defaultTraceSessionID,
		},
		SecretNames: []string{},
	},
	{
		ID: "http_json", Label: "HTTP chat", Modality: ModalityText,
		Description: "A text agent that answers each message with one JSON response.",
		Draft: Draft{
			Transport: Transport{
				URL:          "https://agent.example.com/chat",
				Query:        []KV{},
				Headers:      []KV{{Key: "Authorization", Value: "Bearer {{secret.api_key}}"}},
				Body:         str(`{"message":"{{message}}","sessionId":"{{run.thread_id}}"}`),
				InitMessages: []string{},
			},
			Codec:     Codec{Type: "http_json", Options: map[string]string{"replyPath": "reply"}},
			Variables: []KV{}, TraceSessionID: defaultTraceSessionID,
		},
		SecretNames: []string{"api_key"},
	},
}
