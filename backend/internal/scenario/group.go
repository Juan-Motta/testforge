package scenario

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"testforge/internal/platform/apperr"
	"testforge/internal/platform/database"
	"testforge/internal/workspace"
)

var (
	errGroupMissing = apperr.NotFound("Group not found.")
	errGroupTaken   = apperr.Conflict("A group with this name already exists in this workspace.")
)

// GroupInput is the create/update request.
type GroupInput struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

// Group organizes scenarios.
type Group struct {
	ID            string    `json:"id"`
	Name          string    `json:"name"`
	Description   string    `json:"description"`
	ScenarioCount int       `json:"scenarioCount"`
	CreatedAt     time.Time `json:"createdAt"`
}

const selectGroups = `SELECT g.id, g.name, g.description, (SELECT count(*) FROM scenarios s WHERE s.group_id = g.id), g.created_at
	FROM scenario_groups g`

func scanGroup(row interface{ Scan(...any) error }) (Group, error) {
	var g Group
	err := row.Scan(&g.ID, &g.Name, &g.Description, &g.ScenarioCount, &g.CreatedAt)
	return g, err
}

// ListGroups returns the workspace groups by name.
func (s *Service) ListGroups(ctx context.Context, ws workspace.Workspace) ([]Group, error) {
	rows, err := s.pool.Query(ctx, selectGroups+` WHERE g.workspace_id = $1 ORDER BY lower(g.name), g.id`, ws.ID)
	if err != nil {
		return nil, fmt.Errorf("list groups: %w", err)
	}
	return pgx.CollectRows(rows, func(r pgx.CollectableRow) (Group, error) { return scanGroup(r) })
}

func (s *Service) getGroup(ctx context.Context, ws workspace.Workspace, id string) (Group, error) {
	if !uuidPattern.MatchString(id) {
		return Group{}, errGroupMissing
	}
	g, err := scanGroup(s.pool.QueryRow(ctx, selectGroups+` WHERE g.workspace_id = $1 AND g.id = $2`, ws.ID, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return Group{}, errGroupMissing
	}
	return g, err
}

func validateGroup(in *GroupInput) error {
	fields := map[string]string{}
	in.Name = textLen(fields, "name", in.Name, 1, 120)
	if len([]rune(in.Description)) > 2000 {
		fields["description"] = "Use at most 2000 characters."
	}
	if err := apperr.Invalid(fields); err != nil {
		return err
	}
	return nil
}

// CreateGroup stores a group. Member or above.
func (s *Service) CreateGroup(ctx context.Context, ws workspace.Workspace, in GroupInput) (Group, error) {
	if err := canAuthor(ws); err != nil {
		return Group{}, err
	}
	if err := validateGroup(&in); err != nil {
		return Group{}, err
	}
	var id string
	err := s.pool.QueryRow(ctx, `INSERT INTO scenario_groups (workspace_id, name, description) VALUES ($1, $2, $3) RETURNING id`,
		ws.ID, in.Name, in.Description).Scan(&id)
	if database.IsUniqueViolation(err, "scenario_groups_workspace_name_idx") {
		return Group{}, errGroupTaken
	}
	if err != nil {
		return Group{}, fmt.Errorf("insert group: %w", err)
	}
	return s.getGroup(ctx, ws, id)
}

// UpdateGroup renames a group. Member or above.
func (s *Service) UpdateGroup(ctx context.Context, ws workspace.Workspace, id string, in GroupInput) (Group, error) {
	if err := canAuthor(ws); err != nil {
		return Group{}, err
	}
	if _, err := s.getGroup(ctx, ws, id); err != nil {
		return Group{}, err
	}
	if err := validateGroup(&in); err != nil {
		return Group{}, err
	}
	_, err := s.pool.Exec(ctx, `UPDATE scenario_groups SET name = $3, description = $4 WHERE workspace_id = $1 AND id = $2`,
		ws.ID, id, in.Name, in.Description)
	if database.IsUniqueViolation(err, "scenario_groups_workspace_name_idx") {
		return Group{}, errGroupTaken
	}
	if err != nil {
		return Group{}, fmt.Errorf("update group: %w", err)
	}
	return s.getGroup(ctx, ws, id)
}

// DeleteGroup removes a group; its scenarios become ungrouped. Member or above.
func (s *Service) DeleteGroup(ctx context.Context, ws workspace.Workspace, id string) error {
	if err := canAuthor(ws); err != nil {
		return err
	}
	if !uuidPattern.MatchString(id) {
		return errGroupMissing
	}
	tag, err := s.pool.Exec(ctx, `DELETE FROM scenario_groups WHERE workspace_id = $1 AND id = $2`, ws.ID, id)
	if err != nil {
		return fmt.Errorf("delete group: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return errGroupMissing
	}
	return nil
}
