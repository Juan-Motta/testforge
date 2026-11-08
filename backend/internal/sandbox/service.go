package sandbox

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"testforge/internal/agent"
	"testforge/internal/apikey"
	"testforge/internal/authn"
	"testforge/internal/connection"
	"testforge/internal/platform/apperr"
	"testforge/internal/platform/database"
	"testforge/internal/workspace"
)

// Credentials opens a workspace connection.
type Credentials interface {
	Credentials(ctx context.Context, workspaceID, id string) (connection.Credentials, error)
}

// AgentCreator creates regular agents (link).
type AgentCreator interface {
	Create(ctx context.Context, p authn.Principal, ws workspace.Workspace, in agent.Input) (agent.Agent, error)
}

// KeyManager creates and revokes workspace API keys (link).
type KeyManager interface {
	Create(ctx context.Context, p authn.Principal, ws workspace.Workspace, in apikey.CreateInput) (apikey.CreatedKey, error)
	Revoke(ctx context.Context, p authn.Principal, ws workspace.Workspace, keyID string) error
}

// Options configure the service.
type Options struct {
	AllowPrivate bool
	// PublicAPIURL is where run workers reach this API (linked agents call it).
	PublicAPIURL string
	// Rand returns a float in [0, 1) for fault injection; default math/rand/v2.
	Rand func() float64
}

// Service implements sandbox use cases.
type Service struct {
	pool   *pgxpool.Pool
	conns  Credentials
	agents AgentCreator
	keys   KeyManager
	opts   Options
}

// NewService builds the service.
func NewService(pool *pgxpool.Pool, conns Credentials, agents AgentCreator, keys KeyManager, opts Options) *Service {
	if opts.Rand == nil {
		opts.Rand = defaultRand
	}
	return &Service{pool: pool, conns: conns, agents: agents, keys: keys, opts: opts}
}

var errNameTaken = apperr.Conflict("A sandbox agent with this name already exists.")

// canAuthor allows every role but viewer.
func canAuthor(ws workspace.Workspace) error {
	if ws.Role == workspace.RoleViewer {
		return apperr.Forbidden("Viewers cannot change or use sandbox agents.")
	}
	return nil
}

const selectAgent = `SELECT id, name, description, modality, coalesce(connection_id::text, ''), model, system_prompt,
	temperature, memory, tools, created_at, updated_at FROM sandbox_agents`

func scanAgent(row pgx.Row) (Agent, error) {
	var a Agent
	var memory, tools []byte
	if err := row.Scan(&a.ID, &a.Name, &a.Description, &a.Modality, &a.ConnectionID, &a.Model, &a.SystemPrompt,
		&a.Temperature, &memory, &tools, &a.CreatedAt, &a.UpdatedAt); err != nil {
		return Agent{}, err
	}
	if err := json.Unmarshal(memory, &a.Memory); err != nil {
		return Agent{}, err
	}
	if err := json.Unmarshal(tools, &a.Tools); err != nil {
		return Agent{}, err
	}
	return a, nil
}

