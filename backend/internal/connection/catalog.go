package connection

import (
	"regexp"
	"strings"

	"testforge/internal/platform/egress"
)

// Kind identifies a provider.
type Kind string

const (
	KindOpenAI      Kind = "openai"
	KindAnthropic   Kind = "anthropic"
	KindAzureOpenAI Kind = "azure_openai"
	KindAzureSpeech Kind = "azure_speech"
	KindLangfuse    Kind = "langfuse"
)

// Capability is something a connection can be used for.
type Capability string

const (
	CapLLM      Capability = "llm"
	CapTTS      Capability = "tts"
	CapSTT      Capability = "stt"
	CapRealtime Capability = "realtime"
	CapTraces   Capability = "traces"
)

// FieldType is how a config field is entered and validated.
type FieldType string

const (
	FieldText FieldType = "text"
	FieldURL  FieldType = "url"
)

// Field describes one non-secret config value.
type Field struct {
	Key         string    `json:"key"`
	Label       string    `json:"label"`
	Type        FieldType `json:"type"`
	Required    bool      `json:"required"`
	Default     *string   `json:"default"`
	Placeholder *string   `json:"placeholder"`
	Help        *string   `json:"help"`
	Pattern     *string   `json:"pattern"`
	// PatternMessage explains a pattern mismatch; not part of the public catalog.
	PatternMessage string `json:"-"`
}

// KindSpec is a catalog entry.
type KindSpec struct {
	Kind         Kind         `json:"kind"`
	Label        string       `json:"label"`
	Description  string       `json:"description"`
	Capabilities []Capability `json:"capabilities"`
	Fields       []Field      `json:"fields"`
	SecretLabel  string       `json:"secretLabel"`
	DocsURL      string       `json:"docsUrl"`
}

func ptr(s string) *string { return &s }

// Catalog lists the supported kinds in display order.
var Catalog = []KindSpec{
	{
		Kind:         KindOpenAI,
		Label:        "OpenAI",
		Description:  "GPT models for simulated users and judging, plus speech and realtime voice.",
		Capabilities: []Capability{CapLLM, CapTTS, CapSTT, CapRealtime},
		Fields: []Field{
			{Key: "baseUrl", Label: "API base URL", Type: FieldURL, Default: ptr("https://api.openai.com/v1"),
				Help: ptr("Change only for a proxy or an OpenAI-compatible gateway.")},
			{Key: "organization", Label: "Organization ID", Type: FieldText, Placeholder: ptr("org-..."),
				Help: ptr("Optional. Bills usage to a specific organization."), Pattern: ptr(`^org-[A-Za-z0-9]+$`),
				PatternMessage: "Organization IDs start with org-."},
		},
		SecretLabel: "API key",
		DocsURL:     "https://platform.openai.com/api-keys",
	},
	{
		Kind:         KindAnthropic,
		Label:        "Anthropic",
		Description:  "Claude models for simulated users and judging.",
		Capabilities: []Capability{CapLLM},
		Fields: []Field{
			{Key: "baseUrl", Label: "API base URL", Type: FieldURL, Default: ptr("https://api.anthropic.com"),
				Help: ptr("Change only for a proxy or gateway.")},
		},
		SecretLabel: "API key",
		DocsURL:     "https://console.anthropic.com/settings/keys",
	},
	{
		Kind:         KindAzureOpenAI,
		Label:        "Azure OpenAI",
		Description:  "OpenAI models deployed in your Azure subscription.",
		Capabilities: []Capability{CapLLM, CapTTS, CapSTT, CapRealtime},
		Fields: []Field{
			{Key: "endpoint", Label: "Endpoint", Type: FieldURL, Required: true, Placeholder: ptr("https://my-resource.openai.azure.com")},
			{Key: "apiVersion", Label: "API version", Type: FieldText, Required: true, Default: ptr("2024-10-21"),
				Pattern: ptr(`^\d{4}-\d{2}-\d{2}(-preview)?$`), PatternMessage: "Use a date like 2024-10-21, optionally ending in -preview."},
		},
		SecretLabel: "API key",
		DocsURL:     "https://learn.microsoft.com/azure/ai-services/openai/quickstart",
	},
	{
		Kind:         KindAzureSpeech,
		Label:        "Azure AI Speech",
		Description:  "Text to speech and speech to text with Azure neural voices.",
		Capabilities: []Capability{CapTTS, CapSTT},
		Fields: []Field{
			{Key: "region", Label: "Region", Type: FieldText, Required: true, Placeholder: ptr("eastus"),
				Pattern: ptr(`^[a-z0-9]{1,40}$`), PatternMessage: "Use the region identifier, like eastus or westeurope."},
		},
		SecretLabel: "Resource key",
		DocsURL:     "https://learn.microsoft.com/azure/ai-services/speech-service/overview",
	},
	{
		Kind:         KindLangfuse,
		Label:        "Langfuse",
		Description:  "Pull your agent's traces to evaluate latency, tool calls and errors.",
		Capabilities: []Capability{CapTraces},
		Fields: []Field{
			{Key: "host", Label: "Host", Type: FieldURL, Required: true, Default: ptr("https://cloud.langfuse.com"),
				Help: ptr("Use https://us.cloud.langfuse.com for the US region, or your self-hosted URL.")},
			{Key: "publicKey", Label: "Public key", Type: FieldText, Required: true, Placeholder: ptr("pk-lf-..."),
				Pattern: ptr(`^pk-lf-\S+$`), PatternMessage: "Langfuse public keys start with pk-lf-."},
		},
		SecretLabel: "Secret key",
		DocsURL:     "https://langfuse.com/faq/all/where-are-langfuse-api-keys",
	},
}

// Spec returns the catalog entry for kind.
func Spec(kind Kind) (KindSpec, bool) {
	for _, s := range Catalog {
		if s.Kind == kind {
			return s, true
		}
	}
	return KindSpec{}, false
}

var patternCache = map[string]*regexp.Regexp{}

func init() {
	for _, s := range Catalog {
		for _, f := range s.Fields {
			if f.Pattern != nil {
				patternCache[*f.Pattern] = regexp.MustCompile(*f.Pattern)
			}
		}
	}
}

// normalizeConfig applies defaults, trims values, validates against the spec
// and returns the stored config. Field errors use `config.<key>` names.
func normalizeConfig(spec KindSpec, in map[string]string, allowPrivate bool, fields map[string]string) map[string]string {
	known := map[string]bool{}
	out := map[string]string{}
	for _, f := range spec.Fields {
		known[f.Key] = true
		name := "config." + f.Key
		v := strings.TrimSpace(in[f.Key])
		if v == "" && f.Default != nil {
			v = *f.Default
		}
		if v == "" {
			if f.Required {
				fields[name] = "Required."
			}
			continue
		}
		switch f.Type {
		case FieldURL:
			normalized, err := egress.CheckURL(v, allowPrivate)
			if err != nil {
				fields[name] = err.Error()
				continue
			}
			v = normalized
		case FieldText:
			if len(v) > 500 {
				fields[name] = "Use at most 500 characters."
				continue
			}
			if f.Pattern != nil && !patternCache[*f.Pattern].MatchString(v) {
				fields[name] = f.PatternMessage
				continue
			}
		}
		out[f.Key] = v
	}
	for k := range in {
		if !known[k] {
			fields["config."+k] = "Unknown setting for this provider."
		}
	}
	return out
}
