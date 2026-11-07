package agent

import (
	"fmt"
	"regexp"
	"strings"

	"testforge/internal/agent/codec"
	"testforge/internal/agent/tmpl"
	"testforge/internal/platform/egress"
)

var (
	namePattern   = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_]{0,39}$`)
	headerPattern = regexp.MustCompile("^[A-Za-z0-9!#$%&'*+.^_`|~-]{1,200}$")
)

// forbiddenHeaders are set by the HTTP or WebSocket client itself.
var forbiddenHeaders = map[string]bool{
	"host": true, "content-length": true, "connection": true, "upgrade": true, "transfer-encoding": true,
	"te": true, "trailer": true, "keep-alive": true, "proxy-authorization": true, "proxy-connection": true,
	"sec-websocket-key": true, "sec-websocket-version": true, "sec-websocket-extensions": true, "sec-websocket-accept": true,
}

// ref is a template reference found at a field path.
type ref struct {
	field string
	ref   tmpl.Ref
}

// validator accumulates field errors and references while checking an Input.
type validator struct {
	allowPrivate bool
	fields       map[string]string
	refs         []ref
}

func (v *validator) fail(field, msg string) {
	if _, exists := v.fields[field]; !exists {
		v.fields[field] = msg
	}
}

// template parses a template, checks its namespaces and records its references.
func (v *validator) template(field, s string, allowed ...string) (tmpl.Template, bool) {
	t, err := tmpl.Parse(s)
	if err != nil {
		v.fail(field, capitalize(err.Error())+".")
		return tmpl.Template{}, false
	}
	for _, r := range t.Refs() {
		ok := false
		for _, ns := range allowed {
			if r.NS == ns {
				ok = true
			}
		}
		if !ok {
			v.fail(field, fmt.Sprintf("{{%s}} is not available here.", r))
			return tmpl.Template{}, false
		}
		v.refs = append(v.refs, ref{field: field, ref: r})
	}
	return t, true
}

func (v *validator) jsonTemplate(field, s string, allowed ...string) {
	t, ok := v.template(field, s, allowed...)
	if ok && !codec.ValidJSONTemplate(t) {
		v.fail(field, "Must be valid JSON. Put templates inside strings, like \"{{run.thread_id}}\".")
	}
}

// agentURL checks a URL template: literal scheme and host, optional templated path.
func (v *validator) agentURL(field, raw string, schemes [2]string) {
	raw = strings.TrimSpace(raw)
	scheme, rest, ok := strings.Cut(raw, "://")
	if !ok || rest == "" {
		v.fail(field, "Enter an absolute URL, like "+schemes[0]+"://host/path.")
		return
	}
	scheme = strings.ToLower(scheme)
	switch {
	case scheme == schemes[0]:
	case scheme == schemes[1] && v.allowPrivate:
	default:
		v.fail(field, "Use a "+schemes[0]+":// URL.")
		return
	}
	host, path, _ := strings.Cut(rest, "/")
	if strings.ContainsAny(raw, "?#") {
		v.fail(field, "Remove the query string or fragment; use Query parameters instead.")
		return
	}
	if strings.Contains(host, "{{") {
		v.fail(field, "The host must be literal; templates are allowed in the path only.")
		return
	}
	check := "https"
	if scheme == "ws" || scheme == "http" {
		check = "http"
	}
	if _, err := egress.CheckURL(check+"://"+host, v.allowPrivate); err != nil {
		v.fail(field, err.Error())
		return
	}
	v.template(field, "/"+path, tmpl.NSRun, tmpl.NSVar, tmpl.NSSecret, tmpl.NSSession)
}

func (v *validator) kvs(field string, items []KV, headers bool, allowed ...string) {
	for i, kv := range items {
		f := fmt.Sprintf("%s.%d", field, i)
		key := strings.TrimSpace(kv.Key)
		switch {
		case key == "":
			v.fail(f+".key", "Required.")
		case headers && !headerPattern.MatchString(key):
			v.fail(f+".key", "Use a valid header name.")
		case headers && (forbiddenHeaders[strings.ToLower(key)] || strings.HasPrefix(strings.ToLower(key), "proxy-")):
			v.fail(f+".key", "This header is set automatically.")
		case len(key) > 200:
			v.fail(f+".key", "Use at most 200 characters.")
		}
		if len(kv.Value) > 4096 {
			v.fail(f+".value", "Use at most 4096 characters.")
			continue
		}
		if headers && strings.ContainsAny(kv.Value, "\r\n") {
			v.fail(f+".value", "Header values cannot contain line breaks.")
			continue
		}
		v.template(f+".value", kv.Value, allowed...)
	}
}

