// Package agent manages the agents under test: a generic transport (WebSocket
// or HTTP) plus a codec, with templated requests, sealed secrets and a probe
// that checks the agent answers.
package agent

import "time"

// Modalities.
const (
	ModalityVoice = "voice"
	ModalityText  = "text"
)

// KV is an editable key/value pair whose value may be a template.
type KV struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// Capture copies a field of the pre-session response into {{session.<name>}}.
type Capture struct {
	Name string `json:"name"`
	Path string `json:"path"`
}

// PreSession is an optional HTTP call made before connecting.
type PreSession struct {
	Method   string    `json:"method"`
	URL      string    `json:"url"`
	Headers  []KV      `json:"headers"`
	Body     *string   `json:"body"`
	Captures []Capture `json:"captures"`
}

// Transport is how to reach the agent.
type Transport struct {
	URL          string      `json:"url"`
	Query        []KV        `json:"query"`
	Headers      []KV        `json:"headers"`
	Body         *string     `json:"body"`
	InitMessages []string    `json:"initMessages"`
	PreSession   *PreSession `json:"preSession"`
}

// Codec selects the wire protocol.
type Codec struct {
	Type    string            `json:"type"`
	Options map[string]string `json:"options"`
}

// SecretInput is a secret in a create or update request.
type SecretInput struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// Input is the create/update request.
type Input struct {
	Name              string        `json:"name"`
	Description       string        `json:"description"`
	Modality          string        `json:"modality"`
	Transport         Transport     `json:"transport"`
	Codec             Codec         `json:"codec"`
	Variables         []KV          `json:"variables"`
	Secrets           []SecretInput `json:"secrets"`
	TraceConnectionID *string       `json:"traceConnectionId"`
	TraceSessionID    string        `json:"traceSessionId"`
}

// SecretInfo is the public view of a stored secret.
type SecretInfo struct {
	Name string `json:"name"`
	Hint string `json:"hint"`
}

// Creator identifies who created an agent.
type Creator struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// Agent is the public view of a stored agent (no secret values).
type Agent struct {
	ID                string       `json:"id"`
	Name              string       `json:"name"`
	Description       string       `json:"description"`
	Modality          string       `json:"modality"`
	Transport         Transport    `json:"transport"`
	Codec             Codec        `json:"codec"`
	Variables         []KV         `json:"variables"`
	Secrets           []SecretInfo `json:"secrets"`
	TraceConnectionID *string      `json:"traceConnectionId"`
	TraceSessionID    string       `json:"traceSessionId"`
	CreatedBy         *Creator     `json:"createdBy"`
	CreatedAt         time.Time    `json:"createdAt"`
	UpdatedAt         time.Time    `json:"updatedAt"`
}

// storedSecret is one element of the agents.secrets column.
type storedSecret struct {
	Name       string `json:"name"`
	Ciphertext string `json:"ciphertext"`
	Hint       string `json:"hint"`
}

const defaultTraceSessionID = "{{run.thread_id}}"
