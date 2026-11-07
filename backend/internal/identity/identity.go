// Package identity manages users, passwords, sessions and log-in throttling.
package identity

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"net/mail"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"testforge/internal/platform/apperr"
	"testforge/internal/platform/database"
)

const (
	maxFailedLogins = 10
	throttleWindow  = 15 * time.Minute
)

var errInvalidCredentials = apperr.Unauthorized("Email or password is incorrect.")

// User is an account holder.
type User struct {
	ID        string    `json:"id"`
	Email     string    `json:"email"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"createdAt"`
}

// SignupInput is the sign-up request.
type SignupInput struct {
	Name     string `json:"name"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

// ClientMeta describes the client that opens a session.
type ClientMeta struct {
	IP        string
	UserAgent string
}

// UserCreatedHook runs inside the sign-up transaction, e.g. to provision a
// personal workspace.
type UserCreatedHook func(ctx context.Context, tx pgx.Tx, user User) error

// Service implements the identity use cases.
type Service struct {
	pool       *pgxpool.Pool
	sessionTTL time.Duration
	onSignup   UserCreatedHook
	dummyHash  string
}

// NewService builds the service. onSignup may be nil.
func NewService(pool *pgxpool.Pool, sessionTTL time.Duration, onSignup UserCreatedHook) *Service {
	dummy, err := HashPassword("timing-equalizer")
	if err != nil {
		panic(fmt.Sprintf("identity: hash dummy password: %v", err))
	}
	return &Service{pool: pool, sessionTTL: sessionTTL, onSignup: onSignup, dummyHash: dummy}
}

// SessionTTL is how long a new session lasts.
func (s *Service) SessionTTL() time.Duration { return s.sessionTTL }

// Signup creates the user, runs the sign-up hook and opens a session.
func (s *Service) Signup(ctx context.Context, in SignupInput, meta ClientMeta) (User, string, error) {
	name := strings.TrimSpace(in.Name)
	email := normalizeEmail(in.Email)
	fields := map[string]string{}
	if name == "" || len([]rune(name)) > 100 {
		fields["name"] = "Enter a name of 1 to 100 characters."
	}
	if !validEmail(email) {
		fields["email"] = "Enter a valid email address."
	}
	if n := len([]rune(in.Password)); n < 8 || n > 256 {
		fields["password"] = "Use at least 8 characters."
	}
	if err := apperr.Invalid(fields); err != nil {
		return User{}, "", err
	}

	hash, err := HashPassword(in.Password)
	if err != nil {
		return User{}, "", err
	}

	var user User
	var token string
	err = database.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		err := tx.QueryRow(ctx,
			`INSERT INTO users (email, name, password_hash) VALUES ($1, $2, $3)
			 RETURNING id, email, name, created_at`, email, name, hash,
		).Scan(&user.ID, &user.Email, &user.Name, &user.CreatedAt)
		if database.IsUniqueViolation(err, "users_email_key") {
			return apperr.Conflict("An account with this email already exists.")
		}
		if err != nil {
			return fmt.Errorf("insert user: %w", err)
		}
		if s.onSignup != nil {
			if err := s.onSignup(ctx, tx, user); err != nil {
				return err
			}
		}
		token, err = s.createSession(ctx, tx, user.ID, meta)
		return err
	})
	if err != nil {
		return User{}, "", err
	}
	return user, token, nil
}

// Login verifies credentials and opens a session. Unknown emails and wrong
// passwords are indistinguishable, including in timing.
func (s *Service) Login(ctx context.Context, email, password string, meta ClientMeta) (User, string, error) {
	email = normalizeEmail(email)
	if email == "" || password == "" {
		return User{}, "", errInvalidCredentials
	}

	var failures int
	err := s.pool.QueryRow(ctx,
		`SELECT count(*) FROM login_attempts
		 WHERE email = $1 AND NOT succeeded AND created_at > now() - make_interval(secs => $2)
		   AND created_at > coalesce((SELECT max(created_at) FROM login_attempts WHERE email = $1 AND succeeded), '-infinity')`,
		email, throttleWindow.Seconds(),
	).Scan(&failures)
	if err != nil {
		return User{}, "", fmt.Errorf("count login failures: %w", err)
	}
	if failures >= maxFailedLogins {
		return User{}, "", apperr.RateLimited("Too many failed attempts. Try again in a few minutes.")
	}

	var user User
	var hash string
	err = s.pool.QueryRow(ctx,
		`SELECT id, email, name, created_at, password_hash FROM users WHERE email = $1`, email,
	).Scan(&user.ID, &user.Email, &user.Name, &user.CreatedAt, &hash)
	found := err == nil
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return User{}, "", fmt.Errorf("find user: %w", err)
	}
	if !found {
		hash = s.dummyHash
	}
	ok := VerifyPassword(hash, password) && found

	if _, err := s.pool.Exec(ctx,
		`INSERT INTO login_attempts (email, ip, succeeded) VALUES ($1, $2, $3)`, email, meta.IP, ok,
	); err != nil {
		return User{}, "", fmt.Errorf("record login attempt: %w", err)
	}
	if !ok {
		return User{}, "", errInvalidCredentials
	}

	token, err := s.createSession(ctx, s.pool, user.ID, meta)
	if err != nil {
		return User{}, "", err
	}
	return user, token, nil
}

// Logout deletes the session behind token. Unknown tokens are ignored.
func (s *Service) Logout(ctx context.Context, token string) error {
	if token == "" {
		return nil
	}
	_, err := s.pool.Exec(ctx, `DELETE FROM sessions WHERE token_hash = $1`, hashToken(token))
	return err
}

// AuthenticateSession returns the user ID for a live session token.
func (s *Service) AuthenticateSession(ctx context.Context, token string) (string, bool, error) {
	if token == "" {
		return "", false, nil
	}
	var userID string
	err := s.pool.QueryRow(ctx,
		`SELECT user_id FROM sessions WHERE token_hash = $1 AND expires_at > now()`, hashToken(token),
	).Scan(&userID)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("find session: %w", err)
	}
	return userID, true, nil
}

// UserByID loads a user.
func (s *Service) UserByID(ctx context.Context, id string) (User, error) {
	var u User
	err := s.pool.QueryRow(ctx, `SELECT id, email, name, created_at FROM users WHERE id = $1`, id).
		Scan(&u.ID, &u.Email, &u.Name, &u.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return User{}, apperr.Unauthorized("Your account no longer exists.")
	}
	return u, err
}

func (s *Service) createSession(ctx context.Context, q database.DBTX, userID string, meta ClientMeta) (string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("generate session token: %w", err)
	}
	token := base64.RawURLEncoding.EncodeToString(raw)
	_, err := q.Exec(ctx,
		`INSERT INTO sessions (user_id, token_hash, user_agent, ip, expires_at)
		 VALUES ($1, $2, $3, $4, now() + make_interval(secs => $5))`,
		userID, hashToken(token), truncate(meta.UserAgent, 512), meta.IP, s.sessionTTL.Seconds())
	if err != nil {
		return "", fmt.Errorf("insert session: %w", err)
	}
	return token, nil
}

func hashToken(token string) []byte {
	sum := sha256.Sum256([]byte(token))
	return sum[:]
}

func normalizeEmail(email string) string { return strings.ToLower(strings.TrimSpace(email)) }

func validEmail(email string) bool {
	if len(email) > 254 {
		return false
	}
	addr, err := mail.ParseAddress(email)
	return err == nil && addr.Address == email && strings.Contains(email[strings.LastIndex(email, "@"):], ".")
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}