// validateInput checks everything that does not need the database and
// normalizes the input in place (trimmed names, defaults, empty slices).
func validateInput(in *Input, allowPrivate bool) map[string]string {
	v := &validator{allowPrivate: allowPrivate, fields: map[string]string{}}
	normalize(in)

	if in.Name == "" || len([]rune(in.Name)) > 100 {
		v.fail("name", "Enter a name of 1 to 100 characters.")
	}
	if len([]rune(in.Description)) > 20000 {
		v.fail("description", "Use at most 20000 characters.")
	}
	voice := in.Modality == ModalityVoice
	if !voice && in.Modality != ModalityText {
		v.fail("modality", "Choose voice or text.")
		return v.fields
	}

	tr := in.Transport
	all := []string{tmpl.NSRun, tmpl.NSVar, tmpl.NSSecret, tmpl.NSSession}
	if voice {
		v.agentURL("transport.url", tr.URL, [2]string{"wss", "ws"})
	} else {
		v.agentURL("transport.url", tr.URL, [2]string{"https", "http"})
	}
	v.kvs("transport.query", tr.Query, false, all...)
	v.kvs("transport.headers", tr.Headers, true, all...)

	switch {
	case voice && tr.Body != nil:
		v.fail("transport.body", "Voice agents do not send a request body.")
	case !voice && (tr.Body == nil || strings.TrimSpace(*tr.Body) == ""):
		v.fail("transport.body", "Required: a JSON body that contains {{message}}.")
	case !voice:
		v.jsonTemplate("transport.body", *tr.Body, append(all, tmpl.NSMessage)...)
		if _, bad := v.fields["transport.body"]; !bad && !strings.Contains(*tr.Body, "{{message}}") && !strings.Contains(*tr.Body, "{{ message }}") {
			v.fail("transport.body", "Must contain {{message}}.")
		}
	}
	switch {
	case !voice && len(tr.InitMessages) > 0:
		v.fail("transport.initMessages", "Text agents do not send initial messages.")
	case len(tr.InitMessages) > 10:
		v.fail("transport.initMessages", "Use at most 10 initial messages.")
	default:
		for i, m := range tr.InitMessages {
			v.jsonTemplate(fmt.Sprintf("transport.initMessages.%d", i), m, all...)
		}
	}

	captures := map[string]bool{}
	if ps := tr.PreSession; ps != nil {
		if ps.Method != "GET" && ps.Method != "POST" {
			v.fail("transport.preSession.method", "Use GET or POST.")
		}
		v.agentURL("transport.preSession.url", ps.URL, [2]string{"https", "http"})
		noSession := []string{tmpl.NSRun, tmpl.NSVar, tmpl.NSSecret}
		v.kvs("transport.preSession.headers", ps.Headers, true, noSession...)
		if ps.Body != nil && *ps.Body != "" {
			if ps.Method == "GET" {
				v.fail("transport.preSession.body", "GET requests have no body.")
			} else {
				v.jsonTemplate("transport.preSession.body", *ps.Body, noSession...)
			}
		}
		for i, c := range ps.Captures {
			f := fmt.Sprintf("transport.preSession.captures.%d", i)
			switch {
			case !namePattern.MatchString(c.Name):
				v.fail(f+".name", "Use letters, digits and underscores, starting with a letter.")
			case captures[c.Name]:
				v.fail(f+".name", "Already used.")
			}
			captures[c.Name] = true
			if c.Path == "" || len(c.Path) > 200 {
				v.fail(f+".path", "Enter a dot path, like data.session_id.")
			}
		}
	}

	spec, ok := codec.SpecFor(in.Codec.Type)
	switch {
	case !ok:
		v.fail("codec.type", "Choose a supported codec.")
	case spec.Modality != in.Modality:
		v.fail("codec.type", "This codec is for "+spec.Modality+" agents.")
	default:
		opts, err := codec.Options(in.Codec.Type, in.Codec.Options)
		if oe, isOptErr := err.(*codec.OptionError); isOptErr {
			for k, msg := range oe.Fields {
				v.fail("codec.options."+k, msg)
			}
		} else {
			in.Codec.Options = opts
			for _, f := range spec.Fields {
				if f.Type == codec.FieldJSON && opts[f.Key] != "" {
					v.template("codec.options."+f.Key, opts[f.Key], tmpl.NSRun, tmpl.NSVar, tmpl.NSSession, tmpl.NSAudio)
				}
			}
		}
	}

	vars := map[string]bool{}
	for i, kv := range in.Variables {
		f := fmt.Sprintf("variables.%d", i)
		switch {
		case !namePattern.MatchString(kv.Key):
			v.fail(f+".key", "Use letters, digits and underscores, starting with a letter.")
		case vars[kv.Key]:
			v.fail(f+".key", "Already used.")
		}
		vars[kv.Key] = true
		if len(kv.Value) > 4096 {
			v.fail(f+".value", "Use at most 4096 characters.")
		} else if strings.Contains(kv.Value, "{{") {
			v.fail(f+".value", "Variables are plain values; templates are not expanded here.")
		}
	}
	if len(in.Variables) > 50 {
		v.fail("variables", "Use at most 50 variables.")
	}

	secrets := map[string]bool{}
	for i, s := range in.Secrets {
		f := fmt.Sprintf("secrets.%d", i)
		switch {
		case !namePattern.MatchString(s.Name):
			v.fail(f+".name", "Use letters, digits and underscores, starting with a letter.")
		case secrets[s.Name]:
			v.fail(f+".name", "Already used.")
		}
		secrets[s.Name] = true
		if len(s.Value) > 4096 {
			v.fail(f+".value", "Use at most 4096 characters.")
		}
	}
	if len(in.Secrets) > 20 {
		v.fail("secrets", "Use at most 20 secrets.")
	}

	if len(in.TraceSessionID) > 500 {
		v.fail("traceSessionId", "Use at most 500 characters.")
	} else {
		v.template("traceSessionId", in.TraceSessionID, tmpl.NSRun, tmpl.NSVar, tmpl.NSSession)
	}

	for _, r := range v.refs {
		switch r.ref.NS {
		case tmpl.NSVar:
			if !vars[r.ref.Name] {
				v.fail(r.field, fmt.Sprintf("{{%s}} is not defined. Add it under Variables.", r.ref))
			}
		case tmpl.NSSecret:
			if !secrets[r.ref.Name] {
				v.fail(r.field, fmt.Sprintf("{{%s}} is not defined. Add it under Secrets.", r.ref))
			}
		case tmpl.NSSession:
			if !captures[r.ref.Name] {
				v.fail(r.field, fmt.Sprintf("{{%s}} is not captured by the pre-session call.", r.ref))
			}
		}
	}
	return v.fields
}

