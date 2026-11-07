package server_test

import (
	"net/http"
	"net/url"
	"strings"
	"testing"
)

type scenarioFixture struct {
	e        *env
	owner    *client
	slug     string
	voice    map[string]any // vivi_live agent (no interruptions)
	realtime map[string]any // openai_realtime agent with a trace connection
	text     map[string]any
	persona  map[string]any
	llm      map[string]any // anthropic connection
	speech   map[string]any // azure_speech connection
	lf       map[string]any
}

func (f *scenarioFixture) path(p string) string { return "/api/v1/workspaces/" + f.slug + p }

func newScenarioFixture(t *testing.T) *scenarioFixture {
	t.Helper()
	e := newEnv(t)
	owner, slug := ownerWorkspace(t, e)
	f := &scenarioFixture{e: e, owner: owner, slug: slug}
	f.llm = createConnection(t, owner, slug, map[string]any{"name": "Claude", "kind": "anthropic", "secret": "sk-ant-1", "config": map[string]any{}})
	f.speech = createConnection(t, owner, slug, map[string]any{"name": "Speech", "kind": "azure_speech", "secret": "speech-key", "config": map[string]any{"region": "eastus"}})
	f.lf = createConnection(t, owner, slug, map[string]any{"name": "LF", "kind": "langfuse", "secret": "sk-lf-1", "config": map[string]any{"publicKey": "pk-lf-1"}})

	f.voice = createAgent(t, owner, slug, viviLiveAgent("wss://agents.example.com"))
	rt := viviLiveAgent("wss://agents.example.com")
	rt["name"] = "Realtime voice"
	rt["codec"] = map[string]any{"type": "openai_realtime", "options": map[string]any{}}
	rt["traceConnectionId"] = f.lf["id"]
	f.realtime = createAgent(t, owner, slug, rt)
	f.text = createAgent(t, owner, slug, map[string]any{
		"name": "Chat", "description": "", "modality": "text",
		"transport": map[string]any{"url": "https://chat.example.com/c", "query": []any{}, "headers": []any{},
			"body": `{"message":"{{message}}"}`, "initMessages": []any{}, "preSession": nil},
		"codec": map[string]any{"type": "http_json", "options": map[string]any{}}, "variables": []any{}, "secrets": []any{},
		"traceConnectionId": nil, "traceSessionId": "",
	})
	r := owner.do("POST", f.path("/personas"), personaBody("Impatient caller"))
	expectStatus(t, r, http.StatusCreated)
	f.persona = r.body
	return f
}

func personaBody(name string) map[string]any {
	return map[string]any{"name": name, "locale": "es-MX", "temperament": "hurried", "notes": "Wants to finish fast.", "voice": nil}
}

func (f *scenarioFixture) scenario(agent map[string]any, name string) map[string]any {
	return map[string]any{
		"name": name, "agentId": agent["id"], "personaId": f.persona["id"], "groupId": nil,
		"tags": []any{"booking", " smoke ", "booking"}, "context": "", "goal": "Move Thursday's appointment to Friday.",
		"instructions": "Be brief.", "facts": []any{map[string]any{"key": "patientId", "value": "4471"}},
		"criteria": []any{
			map[string]any{"id": nil, "text": "Verifies identity", "kind": "must", "source": "transcript", "weight": 60},
			map[string]any{"id": nil, "text": "Confirms the new slot", "kind": "should", "source": "transcript", "weight": 40},
		},
		"endConditions": map[string]any{"maxDurationSeconds": 0, "maxSilenceSeconds": 0, "maxTurns": 0, "hangUpWhen": ""},
		"interruptions": []any{},
		"simulator":     map[string]any{"mode": "", "connectionId": nil, "model": ""},
		"judge":         map[string]any{"connectionId": nil, "model": ""},
	}
}

func (f *scenarioFixture) create(t *testing.T, body map[string]any) map[string]any {
	t.Helper()
	r := f.owner.do("POST", f.path("/scenarios"), body)
	expectStatus(t, r, http.StatusCreated)
	return r.body
}

func fieldErrors(t *testing.T, r response) map[string]any {
	t.Helper()
	expectStatus(t, r, http.StatusUnprocessableEntity)
	fields, _ := r.body["error"].(map[string]any)["fields"].(map[string]any)
	return fields
}

