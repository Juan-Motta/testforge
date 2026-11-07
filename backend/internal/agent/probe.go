package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/coder/websocket"

	"testforge/internal/agent/codec"
	"testforge/internal/agent/tmpl"
	"testforge/internal/platform/egress"
)

const (
	maxProbeEvents = 200
	closeAckWait   = 2 * time.Second
	connectTimeout = 10 * time.Second
)

// ProbeEvent is one line of the probe log.
type ProbeEvent struct {
	AtMs   int    `json:"atMs"`
	Kind   string `json:"kind"`
	Detail string `json:"detail"`
}

// ProbeResult is the outcome of a probe.
type ProbeResult struct {
	OK             bool         `json:"ok"`
	Summary        string       `json:"summary"`
	ThreadID       string       `json:"threadId"`
	TraceSessionID string       `json:"traceSessionId"`
	Events         []ProbeEvent `json:"events"`
	AudioMs        int          `json:"audioMs"`
	Transcript     string       `json:"transcript"`
	Reply          string       `json:"reply"`
}

// Prober connects to agents to check they answer.
type Prober struct {
	allowPrivate bool
}

// NewProber builds a prober whose connections follow the egress policy.
func NewProber(allowPrivate bool) *Prober { return &Prober{allowPrivate: allowPrivate} }

// probeRun holds the state of one probe.
type probeRun struct {
	start   time.Time
	redact  *tmpl.Redactor
	res     *ProbeResult
	dropped int
	vals    tmpl.Values
}

func (r *probeRun) log(kind, format string, args ...any) {
	if len(r.res.Events) >= maxProbeEvents {
		r.dropped++
		return
	}
	detail := r.redact.Redact(fmt.Sprintf(format, args...))
	r.res.Events = append(r.res.Events, ProbeEvent{AtMs: int(time.Since(r.start).Milliseconds()), Kind: kind, Detail: detail})
}

// clip redacts untrusted text before truncating it, so a secret cut at the
// limit cannot leak its first characters.
func (r *probeRun) clip(s string, n int) string { return truncate(r.redact.Redact(s), n) }

func (r *probeRun) fail(summary string) ProbeResult {
	r.log("error", "%s", summary)
	r.res.OK = false
	r.res.Summary = r.redact.Redact(summary)
	return r.finish()
}

func (r *probeRun) finish() ProbeResult {
	if r.dropped > 0 {
		r.res.Events = append(r.res.Events, ProbeEvent{AtMs: int(time.Since(r.start).Milliseconds()), Kind: "info",
			Detail: fmt.Sprintf("%d more events not shown.", r.dropped)})
	}
	if r.res.Events == nil {
		r.res.Events = []ProbeEvent{}
	}
	return *r.res
}

// Run probes the agent. It never returns an error: failures are reported in the result.
func (p *Prober) Run(ctx context.Context, a Agent, secrets map[string]string, message string, duration time.Duration) ProbeResult {
	threadID, _ := newID()
	secretValues := make([]string, 0, len(secrets))
	vals := tmpl.Values{"run.thread_id": threadID}
	for k, v := range secrets {
		vals["secret."+k] = v
		secretValues = append(secretValues, v)
	}
	for _, v := range a.Variables {
		vals["var."+v.Key] = v.Value
	}
	run := &probeRun{start: time.Now(), redact: tmpl.NewRedactor(secretValues), vals: vals,
		res: &ProbeResult{ThreadID: threadID, Events: []ProbeEvent{}}}

	if ps := a.Transport.PreSession; ps != nil {
		if err := p.preSession(ctx, run, ps); err != nil {
			return run.fail(describe(err))
		}
	}
	trace, err := render(a.TraceSessionID, vals, tmpl.Raw)
	if err != nil {
		return run.fail("Could not resolve the trace session id: " + err.Error())
	}
	run.res.TraceSessionID = trace

	if a.Modality == ModalityText {
		return p.text(ctx, run, a, message, duration)
	}
	return p.voice(ctx, run, a, duration)
}

