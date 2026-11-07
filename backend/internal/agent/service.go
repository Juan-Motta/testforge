package agent

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"testforge/internal/agent/tmpl"
	"testforge/internal/authn"
	"testforge/internal/connection"
	"testforge/internal/platform/apperr"
	"testforge/internal/platform/database"
	"testforge/internal/platform/sealbox"
	"testforge/internal/workspace"
)

var (
	uuidPattern  = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)
	errNotFound  = apperr.NotFound("Agent not found.")
	errNameTaken = apperr.Conflict("An agent with this name already exists in this workspace.")
)

// Service implements agent use cases.
type Service struct {
	pool         *pgxpool.Pool
	box          *sealbox.Box
	allowPrivate bool
	prober       *Prober
}

// NewService builds the service.
func NewService(pool *pgxpool.Pool, box *sealbox.Box, allowPrivate bool) *Service {
	return &Service{pool: pool, box: box, allowPrivate: allowPrivate, prober: NewProber(allowPrivate)}
}

// List returns the workspace agents by name. Any member.
func (s *Service) List(ctx context.Context, ws workspace.Workspace) ([]Agent, error) {
	rows, err := s.pool.Query(ctx, selectAgents+` WHERE a.workspace_id = $1 ORDER BY lower(a.name), a.id`, ws.ID)
	if err != nil {
		return nil, fmt.Errorf("list agents: %w", err)
	}
	return pgx.CollectRows(rows, func(r pgx.CollectableRow) (Agent, error) {
		a, _, err := scanAgent(r)
		return a, err
	})
}

// Get returns one agent. Any member.
func (s *Service) Get(ctx context.Context, ws workspace.Workspace, id string) (Agent, error) {
	a, _, err := s.load(ctx, ws, id)
	return a, err
}

