package server

import (
	"net/http"

	"testforge/internal/authn"
	"testforge/internal/platform/httpx"
	"testforge/internal/sandbox"
	"testforge/internal/workspace"
)

func (s *server) listSandboxAgents(w http.ResponseWriter, r *http.Request, _ authn.Principal, ws workspace.Workspace) {
	items, err := s.sandbox.List(r.Context(), ws)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, listResponse[sandbox.Agent]{Items: items})
}

func (s *server) getSandboxAgent(w http.ResponseWriter, r *http.Request, _ authn.Principal, ws workspace.Workspace) {
	a, err := s.sandbox.Get(r.Context(), ws, r.PathValue("id"))
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, a)
}

func (s *server) createSandboxAgent(w http.ResponseWriter, r *http.Request, p authn.Principal, ws workspace.Workspace) {
	var in sandbox.Input
	if err := httpx.Decode(r, &in); err != nil {
		httpx.Error(w, r, err)
		return
	}
	a, err := s.sandbox.Create(r.Context(), p, ws, in)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, a)
}

func (s *server) updateSandboxAgent(w http.ResponseWriter, r *http.Request, _ authn.Principal, ws workspace.Workspace) {
	var in sandbox.Input
	if err := httpx.Decode(r, &in); err != nil {
		httpx.Error(w, r, err)
		return
	}
	a, err := s.sandbox.Update(r.Context(), ws, r.PathValue("id"), in)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, a)
}

func (s *server) deleteSandboxAgent(w http.ResponseWriter, r *http.Request, _ authn.Principal, ws workspace.Workspace) {
	if err := s.sandbox.Delete(r.Context(), ws, r.PathValue("id")); err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.NoContent(w)
}

func (s *server) chatSandboxAgent(w http.ResponseWriter, r *http.Request, p authn.Principal, ws workspace.Workspace) {
	var in sandbox.ChatRequest
	if err := httpx.Decode(r, &in); err != nil {
		httpx.Error(w, r, err)
		return
	}
	reply, err := s.sandbox.Chat(r.Context(), p, ws, r.PathValue("id"), in)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, reply)
}

func (s *server) listSandboxMemories(w http.ResponseWriter, r *http.Request, _ authn.Principal, ws workspace.Workspace) {
	items, err := s.sandbox.Memories(r.Context(), ws, r.PathValue("id"), r.URL.Query().Get("userId"))
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, listResponse[sandbox.StoredMemory]{Items: items})
}

func (s *server) deleteSandboxMemory(w http.ResponseWriter, r *http.Request, _ authn.Principal, ws workspace.Workspace) {
	if err := s.sandbox.DeleteMemory(r.Context(), ws, r.PathValue("id"), r.PathValue("memoryId")); err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.NoContent(w)
}

func (s *server) linkSandboxAgent(w http.ResponseWriter, r *http.Request, p authn.Principal, ws workspace.Workspace) {
	a, err := s.sandbox.Link(r.Context(), p, ws, r.PathValue("id"))
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, a)
}

func (s *server) generateSandboxDraft(w http.ResponseWriter, r *http.Request, _ authn.Principal, ws workspace.Workspace) {
	var in sandbox.GenerateRequest
	if err := httpx.Decode(r, &in); err != nil {
		httpx.Error(w, r, err)
		return
	}
	d, err := s.sandbox.Generate(r.Context(), ws, in)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, d)
}
