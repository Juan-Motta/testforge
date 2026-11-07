// Package scenario manages what the simulated user does: reusable personas,
// scenario groups and scenarios with goals, hidden facts and weighted criteria.
package scenario

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"testforge/internal/connection"
	"testforge/internal/platform/apperr"
	"testforge/internal/workspace"
)

var uuidPattern = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

// Service implements personas, groups and scenarios.
type Service struct {
	pool *pgxpool.Pool
}

// NewService builds the service.
func NewService(pool *pgxpool.Pool) *Service { return &Service{pool: pool} }

// canAuthor allows every role but viewer: writing scenarios is everyday work.
func canAuthor(ws workspace.Workspace) error {
	if ws.Role == workspace.RoleViewer {
		return apperr.Forbidden("Viewers cannot change scenarios or personas.")
	}
	return nil
}

func setOnce(fields map[string]string, k, msg string) {
	if _, ok := fields[k]; !ok {
		fields[k] = msg
	}
}

func textLen(fields map[string]string, key, value string, min, max int) string {
	value = strings.TrimSpace(value)
	n := len([]rune(value))
	switch {
	case n < min && min == 1:
		setOnce(fields, key, "Required.")
	case n < min || n > max:
		setOnce(fields, key, fmt.Sprintf("Use %d to %d characters.", min, max))
	}
	return value
}

// checkConnection verifies that id is a connection of the workspace with one
// of the capabilities.
func (s *Service) checkConnection(ctx context.Context, ws workspace.Workspace, id *string, field string, fields map[string]string, caps ...connection.Capability) error {
	if id == nil {
		return nil
	}
	if !uuidPattern.MatchString(*id) {
		setOnce(fields, field, "Choose a connection of this workspace.")
		return nil
	}
	var kind connection.Kind
	err := s.pool.QueryRow(ctx, `SELECT kind FROM connections WHERE workspace_id = $1 AND id = $2`, ws.ID, *id).Scan(&kind)
	if errors.Is(err, pgx.ErrNoRows) {
		setOnce(fields, field, "Choose a connection of this workspace.")
		return nil
	}
	if err != nil {
		return fmt.Errorf("check connection: %w", err)
	}
	spec, _ := connection.Spec(kind)
	for _, c := range caps {
		if slices.Contains(spec.Capabilities, c) {
			return nil
		}
	}
	names := make([]string, len(caps))
	for i, c := range caps {
		names[i] = string(c)
	}
	setOnce(fields, field, "This connection does not provide "+strings.Join(names, " or ")+".")
	return nil
}

func inUse(what string, n int) error {
	noun := "scenarios"
	if n == 1 {
		noun = "scenario"
	}
	return apperr.Conflict(fmt.Sprintf("This %s is used by %d %s. Delete or change them first.", what, n, noun))
}

func shortID() string {
	b := make([]byte, 5)
	_, _ = rand.Read(b)
	return "c_" + hex.EncodeToString(b)
}