func (p *Prober) preSession(ctx context.Context, run *probeRun, ps *PreSession) error {
	return runPreSession(ctx, egress.NewClient(p.allowPrivate, connectTimeout), ps, run.vals, func(kind, detail string) {
		run.log(kind, "%s", run.redact.Redact(detail))
	})
}

// runPreSession calls the pre-session endpoint and stores its captures in vals.
func runPreSession(ctx context.Context, client *http.Client, ps *PreSession, vals tmpl.Values, log func(kind, detail string)) error {
	target, err := buildURL(ps.URL, nil, vals)
	if err != nil {
		return err
	}
	var body io.Reader
	if ps.Body != nil && *ps.Body != "" {
		rendered, err := render(*ps.Body, vals, tmpl.JSON)
		if err != nil {
			return err
		}
		body = strings.NewReader(rendered)
	}
	ctx, cancel := context.WithTimeout(ctx, connectTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, ps.Method, target, body)
	if err != nil {
		return err
	}
	if err := setHeaders(req.Header, ps.Headers, vals); err != nil {
		return err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	log("send", "Pre-session "+ps.Method+" "+displayURL(target))
	res, err := client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	log("receive", fmt.Sprintf("Pre-session answered HTTP %d", res.StatusCode))
	if res.StatusCode >= 300 {
		return fmt.Errorf("the pre-session call answered HTTP %d", res.StatusCode)
	}
	var doc any
	if err := json.Unmarshal(raw, &doc); err != nil {
		return errors.New("the pre-session response is not JSON")
	}
	for _, c := range ps.Captures {
		v, ok := codec.Lookup(doc, c.Path)
		s, isScalar := scalar(v)
		if !ok || !isScalar {
			return fmt.Errorf("the pre-session response has no value at %q", c.Path)
		}
		vals["session."+c.Name] = s
		log("info", fmt.Sprintf("Captured {{session.%s}} = %s", c.Name, truncate(s, 80)))
	}
	return nil
}

func (p *Prober) voice(ctx context.Context, run *probeRun, a Agent, duration time.Duration) ProbeResult {
	cvals := tmpl.Values{}
	for k, v := range run.vals {
		if !strings.HasPrefix(k, "secret.") {
			cvals[k] = v
		}
	}
	vc, err := codec.NewVoice(a.Codec.Type, a.Codec.Options, cvals)
	if err != nil {
		return run.fail("The codec configuration is invalid: " + err.Error())
	}
	target, err := buildURL(a.Transport.URL, a.Transport.Query, run.vals)
	if err != nil {
		return run.fail(err.Error())
	}
	header := http.Header{}
	if err := setHeaders(header, a.Transport.Headers, run.vals); err != nil {
		return run.fail(err.Error())
	}
	inits := make([]string, 0, len(a.Transport.InitMessages))
	for _, m := range a.Transport.InitMessages {
		out, err := render(m, run.vals, tmpl.JSON)
		if err != nil {
			return run.fail(err.Error())
		}
		inits = append(inits, out)
	}

	client := egress.NewClient(p.allowPrivate, connectTimeout)
	client.Timeout = 0 // the WebSocket library uses the context instead
	dialCtx, cancelDial := context.WithTimeout(ctx, connectTimeout)
	defer cancelDial()
	conn, _, err := websocket.Dial(dialCtx, target, &websocket.DialOptions{HTTPClient: client, HTTPHeader: header})
	if err != nil {
		return run.fail(describe(err))
	}
	defer conn.CloseNow()
	conn.SetReadLimit(1 << 20)
	connectedMs := int(time.Since(run.start).Milliseconds())
	run.log("connect", "Connected to %s in %d ms", run.displayURL(target), connectedMs)

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	write := func(f codec.Frame, what string) error {
		typ := websocket.MessageText
		if f.Binary {
			typ = websocket.MessageBinary
		}
		wctx, wcancel := context.WithTimeout(ctx, 5*time.Second)
		defer wcancel()
		if err := conn.Write(wctx, typ, f.Data); err != nil {
			return err
		}
		run.log("send", "%s", what)
		return nil
	}
	for _, m := range inits {
		if err := write(codec.Frame{Data: []byte(m)}, "Initial message "+run.frameSummary([]byte(m))); err != nil {
			return run.fail(describe(err))
		}
	}

	frames := make(chan codec.Frame, 16)
	readErr := make(chan error, 1)
	go func() {
		for {
			typ, data, err := conn.Read(ctx)
			if err != nil {
				readErr <- err
				return
			}
			select {
			case frames <- codec.Frame{Binary: typ == websocket.MessageBinary, Data: data}:
			case <-ctx.Done(): // the probe finished; never block forever
				return
			}
		}
	}()

	st := &voiceState{items: map[string]string{}}
	deadline := time.NewTimer(duration)
	defer deadline.Stop()
	closed := false
loop:
	for {
		select {
		case f := <-frames:
			if p.handle(run, vc, st, f, write) {
				closed = true
				break loop
			}
		case err := <-readErr:
			var ce websocket.CloseError
			if errors.As(err, &ce) {
				run.log("close", "The agent closed the connection (%d %s)", ce.Code, ce.Reason)
			} else {
				run.log("error", "Connection lost: %s", describe(err))
				st.errors++
			}
			closed = true
			break loop
		case <-deadline.C:
			break loop
		case <-ctx.Done():
			break loop
		}
	}

	if !closed {
		closeFrames, awaitAck := vc.Close()
		for _, f := range closeFrames {
			_ = write(f, "Close message "+run.frameSummary(f.Data))
		}
		if awaitAck {
			ack := time.NewTimer(closeAckWait)
		wait:
			for {
				select {
				case f := <-frames:
					if p.handle(run, vc, st, f, write) {
						break wait
					}
				case <-readErr:
					break wait
				case <-ack.C:
					run.log("info", "No close acknowledgement within %s", closeAckWait)
					break wait
				}
			}
			ack.Stop()
		}
		_ = conn.Close(websocket.StatusNormalClosure, "probe finished")
		run.log("close", "Closed the connection")
	}

	rate := vc.SampleRate()
	run.res.AudioMs = st.audioBytes * 1000 / (2 * rate)
	run.res.Transcript = run.redact.Redact(st.transcript())
	run.res.OK = st.errors == 0 && (st.ready || st.audioBytes > 0 || run.res.Transcript != "")
	run.res.Summary = run.redact.Redact(voiceSummary(connectedMs, st, run.res))
	return run.finish()
}

type voiceState struct {
	ready       bool
	readyAt     int
	audioBytes  int
	audioChunks int
	errors      int
	items       map[string]string
	order       []string
	anon        int
}

func (s *voiceState) addTranscript(e codec.Event) {
	id := e.ItemID
	if id == "" {
		id = "_" + strconv.Itoa(s.anon)
	}
	if _, seen := s.items[id]; !seen {
		s.order = append(s.order, id)
	}
	if e.Cumulative {
		s.items[id] = e.Text
	} else {
		s.items[id] += e.Text
	}
}

func (s *voiceState) transcript() string {
	parts := make([]string, 0, len(s.order))
	for _, id := range s.order {
		if t := strings.TrimSpace(s.items[id]); t != "" {
			parts = append(parts, t)
		}
	}
	return strings.Join(parts, " ")
}

// handle processes one frame; it reports whether the session closed.
func (p *Prober) handle(run *probeRun, vc codec.Voice, st *voiceState, f codec.Frame, write func(codec.Frame, string) error) bool {
	for _, e := range vc.Decode(f) {
		switch e.Kind {
		case codec.KindReady:
			if !st.ready {
				st.ready = true
				st.readyAt = int(time.Since(run.start).Milliseconds())
			}
			run.log("ready", "Session ready (%s)", e.Type)
			for _, out := range vc.OnReady() {
				_ = write(out, "Ready message "+run.frameSummary(out.Data))
			}
		case codec.KindAudio:
			if st.audioChunks == 0 {
				run.log("audio", "Agent audio started (%s)", e.Type)
			}
			st.audioChunks++
			st.audioBytes += len(e.Audio)
		case codec.KindTranscript:
			if e.Role == codec.RoleAgent {
				st.addTranscript(e)
			}
			run.log("transcript", "%s: %s", e.Role, run.clip(e.Text, 200))
		case codec.KindError:
			st.errors++
			run.log("error", "Agent error (%s): %s", e.Type, run.clip(e.Text, 300))
		case codec.KindClosed:
			run.log("close", "Session closed by the agent (%s)", e.Type)
			return true
		case codec.KindTurnDone, codec.KindBusy:
			run.log("receive", "%s", e.Type)
		default:
			run.log("receive", "%s", run.frameSummary(f.Data))
		}
	}
	return false
}

func voiceSummary(connectedMs int, st *voiceState, res *ProbeResult) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Connected in %d ms.", connectedMs)
	if st.ready {
		fmt.Fprintf(&b, " Ready after %d ms.", st.readyAt)
	}
	if res.AudioMs > 0 {
		fmt.Fprintf(&b, " Received %.1f s of agent audio.", float64(res.AudioMs)/1000)
	} else {
		b.WriteString(" No agent audio received.")
	}
	if res.Transcript != "" {
		fmt.Fprintf(&b, " Agent said: “%s”.", truncate(res.Transcript, 120)) // res.Transcript is redacted
	}
	if st.errors > 0 {
		fmt.Fprintf(&b, " %d error(s) reported.", st.errors)
	}
	if !res.OK && st.errors == 0 {
		b.WriteString(" Nothing recognizable arrived: check the codec settings.")
	}
	return b.String()
}