func TestPersonas(t *testing.T) {
	f := newScenarioFixture(t)
	base := f.path("/personas")

	if f.persona["scenarioCount"].(float64) != 0 || f.persona["voice"] != nil {
		t.Fatalf("persona = %v", f.persona)
	}
	expectStatus(t, f.owner.do("POST", base, personaBody("IMPATIENT CALLER")), http.StatusConflict)

	bad := personaBody("Bad")
	bad["locale"], bad["temperament"] = "spanish!", "grumpy"
	bad["voice"] = map[string]any{"connectionId": f.llm["id"], "voice": "alloy"}
	fields := fieldErrors(t, f.owner.do("POST", base, bad))
	for _, k := range []string{"locale", "temperament", "voice.connectionId"} {
		if fields[k] == nil {
			t.Errorf("missing %q in %v", k, fields)
		}
	}

	voiced := personaBody("Calm caller")
	voiced["voice"] = map[string]any{"connectionId": f.speech["id"], "voice": "es-MX-DaliaNeural"}
	r := f.owner.do("POST", base, voiced)
	expectStatus(t, r, http.StatusCreated)
	item := base + "/" + r.body["id"].(string)
	if r.body["voice"].(map[string]any)["voice"] != "es-MX-DaliaNeural" {
		t.Fatalf("voice = %v", r.body["voice"])
	}

	voiced["notes"] = "Updated"
	upd := f.owner.do("PUT", item, voiced)
	expectStatus(t, upd, http.StatusOK)
	if upd.body["notes"] != "Updated" {
		t.Fatalf("updated = %v", upd.body)
	}

	// Deleting the voice connection clears the persona voice.
	expectStatus(t, f.owner.do("DELETE", f.path("/connections/"+f.speech["id"].(string)), nil), http.StatusNoContent)
	if v := f.owner.do("GET", item, nil).body["voice"]; v != nil {
		t.Fatalf("voice after connection delete = %v", v)
	}

	// A persona in use cannot be deleted.
	f.create(t, f.scenario(f.voice, "Reschedule"))
	used := f.owner.do("DELETE", base+"/"+f.persona["id"].(string), nil)
	expectStatus(t, used, http.StatusConflict)
	if !strings.Contains(used.body["error"].(map[string]any)["message"].(string), "1 scenario") {
		t.Fatalf("conflict = %v", used.body)
	}
	if n := f.owner.do("GET", base+"/"+f.persona["id"].(string), nil).body["scenarioCount"]; n.(float64) != 1 {
		t.Fatalf("scenarioCount = %v", n)
	}
	expectStatus(t, f.owner.do("DELETE", item, nil), http.StatusNoContent)
}

func TestScenarioGroups(t *testing.T) {
	f := newScenarioFixture(t)
	base := f.path("/scenario-groups")
	r := f.owner.do("POST", base, map[string]any{"name": "Booking", "description": "Appointment flows"})
	expectStatus(t, r, http.StatusCreated)
	group := r.body
	expectStatus(t, f.owner.do("POST", base, map[string]any{"name": "booking", "description": ""}), http.StatusConflict)
	if fields := fieldErrors(t, f.owner.do("POST", base, map[string]any{"name": " ", "description": ""})); fields["name"] == nil {
		t.Fatalf("fields = %v", fields)
	}

	body := f.scenario(f.voice, "Grouped")
	body["groupId"] = group["id"]
	sc := f.create(t, body)

	list := f.owner.do("GET", base, nil)
	if list.body["items"].([]any)[0].(map[string]any)["scenarioCount"].(float64) != 1 {
		t.Fatalf("groups = %v", list.body)
	}
	expectStatus(t, f.owner.do("PUT", base+"/"+group["id"].(string), map[string]any{"name": "Bookings", "description": ""}), http.StatusOK)
	expectStatus(t, f.owner.do("DELETE", base+"/"+group["id"].(string), nil), http.StatusNoContent)
	if g := f.owner.do("GET", f.path("/scenarios/"+sc["id"].(string)), nil).body["groupId"]; g != nil {
		t.Fatalf("groupId after group delete = %v", g)
	}
}

