package server

import (
	"net/http"

	"testforge/internal/authn"
	"testforge/internal/platform/httpx"
	"testforge/internal/scenario"
	"testforge/internal/workspace"
)

// respond writes v with status, or the error.
func respond[T any](w http.ResponseWriter, r *http.Request, status int, v T, err error) {
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.JSON(w, status, v)
}

func decodeInto[T any](w http.ResponseWriter, r *http.Request) (T, bool) {
	var in T
	if err := httpx.Decode(r, &in); err != nil {
		httpx.Error(w, r, err)
		return in, false
	}
	return in, true
}

func (s *server) listPersonas(w http.ResponseWriter, r *http.Request, _ authn.Principal, ws workspace.Workspace) {
	items, err := s.scenarios.ListPersonas(r.Context(), ws)
	respond(w, r, http.StatusOK, listResponse[scenario.Persona]{Items: items}, err)
}

func (s *server) getPersona(w http.ResponseWriter, r *http.Request, _ authn.Principal, ws workspace.Workspace) {
	p, err := s.scenarios.GetPersona(r.Context(), ws, r.PathValue("id"))
	respond(w, r, http.StatusOK, p, err)
}

func (s *server) createPersona(w http.ResponseWriter, r *http.Request, _ authn.Principal, ws workspace.Workspace) {
	if in, ok := decodeInto[scenario.PersonaInput](w, r); ok {
		p, err := s.scenarios.CreatePersona(r.Context(), ws, in)
		respond(w, r, http.StatusCreated, p, err)
	}
}

func (s *server) updatePersona(w http.ResponseWriter, r *http.Request, _ authn.Principal, ws workspace.Workspace) {
	if in, ok := decodeInto[scenario.PersonaInput](w, r); ok {
		p, err := s.scenarios.UpdatePersona(r.Context(), ws, r.PathValue("id"), in)
		respond(w, r, http.StatusOK, p, err)
	}
}

func (s *server) deletePersona(w http.ResponseWriter, r *http.Request, _ authn.Principal, ws workspace.Workspace) {
	if err := s.scenarios.DeletePersona(r.Context(), ws, r.PathValue("id")); err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.NoContent(w)
}

func (s *server) listGroups(w http.ResponseWriter, r *http.Request, _ authn.Principal, ws workspace.Workspace) {
	items, err := s.scenarios.ListGroups(r.Context(), ws)
	respond(w, r, http.StatusOK, listResponse[scenario.Group]{Items: items}, err)
}

func (s *server) createGroup(w http.ResponseWriter, r *http.Request, _ authn.Principal, ws workspace.Workspace) {
	if in, ok := decodeInto[scenario.GroupInput](w, r); ok {
		g, err := s.scenarios.CreateGroup(r.Context(), ws, in)
		respond(w, r, http.StatusCreated, g, err)
	}
}

func (s *server) updateGroup(w http.ResponseWriter, r *http.Request, _ authn.Principal, ws workspace.Workspace) {
	if in, ok := decodeInto[scenario.GroupInput](w, r); ok {
		g, err := s.scenarios.UpdateGroup(r.Context(), ws, r.PathValue("id"), in)
		respond(w, r, http.StatusOK, g, err)
	}
}

func (s *server) deleteGroup(w http.ResponseWriter, r *http.Request, _ authn.Principal, ws workspace.Workspace) {
	if err := s.scenarios.DeleteGroup(r.Context(), ws, r.PathValue("id")); err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.NoContent(w)
}

func (s *server) listScenarios(w http.ResponseWriter, r *http.Request, _ authn.Principal, ws workspace.Workspace) {
	q := r.URL.Query()
	items, err := s.scenarios.ListScenarios(r.Context(), ws, scenario.Filter{
		AgentID: q.Get("agentId"), GroupID: q.Get("groupId"), Tag: q.Get("tag"), Query: q.Get("q"),
	})
	respond(w, r, http.StatusOK, listResponse[scenario.Scenario]{Items: items}, err)
}

func (s *server) getScenario(w http.ResponseWriter, r *http.Request, _ authn.Principal, ws workspace.Workspace) {
	sc, err := s.scenarios.GetScenario(r.Context(), ws, r.PathValue("id"))
	respond(w, r, http.StatusOK, sc, err)
}

func (s *server) createScenario(w http.ResponseWriter, r *http.Request, p authn.Principal, ws workspace.Workspace) {
	if in, ok := decodeInto[scenario.Input](w, r); ok {
		sc, err := s.scenarios.CreateScenario(r.Context(), p, ws, in)
		respond(w, r, http.StatusCreated, sc, err)
	}
}

func (s *server) updateScenario(w http.ResponseWriter, r *http.Request, _ authn.Principal, ws workspace.Workspace) {
	if in, ok := decodeInto[scenario.Input](w, r); ok {
		sc, err := s.scenarios.UpdateScenario(r.Context(), ws, r.PathValue("id"), in)
		respond(w, r, http.StatusOK, sc, err)
	}
}

func (s *server) deleteScenario(w http.ResponseWriter, r *http.Request, _ authn.Principal, ws workspace.Workspace) {
	if err := s.scenarios.DeleteScenario(r.Context(), ws, r.PathValue("id")); err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.NoContent(w)
}

func (s *server) duplicateScenario(w http.ResponseWriter, r *http.Request, p authn.Principal, ws workspace.Workspace) {
	sc, err := s.scenarios.DuplicateScenario(r.Context(), p, ws, r.PathValue("id"))
	respond(w, r, http.StatusCreated, sc, err)
}
