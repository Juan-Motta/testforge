// Package workspace manages workspaces, memberships and role checks. Every
// product resource belongs to a workspace, so Resolve is the tenancy gate.
package workspace

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/text/unicode/norm"

	"testforge/internal/authn"
	"testforge/internal/platform/apperr"
	"testforge/internal/platform/database"
)

// Role is a member's role in a workspace.
type Role string

const (
	RoleOwner  Role = "owner"
	RoleAdmin  Role = "admin"
	RoleMember Role = "member"
	RoleViewer Role = "viewer"
)

// CanAdminister reports whether the role may manage workspace settings such as API keys.
func (r Role) CanAdminister() bool { return r == RoleOwner || r == RoleAdmin }

// Workspace is a workspace as seen by one caller (Role is the caller's role).
type Workspace struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Slug      string    `json:"slug"`
	Role      Role      `json:"role"`
	CreatedAt time.Time `json:"createdAt"`
}

var errNotFound = apperr.NotFound("Workspace not found.")

// Service implements workspace use cases.
type Service struct {
	pool *pgxpool.Pool
}

// NewService builds the service.
func NewService(pool *pgxpool.Pool) *Service { return &Service{pool: pool} }

// Create makes a workspace owned by the principal's user. Session only.
func (s *Service) Create(ctx context.Context, p authn.Principal, name string) (Workspace, error) {
	if p.Method != authn.MethodSession {
		return Workspace{}, apperr.Forbidden("Creating workspaces requires signing in.")
	}
	name = strings.TrimSpace(name)
	if err := validateName(name); err != nil {
		return Workspace{}, err
	}
	var ws Workspace
	err := database.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		var err error
		ws, err = create(ctx, tx, p.UserID, name)
		return err
	})
	return ws, err
}

// CreatePersonal provisions the personal workspace of a new user inside the
// sign-up transaction.
func CreatePersonal(ctx context.Context, tx pgx.Tx, userID, userName string) error {
	_, err := create(ctx, tx, userID, userName+"'s workspace")
	return err
}

// List returns the workspaces the principal can reach.
func (s *Service) List(ctx context.Context, p authn.Principal) ([]Workspace, error) {
	if p.Method == authn.MethodAPIKey {
		ws, err := s.byID(ctx, p.WorkspaceID)
		if err != nil {
			return nil, err
		}
		return []Workspace{ws}, nil
	}
	rows, err := s.pool.Query(ctx,
		`SELECT w.id, w.name, w.slug, m.role, w.created_at
		 FROM workspaces w JOIN workspace_members m ON m.workspace_id = w.id
		 WHERE m.user_id = $1 ORDER BY w.created_at, w.id`, p.UserID)
	if err != nil {
		return nil, fmt.Errorf("list workspaces: %w", err)
	}
	return pgx.CollectRows(rows, func(r pgx.CollectableRow) (Workspace, error) { return scanWorkspace(r) })
}

// Resolve returns the workspace identified by slug if the principal can reach
// it, with the principal's role. Unreachable workspaces are reported as not
// found so their existence does not leak.
func (s *Service) Resolve(ctx context.Context, p authn.Principal, slug string) (Workspace, error) {
	if p.Method == authn.MethodAPIKey {
		ws, err := s.byID(ctx, p.WorkspaceID)
		if err != nil || ws.Slug != slug {
			return Workspace{}, errNotFound
		}
		return ws, nil
	}
	row := s.pool.QueryRow(ctx,
		`SELECT w.id, w.name, w.slug, m.role, w.created_at
		 FROM workspaces w JOIN workspace_members m ON m.workspace_id = w.id
		 WHERE w.slug = $1 AND m.user_id = $2`, slug, p.UserID)
	ws, err := scanWorkspace(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return Workspace{}, errNotFound
	}
	return ws, err
}

// byID loads a workspace for an API key, which acts with the member role.
func (s *Service) byID(ctx context.Context, id string) (Workspace, error) {
	ws := Workspace{Role: RoleMember}
	err := s.pool.QueryRow(ctx, `SELECT id, name, slug, created_at FROM workspaces WHERE id = $1`, id).
		Scan(&ws.ID, &ws.Name, &ws.Slug, &ws.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return Workspace{}, errNotFound
	}
	return ws, err
}

func create(ctx context.Context, tx pgx.Tx, ownerID, name string) (Workspace, error) {
	base := Slugify(name)
	ws := Workspace{Name: name, Role: RoleOwner}
	for attempt := 0; ; attempt++ {
		slug := base
		if attempt > 0 {
			slug = base + "-" + randomSuffix()
		}
		err := tx.QueryRow(ctx,
			`INSERT INTO workspaces (name, slug) VALUES ($1, $2)
			 ON CONFLICT (slug) DO NOTHING RETURNING id, slug, created_at`, name, slug,
		).Scan(&ws.ID, &ws.Slug, &ws.CreatedAt)
		if err == nil {
			break
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return Workspace{}, fmt.Errorf("insert workspace: %w", err)
		}
		if attempt >= 5 {
			return Workspace{}, fmt.Errorf("insert workspace: no free slug for %q", base)
		}
	}
	if _, err := tx.Exec(ctx,
		`INSERT INTO workspace_members (workspace_id, user_id, role) VALUES ($1, $2, $3)`,
		ws.ID, ownerID, RoleOwner,
	); err != nil {
		return Workspace{}, fmt.Errorf("insert owner membership: %w", err)
	}
	return ws, nil
}

func validateName(name string) error {
	if name == "" || len([]rune(name)) > 100 {
		return apperr.Invalid(map[string]string{"name": "Enter a name of 1 to 100 characters."})
	}
	return nil
}

func scanWorkspace(row interface{ Scan(...any) error }) (Workspace, error) {
	var ws Workspace
	err := row.Scan(&ws.ID, &ws.Name, &ws.Slug, &ws.Role, &ws.CreatedAt)
	return ws, err
}

// Slugify turns a name into a URL-safe slug: lowercase ASCII letters and
// digits separated by single hyphens. Names without any of them become "workspace".
func Slugify(name string) string {
	var b strings.Builder
	pendingHyphen := false
	// NFD splits accented letters into base letter + combining mark ("é" → "e" + "´"),
	// so dropping the marks keeps "Señal" readable as "senal".
	for _, r := range norm.NFD.String(strings.ToLower(name)) {
		if unicode.Is(unicode.Mn, r) {
			continue
		}
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			if pendingHyphen && b.Len() > 0 {
				b.WriteByte('-')
			}
			pendingHyphen = false
			b.WriteRune(r)
			continue
		}
		if unicode.IsSpace(r) || unicode.IsPunct(r) || unicode.IsSymbol(r) || r > unicode.MaxASCII {
			pendingHyphen = true
		}
	}
	slug := b.String()
	if len(slug) > 48 {
		slug = strings.TrimRight(slug[:48], "-")
	}
	if slug == "" {
		return "workspace"
	}
	return slug
}

func randomSuffix() string {
	b := make([]byte, 3)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
