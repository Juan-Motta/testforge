// Package tmpl parses and renders the small template language used in agent
// transports: `{{run.thread_id}}`, `{{var.<key>}}`, `{{secret.<name>}}`,
// `{{session.<name>}}`, `{{message}}` and `{{audio}}`.
//
// Rendering is context-aware: values are JSON-string escaped inside JSON
// templates and path-escaped inside URL paths, so user data can never change
// the structure of a request.
package tmpl

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"strings"
)

// Namespaces a reference can use.
const (
	NSRun     = "run"
	NSVar     = "var"
	NSSecret  = "secret"
	NSSession = "session"
	NSMessage = "message"
	NSAudio   = "audio"
)

var (
	exprPattern = regexp.MustCompile(`\{\{\s*([a-z_]+)(?:\.([A-Za-z][A-Za-z0-9_]{0,39}))?\s*\}\}`)
	namePattern = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_]{0,39}$`)
)

// Ref is one reference inside a template.
type Ref struct {
	NS   string
	Name string
}

func (r Ref) String() string {
	if r.Name == "" {
		return r.NS
	}
	return r.NS + "." + r.Name
}

type part struct {
	literal string
	ref     *Ref
}

// Template is a parsed template.
type Template struct {
	parts []part
}

// Parse validates the syntax and the namespaces of every expression.
func Parse(s string) (Template, error) {
	var t Template
	rest := s
	for {
		loc := exprPattern.FindStringSubmatchIndex(rest)
		before := rest
		if loc != nil {
			before = rest[:loc[0]]
		}
		// "}}" is ordinary text (nested JSON objects end with it); a stray "{{" is a typo.
		if strings.Contains(before, "{{") {
			return Template{}, errors.New("contains an unrecognized {{...}} expression")
		}
		if before != "" {
			t.parts = append(t.parts, part{literal: before})
		}
		if loc == nil {
			return t, nil
		}
		ns := rest[loc[2]:loc[3]]
		name := ""
		if loc[4] >= 0 {
			name = rest[loc[4]:loc[5]]
		}
		ref := Ref{NS: ns, Name: name}
		if err := checkRef(ref); err != nil {
			return Template{}, err
		}
		t.parts = append(t.parts, part{ref: &ref})
		rest = rest[loc[1]:]
	}
}

func checkRef(r Ref) error {
	switch r.NS {
	case NSRun:
		if r.Name != "thread_id" {
			return fmt.Errorf("unknown variable {{%s}}: only {{run.thread_id}} exists", r)
		}
	case NSVar, NSSecret, NSSession:
		if !namePattern.MatchString(r.Name) {
			return fmt.Errorf("{{%s}} needs a name, like {{%s.example}}", r, r.NS)
		}
	case NSMessage, NSAudio:
		if r.Name != "" {
			return fmt.Errorf("use {{%s}} without a name", r.NS)
		}
	default:
		return fmt.Errorf("unknown variable {{%s}}", r)
	}
	return nil
}

// Refs lists the references in order of appearance.
func (t Template) Refs() []Ref {
	var refs []Ref
	for _, p := range t.parts {
		if p.ref != nil {
			refs = append(refs, *p.ref)
		}
	}
	return refs
}

// HasRefs reports whether the template references anything.
func (t Template) HasRefs() bool { return len(t.Refs()) > 0 }

// Values maps a reference string (e.g. "var.channelId", "message") to its value.
type Values map[string]string

// Escaper adapts a value to the template's context.
type Escaper func(string) string

// Raw inserts values unchanged (headers, query values before URL encoding).
func Raw(s string) string { return s }

// Path escapes a value for a URL path segment.
func Path(s string) string { return url.PathEscape(s) }

// JSON escapes a value for the inside of a JSON string literal.
func JSON(s string) string {
	b, _ := json.Marshal(s)
	return string(b[1 : len(b)-1])
}

// Render substitutes every reference, escaping values with esc.
func (t Template) Render(vals Values, esc Escaper) (string, error) {
	var b strings.Builder
	for _, p := range t.parts {
		if p.ref == nil {
			b.WriteString(p.literal)
			continue
		}
		v, ok := vals[p.ref.String()]
		if !ok {
			return "", fmt.Errorf("no value for {{%s}}", p.ref)
		}
		b.WriteString(esc(v))
	}
	return b.String(), nil
}

// Redactor replaces secret values in text meant for logs.
type Redactor struct {
	secrets []string
}

// NewRedactor ignores values shorter than 4 characters, which would mangle
// unrelated text without protecting anything meaningful.
func NewRedactor(secrets []string) *Redactor {
	r := &Redactor{}
	for _, s := range secrets {
		if len(s) >= 4 {
			r.secrets = append(r.secrets, s, JSON(s))
		}
	}
	return r
}

// Redact masks every known secret value.
func (r *Redactor) Redact(s string) string {
	for _, secret := range r.secrets {
		s = strings.ReplaceAll(s, secret, "••••")
	}
	return s
}
