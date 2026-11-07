package scenario

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"testforge/internal/connection"
	"testforge/internal/platform/apperr"
	"testforge/internal/platform/database"
	"testforge/internal/workspace"
)

var (
	// language[-Script][-REGION], e.g. en, es-MX, zh-Hant-TW, es-419 (case-insensitive).
	localePattern     = regexp.MustCompile(`^(?i)[a-z]{2,3}(-[a-z]{4})?(-([a-z]{2}|[0-9]{3}))?$`)
	temperaments      = []string{"polite", "hurried", "skeptical", "confused", "angry", "friendly", "calm", "detailed"}
	errPersonaMissing = apperr.NotFound("Persona not found.")
	errPersonaTaken   = apperr.Conflict("A persona with this name already exists in this workspace.")
)

// Voice is how the persona sounds.
type Voice struct {
	ConnectionID string `json:"connectionId"`
	Voice        string `json:"voice"`
}

// PersonaInput is the create/update request.
type PersonaInput struct {
	Name        string `json:"name"`
	Locale      string `json:"locale"`
	Temperament string `json:"temperament"`
	Notes       string `json:"notes"`
	Voice       *Voice `json:"voice"`
}

// Persona is a reusable simulated user.
type Persona struct {
	ID            string    `json:"id"`
	Name          string    `json:"name"`
	Locale        string    `json:"locale"`
	Temperament   string    `json:"temperament"`
	Notes         string    `json:"notes"`
	Voice         *Voice    `json:"voice"`
	ScenarioCount int       `json:"scenarioCount"`
	CreatedAt     time.Time `json:"createdAt"`
	UpdatedAt     time.Time `json:"updatedAt"`
}

const selectPersonas = `SELECT p.id, p.name, p.locale, p.temperament, p.notes, p.voice_connection_id, p.voice_name,
	(SELECT count(*) FROM scenarios s WHERE s.persona_id = p.id), p.created_at, p.updated_at FROM personas p`

func scanPersona(row interface{ Scan(...any) error }) (Persona, error) {
	var p Persona
	var connID, voice *string
	err := row.Scan(&p.ID, &p.Name, &p.Locale, &p.Temperament, &p.Notes, &connID, &voice, &p.ScenarioCount, &p.CreatedAt, &p.UpdatedAt)
	if connID != nil && voice != nil {
		p.Voice = &Voice{ConnectionID: *connID, Voice: *voice}
	}
	return p, err
}

// ListPersonas returns the workspace personas by name.
func (s *Service) ListPersonas(ctx context.Context, ws workspace.Workspace) ([]Persona, error) {
	rows, err := s.pool.Query(ctx, selectPersonas+` WHERE p.workspace_id = $1 ORDER BY lower(p.name), p.id`, ws.ID)
	if err != nil {
		return nil, fmt.Errorf("list personas: %w", err)
	}
	return pgx.CollectRows(rows, func(r pgx.CollectableRow) (Persona, error) { return scanPersona(r) })
}