func TestScenarioLifecycle(t *testing.T) {
	f := newScenarioFixture(t)
	sc := f.create(t, f.scenario(f.voice, "Reschedule"))

	if tags := sc["tags"].([]any); len(tags) != 2 || tags[0] != "booking" || tags[1] != "smoke" {
		t.Fatalf("tags = %v", tags)
	}
	criteria := sc["criteria"].([]any)
	id0 := criteria[0].(map[string]any)["id"].(string)
	if !strings.HasPrefix(id0, "c_") || criteria[1].(map[string]any)["id"] == id0 {
		t.Fatalf("criteria = %v", criteria)
	}
	ec := sc["endConditions"].(map[string]any)
	if ec["maxDurationSeconds"].(float64) != 360 || ec["maxSilenceSeconds"].(float64) != 15 || ec["maxTurns"].(float64) != 20 || ec["hangUpWhen"] != "goal_or_goodbye" {
		t.Fatalf("endConditions defaults = %v", ec)
	}
	if sc["modality"] != "voice" || sc["simulator"].(map[string]any)["mode"] != "pipeline" {
		t.Fatalf("scenario = %v", sc)
	}

	item := f.path("/scenarios/" + sc["id"].(string))
	upd := f.scenario(f.voice, "Reschedule v2")
	upd["criteria"] = []any{
		map[string]any{"id": id0, "text": "Verifies identity first", "kind": "must", "source": "transcript", "weight": 50},
		map[string]any{"id": nil, "text": "Offers two slots", "kind": "should", "source": "timing", "weight": 50},
	}
	upd["simulator"] = map[string]any{"mode": "pipeline", "connectionId": f.llm["id"], "model": "claude-sonnet-5-5"}
	upd["judge"] = map[string]any{"connectionId": f.llm["id"], "model": "claude-opus-5-5"}
	r := f.owner.do("PUT", item, upd)
	expectStatus(t, r, http.StatusOK)
	if r.body["criteria"].([]any)[0].(map[string]any)["id"] != id0 || r.body["judge"].(map[string]any)["model"] != "claude-opus-5-5" {
		t.Fatalf("updated = %v", r.body)
	}

	moved := clone(upd)
	moved["agentId"] = f.realtime["id"]
	if fields := fieldErrors(t, f.owner.do("PUT", item, moved)); fields["agentId"] == nil {
		t.Fatalf("agent change: %v", fields)
	}

	// Names are unique per agent, not per workspace.
	expectStatus(t, f.owner.do("POST", f.path("/scenarios"), f.scenario(f.voice, "RESCHEDULE V2")), http.StatusConflict)
	f.create(t, f.scenario(f.realtime, "Reschedule v2"))

	dup := f.owner.do("POST", item+"/duplicate", nil)
	expectStatus(t, dup, http.StatusCreated)
	dup2 := f.owner.do("POST", item+"/duplicate", nil)
	if dup.body["name"] != "Reschedule v2 (copy)" || dup2.body["name"] != "Reschedule v2 (copy 2)" {
		t.Fatalf("copies = %v / %v", dup.body["name"], dup2.body["name"])
	}

	// The simulator connection is cleared when the connection is deleted.
	expectStatus(t, f.owner.do("DELETE", f.path("/connections/"+f.llm["id"].(string)), nil), http.StatusNoContent)
	after := f.owner.do("GET", item, nil).body
	if after["simulator"].(map[string]any)["connectionId"] != nil || after["judge"].(map[string]any)["connectionId"] != nil {
		t.Fatalf("after connection delete = %v %v", after["simulator"], after["judge"])
	}

	// Agents in use cannot be deleted.
	r = f.owner.do("DELETE", f.path("/agents/"+f.voice["id"].(string)), nil)
	expectStatus(t, r, http.StatusConflict)
	expectStatus(t, f.owner.do("DELETE", item, nil), http.StatusNoContent)
	expectStatus(t, f.owner.do("GET", item, nil), http.StatusNotFound)
}