func (s *Service) load(ctx context.Context, ws workspace.Workspace, id string) (Agent, []storedSecret, error) {
	if !uuidPattern.MatchString(id) {
		return Agent{}, nil, errNotFound
	}
	a, stored, err := scanAgent(s.pool.QueryRow(ctx, selectAgents+` WHERE a.workspace_id = $1 AND a.id = $2`, ws.ID, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return Agent{}, nil, errNotFound
	}
	return a, stored, err
}

// Create stores a new agent. Owner or admin.
func (s *Service) Create(ctx context.Context, p authn.Principal, ws workspace.Workspace, in Input) (Agent, error) {
	if err := canManage(ws); err != nil {
		return Agent{}, err
	}
	fields := validateInput(&in, s.allowPrivate)
	for i, sec := range in.Secrets {
		if sec.Value == "" {
			setOnce(fields, fmt.Sprintf("secrets.%d.value", i), "Required.")
		}
	}
	if err := s.checkTraceConnection(ctx, ws, in.TraceConnectionID, fields); err != nil {
		return Agent{}, err
	}
	if err := apperr.Invalid(fields); err != nil {
		return Agent{}, err
	}

	id, err := newID()
	if err != nil {
		return Agent{}, err
	}
	stored, err := s.sealSecrets(ws.ID, id, in.Secrets, nil)
	if err != nil {
		return Agent{}, err
	}
	transport, codecJSON, vars, secrets := marshalParts(in, stored)
	_, err = s.pool.Exec(ctx,
		`INSERT INTO agents (id, workspace_id, name, description, modality, transport, codec, variables, secrets,
		   trace_connection_id, trace_session_id, created_by)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)`,
		id, ws.ID, in.Name, in.Description, in.Modality, transport, codecJSON, vars, secrets,
		in.TraceConnectionID, in.TraceSessionID, p.UserID)
	if database.IsUniqueViolation(err, "agents_workspace_name_idx") {
		return Agent{}, errNameTaken
	}
	if err != nil {
		return Agent{}, fmt.Errorf("insert agent: %w", err)
	}
	return s.Get(ctx, ws, id)
}

// Update replaces an agent's definition. Owner or admin.
func (s *Service) Update(ctx context.Context, ws workspace.Workspace, id string, in Input) (Agent, error) {
	if err := canManage(ws); err != nil {
		return Agent{}, err
	}
	current, stored, err := s.load(ctx, ws, id)
	if err != nil {
		return Agent{}, err
	}
	fields := validateInput(&in, s.allowPrivate)
	if in.Modality != current.Modality {
		setOnce(fields, "modality", "The modality cannot change. Create a new agent instead.")
	}
	byName := map[string]storedSecret{}
	for _, st := range stored {
		byName[st.Name] = st
	}
	before, after := secretHosts(current.Transport), secretHosts(in.Transport)
	for i, sec := range in.Secrets {
		if sec.Value != "" {
			continue
		}
		if _, kept := byName[sec.Name]; !kept {
			setOnce(fields, fmt.Sprintf("secrets.%d.value", i), "Enter a value for this new secret.")
			continue
		}
		for host := range after[sec.Name] {
			if !before[sec.Name][host] {
				setOnce(fields, "secrets", fmt.Sprintf("Enter {{secret.%s}} again: it would now be sent to %s.", sec.Name, host))
			}
		}
	}
	if err := s.checkTraceConnection(ctx, ws, in.TraceConnectionID, fields); err != nil {
		return Agent{}, err
	}
	if err := apperr.Invalid(fields); err != nil {
		return Agent{}, err
	}

	sealed, err := s.sealSecrets(ws.ID, id, in.Secrets, byName)
	if err != nil {
		return Agent{}, err
	}
	transport, codecJSON, vars, secrets := marshalParts(in, sealed)
	_, err = s.pool.Exec(ctx,
		`UPDATE agents SET name = $3, description = $4, transport = $5, codec = $6, variables = $7, secrets = $8,
		   trace_connection_id = $9, trace_session_id = $10, updated_at = now()
		 WHERE workspace_id = $1 AND id = $2`,
		ws.ID, id, in.Name, in.Description, transport, codecJSON, vars, secrets, in.TraceConnectionID, in.TraceSessionID)
	if database.IsUniqueViolation(err, "agents_workspace_name_idx") {
		return Agent{}, errNameTaken
	}
	if err != nil {
		return Agent{}, fmt.Errorf("update agent: %w", err)
	}
	return s.Get(ctx, ws, id)
}

// Delete removes an agent. Owner or admin.
func (s *Service) Delete(ctx context.Context, ws workspace.Workspace, id string) error {
	if err := canManage(ws); err != nil {
		return err
	}
	if !uuidPattern.MatchString(id) {
		return errNotFound
	}
	tag, err := s.pool.Exec(ctx, `DELETE FROM agents WHERE workspace_id = $1 AND id = $2`, ws.ID, id)
	if database.IsForeignKeyViolation(err) {
		var n int
		_ = s.pool.QueryRow(ctx, `SELECT count(*) FROM scenarios WHERE agent_id = $1`, id).Scan(&n)
		noun := "scenarios"
		if n == 1 {
			noun = "scenario"
		}
		return apperr.Conflict(fmt.Sprintf("This agent is used by %d %s. Delete them first.", n, noun))
	}
	if err != nil {
		return fmt.Errorf("delete agent: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return errNotFound
	}
	return nil
}

// ProbeRequest is the probe request body.
type ProbeRequest struct {
	Message         *string `json:"message"`
	DurationSeconds *int    `json:"durationSeconds"`
}

// Probe connects to the agent with its stored configuration. Member or above.
func (s *Service) Probe(ctx context.Context, ws workspace.Workspace, id string, req ProbeRequest) (ProbeResult, error) {
	if ws.Role == workspace.RoleViewer {
		return ProbeResult{}, apperr.Forbidden("Viewers cannot probe agents.")
	}
	a, stored, err := s.load(ctx, ws, id)
	if err != nil {
		return ProbeResult{}, err
	}
	fields := map[string]string{}
	// Text agents may think for a while; voice probes only need to hear the greeting.
	duration := 8 * time.Second
	if a.Modality == ModalityText {
		duration = 30 * time.Second
	}
	if req.DurationSeconds != nil {
		if *req.DurationSeconds < 1 || *req.DurationSeconds > 30 {
			fields["durationSeconds"] = "Choose between 1 and 30 seconds."
		}
		duration = time.Duration(*req.DurationSeconds) * time.Second
	}
	message := "Hello"
	if req.Message != nil {
		if n := len([]rune(*req.Message)); n < 1 || n > 2000 {
			fields["message"] = "Enter a message of 1 to 2000 characters."
		}
		message = *req.Message
	}
	if err := apperr.Invalid(fields); err != nil {
		return ProbeResult{}, err
	}
	secrets, err := s.openSecrets(ws.ID, a.ID, stored)
	if err != nil {
		return ProbeResult{}, err
	}
	// Bound the whole probe (pre-session, dial, listening, close) below the
	// server's write timeout so the client always gets an answer.
	ctx, cancel := context.WithTimeout(ctx, duration+20*time.Second)
	defer cancel()
	return s.prober.Run(ctx, a, secrets, message, duration), nil
}

func (s *Service) checkTraceConnection(ctx context.Context, ws workspace.Workspace, id *string, fields map[string]string) error {
	if id == nil {
		return nil
	}
	if !uuidPattern.MatchString(*id) {
		setOnce(fields, "traceConnectionId", "Choose a Langfuse connection of this workspace.")
		return nil
	}
	var kind connection.Kind
	err := s.pool.QueryRow(ctx, `SELECT kind FROM connections WHERE workspace_id = $1 AND id = $2`, ws.ID, *id).Scan(&kind)
	if errors.Is(err, pgx.ErrNoRows) {
		setOnce(fields, "traceConnectionId", "Choose a Langfuse connection of this workspace.")
		return nil
	}
	if err != nil {
		return fmt.Errorf("check trace connection: %w", err)
	}
	spec, _ := connection.Spec(kind)
	if !slices.Contains(spec.Capabilities, connection.CapTraces) {
		setOnce(fields, "traceConnectionId", "This connection does not provide traces.")
	}
	return nil
}

func (s *Service) sealSecrets(wsID, id string, in []SecretInput, keep map[string]storedSecret) ([]storedSecret, error) {
	out := make([]storedSecret, 0, len(in))
	for _, sec := range in {
		if sec.Value == "" {
			out = append(out, keep[sec.Name])
			continue
		}
		sealed, err := s.box.Seal(sec.Value, secretContext(wsID, id, sec.Name))
		if err != nil {
			return nil, err
		}
		out = append(out, storedSecret{Name: sec.Name, Ciphertext: sealed, Hint: hint(sec.Value)})
	}
	return out, nil
}

// secretHosts maps each referenced secret to the hosts it is sent to. A stored
// secret may only reach hosts it was already sent to; anything else requires
// entering it again, so an admin cannot redirect a credential they cannot read.
func secretHosts(t Transport) map[string]map[string]bool {
	out := map[string]map[string]bool{}
	add := func(host string, templates ...string) {
		for _, raw := range templates {
			parsed, err := tmpl.Parse(raw)
			if err != nil {
				continue
			}
			for _, r := range parsed.Refs() {
				if r.NS == tmpl.NSSecret {
					if out[r.Name] == nil {
						out[r.Name] = map[string]bool{}
					}
					out[r.Name][host] = true
				}
			}
		}
	}
	host := urlHost(t.URL)
	add(host, t.URL)
	for _, kv := range append(append([]KV{}, t.Query...), t.Headers...) {
		add(host, kv.Value)
	}
	if t.Body != nil {
		add(host, *t.Body)
	}
	add(host, t.InitMessages...)
	if ps := t.PreSession; ps != nil {
		psHost := urlHost(ps.URL)
		add(psHost, ps.URL)
		for _, kv := range ps.Headers {
			add(psHost, kv.Value)
		}
		if ps.Body != nil {
			add(psHost, *ps.Body)
		}
	}
	return out
}

// urlHost returns the lower-cased "scheme://host" of a URL template.
func urlHost(raw string) string {
	scheme, rest, _ := strings.Cut(strings.TrimSpace(raw), "://")
	host, _, _ := strings.Cut(rest, "/")
	return strings.ToLower(scheme + "://" + host)
}

func canManage(ws workspace.Workspace) error {
	if !ws.Role.CanAdminister() {
		return apperr.Forbidden("Only owners and admins can manage agents.")
	}
	return nil
}

func setOnce(fields map[string]string, k, msg string) {
	if _, ok := fields[k]; !ok {
		fields[k] = msg
	}
}

// secretContext binds a sealed secret to its workspace, agent and name.
func secretContext(wsID, id, name string) string {
	return "agent/" + wsID + "/" + id + "/" + name
}

func hint(secret string) string {
	runes := []rune(secret)
	if len(runes) < 8 {
		return "••••"
	}
	return "•••• " + string(runes[len(runes)-4:])
}

func newID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate id: %w", err)
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16]), nil
}

