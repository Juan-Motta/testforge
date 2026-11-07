package server

import (
	"net/http"

	"testforge/internal/agent"
	"testforge/internal/agent/codec"
	"testforge/internal/authn"
	"testforge/internal/platform/httpx"
	"testforge/internal/workspace"
)

func (s *server) listAgentCodecs(w http.ResponseWriter, _ *http.Request, _ authn.Principal) {
	httpx.JSON(w, http.StatusOK, listResponse[codec.Spec]{Items: codec.Catalog})
}

func (s *server) listAgentPresets(w http.ResponseWriter, _ *http.Request, _ authn.Principal) {
	httpx.JSON(w, http.StatusOK, listResponse[agent.Preset]{Items: agent.Presets})
}

func (s *server) listAgents(w http.ResponseWriter, r *http.Request, _ authn.Principal, ws workspace.Workspace) {
	items, err := s.agents.List(r.Context(), ws)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, listResponse[agent.Agent]{Items: items})
}

func (s *server) getAgent(w http.ResponseWriter, r *http.Request, _ authn.Principal, ws workspace.Workspace) {
	a, err := s.agents.Get(r.Context(), ws, r.PathValue("id"))
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, a)
}

func (s *server) createAgent(w http.ResponseWriter, r *http.Request, p authn.Principal, ws workspace.Workspace) {
	var in agent.Input
	if err := httpx.Decode(r, &in); err != nil {
		httpx.Error(w, r, err)
		return
	}
	a, err := s.agents.Create(r.Context(), p, ws, in)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, a)
}

func (s *server) updateAgent(w http.ResponseWriter, r *http.Request, _ authn.Principal, ws workspace.Workspace) {
	var in agent.Input
	if err := httpx.Decode(r, &in); err != nil {
		httpx.Error(w, r, err)
		return
	}
	a, err := s.agents.Update(r.Context(), ws, r.PathValue("id"), in)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, a)
}

func (s *server) deleteAgent(w http.ResponseWriter, r *http.Request, _ authn.Principal, ws workspace.Workspace) {
	if err := s.agents.Delete(r.Context(), ws, r.PathValue("id")); err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.NoContent(w)
}

func (s *server) probeAgent(w http.ResponseWriter, r *http.Request, _ authn.Principal, ws workspace.Workspace) {
	var req agent.ProbeRequest
	if r.ContentLength != 0 {
		if err := httpx.Decode(r, &req); err != nil {
			httpx.Error(w, r, err)
			return
		}
	}
	res, err := s.agents.Probe(r.Context(), ws, r.PathValue("id"), req)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, res)
}