func normalize(in *Input) {
	in.Name = strings.TrimSpace(in.Name)
	in.Transport.URL = strings.TrimSpace(in.Transport.URL)
	if in.Transport.Query == nil {
		in.Transport.Query = []KV{}
	}
	if in.Transport.Headers == nil {
		in.Transport.Headers = []KV{}
	}
	if in.Transport.InitMessages == nil {
		in.Transport.InitMessages = []string{}
	}
	if in.Transport.Body != nil && strings.TrimSpace(*in.Transport.Body) == "" {
		in.Transport.Body = nil
	}
	if ps := in.Transport.PreSession; ps != nil {
		ps.URL = strings.TrimSpace(ps.URL)
		ps.Method = strings.ToUpper(strings.TrimSpace(ps.Method))
		if ps.Headers == nil {
			ps.Headers = []KV{}
		}
		if ps.Captures == nil {
			ps.Captures = []Capture{}
		}
	}
	for i := range in.Transport.Query {
		in.Transport.Query[i].Key = strings.TrimSpace(in.Transport.Query[i].Key)
	}
	for i := range in.Transport.Headers {
		in.Transport.Headers[i].Key = strings.TrimSpace(in.Transport.Headers[i].Key)
	}
	if in.Codec.Options == nil {
		in.Codec.Options = map[string]string{}
	}
	if in.Variables == nil {
		in.Variables = []KV{}
	}
	for i := range in.Variables {
		in.Variables[i].Key = strings.TrimSpace(in.Variables[i].Key)
	}
	if in.Secrets == nil {
		in.Secrets = []SecretInput{}
	}
	for i := range in.Secrets {
		in.Secrets[i].Name = strings.TrimSpace(in.Secrets[i].Name)
		in.Secrets[i].Value = strings.TrimSpace(in.Secrets[i].Value)
	}
	in.TraceSessionID = strings.TrimSpace(in.TraceSessionID)
	if in.TraceSessionID == "" {
		in.TraceSessionID = defaultTraceSessionID
	}
}

func capitalize(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}