func marshalParts(in Input, secrets []storedSecret) (transport, codecJSON, vars, secretsJSON []byte) {
	transport, _ = json.Marshal(in.Transport)
	codecJSON, _ = json.Marshal(in.Codec)
	vars, _ = json.Marshal(in.Variables)
	secretsJSON, _ = json.Marshal(secrets)
	return
}

const selectAgents = `SELECT a.id, a.name, a.description, a.modality, a.transport, a.codec, a.variables, a.secrets,
	a.trace_connection_id, a.trace_session_id, u.id, u.name, a.created_at, a.updated_at
	FROM agents a LEFT JOIN users u ON u.id = a.created_by`

func scanAgent(row interface{ Scan(...any) error }) (Agent, []storedSecret, error) {
	var a Agent
	var transport, codecJSON, vars, secretsJSON []byte
	var creatorID, creatorName *string
	if err := row.Scan(&a.ID, &a.Name, &a.Description, &a.Modality, &transport, &codecJSON, &vars, &secretsJSON,
		&a.TraceConnectionID, &a.TraceSessionID, &creatorID, &creatorName, &a.CreatedAt, &a.UpdatedAt); err != nil {
		return Agent{}, nil, err
	}
	var stored []storedSecret
	for _, part := range []struct {
		raw []byte
		dst any
	}{{transport, &a.Transport}, {codecJSON, &a.Codec}, {vars, &a.Variables}, {secretsJSON, &stored}} {
		if err := json.Unmarshal(part.raw, part.dst); err != nil {
			return Agent{}, nil, fmt.Errorf("decode agent: %w", err)
		}
	}
	a.Secrets = make([]SecretInfo, len(stored))
	for i, st := range stored {
		a.Secrets[i] = SecretInfo{Name: st.Name, Hint: st.Hint}
	}
	if creatorID != nil && creatorName != nil {
		a.CreatedBy = &Creator{ID: *creatorID, Name: *creatorName}
	}
	return a, stored, nil
}

func (s *Service) openSecrets(wsID, agentID string, stored []storedSecret) (map[string]string, error) {
	secrets := map[string]string{}
	for _, st := range stored {
		plain, err := s.box.Open(st.Ciphertext, secretContext(wsID, agentID, st.Name))
		if err != nil {
			return nil, fmt.Errorf("open agent secret %q: %w", st.Name, err)
		}
		secrets[st.Name] = plain
	}
	return secrets, nil
}

// Secrets returns the plaintext secrets of an agent, for the worker. Never
// expose the result through the API.
func (s *Service) Secrets(ctx context.Context, workspaceID, agentID string) (map[string]string, error) {
	_, stored, err := s.load(ctx, workspace.Workspace{ID: workspaceID}, agentID)
	if err != nil {
		return nil, err
	}
	return s.openSecrets(workspaceID, agentID, stored)
}
