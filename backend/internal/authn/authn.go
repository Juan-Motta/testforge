// Package authn carries the authenticated principal through request contexts.
package authn

import "context"

// Method is how the caller authenticated.
type Method string

const (
	MethodSession Method = "session"
	MethodAPIKey  Method = "api_key"
)

// Principal is the authenticated caller.
type Principal struct {
	UserID string
	Method Method
	// WorkspaceID is set for API keys: the only workspace the key can reach.
	WorkspaceID string
	// APIKeyID is set for API keys.
	APIKeyID string
}

type ctxKey struct{}

// With stores p in ctx.
func With(ctx context.Context, p Principal) context.Context {
	return context.WithValue(ctx, ctxKey{}, p)
}

// From returns the principal in ctx, if any.
func From(ctx context.Context) (Principal, bool) {
	p, ok := ctx.Value(ctxKey{}).(Principal)
	return p, ok
}
