// Package connection manages workspace provider connections: model and speech
// providers used to simulate users and judge runs, and trace sources such as
// Langfuse. Secrets are sealed at rest and never leave the service.
package connection

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"testforge/internal/authn"
	"testforge/internal/platform/apperr"
	"testforge/internal/platform/database"
	"testforge/internal/platform/sealbox"
	"testforge/internal/workspace"
)

var (
	uuidPattern = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)
	errNotFound = apperr.NotFound("Connection not found.")
)

// Creator identifies who created a connection.
type Creator struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// Connection is the public view of a stored connection (no secret).
type Connection struct {
	ID           string            `json:"id"`
	Name         string            `json:"name"`
	Kind         Kind              `json:"kind"`
	Config       map[string]string `json:"config"`
	SecretHint   string            `json:"secretHint"`
	Capabilities []Capability      `json:"capabilities"`
	LastTest     *TestResult       `json:"lastTest"`
	CreatedBy    *Creator          `json:"createdBy"`
	CreatedAt    time.Time         `json:"createdAt"`
	UpdatedAt    time.Time         `json:"updatedAt"`
}

// CreateInput is the create request.
type CreateInput struct {
	Name   string            `json:"name"`
	Kind   Kind              `json:"kind"`
	Config map[string]string `json:"config"`
	Secret string            `json:"secret"`
}

// UpdateInput is the partial update request; nil means "keep".
type UpdateInput struct {
	Name   *string            `json:"name"`
	Config *map[string]string `json:"config"`
	Secret *string            `json:"secret"`
}

// Service implements connection use cases.
type Service struct {
	pool         *pgxpool.Pool
	box          *sealbox.Box
	tester       *Tester
	allowPrivate bool
}

// NewService builds the service.
func NewService(pool *pgxpool.Pool, box *sealbox.Box, allowPrivate bool) *Service {
	return &Service{pool: pool, box: box, tester: NewTester(allowPrivate), allowPrivate: allowPrivate}
}

// List returns the workspace connections ordered by name. Any member.
func (s *Service) List(ctx context.Context, ws workspace.Workspace) ([]Connection, error) {
	rows, err := s.pool.Query(ctx, selectConnections+` WHERE c.workspace_id = $1 ORDER BY lower(c.name), c.id`, ws.ID)
	if err != nil {
		return nil, fmt.Errorf("list connections: %w", err)
	}
	return pgx.CollectRows(rows, func(r pgx.CollectableRow) (Connection, error) { return scanConnection(r) })
}