func TestScenarioValidation(t *testing.T) {
	f := newScenarioFixture(t)
	bob := f.e.browser()
	bobSlug := firstWorkspace(t, signup(t, bob, "Bob", "bob@example.com"))["slug"].(string)
	r := bob.do("POST", "/api/v1/workspaces/"+bobSlug+"/personas", personaBody("Foreign"))
	expectStatus(t, r, http.StatusCreated)
	foreignPersona := r.body["id"]

	cases := []struct {
		name   string
		agent  map[string]any
		mutate func(b map[string]any)
		field  string
	}{
		{"weights", f.voice, func(b map[string]any) { b["criteria"].([]any)[0].(map[string]any)["weight"] = 10 }, "criteria"},
		{"no criteria", f.voice, func(b map[string]any) { b["criteria"] = []any{} }, "criteria"},
		{"bad kind", f.voice, func(b map[string]any) { b["criteria"].([]any)[1].(map[string]any)["kind"] = "could" }, "criteria.1.kind"},
		{"trace without connection", f.voice, func(b map[string]any) { b["criteria"].([]any)[0].(map[string]any)["source"] = "trace" }, "criteria.0.source"},
		{"timing on text", f.text, func(b map[string]any) { b["criteria"].([]any)[0].(map[string]any)["source"] = "timing" }, "criteria.0.source"},
		{"duplicate ids", f.voice, func(b map[string]any) {
			b["criteria"].([]any)[0].(map[string]any)["id"] = "c_same"
			b["criteria"].([]any)[1].(map[string]any)["id"] = "c_same"
		}, "criteria.1.id"},
		{"interruptions on vivi_live", f.voice, func(b map[string]any) {
			b["interruptions"] = []any{map[string]any{"agentTurn": 1, "afterMs": 800}}
		}, "interruptions"},
		{"interruptions on text", f.text, func(b map[string]any) {
			b["interruptions"] = []any{map[string]any{"agentTurn": 1, "afterMs": 800}}
		}, "interruptions"},
		{"interruption too early", f.realtime, func(b map[string]any) {
			b["interruptions"] = []any{map[string]any{"agentTurn": 1, "afterMs": 100}}
		}, "interruptions.0.afterMs"},
		{"duplicate fact", f.voice, func(b map[string]any) {
			b["facts"] = []any{map[string]any{"key": "id", "value": "1"}, map[string]any{"key": "ID", "value": "2"}}
		}, "facts.1.key"},
		{"empty goal", f.voice, func(b map[string]any) { b["goal"] = " " }, "goal"},
		{"long tag", f.voice, func(b map[string]any) { b["tags"] = []any{strings.Repeat("x", 41)} }, "tags.0"},
		{"bad duration", f.voice, func(b map[string]any) {
			b["endConditions"].(map[string]any)["maxDurationSeconds"] = 5
		}, "endConditions.maxDurationSeconds"},
		{"bad hang up", f.voice, func(b map[string]any) { b["endConditions"].(map[string]any)["hangUpWhen"] = "maybe" }, "endConditions.hangUpWhen"},
		{"text mode on voice", f.voice, func(b map[string]any) { b["simulator"].(map[string]any)["mode"] = "text" }, "simulator.mode"},
		{"realtime needs realtime connection", f.voice, func(b map[string]any) {
			b["simulator"] = map[string]any{"mode": "realtime", "connectionId": f.llm["id"], "model": "x"}
		}, "simulator.connectionId"},
		{"model required", f.voice, func(b map[string]any) {
			b["simulator"] = map[string]any{"mode": "pipeline", "connectionId": f.llm["id"], "model": ""}
		}, "simulator.model"},
		{"judge needs llm", f.voice, func(b map[string]any) { b["judge"] = map[string]any{"connectionId": f.lf["id"], "model": "x"} }, "judge.connectionId"},
		{"foreign persona", f.voice, func(b map[string]any) { b["personaId"] = foreignPersona }, "personaId"},
		{"unknown agent", f.voice, func(b map[string]any) { b["agentId"] = "00000000-0000-0000-0000-000000000000" }, "agentId"},
		{"unknown group", f.voice, func(b map[string]any) { b["groupId"] = "00000000-0000-0000-0000-000000000000" }, "groupId"},
	}
	for _, tc := range cases {
		b := f.scenario(tc.agent, "Case "+tc.name)
		tc.mutate(b)
		r := f.owner.do("POST", f.path("/scenarios"), b)
		if r.status != http.StatusUnprocessableEntity {
			t.Errorf("%s: status %d %v", tc.name, r.status, r.body)
			continue
		}
		fields, _ := r.body["error"].(map[string]any)["fields"].(map[string]any)
		if fields[tc.field] == nil {
			t.Errorf("%s: missing %q in %v", tc.name, tc.field, fields)
		}
	}

	// Valid variants: trace source with a trace connection, interruptions on openai_realtime, text agent defaults.
	ok := f.scenario(f.realtime, "Barge in")
	ok["criteria"].([]any)[0].(map[string]any)["source"] = "trace"
	ok["interruptions"] = []any{map[string]any{"agentTurn": 2, "afterMs": 900}}
	f.create(t, ok)
	text := f.create(t, f.scenario(f.text, "Chat case"))
	if text["simulator"].(map[string]any)["mode"] != "text" || text["modality"] != "text" {
		t.Fatalf("text scenario = %v", text)
	}
}