// GetPersona returns one persona.
func (s *Service) GetPersona(ctx context.Context, ws workspace.Workspace, id string) (Persona, error) {
	if !uuidPattern.MatchString(id) {
		return Persona{}, errPersonaMissing
	}
	p, err := scanPersona(s.pool.QueryRow(ctx, selectPersonas+` WHERE p.workspace_id = $1 AND p.id = $2`, ws.ID, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return Persona{}, errPersonaMissing
	}
	return p, err
}

func (s *Service) validatePersona(ctx context.Context, ws workspace.Workspace, in *PersonaInput) (map[string]string, error) {
	fields := map[string]string{}
	in.Name = textLen(fields, "name", in.Name, 1, 120)
	in.Locale = strings.TrimSpace(in.Locale)
	if !localePattern.MatchString(in.Locale) {
		fields["locale"] = "Use a language tag like en, es-MX or pt-BR."
	}
	valid := false
	for _, t := range temperaments {
		valid = valid || t == in.Temperament
	}
	if !valid {
		fields["temperament"] = "Choose one of: " + strings.Join(temperaments, ", ") + "."
	}
	if len([]rune(in.Notes)) > 2000 {
		fields["notes"] = "Use at most 2000 characters."
	}
	if in.Voice != nil {
		in.Voice.Voice = textLen(fields, "voice.voice", in.Voice.Voice, 1, 100)
		if err := s.checkConnection(ctx, ws, &in.Voice.ConnectionID, "voice.connectionId", fields, connection.CapTTS, connection.CapRealtime); err != nil {
			return nil, err
		}
	}
	return fields, nil
}

// CreatePersona stores a persona. Member or above.
func (s *Service) CreatePersona(ctx context.Context, ws workspace.Workspace, in PersonaInput) (Persona, error) {
	if err := canAuthor(ws); err != nil {
		return Persona{}, err
	}
	fields, err := s.validatePersona(ctx, ws, &in)
	if err != nil {
		return Persona{}, err
	}
	if err := apperr.Invalid(fields); err != nil {
		return Persona{}, err
	}
	var id string
	connID, voice := voiceColumns(in.Voice)
	err = s.pool.QueryRow(ctx,
		`INSERT INTO personas (workspace_id, name, locale, temperament, notes, voice_connection_id, voice_name)
		 VALUES ($1, $2, $3, $4, $5, $6, $7) RETURNING id`,
		ws.ID, in.Name, in.Locale, in.Temperament, in.Notes, connID, voice).Scan(&id)
	if database.IsUniqueViolation(err, "personas_workspace_name_idx") {
		return Persona{}, errPersonaTaken
	}
	if err != nil {
		return Persona{}, fmt.Errorf("insert persona: %w", err)
	}
	return s.GetPersona(ctx, ws, id)
}

// UpdatePersona replaces a persona. Member or above.
func (s *Service) UpdatePersona(ctx context.Context, ws workspace.Workspace, id string, in PersonaInput) (Persona, error) {
	if err := canAuthor(ws); err != nil {
		return Persona{}, err
	}
	if _, err := s.GetPersona(ctx, ws, id); err != nil {
		return Persona{}, err
	}
	fields, err := s.validatePersona(ctx, ws, &in)
	if err != nil {
		return Persona{}, err
	}
	if err := apperr.Invalid(fields); err != nil {
		return Persona{}, err
	}
	connID, voice := voiceColumns(in.Voice)
	_, err = s.pool.Exec(ctx,
		`UPDATE personas SET name = $3, locale = $4, temperament = $5, notes = $6, voice_connection_id = $7, voice_name = $8, updated_at = now()
		 WHERE workspace_id = $1 AND id = $2`, ws.ID, id, in.Name, in.Locale, in.Temperament, in.Notes, connID, voice)
	if database.IsUniqueViolation(err, "personas_workspace_name_idx") {
		return Persona{}, errPersonaTaken
	}
	if err != nil {
		return Persona{}, fmt.Errorf("update persona: %w", err)
	}
	return s.GetPersona(ctx, ws, id)
}

// DeletePersona removes a persona that no scenario uses. Member or above.
func (s *Service) DeletePersona(ctx context.Context, ws workspace.Workspace, id string) error {
	if err := canAuthor(ws); err != nil {
		return err
	}
	p, err := s.GetPersona(ctx, ws, id)
	if err != nil {
		return err
	}
	if p.ScenarioCount > 0 {
		return inUse("persona", p.ScenarioCount)
	}
	_, err = s.pool.Exec(ctx, `DELETE FROM personas WHERE workspace_id = $1 AND id = $2`, ws.ID, id)
	if database.IsForeignKeyViolation(err) {
		return inUse("persona", 1)
	}
	return err
}

func voiceColumns(v *Voice) (*string, *string) {
	if v == nil {
		return nil, nil
	}
	return &v.ConnectionID, &v.Voice
}
