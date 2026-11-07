package connection

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"testforge/internal/platform/egress"
)

const testTimeout = 10 * time.Second

// TestResult is the outcome of a connection test.
type TestResult struct {
	OK        bool      `json:"ok"`
	Message   string    `json:"message"`
	LatencyMs int       `json:"latencyMs"`
	TestedAt  time.Time `json:"testedAt"`
}

// Tester checks credentials against each provider with a cheap,
// read-only request.
type Tester struct {
	client *http.Client
	// speechURL builds the Azure Speech token endpoint; tests override it.
	speechURL func(region string) string
}

// NewTester builds a tester whose outbound calls follow the egress policy.
func NewTester(allowPrivate bool) *Tester {
	return &Tester{
		client: egress.NewClient(allowPrivate, testTimeout),
		speechURL: func(region string) string {
			return "https://" + region + ".api.cognitive.microsoft.com/sts/v1.0/issueToken"
		},
	}
}

type listResponse struct {
	Data []struct {
		Name string `json:"name"`
	} `json:"data"`
}

// Test runs the provider check. It never returns the secret in the message.
func (t *Tester) Test(ctx context.Context, kind Kind, cfg map[string]string, secret string) TestResult {
	ctx, cancel := context.WithTimeout(ctx, testTimeout)
	defer cancel()

	start := time.Now()
	ok, msg := t.check(ctx, kind, cfg, secret)
	return TestResult{OK: ok, Message: msg, LatencyMs: int(time.Since(start).Milliseconds()), TestedAt: time.Now().UTC()}
}

func (t *Tester) check(ctx context.Context, kind Kind, cfg map[string]string, secret string) (bool, string) {
	var req *http.Request
	var err error
	switch kind {
	case KindOpenAI:
		req, err = http.NewRequestWithContext(ctx, http.MethodGet, cfg["baseUrl"]+"/models", nil)
		if err == nil {
			req.Header.Set("Authorization", "Bearer "+secret)
			if org := cfg["organization"]; org != "" {
				req.Header.Set("OpenAI-Organization", org)
			}
		}
	case KindAnthropic:
		req, err = http.NewRequestWithContext(ctx, http.MethodGet, cfg["baseUrl"]+"/v1/models", nil)
		if err == nil {
			req.Header.Set("X-Api-Key", secret)
			req.Header.Set("Anthropic-Version", "2023-06-01")
		}
	case KindAzureOpenAI:
		q := url.Values{"api-version": {cfg["apiVersion"]}}
		req, err = http.NewRequestWithContext(ctx, http.MethodGet, cfg["endpoint"]+"/openai/models?"+q.Encode(), nil)
		if err == nil {
			req.Header.Set("Api-Key", secret)
		}
	case KindAzureSpeech:
		req, err = http.NewRequestWithContext(ctx, http.MethodPost, t.speechURL(cfg["region"]), http.NoBody)
		if err == nil {
			req.Header.Set("Ocp-Apim-Subscription-Key", secret)
		}
	case KindLangfuse:
		req, err = http.NewRequestWithContext(ctx, http.MethodGet, cfg["host"]+"/api/public/projects", nil)
		if err == nil {
			req.Header.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte(cfg["publicKey"]+":"+secret)))
		}
	default:
		return false, "This provider cannot be tested."
	}
	if err != nil {
		return false, "The configured URL is not valid."
	}

	res, err := t.client.Do(req)
	if err != nil {
		return false, describeTransportError(err)
	}
	defer res.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(res.Body, 1<<20))

	switch {
	case res.StatusCode == http.StatusUnauthorized || res.StatusCode == http.StatusForbidden:
		return false, fmt.Sprintf("The provider rejected the credentials (HTTP %d).", res.StatusCode)
	case res.StatusCode == http.StatusNotFound:
		return false, "The provider answered HTTP 404. Check the URL, region or API version."
	case res.StatusCode >= 300:
		return false, fmt.Sprintf("The provider answered HTTP %d.", res.StatusCode)
	}

	switch kind {
	case KindAzureSpeech:
		return true, "Connected. The key can issue speech tokens."
	case KindLangfuse:
		var lr listResponse
		if json.Unmarshal(body, &lr) == nil && len(lr.Data) > 0 && lr.Data[0].Name != "" {
			return true, fmt.Sprintf("Connected to project %q.", lr.Data[0].Name)
		}
		return true, "Connected to Langfuse."
	default:
		var lr listResponse
		if json.Unmarshal(body, &lr) == nil && len(lr.Data) > 0 {
			return true, fmt.Sprintf("Connected. %d models available.", len(lr.Data))
		}
		return true, "Connected."
	}
}

func describeTransportError(err error) string {
	if errors.Is(err, egress.ErrBlocked) {
		return "Blocked: the host resolves to a private network address. Set ALLOW_PRIVATE_NETWORK=true only for local development."
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return "The provider did not answer within 10 seconds."
	}
	var urlErr *url.Error
	if errors.As(err, &urlErr) {
		err = urlErr.Err
	}
	return "Could not reach the provider: " + err.Error()
}
