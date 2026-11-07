package server

import (
	"net/http"

	"testforge/internal/authn"
	"testforge/internal/connection"
	"testforge/internal/platform/httpx"
	"testforge/internal/workspace"
)

func (s *server) listConnectionKinds(w http.ResponseWriter, _ *http.Request, _ authn.Principal) {
	httpx.JSON(w, http.StatusOK, listResponse[connection.KindSpec]{Items: connection.Catalog})
}

func (s *server) listConnections(w http.ResponseWriter, r *http.Request, _ authn.Principal, ws workspace.Workspace) {
	items, err := s.conns.List(r.Context(), ws)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, listResponse[connection.Connection]{Items: items})
}

func (s *server) getConnection(w http.ResponseWriter, r *http.Request, _ authn.Principal, ws workspace.Workspace) {
	c, err := s.conns.Get(r.Context(), ws, r.PathValue("id"))
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, c)
}

func (s *server) createConnection(w http.ResponseWriter, r *http.Request, p authn.Principal, ws workspace.Workspace) {
	var in connection.CreateInput
	if err := httpx.Decode(r, &in); err != nil {
		httpx.Error(w, r, err)
		return
	}
	c, err := s.conns.Create(r.Context(), p, ws, in)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, c)
}

func (s *server) updateConnection(w http.ResponseWriter, r *http.Request, _ authn.Principal, ws workspace.Workspace) {
	var in connection.UpdateInput
	if err := httpx.Decode(r, &in); err != nil {
		httpx.Error(w, r, err)
		return
	}
	c, err := s.conns.Update(r.Context(), ws, r.PathValue("id"), in)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, c)
}

func (s *server) deleteConnection(w http.ResponseWriter, r *http.Request, _ authn.Principal, ws workspace.Workspace) {
	if err := s.conns.Delete(r.Context(), ws, r.PathValue("id")); err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.NoContent(w)
}

func (s *server) testConnection(w http.ResponseWriter, r *http.Request, _ authn.Principal, ws workspace.Workspace) {
	res, err := s.conns.Test(r.Context(), ws, r.PathValue("id"))
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, res)
}
