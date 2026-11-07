// Package apikey manages workspace API keys used by the CLI, the MCP server
// and other programmatic clients. Only a SHA-256 hash of each key is stored.
package apikey

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"testforge/internal/authn"
	"testforge/internal/platform/apperr"
	"testforge/internal/workspace"
)

const (
	secretPrefix = "tf_"
	// displayPrefixLen covers "tf_" plus 8 characters of the secret.
	displayPrefixLen = 11
)

var uuidPattern = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

// Creator identifies who created a key.
type Creator struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// Key is an API key without its secret.
type Key struct {
	ID         string     `json:"id"`
	Name       string     `json:"name"`
	Prefix     string     `json:"prefix"`
	CreatedAt  time.Time  `json:"createdAt"`
	LastUsedAt *time.Time `json:"lastUsedAt"`
	ExpiresAt  *time.Time `json:"expiresAt"`
	RevokedAt  *time.Time `json:"revokedAt"`
	CreatedBy  Creator    `json:"createdBy"`
}

// CreatedKey is returned once, at creation, with the full secret.
type CreatedKey struct {
	Key
	Secret string `json:"secret"`
}

// CreateInput is the create request.
type CreateInput struct {
	Name          string `json:"name"`
	ExpiresInDays *int   `json:"expiresInDays"`
}

// Service implements API key use cases.
type Service struct {
	pool *pgxpool.Pool
}

// NewService builds the service.
func NewService(pool *pgxpool.Pool) *Service { return &Service{pool: pool} }

var errSessionOnly = apperr.Forbidden("Managing API keys requires signing in.")

// List returns the workspace keys, newest first. Session only.
func (s *Service) List(ctx context.Context, p authn.Principal, ws workspace.Workspace) ([]Key, error) {
	if p.Method != authn.MethodSession {
		return nil, errSessionOnly
	}
	rows, err := s.pool.Query(ctx, selectKeys+` WHERE k.workspace_id = $1 ORDER BY k.created_at DESC, k.id`, ws.ID)
	if err != nil {
		return nil, fmt.Errorf("list api keys: %w", err)
	}
	return pgx.CollectRows(rows, func(r pgx.CollectableRow) (Key, error) { return scanKey(r) })
}

// Create issues a key. Session only, owner or admin.
func (s *Service) Create(ctx context.Context, p authn.Principal, ws workspace.Workspace, in CreateInput) (CreatedKey, error) {
	if err := canManage(p, ws); err != nil {
		return CreatedKey{}, err
	}
	name := strings.TrimSpace(in.Name)
	fields := map[string]string{}
	if name == "" || len([]rune(name)) > 100 {
		fields["name"] = "Enter a name of 1 to 100 characters."
	}
	if in.ExpiresInDays != nil && (*in.ExpiresInDays < 1 || *in.ExpiresInDays > 3650) {
		fields["expiresInDays"] = "Choose between 1 and 3650 days."
	}
	if err := apperr.Invalid(fields); err != nil {
		return CreatedKey{}, err
	}

	secret, err := newSecret()
	if err != nil {
		return CreatedKey{}, err
	}
	var expiresAt *time.Time
	if in.ExpiresInDays != nil {
		t := time.Now().Add(time.Duration(*in.ExpiresInDays) * 24 * time.Hour)
		expiresAt = &t
	}

	var id string
	err = s.pool.QueryRow(ctx,
		`INSERT INTO api_keys (workspace_id, created_by, name, prefix, key_hash, expires_at)
		 VALUES ($1, $2, $3, $4, $5, $6) RETURNING id`,
		ws.ID, p.UserID, name, secret[:displayPrefixLen], hashSecret(secret), expiresAt,
	).Scan(&id)
	if err != nil {
		return CreatedKey{}, fmt.Errorf("insert api key: %w", err)
	}
	key, err := scanKey(s.pool.QueryRow(ctx, selectKeys+` WHERE k.id = $1`, id))
	if err != nil {
		return CreatedKey{}, fmt.Errorf("load api key: %w", err)
	}
	return CreatedKey{Key: key, Secret: secret}, nil
}

// Revoke revokes a key of the workspace. Revoking twice is a no-op.
func (s *Service) Revoke(ctx context.Context, p authn.Principal, ws workspace.Workspace, keyID string) error {
	if err := canManage(p, ws); err != nil {
		return err
	}
	if !uuidPattern.MatchString(keyID) {
		return apperr.NotFound("API key not found.")
	}
	tag, err := s.pool.Exec(ctx,
		`UPDATE api_keys SET revoked_at = coalesce(revoked_at, now()) WHERE id = $1 AND workspace_id = $2`,
		keyID, ws.ID)
	if err != nil {
		return fmt.Errorf("revoke api key: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return apperr.NotFound("API key not found.")
	}
	return nil
}

// Authenticate resolves a secret to a principal. The key must be active and its
// creator must still be a member of the key's workspace.
func (s *Service) Authenticate(ctx context.Context, secret string) (authn.Principal, bool, error) {
	if !strings.HasPrefix(secret, secretPrefix) || len(secret) < displayPrefixLen {
		return authn.Principal{}, false, nil
	}
	p := authn.Principal{Method: authn.MethodAPIKey}
	err := s.pool.QueryRow(ctx,
		`UPDATE api_keys k SET last_used_at = now()
		 FROM workspace_members m
		 WHERE k.key_hash = $1
		   AND k.revoked_at IS NULL
		   AND (k.expires_at IS NULL OR k.expires_at > now())
		   AND m.workspace_id = k.workspace_id AND m.user_id = k.created_by
		 RETURNING k.id, k.workspace_id, k.created_by`,
		hashSecret(secret),
	).Scan(&p.APIKeyID, &p.WorkspaceID, &p.UserID)
	if errors.Is(err, pgx.ErrNoRows) {
		return authn.Principal{}, false, nil
	}
	if err != nil {
		return authn.Principal{}, false, fmt.Errorf("authenticate api key: %w", err)
	}
	return p, true, nil
}

func canManage(p authn.Principal, ws workspace.Workspace) error {
	if p.Method != authn.MethodSession {
		return errSessionOnly
	}
	if !ws.Role.CanAdminister() {
		return apperr.Forbidden("Only owners and admins can manage API keys.")
	}
	return nil
}

const selectKeys = `SELECT k.id, k.name, k.prefix, k.created_at, k.last_used_at, k.expires_at, k.revoked_at, u.id, u.name
	FROM api_keys k JOIN users u ON u.id = k.created_by`

func scanKey(row interface{ Scan(...any) error }) (Key, error) {
	var k Key
	err := row.Scan(&k.ID, &k.Name, &k.Prefix, &k.CreatedAt, &k.LastUsedAt, &k.ExpiresAt, &k.RevokedAt, &k.CreatedBy.ID, &k.CreatedBy.Name)
	return k, err
}

func newSecret() (string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("generate api key: %w", err)
	}
	return secretPrefix + base64.RawURLEncoding.EncodeToString(raw), nil
}

func hashSecret(secret string) []byte {
	sum := sha256.Sum256([]byte(secret))
	return sum[:]
}
