package tmpl

import (
	"encoding/json"
	"strings"
	"testing"
)

func mustParse(t *testing.T, s string) Template {
	t.Helper()
	tp, err := Parse(s)
	if err != nil {
		t.Fatalf("Parse(%q): %v", s, err)
	}
	return tp
}

func TestParseAndRefs(t *testing.T) {
	tp := mustParse(t, "a {{ run.thread_id }} b {{var.channelId}}{{secret.api_key}} {{session.sid}} {{message}} {{audio}}")
	var got []string
	for _, r := range tp.Refs() {
		got = append(got, r.String())
	}
	want := "run.thread_id var.channelId secret.api_key session.sid message audio"
	if strings.Join(got, " ") != want {
		t.Fatalf("refs = %v, want %s", got, want)
	}
	if mustParse(t, "no templates").HasRefs() {
		t.Fatal("literal reported refs")
	}
}

func TestParseRejectsUnknownExpressions(t *testing.T) {
	for _, s := range []string{"{{run.other}}", "{{env.HOME}}", "{{var}}", "{{secret.1x}}", "{{ message.x }}", "x {{ y", "{{var.a-b}}", "{{run.thread_id"} {
		if _, err := Parse(s); err == nil {
			t.Errorf("Parse(%q) accepted", s)
		}
	}
}

func TestRenderEscapesPerContext(t *testing.T) {
	vals := Values{"var.name": `Ana "x" \ </script>`, "message": "line1\nline2", "secret.k": "s3cr3t", "run.thread_id": "t-1"}

	body := mustParse(t, `{"message":"{{message}}","name":"{{var.name}}","t":"{{run.thread_id}}"}`)
	out, err := body.Render(vals, JSON)
	if err != nil {
		t.Fatal(err)
	}
	var decoded map[string]string
	if err := json.Unmarshal([]byte(out), &decoded); err != nil {
		t.Fatalf("rendered JSON invalid: %v\n%s", err, out)
	}
	if decoded["message"] != "line1\nline2" || decoded["name"] != `Ana "x" \ </script>` {
		t.Fatalf("decoded = %v", decoded)
	}

	path := mustParse(t, "/channelApi/{{var.name}}/realtime")
	if got, _ := path.Render(Values{"var.name": "a/b c?d"}, Path); got != "/channelApi/a%2Fb%20c%3Fd/realtime" {
		t.Fatalf("path = %q", got)
	}

	if got, _ := mustParse(t, "Bearer {{secret.k}}").Render(vals, Raw); got != "Bearer s3cr3t" {
		t.Fatalf("raw = %q", got)
	}
}

func TestRenderFailsOnMissingValue(t *testing.T) {
	if _, err := mustParse(t, "{{var.missing}}").Render(Values{}, Raw); err == nil {
		t.Fatal("missing value rendered")
	}
}

func TestRedactor(t *testing.T) {
	r := NewRedactor([]string{"s3cr3t", "", "ab"})
	if got := r.Redact("Bearer s3cr3t and s3cr3t"); got != "Bearer •••• and ••••" {
		t.Fatalf("redacted = %q", got)
	}
	// Very short values are not redacted to avoid mangling unrelated text.
	if got := r.Redact("abc"); got != "abc" {
		t.Fatalf("short secret redaction = %q", got)
	}
}

func TestNestedJSONClosingBracesAreLiteral(t *testing.T) {
	tp := mustParse(t, `{"a":{"b":"{{audio}}"}}`)
	if got, _ := tp.Render(Values{"audio": "x"}, JSON); got != `{"a":{"b":"x"}}` {
		t.Fatalf("render = %q", got)
	}
}

func TestRedactorCoversURLEscapedForms(t *testing.T) {
	secret := "tok/AbCdEf+GhIj=="
	r := NewRedactor([]string{secret})
	for _, form := range []string{secret, Path(secret), JSON(secret), queryEscape(secret)} {
		if got := r.Redact("x " + form + " y"); got != "x •••• y" {
			t.Errorf("Redact(%q) = %q", form, got)
		}
	}
}

func TestPathEscapesDotSegments(t *testing.T) {
	for in, want := range map[string]string{"..": "%2E%2E", ".": "%2E", "a.b": "a.b", "...": "..."} {
		if got := Path(in); got != want {
			t.Errorf("Path(%q) = %q, want %q", in, got, want)
		}
	}
}
