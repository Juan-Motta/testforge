// Package server wires features into the HTTP API described by api/openapi.yaml.
package server

import (
	"context"
	"net"
	"net/http"
	"slices"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"testforge/internal/apikey"
	"testforge/internal/authn"
	"testforge/internal/connection"
	"testforge/internal/identity"
	"testforge/internal/platform/apperr"
	"testforge/internal/platform/config"
	"testforge/internal/platform/httpx"
	"testforge/internal/platform/sealbox"
	"testforge/internal/workspace"
)

const sessionCookie = "tf_session"

type server struct {
	cfg        config.Config
	pool       *pgxpool.Pool
	identity   *identity.Service
	workspaces *workspace.Service
	apiKeys    *apikey.Service
	conns      *connection.Service
}

// New builds the API handler. It fails when the secret key is invalid.
func New(pool *pgxpool.Pool, cfg config.Config) (http.Handler, error) {
	box, err := sealbox.New(cfg.SecretKey)
	if err != nil {
		return nil, err
	}
	s := &server{
		cfg:        cfg,
		pool:       pool,
		workspaces: workspace.NewService(pool),
		apiKeys:    apikey.NewService(pool),
		conns:      connection.NewService(pool, box, cfg.AllowPrivateNetwork),
	}
	s.identity = identity.NewService(pool, cfg.SessionTTL, func(ctx context.Context, tx pgx.Tx, u identity.User) error {
		return workspace.CreatePersonal(ctx, tx, u.ID, u.Name)
	})

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", s.health)

	mux.HandleFunc("POST /api/v1/auth/signup", s.signup)
	mux.HandleFunc("POST /api/v1/auth/login", s.login)
	mux.HandleFunc("POST /api/v1/auth/logout", s.logout)
	mux.Handle("GET /api/v1/auth/me", s.authed(s.me))

	mux.Handle("GET /api/v1/workspaces", s.authed(s.listWorkspaces))
	mux.Handle("POST /api/v1/workspaces", s.authed(s.createWorkspace))
	mux.Handle("GET /api/v1/workspaces/{slug}", s.inWorkspace(s.getWorkspace))
	mux.Handle("GET /api/v1/workspaces/{slug}/api-keys", s.inWorkspace(s.listAPIKeys))
	mux.Handle("POST /api/v1/workspaces/{slug}/api-keys", s.inWorkspace(s.createAPIKey))
	mux.Handle("DELETE /api/v1/workspaces/{slug}/api-keys/{keyId}", s.inWorkspace(s.revokeAPIKey))

	mux.Handle("GET /api/v1/connection-kinds", s.authed(s.listConnectionKinds))
	mux.Handle("GET /api/v1/workspaces/{slug}/connections", s.inWorkspace(s.listConnections))
	mux.Handle("POST /api/v1/workspaces/{slug}/connections", s.inWorkspace(s.createConnection))
	mux.Handle("GET /api/v1/workspaces/{slug}/connections/{id}", s.inWorkspace(s.getConnection))
	mux.Handle("PATCH /api/v1/workspaces/{slug}/connections/{id}", s.inWorkspace(s.updateConnection))
	mux.Handle("DELETE /api/v1/workspaces/{slug}/connections/{id}", s.inWorkspace(s.deleteConnection))
	mux.Handle("POST /api/v1/workspaces/{slug}/connections/{id}/test", s.inWorkspace(s.testConnection))

	mux.HandleFunc("/api/", func(w http.ResponseWriter, r *http.Request) {
		httpx.Error(w, r, apperr.NotFound("Route not found."))
	})

	return httpx.Recover(httpx.Logging(s.authenticate(s.checkOrigin(mux)))), nil
}

// authenticate resolves the principal from a bearer API key or the session
// cookie. A present but invalid bearer is rejected outright; an invalid
// cookie is treated as anonymous.
func (s *server) authenticate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		if header := r.Header.Get("Authorization"); header != "" {
			// The auth scheme name is case-insensitive (RFC 9110 §11.1).
			scheme, secret, ok := strings.Cut(header, " ")
			if !ok || !strings.EqualFold(scheme, "Bearer") {
				httpx.Error(w, r, apperr.Unauthorized("Use an Authorization: Bearer <api key> header."))
				return
			}
			p, valid, err := s.apiKeys.Authenticate(ctx, strings.TrimSpace(secret))
			if err != nil {
				httpx.Error(w, r, err)
				return
			}
			if !valid {
				httpx.Error(w, r, apperr.Unauthorized("API key is invalid, expired or revoked."))
				return
			}
			next.ServeHTTP(w, r.WithContext(authn.With(ctx, p)))
			return
		}
		if c, err := r.Cookie(sessionCookie); err == nil {
			userID, valid, err := s.identity.AuthenticateSession(ctx, c.Value)
			if err != nil {
				httpx.Error(w, r, err)
				return
			}
			if valid {
				ctx = authn.With(ctx, authn.Principal{UserID: userID, Method: authn.MethodSession})
			}
		}
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// checkOrigin blocks cross-site requests: unsafe methods without a bearer
// token (i.e. browser requests that may carry the session cookie) must come
// from a configured app origin.
func (s *server) checkOrigin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet, http.MethodHead, http.MethodOptions:
		default:
			if r.Header.Get("Authorization") == "" && !slices.Contains(s.cfg.AppOrigins, r.Header.Get("Origin")) {
				httpx.Error(w, r, apperr.Forbidden("Request origin is not allowed."))
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

type authedHandler func(w http.ResponseWriter, r *http.Request, p authn.Principal)

func (s *server) authed(h authedHandler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p, ok := authn.From(r.Context())
		if !ok {
			httpx.Error(w, r, apperr.Unauthorized("Sign in to continue."))
			return
		}
		h(w, r, p)
	})
}

type workspaceHandler func(w http.ResponseWriter, r *http.Request, p authn.Principal, ws workspace.Workspace)

// inWorkspace authenticates and resolves the {slug} workspace for the principal.
func (s *server) inWorkspace(h workspaceHandler) http.Handler {
	return s.authed(func(w http.ResponseWriter, r *http.Request, p authn.Principal) {
		ws, err := s.workspaces.Resolve(r.Context(), p, r.PathValue("slug"))
		if err != nil {
			httpx.Error(w, r, err)
			return
		}
		h(w, r, p, ws)
	})
}

func clientMeta(r *http.Request) identity.ClientMeta {
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		ip = r.RemoteAddr
	}
	return identity.ClientMeta{IP: ip, UserAgent: r.UserAgent()}
}
