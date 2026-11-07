package server

import (
	"context"
	"net/http"

	"testforge/internal/apikey"
	"testforge/internal/authn"
	"testforge/internal/identity"
	"testforge/internal/platform/httpx"
	"testforge/internal/workspace"
)

type meResponse struct {
	User       identity.User         `json:"user"`
	Workspaces []workspace.Workspace `json:"workspaces"`
	AuthMethod authn.Method          `json:"authMethod"`
}

type listResponse[T any] struct {
	Items []T `json:"items"`
}

func (s *server) health(w http.ResponseWriter, r *http.Request) {
	if err := s.pool.Ping(r.Context()); err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *server) signup(w http.ResponseWriter, r *http.Request) {
	var in identity.SignupInput
	if err := httpx.Decode(r, &in); err != nil {
		httpx.Error(w, r, err)
		return
	}
	user, token, err := s.identity.Signup(r.Context(), in, clientMeta(r))
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	s.startSession(w, r, http.StatusCreated, user, token)
}

func (s *server) login(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := httpx.Decode(r, &in); err != nil {
		httpx.Error(w, r, err)
		return
	}
	user, token, err := s.identity.Login(r.Context(), in.Email, in.Password, clientMeta(r))
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	s.startSession(w, r, http.StatusOK, user, token)
}

func (s *server) startSession(w http.ResponseWriter, r *http.Request, status int, user identity.User, token string) {
	p := authn.Principal{UserID: user.ID, Method: authn.MethodSession}
	me, err := s.buildMe(r.Context(), p, user)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookie,
		Value:    token,
		Path:     "/",
		MaxAge:   int(s.identity.SessionTTL().Seconds()),
		HttpOnly: true,
		Secure:   s.cfg.CookieSecure,
		SameSite: http.SameSiteLaxMode,
	})
	httpx.JSON(w, status, me)
}

func (s *server) logout(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie(sessionCookie); err == nil {
		if err := s.identity.Logout(r.Context(), c.Value); err != nil {
			httpx.Error(w, r, err)
			return
		}
	}
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookie,
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   s.cfg.CookieSecure,
		SameSite: http.SameSiteLaxMode,
	})
	httpx.NoContent(w)
}

func (s *server) me(w http.ResponseWriter, r *http.Request, p authn.Principal) {
	user, err := s.identity.UserByID(r.Context(), p.UserID)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	me, err := s.buildMe(r.Context(), p, user)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, me)
}

func (s *server) buildMe(ctx context.Context, p authn.Principal, user identity.User) (meResponse, error) {
	ws, err := s.workspaces.List(ctx, p)
	if err != nil {
		return meResponse{}, err
	}
	return meResponse{User: user, Workspaces: ws, AuthMethod: p.Method}, nil
}

func (s *server) listWorkspaces(w http.ResponseWriter, r *http.Request, p authn.Principal) {
	ws, err := s.workspaces.List(r.Context(), p)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, listResponse[workspace.Workspace]{Items: ws})
}

func (s *server) createWorkspace(w http.ResponseWriter, r *http.Request, p authn.Principal) {
	var in struct {
		Name string `json:"name"`
	}
	if err := httpx.Decode(r, &in); err != nil {
		httpx.Error(w, r, err)
		return
	}
	ws, err := s.workspaces.Create(r.Context(), p, in.Name)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, ws)
}

func (s *server) getWorkspace(w http.ResponseWriter, _ *http.Request, _ authn.Principal, ws workspace.Workspace) {
	httpx.JSON(w, http.StatusOK, ws)
}

func (s *server) listAPIKeys(w http.ResponseWriter, r *http.Request, p authn.Principal, ws workspace.Workspace) {
	keys, err := s.apiKeys.List(r.Context(), p, ws)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, listResponse[apikey.Key]{Items: keys})
}

func (s *server) createAPIKey(w http.ResponseWriter, r *http.Request, p authn.Principal, ws workspace.Workspace) {
	var in apikey.CreateInput
	if err := httpx.Decode(r, &in); err != nil {
		httpx.Error(w, r, err)
		return
	}
	key, err := s.apiKeys.Create(r.Context(), p, ws, in)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, key)
}

func (s *server) revokeAPIKey(w http.ResponseWriter, r *http.Request, p authn.Principal, ws workspace.Workspace) {
	if err := s.apiKeys.Revoke(r.Context(), p, ws, r.PathValue("keyId")); err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.NoContent(w)
}
