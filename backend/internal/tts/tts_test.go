package tts

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"testforge/internal/connection"
)

func TestOpenAISpeech(t *testing.T) {
	var gotPath, gotAuth, gotBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		gotPath, gotAuth, gotBody = r.URL.RequestURI(), r.Header.Get("Authorization")+r.Header.Get("Api-Key"), string(b)
		_, _ = w.Write([]byte{1, 0, 2, 0})
	}))
	defer srv.Close()

	s, err := New(connection.Credentials{Kind: connection.KindOpenAI, Config: map[string]string{"baseUrl": srv.URL + "/v1"}, Secret: "sk-1"}, "alloy", "", "es-MX", true)
	if err != nil {
		t.Fatal(err)
	}
	pcm, err := s.Synthesize(context.Background(), "Hola")
	if err != nil || len(pcm) != 4 {
		t.Fatalf("pcm = %v, %v", pcm, err)
	}
	if gotPath != "/v1/audio/speech" || gotAuth != "Bearer sk-1" || !strings.Contains(gotBody, `"model":"gpt-4o-mini-tts"`) ||
		!strings.Contains(gotBody, `"response_format":"pcm"`) || !strings.Contains(gotBody, `"voice":"alloy"`) {
		t.Fatalf("request %s %s %s", gotPath, gotAuth, gotBody)
	}

	az, _ := New(connection.Credentials{Kind: connection.KindAzureOpenAI, Config: map[string]string{"endpoint": srv.URL, "apiVersion": "2025-03-01-preview"}, Secret: "az-1"}, "nova", "tts-dep", "en", true)
	if _, err := az.Synthesize(context.Background(), "Hi"); err != nil {
		t.Fatal(err)
	}
	if gotPath != "/openai/deployments/tts-dep/audio/speech?api-version=2025-03-01-preview" || gotAuth != "az-1" {
		t.Fatalf("azure request %s %s", gotPath, gotAuth)
	}
	if _, err := New(connection.Credentials{Kind: connection.KindAzureOpenAI, Config: map[string]string{"endpoint": srv.URL}}, "nova", "", "en", true); err == nil {
		t.Fatal("azure openai without deployment accepted")
	}
}

func TestAzureSpeechSSML(t *testing.T) {
	var gotBody, gotFormat, gotKey string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		gotBody, gotFormat, gotKey = string(b), r.Header.Get("X-Microsoft-OutputFormat"), r.Header.Get("Ocp-Apim-Subscription-Key")
		_, _ = w.Write([]byte{9, 0})
	}))
	defer srv.Close()
	s, err := New(connection.Credentials{Kind: connection.KindAzureSpeech, Config: map[string]string{"region": "eastus"}, Secret: "key-1"}, "es-MX-DaliaNeural", "", "es-MX", true)
	if err != nil {
		t.Fatal(err)
	}
	s.(*azureSpeech).url = srv.URL
	if _, err := s.Synthesize(context.Background(), `Tom & "Jerry" <3`); err != nil {
		t.Fatal(err)
	}
	if gotFormat != "raw-24khz-16bit-mono-pcm" || gotKey != "key-1" ||
		!strings.Contains(gotBody, `<voice name="es-MX-DaliaNeural">Tom &amp; &#34;Jerry&#34; &lt;3</voice>`) || !strings.Contains(gotBody, `xml:lang="es-MX"`) {
		t.Fatalf("ssml = %s (%s)", gotBody, gotFormat)
	}
}
