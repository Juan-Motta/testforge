// Package tts turns the simulated user's words into speech: PCM16 mono at
// 24 kHz, through the persona's voice connection.
package tts

import (
	"bytes"
	"context"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"testforge/internal/connection"
	"testforge/internal/platform/egress"
)

// SampleRate of every synthesized clip.
const SampleRate = 24000

const timeout = 60 * time.Second

// Synthesizer speaks text.
type Synthesizer interface {
	Synthesize(ctx context.Context, text string) ([]byte, error)
}

// New builds a synthesizer for a persona voice.
func New(c connection.Credentials, voice, model, locale string, allowPrivate bool) (Synthesizer, error) {
	client := egress.NewClient(allowPrivate, timeout)
	switch c.Kind {
	case connection.KindOpenAI:
		if model == "" {
			model = "gpt-4o-mini-tts"
		}
		return &openAISpeech{http: client, url: c.Config["baseUrl"] + "/audio/speech", auth: "Authorization", key: "Bearer " + c.Secret, voice: voice, model: model}, nil
	case connection.KindAzureOpenAI:
		if model == "" {
			return nil, errors.New("set the Azure OpenAI TTS deployment as the persona voice model")
		}
		return &openAISpeech{http: client, auth: "Api-Key", key: c.Secret, voice: voice, model: model,
			url: c.Config["endpoint"] + "/openai/deployments/" + url.PathEscape(model) + "/audio/speech?api-version=" + url.QueryEscape(c.Config["apiVersion"])}, nil
	case connection.KindAzureSpeech:
		return &azureSpeech{http: client, key: c.Secret, voice: voice, locale: locale,
			url: "https://" + c.Config["region"] + ".tts.speech.microsoft.com/cognitiveservices/v1"}, nil
	}
	return nil, fmt.Errorf("%s connections cannot synthesize speech", c.Kind)
}

func do(client *http.Client, req *http.Request) ([]byte, error) {
	res, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("calling the speech provider: %w", err)
	}
	defer res.Body.Close()
	data, err := io.ReadAll(io.LimitReader(res.Body, 50<<20))
	if err != nil {
		return nil, err
	}
	if res.StatusCode >= 300 {
		return nil, fmt.Errorf("the speech provider answered HTTP %d", res.StatusCode)
	}
	if len(data)%2 != 0 {
		data = data[:len(data)-1]
	}
	return data, nil
}

type openAISpeech struct {
	http                         *http.Client
	url, auth, key, voice, model string
}

func (s *openAISpeech) Synthesize(ctx context.Context, text string) ([]byte, error) {
	body, _ := json.Marshal(map[string]string{"model": s.model, "voice": s.voice, "input": text, "response_format": "pcm"})
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(s.auth, s.key)
	return do(s.http, req)
}

type azureSpeech struct {
	http                    *http.Client
	url, key, voice, locale string
}

func (s *azureSpeech) Synthesize(ctx context.Context, text string) ([]byte, error) {
	var esc bytes.Buffer
	_ = xml.EscapeText(&esc, []byte(text))
	var voice bytes.Buffer
	_ = xml.EscapeText(&voice, []byte(s.voice))
	var lang bytes.Buffer
	_ = xml.EscapeText(&lang, []byte(s.locale))
	ssml := fmt.Sprintf(`<speak version="1.0" xmlns="http://www.w3.org/2001/10/synthesis" xml:lang="%s"><voice name="%s">%s</voice></speak>`,
		lang.String(), voice.String(), esc.String())
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.url, bytes.NewReader([]byte(ssml)))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/ssml+xml")
	req.Header.Set("X-Microsoft-OutputFormat", "raw-24khz-16bit-mono-pcm")
	req.Header.Set("Ocp-Apim-Subscription-Key", s.key)
	req.Header.Set("User-Agent", "TestForge")
	return do(s.http, req)
}