func (p *Prober) text(ctx context.Context, run *probeRun, a Agent, message string, duration time.Duration) ProbeResult {
	tc, err := codec.NewText(a.Codec.Type, a.Codec.Options)
	if err != nil {
		return run.fail("The codec configuration is invalid: " + err.Error())
	}
	target, err := buildURL(a.Transport.URL, a.Transport.Query, run.vals)
	if err != nil {
		return run.fail(err.Error())
	}
	vals := tmpl.Values{"message": message}
	for k, v := range run.vals {
		vals[k] = v
	}
	body := ""
	if a.Transport.Body != nil {
		if body, err = render(*a.Transport.Body, vals, tmpl.JSON); err != nil {
			return run.fail(err.Error())
		}
	}
	timeout := duration
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, target, bytes.NewReader([]byte(body)))
	if err != nil {
		return run.fail(err.Error())
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", tc.Accept())
	if err := setHeaders(req.Header, a.Transport.Headers, run.vals); err != nil {
		return run.fail(err.Error())
	}
	run.log("send", "POST %s: %q", run.displayURL(target), run.clip(message, 200))
	res, err := egress.NewClient(p.allowPrivate, timeout).Do(req)
	if err != nil {
		return run.fail(describe(err))
	}
	defer res.Body.Close()
	run.log("receive", "HTTP %d %s", res.StatusCode, res.Header.Get("Content-Type"))
	if res.StatusCode >= 300 {
		snippet, _ := io.ReadAll(io.LimitReader(res.Body, 8192))
		return run.fail(fmt.Sprintf("The agent answered HTTP %d: %s", res.StatusCode, run.clip(strings.TrimSpace(string(snippet)), 300)))
	}
	reply, err := tc.ReadReply(res, func(kind codec.Kind, detail string) { run.log("receive", "%s", run.clip(detail, 200)) })
	run.res.Reply = run.redact.Redact(reply)
	if err != nil {
		return run.fail("Could not read the reply: " + err.Error())
	}
	elapsed := time.Since(run.start).Milliseconds()
	run.log("reply", "%s", run.clip(reply, 500))
	if strings.TrimSpace(reply) == "" {
		return run.fail("The agent answered with an empty reply.")
	}
	run.res.OK = true
	run.res.Summary = fmt.Sprintf("Replied in %d ms: “%s”.", elapsed, run.clip(reply, 120))
	return run.finish()
}