// List returns the workspace's sandbox agents by name.
func (s *Service) List(ctx context.Context, ws workspace.Workspace) ([]Agent, error) {
	rows, err := s.pool.Query(ctx, selectAgent+` WHERE workspace_id = $1 ORDER BY lower(name)`, ws.ID)
	if err != nil {
		return nil, fmt.Errorf("list sandbox agents: %w", err)
	}
	defer rows.Close()
	out := []Agent{}
	for rows.Next() {
		a, err := scanAgent(rows)
		if err != nil {
			return nil, fmt.Errorf("scan sandbox agent: %w", err)
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// Get returns one sandbox agent.
func (s *Service) Get(ctx context.Context, ws workspace.Workspace, id string) (Agent, error) {
	if !uuidPattern.MatchString(id) {
		return Agent{}, apperr.NotFound("Sandbox agent not found.")
	}
	a, err := scanAgent(s.pool.QueryRow(ctx, selectAgent+` WHERE workspace_id = $1 AND id = $2`, ws.ID, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return Agent{}, apperr.NotFound("Sandbox agent not found.")
	}
	if err != nil {
		return Agent{}, fmt.Errorf("get sandbox agent: %w", err)
	}
	return a, nil
}

func (s *Service) validate(ctx context.Context, ws workspace.Workspace, in *Input) error {
	fields := map[string]string{}
	validateDraft(in, fields)
	if err := s.checkConnection(ctx, ws, in.ConnectionID, fields); err != nil {
		return err
	}
	// apperr.Invalid returns a typed nil; do not return it as an error interface.
	if err := apperr.Invalid(fields); err != nil {
		return err
	}
	return nil
}

// checkConnection requires an OpenAI or Azure OpenAI connection of the workspace.
func (s *Service) checkConnection(ctx context.Context, ws workspace.Workspace, id string, fields map[string]string) error {
	if !uuidPattern.MatchString(id) {
		setOnce(fields, "connectionId", "Choose a connection of this workspace.")
		return nil
	}
	var kind connection.Kind
	err := s.pool.QueryRow(ctx, `SELECT kind FROM connections WHERE workspace_id = $1 AND id = $2`, ws.ID, id).Scan(&kind)
	if errors.Is(err, pgx.ErrNoRows) {
		setOnce(fields, "connectionId", "Choose a connection of this workspace.")
		return nil
	}
	if err != nil {
		return fmt.Errorf("check connection: %w", err)
	}
	if !slices.Contains(toolKinds, kind) {
		setOnce(fields, "connectionId", "Sandbox agents support OpenAI and Azure OpenAI connections.")
	}
	return nil
}

// Create stores a sandbox agent. Member or above.
func (s *Service) Create(ctx context.Context, p authn.Principal, ws workspace.Workspace, in Input) (Agent, error) {
	if err := canAuthor(ws); err != nil {
		return Agent{}, err
	}
	if err := s.validate(ctx, ws, &in); err != nil {
		return Agent{}, err
	}
	memory, tools := marshalParts(in)
	var createdBy *string
	if p.UserID != "" {
		createdBy = &p.UserID
	}
	var id string
	err := s.pool.QueryRow(ctx,
		`INSERT INTO sandbox_agents (workspace_id, name, description, modality, connection_id, model, system_prompt,
		   temperature, memory, tools, created_by)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11) RETURNING id`,
		ws.ID, in.Name, in.Description, in.Modality, in.ConnectionID, in.Model, in.SystemPrompt,
		in.Temperature, memory, tools, createdBy).Scan(&id)
	if database.IsUniqueViolation(err, "sandbox_agents_workspace_name_idx") {
		return Agent{}, errNameTaken
	}
	if err != nil {
		return Agent{}, fmt.Errorf("insert sandbox agent: %w", err)
	}
	return s.Get(ctx, ws, id)
}

// Update replaces a sandbox agent; conversations and memories stay. Member or above.
func (s *Service) Update(ctx context.Context, ws workspace.Workspace, id string, in Input) (Agent, error) {
	if err := canAuthor(ws); err != nil {
		return Agent{}, err
	}
	if _, err := s.Get(ctx, ws, id); err != nil {
		return Agent{}, err
	}
	if err := s.validate(ctx, ws, &in); err != nil {
		return Agent{}, err
	}
	memory, tools := marshalParts(in)
	tag, err := s.pool.Exec(ctx,
		`UPDATE sandbox_agents SET name = $3, description = $4, modality = $5, connection_id = $6, model = $7,
		   system_prompt = $8, temperature = $9, memory = $10, tools = $11, updated_at = now()
		 WHERE workspace_id = $1 AND id = $2`,
		ws.ID, id, in.Name, in.Description, in.Modality, in.ConnectionID, in.Model, in.SystemPrompt,
		in.Temperature, memory, tools)
	if database.IsUniqueViolation(err, "sandbox_agents_workspace_name_idx") {
		return Agent{}, errNameTaken
	}
	if err != nil {
		return Agent{}, fmt.Errorf("update sandbox agent: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return Agent{}, apperr.NotFound("Sandbox agent not found.")
	}
	return s.Get(ctx, ws, id)
}

// Delete removes a sandbox agent with its conversations and memories. Member or above.
func (s *Service) Delete(ctx context.Context, ws workspace.Workspace, id string) error {
	if err := canAuthor(ws); err != nil {
		return err
	}
	if !uuidPattern.MatchString(id) {
		return apperr.NotFound("Sandbox agent not found.")
	}
	tag, err := s.pool.Exec(ctx, `DELETE FROM sandbox_agents WHERE workspace_id = $1 AND id = $2`, ws.ID, id)
	if err != nil {
		return fmt.Errorf("delete sandbox agent: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return apperr.NotFound("Sandbox agent not found.")
	}
	return nil
}

func marshalParts(in Input) (memory, tools []byte) {
	memory, _ = json.Marshal(in.Memory)
	tools, _ = json.Marshal(in.Tools)
	return memory, tools
}
