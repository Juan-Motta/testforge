package scenario

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"testforge/internal/agent/codec"
	"testforge/internal/authn"
	"testforge/internal/connection"
	"testforge/internal/platform/apperr"
	"testforge/internal/platform/database"
	"testforge/internal/workspace"
)

var (
	criterionIDPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{1,40}$`)
	errScenarioMissing = apperr.NotFound("Scenario not found.")
	errScenarioTaken   = apperr.Conflict("This agent already has a scenario with this name.")
)

// Fact is information the simulated user reveals only when asked.
type Fact struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// Criterion is one thing the judge scores.
type Criterion struct {
	ID     *string `json:"id"`
	Text   string  `json:"text"`
	Kind   string  `json:"kind"`
	Source string  `json:"source"`
	Weight int     `json:"weight"`
}

// EndConditions decide when a run stops.
type EndConditions struct {
	MaxDurationSeconds int    `json:"maxDurationSeconds"`
	MaxSilenceSeconds  int    `json:"maxSilenceSeconds"`
	MaxTurns           int    `json:"maxTurns"`
	HangUpWhen         string `json:"hangUpWhen"`
}

// Interruption makes the caller cut the agent off.
type Interruption struct {
	AgentTurn int `json:"agentTurn"`
	AfterMs   int `json:"afterMs"`
}

// Simulator configures the simulated user.
type Simulator struct {
	Mode         string  `json:"mode"`
	ConnectionID *string `json:"connectionId"`
	Model        string  `json:"model"`
}

// ModelChoice is a connection plus a model name.
type ModelChoice struct {
	ConnectionID *string `json:"connectionId"`
	Model        string  `json:"model"`
}

// Input is the create/update request.
type Input struct {
	Name          string         `json:"name"`
	AgentID       string         `json:"agentId"`
	PersonaID     string         `json:"personaId"`
	GroupID       *string        `json:"groupId"`
	Tags          []string       `json:"tags"`
	Context       string         `json:"context"`
	Goal          string         `json:"goal"`
	Instructions  string         `json:"instructions"`
	Facts         []Fact         `json:"facts"`
	Criteria      []Criterion    `json:"criteria"`
	EndConditions EndConditions  `json:"endConditions"`
	Interruptions []Interruption `json:"interruptions"`
	Simulator     Simulator      `json:"simulator"`
	Judge         ModelChoice    `json:"judge"`
}

// Scenario is a stored scenario.
type Scenario struct {
	Input
	ID        string    `json:"id"`
	Modality  string    `json:"modality"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// Filter narrows a scenario list.
type Filter struct {
	AgentID string
	GroupID string // a group id or "none"
	Tag     string
	Query   string
}

const selectScenarios = `SELECT s.id, s.name, s.agent_id, s.persona_id, s.group_id, s.tags, s.context, s.goal, s.instructions,
	s.facts, s.criteria, s.end_conditions, s.interruptions, s.simulator_mode, s.simulator_connection_id, s.simulator_model,
	s.judge_connection_id, s.judge_model, a.modality, s.created_at, s.updated_at
	FROM scenarios s JOIN agents a ON a.id = s.agent_id`

func scanScenario(row interface{ Scan(...any) error }) (Scenario, error) {
	var sc Scenario
	var facts, criteria, ec, intr []byte
	err := row.Scan(&sc.ID, &sc.Name, &sc.AgentID, &sc.PersonaID, &sc.GroupID, &sc.Tags, &sc.Context, &sc.Goal, &sc.Instructions,
		&facts, &criteria, &ec, &intr, &sc.Simulator.Mode, &sc.Simulator.ConnectionID, &sc.Simulator.Model,
		&sc.Judge.ConnectionID, &sc.Judge.Model, &sc.Modality, &sc.CreatedAt, &sc.UpdatedAt)
	if err != nil {
		return Scenario{}, err
	}
	for _, p := range []struct {
		raw []byte
		dst any
	}{{facts, &sc.Facts}, {criteria, &sc.Criteria}, {ec, &sc.EndConditions}, {intr, &sc.Interruptions}} {
		if err := json.Unmarshal(p.raw, p.dst); err != nil {
			return Scenario{}, fmt.Errorf("decode scenario: %w", err)
		}
	}
	if sc.Tags == nil {
		sc.Tags = []string{}
	}
	return sc, nil
}

// ListScenarios returns matching scenarios by name.
func (s *Service) ListScenarios(ctx context.Context, ws workspace.Workspace, f Filter) ([]Scenario, error) {
	where := []string{"s.workspace_id = $1"}
	args := []any{ws.ID}
	arg := func(v any) string { args = append(args, v); return fmt.Sprintf("$%d", len(args)) }
	fields := map[string]string{}
	if f.AgentID != "" {
		if !uuidPattern.MatchString(f.AgentID) {
			fields["agentId"] = "Use an agent id."
		}
		where = append(where, "s.agent_id = "+arg(f.AgentID))
	}
	switch {
	case f.GroupID == "none":
		where = append(where, "s.group_id IS NULL")
	case f.GroupID != "":
		if !uuidPattern.MatchString(f.GroupID) {
			fields["groupId"] = "Use a group id or none."
		}
		where = append(where, "s.group_id = "+arg(f.GroupID))
	}
	if f.Tag != "" {
		where = append(where, arg(f.Tag)+" = ANY(s.tags)")
	}
	if q := strings.TrimSpace(f.Query); q != "" {
		if len(q) > 200 {
			fields["q"] = "Use at most 200 characters."
		}
		like := "%" + strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(q) + "%"
		p := arg(like)
		where = append(where, "(s.name ILIKE "+p+" OR s.goal ILIKE "+p+")")
	}
	if err := apperr.Invalid(fields); err != nil {
		return nil, err
	}
	rows, err := s.pool.Query(ctx, selectScenarios+" WHERE "+strings.Join(where, " AND ")+" ORDER BY lower(s.name), s.id", args...)
	if err != nil {
		return nil, fmt.Errorf("list scenarios: %w", err)
	}
	return pgx.CollectRows(rows, func(r pgx.CollectableRow) (Scenario, error) { return scanScenario(r) })
}

// GetScenario returns one scenario.
func (s *Service) GetScenario(ctx context.Context, ws workspace.Workspace, id string) (Scenario, error) {
	if !uuidPattern.MatchString(id) {
		return Scenario{}, errScenarioMissing
	}
	sc, err := scanScenario(s.pool.QueryRow(ctx, selectScenarios+` WHERE s.workspace_id = $1 AND s.id = $2`, ws.ID, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return Scenario{}, errScenarioMissing
	}
	return sc, err
}

// agentFacts is what scenario validation needs to know about the agent.
type agentFacts struct {
	modality      string
	hasTraces     bool
	interruptible bool
}

func (s *Service) loadAgent(ctx context.Context, ws workspace.Workspace, id string) (*agentFacts, error) {
	if !uuidPattern.MatchString(id) {
		return nil, nil
	}
	var a agentFacts
	var trace *string
	var codecType string
	err := s.pool.QueryRow(ctx,
		`SELECT modality, trace_connection_id, codec->>'type' FROM agents WHERE workspace_id = $1 AND id = $2`, ws.ID, id,
	).Scan(&a.modality, &trace, &codecType)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("load agent: %w", err)
	}
	a.hasTraces = trace != nil
	if v, err := codec.NewVoice(codecType, nil, nil); err == nil {
		a.interruptible = v.Capabilities().Interruptions
	}
	return &a, nil
}

func (s *Service) exists(ctx context.Context, table, wsID, id string) (bool, error) {
	if !uuidPattern.MatchString(id) {
		return false, nil
	}
	var ok bool
	err := s.pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM `+table+` WHERE workspace_id = $1 AND id = $2)`, wsID, id).Scan(&ok)
	return ok, err
}

// validate checks and normalizes the input; it returns the agent's modality.
func (s *Service) validate(ctx context.Context, ws workspace.Workspace, in *Input) (string, map[string]string, error) {
	fields := map[string]string{}
	in.Name = textLen(fields, "name", in.Name, 1, 160)
	in.Goal = textLen(fields, "goal", in.Goal, 1, 5000)
	if len([]rune(in.Context)) > 20000 {
		fields["context"] = "Use at most 20000 characters."
	}
	if len([]rune(in.Instructions)) > 10000 {
		fields["instructions"] = "Use at most 10000 characters."
	}

	a, err := s.loadAgent(ctx, ws, in.AgentID)
	if err != nil {
		return "", nil, err
	}
	if a == nil {
		fields["agentId"] = "Choose an agent of this workspace."
		a = &agentFacts{}
	}
	voice := a.modality == "voice"
	if ok, err := s.exists(ctx, "personas", ws.ID, in.PersonaID); err != nil {
		return "", nil, err
	} else if !ok {
		fields["personaId"] = "Choose a persona of this workspace."
	}
	if in.GroupID != nil {
		if ok, err := s.exists(ctx, "scenario_groups", ws.ID, *in.GroupID); err != nil {
			return "", nil, err
		} else if !ok {
			fields["groupId"] = "Choose a group of this workspace."
		}
	}

	// Tags: trimmed, de-duplicated in order.
	tags := []string{}
	seen := map[string]bool{}
	for i, t := range in.Tags {
		t = strings.TrimSpace(t)
		if n := len([]rune(t)); n < 1 || n > 40 {
			setOnce(fields, fmt.Sprintf("tags.%d", i), "Use 1 to 40 characters.")
			continue
		}
		if !seen[t] {
			seen[t] = true
			tags = append(tags, t)
		}
	}
	if len(tags) > 20 {
		fields["tags"] = "Use at most 20 tags."
	}
	in.Tags = tags

	if in.Facts == nil {
		in.Facts = []Fact{}
	}
	if len(in.Facts) > 50 {
		fields["facts"] = "Use at most 50 facts."
	}
	factKeys := map[string]bool{}
	for i, f := range in.Facts {
		key := strings.ToLower(strings.TrimSpace(f.Key))
		in.Facts[i].Key = textLen(fields, fmt.Sprintf("facts.%d.key", i), f.Key, 1, 80)
		in.Facts[i].Value = textLen(fields, fmt.Sprintf("facts.%d.value", i), f.Value, 1, 500)
		if factKeys[key] {
			setOnce(fields, fmt.Sprintf("facts.%d.key", i), "Already used.")
		}
		factKeys[key] = true
	}

	switch n := len(in.Criteria); {
	case n == 0:
		fields["criteria"] = "Add at least one criterion."
	case n > 30:
		fields["criteria"] = "Use at most 30 criteria."
	}
	total := 0
	ids := map[string]bool{}
	for i, c := range in.Criteria {
		f := fmt.Sprintf("criteria.%d", i)
		if c.ID == nil || *c.ID == "" {
			id := shortID()
			in.Criteria[i].ID = &id
		} else if !criterionIDPattern.MatchString(*c.ID) {
			setOnce(fields, f+".id", "Use letters, digits, _ or -.")
		} else if ids[*c.ID] {
			setOnce(fields, f+".id", "Already used by another criterion.")
		}
		ids[*in.Criteria[i].ID] = true
		in.Criteria[i].Text = textLen(fields, f+".text", c.Text, 1, 500)
		if c.Kind != "must" && c.Kind != "should" {
			setOnce(fields, f+".kind", "Use must or should.")
		}
		switch c.Source {
		case "transcript":
		case "trace":
			if !a.hasTraces {
				setOnce(fields, f+".source", "The agent has no Langfuse connection to read traces from.")
			}
		case "timing":
			if a.modality == "text" {
				setOnce(fields, f+".source", "Timing is measured on voice agents only.")
			}
		default:
			setOnce(fields, f+".source", "Use transcript, trace or timing.")
		}
		if c.Weight < 1 || c.Weight > 100 {
			setOnce(fields, f+".weight", "Use 1 to 100.")
		}
		total += c.Weight
	}
	if len(in.Criteria) > 0 && total != 100 {
		setOnce(fields, "criteria", fmt.Sprintf("Weights must total 100 (they total %d).", total))
	}

	ec := &in.EndConditions
	defaultInt(&ec.MaxDurationSeconds, 360)
	defaultInt(&ec.MaxSilenceSeconds, 15)
	defaultInt(&ec.MaxTurns, 20)
	if ec.HangUpWhen == "" {
		ec.HangUpWhen = "goal_or_goodbye"
	}
	rangeInt(fields, "endConditions.maxDurationSeconds", ec.MaxDurationSeconds, 30, 1800)
	rangeInt(fields, "endConditions.maxSilenceSeconds", ec.MaxSilenceSeconds, 5, 120)
	rangeInt(fields, "endConditions.maxTurns", ec.MaxTurns, 1, 100)
	if ec.HangUpWhen != "goal_or_goodbye" && ec.HangUpWhen != "goodbye" && ec.HangUpWhen != "never" {
		fields["endConditions.hangUpWhen"] = "Use goal_or_goodbye, goodbye or never."
	}

	if in.Interruptions == nil {
		in.Interruptions = []Interruption{}
	}
	switch {
	case len(in.Interruptions) == 0:
	case !voice && a.modality != "":
		fields["interruptions"] = "Only voice agents can be interrupted."
	case voice && !a.interruptible:
		fields["interruptions"] = "This agent's codec does not support interruptions."
	case len(in.Interruptions) > 10:
		fields["interruptions"] = "Use at most 10 interruptions."
	}
	turns := map[int]bool{}
	for i, it := range in.Interruptions {
		f := fmt.Sprintf("interruptions.%d", i)
		if it.AgentTurn < 1 {
			setOnce(fields, f+".agentTurn", "Use 1 or more.")
		} else if turns[it.AgentTurn] {
			setOnce(fields, f+".agentTurn", "Each agent turn can be interrupted once.")
		}
		turns[it.AgentTurn] = true
		rangeInt(fields, f+".afterMs", it.AfterMs, 300, 60000)
	}

	sim := &in.Simulator
	if sim.Mode == "" {
		sim.Mode = "pipeline"
		if a.modality == "text" {
			sim.Mode = "text"
		}
	}
	var simCaps []connection.Capability
	switch {
	case a.modality == "text" && sim.Mode != "text":
		fields["simulator.mode"] = "Text agents use the text simulator."
	case voice && sim.Mode != "pipeline" && sim.Mode != "realtime":
		fields["simulator.mode"] = "Voice agents use pipeline or realtime."
	case sim.Mode == "realtime":
		simCaps = []connection.Capability{connection.CapRealtime}
	default:
		simCaps = []connection.Capability{connection.CapLLM}
	}
	if err := s.modelChoice(ctx, ws, sim.ConnectionID, &sim.Model, "simulator", fields, simCaps...); err != nil {
		return "", nil, err
	}
	if err := s.modelChoice(ctx, ws, in.Judge.ConnectionID, &in.Judge.Model, "judge", fields, connection.CapLLM); err != nil {
		return "", nil, err
	}
	return a.modality, fields, nil
}

func (s *Service) modelChoice(ctx context.Context, ws workspace.Workspace, connID *string, model *string, prefix string, fields map[string]string, caps ...connection.Capability) error {
	*model = strings.TrimSpace(*model)
	if len(*model) > 100 {
		setOnce(fields, prefix+".model", "Use at most 100 characters.")
	}
	if connID == nil {
		return nil
	}
	if *model == "" {
		setOnce(fields, prefix+".model", "Enter the model to use with this connection.")
	}
	if len(caps) == 0 {
		return nil
	}
	return s.checkConnection(ctx, ws, connID, prefix+".connectionId", fields, caps...)
}

func defaultInt(v *int, def int) {
	if *v == 0 {
		*v = def
	}
}

func rangeInt(fields map[string]string, key string, v, min, max int) {
	if v < min || v > max {
		setOnce(fields, key, fmt.Sprintf("Use %d to %d.", min, max))
	}
}

// CreateScenario stores a scenario. Member or above.
func (s *Service) CreateScenario(ctx context.Context, p authn.Principal, ws workspace.Workspace, in Input) (Scenario, error) {
	if err := canAuthor(ws); err != nil {
		return Scenario{}, err
	}
	_, fields, err := s.validate(ctx, ws, &in)
	if err != nil {
		return Scenario{}, err
	}
	if err := apperr.Invalid(fields); err != nil {
		return Scenario{}, err
	}
	id, err := s.insert(ctx, ws, p.UserID, in)
	if err != nil {
		return Scenario{}, err
	}
	return s.GetScenario(ctx, ws, id)
}

func (s *Service) insert(ctx context.Context, ws workspace.Workspace, userID string, in Input) (string, error) {
	facts, criteria, ec, intr := marshal(in)
	var id string
	err := s.pool.QueryRow(ctx,
		`INSERT INTO scenarios (workspace_id, agent_id, persona_id, group_id, name, tags, context, goal, instructions,
		   facts, criteria, end_conditions, interruptions, simulator_mode, simulator_connection_id, simulator_model,
		   judge_connection_id, judge_model, created_by)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19) RETURNING id`,
		ws.ID, in.AgentID, in.PersonaID, in.GroupID, in.Name, in.Tags, in.Context, in.Goal, in.Instructions,
		facts, criteria, ec, intr, in.Simulator.Mode, in.Simulator.ConnectionID, in.Simulator.Model,
		in.Judge.ConnectionID, in.Judge.Model, userID).Scan(&id)
	if database.IsUniqueViolation(err, "scenarios_agent_name_idx") {
		return "", errScenarioTaken
	}
	if err != nil {
		return "", fmt.Errorf("insert scenario: %w", err)
	}
	return id, nil
}

// UpdateScenario replaces a scenario; the agent cannot change. Member or above.
func (s *Service) UpdateScenario(ctx context.Context, ws workspace.Workspace, id string, in Input) (Scenario, error) {
	if err := canAuthor(ws); err != nil {
		return Scenario{}, err
	}
	current, err := s.GetScenario(ctx, ws, id)
	if err != nil {
		return Scenario{}, err
	}
	_, fields, err := s.validate(ctx, ws, &in)
	if err != nil {
		return Scenario{}, err
	}
	if in.AgentID != current.AgentID {
		fields["agentId"] = "The agent cannot change. Duplicate the scenario for another agent instead."
	}
	if err := apperr.Invalid(fields); err != nil {
		return Scenario{}, err
	}
	facts, criteria, ec, intr := marshal(in)
	_, err = s.pool.Exec(ctx,
		`UPDATE scenarios SET persona_id = $3, group_id = $4, name = $5, tags = $6, context = $7, goal = $8, instructions = $9,
		   facts = $10, criteria = $11, end_conditions = $12, interruptions = $13, simulator_mode = $14,
		   simulator_connection_id = $15, simulator_model = $16, judge_connection_id = $17, judge_model = $18, updated_at = now()
		 WHERE workspace_id = $1 AND id = $2`,
		ws.ID, id, in.PersonaID, in.GroupID, in.Name, in.Tags, in.Context, in.Goal, in.Instructions,
		facts, criteria, ec, intr, in.Simulator.Mode, in.Simulator.ConnectionID, in.Simulator.Model,
		in.Judge.ConnectionID, in.Judge.Model)
	if database.IsUniqueViolation(err, "scenarios_agent_name_idx") {
		return Scenario{}, errScenarioTaken
	}
	if err != nil {
		return Scenario{}, fmt.Errorf("update scenario: %w", err)
	}
	return s.GetScenario(ctx, ws, id)
}

// DeleteScenario removes a scenario. Member or above.
func (s *Service) DeleteScenario(ctx context.Context, ws workspace.Workspace, id string) error {
	if err := canAuthor(ws); err != nil {
		return err
	}
	if !uuidPattern.MatchString(id) {
		return errScenarioMissing
	}
	tag, err := s.pool.Exec(ctx, `DELETE FROM scenarios WHERE workspace_id = $1 AND id = $2`, ws.ID, id)
	if err != nil {
		return fmt.Errorf("delete scenario: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return errScenarioMissing
	}
	return nil
}

// DuplicateScenario copies a scenario as "<name> (copy)", numbering when taken. Member or above.
func (s *Service) DuplicateScenario(ctx context.Context, p authn.Principal, ws workspace.Workspace, id string) (Scenario, error) {
	if err := canAuthor(ws); err != nil {
		return Scenario{}, err
	}
	src, err := s.GetScenario(ctx, ws, id)
	if err != nil {
		return Scenario{}, err
	}
	base := src.Name
	if r := []rune(base); len(r) > 145 {
		base = string(r[:145])
	}
	for n := 1; n <= 50; n++ {
		in := src.Input
		in.Name = base + " (copy)"
		if n > 1 {
			in.Name = fmt.Sprintf("%s (copy %d)", base, n)
		}
		newID, err := s.insert(ctx, ws, p.UserID, in)
		if errors.Is(err, errScenarioTaken) {
			continue
		}
		if err != nil {
			return Scenario{}, err
		}
		return s.GetScenario(ctx, ws, newID)
	}
	return Scenario{}, apperr.Conflict("Too many copies of this scenario already exist.")
}

func marshal(in Input) (facts, criteria, ec, intr []byte) {
	facts, _ = json.Marshal(in.Facts)
	criteria, _ = json.Marshal(in.Criteria)
	ec, _ = json.Marshal(in.EndConditions)
	intr, _ = json.Marshal(in.Interruptions)
	return
}