// buildURL renders a URL template (path escaped) and appends rendered query values.
func buildURL(raw string, query []KV, vals tmpl.Values) (string, error) {
	scheme, rest, _ := strings.Cut(raw, "://")
	host, path, _ := strings.Cut(rest, "/")
	renderedPath, err := render("/"+path, vals, tmpl.Path)
	if err != nil {
		return "", err
	}
	for _, seg := range strings.Split(renderedPath, "/") {
		if seg == "." || seg == ".." {
			return "", errors.New("the URL path would contain a . or .. segment")
		}
	}
	target := scheme + "://" + host + strings.TrimSuffix(renderedPath, "/")
	if renderedPath == "/" {
		target = scheme + "://" + host
	}
	if len(query) > 0 {
		q := url.Values{}
		for _, kv := range query {
			v, err := render(kv.Value, vals, tmpl.Raw)
			if err != nil {
				return "", err
			}
			q.Add(kv.Key, v)
		}
		target += "?" + q.Encode()
	}
	return target, nil
}

func setHeaders(h http.Header, headers []KV, vals tmpl.Values) error {
	for _, kv := range headers {
		v, err := render(kv.Value, vals, tmpl.Raw)
		if err != nil {
			return err
		}
		if strings.ContainsAny(v, "\r\n") {
			return fmt.Errorf("header %s would contain a line break", kv.Key)
		}
		h.Set(kv.Key, v)
	}
	return nil
}