func TestScenarioFiltersRolesAndTenancy(t *testing.T) {
	f := newScenarioFixture(t)
	g := f.owner.do("POST", f.path("/scenario-groups"), map[string]any{"name": "Billing", "description": ""}).body
	a := f.scenario(f.voice, "Refund request")
	a["tags"] = []any{"billing"}
	a["groupId"] = g["id"]
	f.create(t, a)
	b := f.scenario(f.text, "Opening hours")
	b["goal"] = "Ask when the clinic opens on Sunday."
	f.create(t, b)

	names := func(query string) string {
		r := f.owner.do("GET", f.path("/scenarios")+query, nil)
		expectStatus(t, r, http.StatusOK)
		var out []string
		for _, s := range r.body["items"].([]any) {
			out = append(out, s.(map[string]any)["name"].(string))
		}
		return strings.Join(out, ",")
	}
	if got := names(""); got != "Opening hours,Refund request" {
		t.Fatalf("all = %s", got)
	}
	if got := names("?agentId=" + f.text["id"].(string)); got != "Opening hours" {
		t.Fatalf("agent filter = %s", got)
	}
	if got := names("?groupId=none"); got != "Opening hours" {
		t.Fatalf("ungrouped = %s", got)
	}
	if got := names("?groupId=" + g["id"].(string)); got != "Refund request" {
		t.Fatalf("group filter = %s", got)
	}
	if got := names("?tag=billing"); got != "Refund request" {
		t.Fatalf("tag filter = %s", got)
	}
	if got := names("?q=" + url.QueryEscape("SUNDAY")); got != "Opening hours" {
		t.Fatalf("q on goal = %s", got)
	}
	if got := names("?q=" + url.QueryEscape("100%")); got != "" {
		t.Fatalf("q with wildcard = %q", got)
	}
	expectStatus(t, f.owner.do("GET", f.path("/scenarios?agentId=nope"), nil), http.StatusUnprocessableEntity)

	member := addMember(t, f.e, f.slug, "member")
	expectStatus(t, member.do("POST", f.path("/scenarios"), f.scenario(f.voice, "By member")), http.StatusCreated)
	expectStatus(t, member.do("POST", f.path("/personas"), personaBody("Member persona")), http.StatusCreated)
	viewer := addMember(t, f.e, f.slug, "viewer")
	expectStatus(t, viewer.do("GET", f.path("/scenarios"), nil), http.StatusOK)
	expectStatus(t, viewer.do("POST", f.path("/scenarios"), f.scenario(f.voice, "By viewer")), http.StatusForbidden)
	expectStatus(t, viewer.do("POST", f.path("/scenario-groups"), map[string]any{"name": "V", "description": ""}), http.StatusForbidden)
	api := f.e.apiClient(createKey(t, f.owner, f.slug)["secret"].(string))
	expectStatus(t, api.do("POST", f.path("/scenarios"), f.scenario(f.voice, "By key")), http.StatusCreated)

	bob := f.e.browser()
	bobSlug := firstWorkspace(t, signup(t, bob, "Bob", "bob@example.com"))["slug"].(string)
	expectStatus(t, bob.do("GET", "/api/v1/workspaces/"+bobSlug+"/personas/"+f.persona["id"].(string), nil), http.StatusNotFound)
	if n := len(bob.do("GET", "/api/v1/workspaces/"+bobSlug+"/scenarios", nil).body["items"].([]any)); n != 0 {
		t.Fatalf("bob sees %d scenarios", n)
	}
}