// Get returns one connection of the workspace. Any member.
func (s *Service) Get(ctx context.Context, ws workspace.Workspace, id string) (Connection, error) {
	if !uuidPattern.MatchString(id) {
		return Connection{}, errNotFound
	}
	c, err := scanConnection(s.pool.QueryRow(ctx, selectConnections+` WHERE c.workspace_id = $1 AND c.id = $2`, ws.ID, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return Connection{}, errNotFound
	}
	return c, err
}

// Create stores a new connection. Owner or admin.
func (s *Service) Create(ctx context.Context, p authn.Principal, ws workspace.Workspace, in CreateInput) (Connection, error) {
	if err := canManage(ws); err != nil {
		return Connection{}, err
	}
	fields := map[string]string{}
	name := validateName(in.Name, fields)
	spec, ok := Spec(in.Kind)
	if !ok {
		fields["kind"] = "Choose a supported provider."
	}
	secret := strings.TrimSpace(in.Secret)
	validateSecret(secret, true, fields)
	var cfg map[string]string
	if ok {
		cfg = normalizeConfig(spec, in.Config, s.allowPrivate, fields)
	}
	if err := apperr.Invalid(fields); err != nil {
		return Connection{}, err
	}

	id, err := newID()
	if err != nil {
		return Connection{}, err
	}
	sealed, err := s.box.Seal(secret, sealContext(ws.ID, id, in.Kind))
	if err != nil {
		return Connection{}, err
	}
	cfgJSON, _ := json.Marshal(cfg)
	_, err = s.pool.Exec(ctx,
		`INSERT INTO connections (id, workspace_id, name, kind, config, secret_ciphertext, secret_hint, created_by)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		id, ws.ID, name, in.Kind, cfgJSON, sealed, secretHint(secret), p.UserID)
	if database.IsUniqueViolation(err, "connections_workspace_name_idx") {
		return Connection{}, errNameTaken
	}
	if err != nil {
		return Connection{}, fmt.Errorf("insert connection: %w", err)
	}
	return s.Get(ctx, ws, id)
}

var errNameTaken = apperr.Conflict("A connection with this name already exists in this workspace.")

// Update changes name, config and/or secret. Owner or admin. Clears the last test.
func (s *Service) Update(ctx context.Context, ws workspace.Workspace, id string, in UpdateInput) (Connection, error) {
	if err := canManage(ws); err != nil {
		return Connection{}, err
	}
	current, err := s.Get(ctx, ws, id)
	if err != nil {
		return Connection{}, err
	}
	spec, _ := Spec(current.Kind)

	fields := map[string]string{}
	name := current.Name
	if in.Name != nil {
		name = validateName(*in.Name, fields)
	}
	cfg := current.Config
	if in.Config != nil {
		cfg = normalizeConfig(spec, *in.Config, s.allowPrivate, fields)
	}
	var newSecret string
	if in.Secret != nil {
		newSecret = strings.TrimSpace(*in.Secret)
		validateSecret(newSecret, false, fields)
	}
	if err := apperr.Invalid(fields); err != nil {
		return Connection{}, err
	}

	cfgJSON, _ := json.Marshal(cfg)
	args := []any{ws.ID, id, name, cfgJSON}
	setSecret := ""
	if newSecret != "" {
		sealed, err := s.box.Seal(newSecret, sealContext(ws.ID, id, current.Kind))
		if err != nil {
			return Connection{}, err
		}
		args = append(args, sealed, secretHint(newSecret))
		setSecret = ", secret_ciphertext = $5, secret_hint = $6"
	}
	_, err = s.pool.Exec(ctx,
		`UPDATE connections SET name = $3, config = $4`+setSecret+`,
		   last_test_ok = NULL, last_test_message = NULL, last_test_latency_ms = NULL, last_tested_at = NULL,
		   updated_at = now()
		 WHERE workspace_id = $1 AND id = $2`, args...)
	if database.IsUniqueViolation(err, "connections_workspace_name_idx") {
		return Connection{}, errNameTaken
	}
	if err != nil {
		return Connection{}, fmt.Errorf("update connection: %w", err)
	}
	return s.Get(ctx, ws, id)
}

// Delete removes a connection. Owner or admin.
func (s *Service) Delete(ctx context.Context, ws workspace.Workspace, id string) error {
	if err := canManage(ws); err != nil {
		return err
	}
	if !uuidPattern.MatchString(id) {
		return errNotFound
	}
	tag, err := s.pool.Exec(ctx, `DELETE FROM connections WHERE workspace_id = $1 AND id = $2`, ws.ID, id)
	if err != nil {
		return fmt.Errorf("delete connection: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return errNotFound
	}
	return nil
}

// Test checks the stored credentials against the provider and records the result.
// Members and above; viewers are read-only.
func (s *Service) Test(ctx context.Context, ws workspace.Workspace, id string) (TestResult, error) {
	if ws.Role == workspace.RoleViewer {
		return TestResult{}, apperr.Forbidden("Viewers cannot test connections.")
	}
	if !uuidPattern.MatchString(id) {
		return TestResult{}, errNotFound
	}
	var kind Kind
	var cfgJSON []byte
	var sealed string
	err := s.pool.QueryRow(ctx,
		`SELECT kind, config, secret_ciphertext FROM connections WHERE workspace_id = $1 AND id = $2`, ws.ID, id,
	).Scan(&kind, &cfgJSON, &sealed)
	if errors.Is(err, pgx.ErrNoRows) {
		return TestResult{}, errNotFound
	}
	if err != nil {
		return TestResult{}, fmt.Errorf("load connection: %w", err)
	}
	var cfg map[string]string
	if err := json.Unmarshal(cfgJSON, &cfg); err != nil {
		return TestResult{}, fmt.Errorf("decode connection config: %w", err)
	}
	secret, err := s.box.Open(sealed, sealContext(ws.ID, id, kind))
	if err != nil {
		return TestResult{}, fmt.Errorf("open connection secret: %w", err)
	}

	res := s.tester.Test(ctx, kind, cfg, secret)
	if _, err := s.pool.Exec(ctx,
		`UPDATE connections SET last_test_ok = $3, last_test_message = $4, last_test_latency_ms = $5, last_tested_at = $6
		 WHERE workspace_id = $1 AND id = $2`,
		ws.ID, id, res.OK, res.Message, res.LatencyMs, res.TestedAt,
	); err != nil {
		return TestResult{}, fmt.Errorf("record connection test: %w", err)
	}
	return res, nil
}

func canManage(ws workspace.Workspace) error {
	if !ws.Role.CanAdminister() {
		return apperr.Forbidden("Only owners and admins can manage connections.")
	}
	return nil
}

func validateName(raw string, fields map[string]string) string {
	name := strings.TrimSpace(raw)
	if name == "" || len([]rune(name)) > 100 {
		fields["name"] = "Enter a name of 1 to 100 characters."
	}
	return name
}

func validateSecret(secret string, required bool, fields map[string]string) {
	switch {
	case secret == "" && required:
		fields["secret"] = "Required."
	case len(secret) > 4096:
		fields["secret"] = "Use at most 4096 characters."
	}
}

// sealContext binds a ciphertext to its workspace, connection and kind.
func sealContext(workspaceID, id string, kind Kind) string {
	return "connection/" + workspaceID + "/" + id + "/" + string(kind)
}

func secretHint(secret string) string {
	if len(secret) < 8 {
		return "••••"
	}
	return "•••• " + secret[len(secret)-4:]
}

func newID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate id: %w", err)
	}
	b[6] = (b[6] & 0x0f) | 0x40 // version 4
	b[8] = (b[8] & 0x3f) | 0x80 // RFC 4122 variant
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16]), nil
}

const selectConnections = `SELECT c.id, c.name, c.kind, c.config, c.secret_hint,
	c.last_test_ok, c.last_test_message, c.last_test_latency_ms, c.last_tested_at,
	u.id, u.name, c.created_at, c.updated_at
	FROM connections c LEFT JOIN users u ON u.id = c.created_by`

func scanConnection(row interface{ Scan(...any) error }) (Connection, error) {
	var c Connection
	var cfgJSON []byte
	var testOK *bool
	var testMsg *string
	var testLatency *int
	var testedAt *time.Time
	var creatorID, creatorName *string
	if err := row.Scan(&c.ID, &c.Name, &c.Kind, &cfgJSON, &c.SecretHint,
		&testOK, &testMsg, &testLatency, &testedAt,
		&creatorID, &creatorName, &c.CreatedAt, &c.UpdatedAt); err != nil {
		return Connection{}, err
	}
	if err := json.Unmarshal(cfgJSON, &c.Config); err != nil {
		return Connection{}, fmt.Errorf("decode connection config: %w", err)
	}
	if spec, ok := Spec(c.Kind); ok {
		c.Capabilities = spec.Capabilities
	} else {
		c.Capabilities = []Capability{}
	}
	if testOK != nil && testedAt != nil {
		c.LastTest = &TestResult{OK: *testOK, TestedAt: *testedAt}
		if testMsg != nil {
			c.LastTest.Message = *testMsg
		}
		if testLatency != nil {
			c.LastTest.LatencyMs = *testLatency
		}
	}
	if creatorID != nil && creatorName != nil {
		c.CreatedBy = &Creator{ID: *creatorID, Name: *creatorName}
	}
	return c, nil
}