func render(s string, vals tmpl.Values, esc tmpl.Escaper) (string, error) {
	t, err := tmpl.Parse(s)
	if err != nil {
		return "", err
	}
	return t.Render(vals, esc)
}

func scalar(v any) (string, bool) {
	switch t := v.(type) {
	case string:
		return t, true
	case float64:
		return strconv.FormatFloat(t, 'f', -1, 64), true
	case bool:
		return strconv.FormatBool(t), true
	}
	return "", false
}

// displayURL drops the query string and redacts secrets in the path.
func (r *probeRun) displayURL(raw string) string { return r.redact.Redact(displayURL(raw)) }

// frameSummary redacts a frame before summarizing it.
func (r *probeRun) frameSummary(data []byte) string {
	return frameSummary([]byte(r.redact.Redact(string(data))))
}

// displayURL drops the query string, which may carry secrets or ids.
func displayURL(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return "the agent URL"
	}
	out := u.Scheme + "://" + u.Host + u.EscapedPath()
	if n := len(u.Query()); n > 0 {
		out += fmt.Sprintf(" (+%d query parameters)", n)
	}
	return out
}

// frameSummary shows a frame's type and a short, audio-free excerpt.
func frameSummary(data []byte) string {
	var m map[string]any
	if json.Unmarshal(data, &m) != nil {
		if !utf8.Valid(data) {
			return fmt.Sprintf("binary frame (%d bytes)", len(data))
		}
		return truncate(string(data), 120)
	}
	shrink(m)
	b, _ := json.Marshal(m)
	return truncate(string(b), 200)
}

func shrink(v any) {
	switch t := v.(type) {
	case map[string]any:
		for k, child := range t {
			if s, ok := child.(string); ok && len(s) > 120 {
				t[k] = fmt.Sprintf("<%d chars>", len(s))
				continue
			}
			shrink(child)
		}
	case []any:
		for _, child := range t {
			shrink(child)
		}
	}
}

func truncate(s string, n int) string {
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	return string([]rune(s)[:n]) + "…"
}

func describe(err error) string {
	switch {
	case errors.Is(err, egress.ErrBlocked):
		return "Blocked: the host resolves to a private network address. Set ALLOW_PRIVATE_NETWORK=true only for local development."
	case errors.Is(err, context.DeadlineExceeded):
		return "The agent did not answer in time."
	}
	var urlErr *url.Error
	if errors.As(err, &urlErr) {
		err = urlErr.Err
	}
	return "Could not reach the agent: " + err.Error()
}
