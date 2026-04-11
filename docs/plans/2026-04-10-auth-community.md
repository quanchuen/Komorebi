# Phase 5 — Auth + Community Features Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking. Complete each task fully — including its commit — before moving to the next.

**Goal:** Implement JWT-based authentication and the full Community bounded context: user registration/login, bcrypt password storage, JWT middleware, and the five community endpoints (contributions, reviews, ride logs), wired into the existing chi router with auth protection.

**Architecture:** Hexagonal / ports-and-adapters matching the existing codebase. A new `auth` package in `internal/app/` owns registration, login, and JWT issuance. A new middleware in `internal/api/` extracts the caller from the JWT and injects a `UserID` into the request context. Community endpoints live in `internal/api/community_handler.go`. Postgres repos in `internal/infra/postgres/` implement the existing domain interfaces in `internal/domain/community/`.

**Tech Stack:** Go 1.22+, chi v5, pgx v5, `golang-jwt/jwt/v5`, `golang.org/x/crypto/bcrypt`.

**Database:** `postgres://osm_dev:osm_dev@localhost:5432/cyclist_map_dev?sslmode=disable`

**Relevant existing code:**
- `internal/domain/community/` — `User`, `Contribution`, `Review`, `RideLog`, repository interfaces (all exist, no changes needed)
- `internal/infra/postgres/route_repo.go` — `genUUID()`, `withTx()`, `ErrNotFound`, WKT helpers (reuse)
- `internal/api/route_handler.go` — `writeJSON()`, `writeError()` (reuse)
- `internal/api/router.go` — `NewRouter(...)` (extend signature)
- `cmd/api/main.go` — wiring point (extend)

**Relevant schema:**
- `community.user` — id (uuid), display_name, email, avatar_url, created_at (migration 000005)
- `community.contribution` — id, user_id, route_geometry (LINESTRINGZ), metadata (jsonb), status, created_at, updated_at (migration 000006)
- `community.review` — id, user_id, route_id, rating (1–5), body, created_at (migration 000006)
- `community.ride_log` — id, user_id, route_id, gpx_track (LINESTRINGZ, nullable), started_at, finished_at, created_at (migration 000006)

**Schema gap (fixed in Task 1):** `community.user` has no `password_hash` column yet. The `community.contribution` table has no `moderator_notes` or `submitted_at` column matching the domain type; it uses `created_at` and `updated_at` instead. The `community.ride_log` table uses `started_at`/`finished_at` timestamps instead of `ridden_at` (unix int) + `duration_s` (int) in the domain. The repos will bridge these mismatches without touching domain types.

---

## Task 1 — Database Migration: Add password_hash to community.user

**Files:**
- `migrations/000017_community_user_password.up.sql` (create)
- `migrations/000017_community_user_password.down.sql` (create)

### Steps

- [ ] 1.1 Create the up migration.

```sql
-- migrations/000017_community_user_password.up.sql
ALTER TABLE community.user
    ADD COLUMN IF NOT EXISTS password_hash TEXT NOT NULL DEFAULT '';
```

- [ ] 1.2 Create the down migration.

```sql
-- migrations/000017_community_user_password.down.sql
ALTER TABLE community.user
    DROP COLUMN IF EXISTS password_hash;
```

- [ ] 1.3 Apply the migration against the dev database.

```bash
DATABASE_URL="postgres://osm_dev:osm_dev@localhost:5432/cyclist_map_dev?sslmode=disable"
psql "$DATABASE_URL" -f migrations/000017_community_user_password.up.sql
```

- [ ] 1.4 Verify the column exists.

```bash
psql "$DATABASE_URL" -c "\d community.user"
# Expect: password_hash | text | not null
```

- [ ] 1.5 Commit: `feat(migrations): add password_hash to community.user (#017)`

---

## Task 2 — Add jwt and bcrypt Dependencies

**Files:**
- `go.mod` (modified by go get)
- `go.sum` (modified by go get)

### Steps

- [ ] 2.1 Add `golang-jwt/jwt/v5` and `golang.org/x/crypto`.

```bash
go get github.com/golang-jwt/jwt/v5
go get golang.org/x/crypto
```

- [ ] 2.2 Verify both appear in `go.mod`.

```bash
grep -E 'golang-jwt|golang.org/x/crypto' go.mod
```

- [ ] 2.3 Commit: `chore(deps): add golang-jwt/jwt/v5 and x/crypto`

---

## Task 3 — Auth Service (register, login, refresh)

**Files:**
- `internal/app/auth_service.go` (create)
- `internal/app/auth_service_test.go` (create)

### Steps

- [ ] 3.1 Write the test file first (pure unit tests, no database).

```go
// internal/app/auth_service_test.go
package app_test

import (
	"errors"
	"testing"
	"time"

	"github.com/cyclist-map/cyclist-map/internal/app"
	"github.com/cyclist-map/cyclist-map/internal/domain/community"
)

// --- stub repo ---

type stubUserRepo struct {
	users  map[string]*community.User
	byEmail map[string]*community.User
	hashes map[string]string // userID → password_hash
}

func newStubUserRepo() *stubUserRepo {
	return &stubUserRepo{
		users:   make(map[string]*community.User),
		byEmail: make(map[string]*community.User),
		hashes:  make(map[string]string),
	}
}

func (s *stubUserRepo) Create(u *community.User) error {
	if _, exists := s.byEmail[u.Email]; exists {
		return app.ErrEmailTaken
	}
	s.users[u.ID] = u
	s.byEmail[u.Email] = u
	return nil
}

func (s *stubUserRepo) GetByID(id string) (*community.User, error) {
	u, ok := s.users[id]
	if !ok {
		return nil, app.ErrUserNotFound
	}
	return u, nil
}

func (s *stubUserRepo) GetByEmail(email string) (*community.User, error) {
	u, ok := s.byEmail[email]
	if !ok {
		return nil, app.ErrUserNotFound
	}
	return u, nil
}

func (s *stubUserRepo) SetPasswordHash(userID, hash string) error {
	s.hashes[userID] = hash
	return nil
}

func (s *stubUserRepo) GetPasswordHash(userID string) (string, error) {
	h, ok := s.hashes[userID]
	if !ok {
		return "", app.ErrUserNotFound
	}
	return h, nil
}

func (s *stubUserRepo) Update(u *community.User) error {
	s.users[u.ID] = u
	return nil
}

func (s *stubUserRepo) Delete(id string) error {
	u, ok := s.users[id]
	if !ok {
		return app.ErrUserNotFound
	}
	delete(s.byEmail, u.Email)
	delete(s.users, id)
	return nil
}

// --- tests ---

const testSecret = "test-secret-key-at-least-32-chars-long"

func newTestAuthService(t *testing.T) (*app.AuthService, *stubUserRepo) {
	t.Helper()
	repo := newStubUserRepo()
	svc, err := app.NewAuthService(repo, testSecret, 15*time.Minute, 7*24*time.Hour)
	if err != nil {
		t.Fatalf("NewAuthService: %v", err)
	}
	return svc, repo
}

func TestRegister_Success(t *testing.T) {
	svc, _ := newTestAuthService(t)
	u, err := svc.Register("Yuki Tanaka", "yuki@example.com", "secret123")
	if err != nil {
		t.Fatalf("Register: %v", err)
	}
	if u.ID == "" {
		t.Error("expected non-empty ID")
	}
	if u.DisplayName != "Yuki Tanaka" {
		t.Errorf("unexpected DisplayName: %q", u.DisplayName)
	}
}

func TestRegister_EmptyPassword(t *testing.T) {
	svc, _ := newTestAuthService(t)
	_, err := svc.Register("Yuki", "yuki@example.com", "")
	if !errors.Is(err, app.ErrWeakPassword) {
		t.Errorf("expected ErrWeakPassword, got %v", err)
	}
}

func TestRegister_DuplicateEmail(t *testing.T) {
	svc, _ := newTestAuthService(t)
	_, _ = svc.Register("Yuki", "yuki@example.com", "password1")
	_, err := svc.Register("Kenji", "yuki@example.com", "password2")
	if !errors.Is(err, app.ErrEmailTaken) {
		t.Errorf("expected ErrEmailTaken, got %v", err)
	}
}

func TestLogin_Success(t *testing.T) {
	svc, _ := newTestAuthService(t)
	_, _ = svc.Register("Yuki", "yuki@example.com", "correcthorse")
	tokens, err := svc.Login("yuki@example.com", "correcthorse")
	if err != nil {
		t.Fatalf("Login: %v", err)
	}
	if tokens.AccessToken == "" {
		t.Error("expected non-empty AccessToken")
	}
	if tokens.RefreshToken == "" {
		t.Error("expected non-empty RefreshToken")
	}
}

func TestLogin_WrongPassword(t *testing.T) {
	svc, _ := newTestAuthService(t)
	_, _ = svc.Register("Yuki", "yuki@example.com", "correcthorse")
	_, err := svc.Login("yuki@example.com", "wrongpassword")
	if !errors.Is(err, app.ErrBadCredentials) {
		t.Errorf("expected ErrBadCredentials, got %v", err)
	}
}

func TestLogin_UnknownEmail(t *testing.T) {
	svc, _ := newTestAuthService(t)
	_, err := svc.Login("nobody@example.com", "password")
	if !errors.Is(err, app.ErrBadCredentials) {
		t.Errorf("expected ErrBadCredentials, got %v", err)
	}
}

func TestRefresh_Success(t *testing.T) {
	svc, _ := newTestAuthService(t)
	_, _ = svc.Register("Yuki", "yuki@example.com", "password1")
	tokens, _ := svc.Login("yuki@example.com", "password1")
	newTokens, err := svc.Refresh(tokens.RefreshToken)
	if err != nil {
		t.Fatalf("Refresh: %v", err)
	}
	if newTokens.AccessToken == "" {
		t.Error("expected new AccessToken")
	}
}

func TestRefresh_InvalidToken(t *testing.T) {
	svc, _ := newTestAuthService(t)
	_, err := svc.Refresh("not.a.valid.token")
	if !errors.Is(err, app.ErrInvalidToken) {
		t.Errorf("expected ErrInvalidToken, got %v", err)
	}
}

func TestValidateAccessToken_Success(t *testing.T) {
	svc, _ := newTestAuthService(t)
	u, _ := svc.Register("Yuki", "yuki@example.com", "pass123")
	tokens, _ := svc.Login("yuki@example.com", "pass123")
	userID, err := svc.ValidateAccessToken(tokens.AccessToken)
	if err != nil {
		t.Fatalf("ValidateAccessToken: %v", err)
	}
	if userID != u.ID {
		t.Errorf("expected userID %q, got %q", u.ID, userID)
	}
}

func TestValidateAccessToken_Invalid(t *testing.T) {
	svc, _ := newTestAuthService(t)
	_, err := svc.ValidateAccessToken("garbage")
	if !errors.Is(err, app.ErrInvalidToken) {
		t.Errorf("expected ErrInvalidToken, got %v", err)
	}
}
```

- [ ] 3.2 Create `internal/app/auth_service.go`.

```go
// internal/app/auth_service.go
package app

import (
	"errors"
	"fmt"
	"time"

	"github.com/cyclist-map/cyclist-map/internal/domain/community"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

// Sentinel errors for auth operations.
var (
	ErrWeakPassword   = errors.New("auth: password must not be empty")
	ErrEmailTaken     = errors.New("auth: email already registered")
	ErrUserNotFound   = errors.New("auth: user not found")
	ErrBadCredentials = errors.New("auth: invalid email or password")
	ErrInvalidToken   = errors.New("auth: invalid or expired token")
)

// TokenPair holds an access token and a refresh token.
type TokenPair struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"` // seconds
}

// AuthUserRepo extends community.UserRepository with auth-specific operations.
type AuthUserRepo interface {
	community.UserRepository
	GetByEmail(email string) (*community.User, error)
	SetPasswordHash(userID, hash string) error
	GetPasswordHash(userID string) (string, error)
}

// AuthService handles registration, login, and JWT lifecycle.
type AuthService struct {
	repo         AuthUserRepo
	secret       []byte
	accessTTL    time.Duration
	refreshTTL   time.Duration
}

// NewAuthService creates an AuthService. Returns an error if secret is empty.
func NewAuthService(repo AuthUserRepo, secret string, accessTTL, refreshTTL time.Duration) (*AuthService, error) {
	if secret == "" {
		return nil, fmt.Errorf("auth: JWT secret must not be empty")
	}
	return &AuthService{
		repo:       repo,
		secret:     []byte(secret),
		accessTTL:  accessTTL,
		refreshTTL: refreshTTL,
	}, nil
}

// Register creates a new user, hashes the password, and persists both.
func (s *AuthService) Register(displayName, email, password string) (*community.User, error) {
	if password == "" {
		return nil, ErrWeakPassword
	}
	u, err := community.NewUser(displayName, email)
	if err != nil {
		return nil, err
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("auth: bcrypt: %w", err)
	}
	if err := s.repo.Create(u); err != nil {
		return nil, err // ErrEmailTaken propagates unchanged
	}
	if err := s.repo.SetPasswordHash(u.ID, string(hash)); err != nil {
		return nil, fmt.Errorf("auth: store password hash: %w", err)
	}
	return u, nil
}

// Login authenticates by email/password and returns a JWT token pair.
// Returns ErrBadCredentials for any auth failure (no oracle for callers).
func (s *AuthService) Login(email, password string) (TokenPair, error) {
	u, err := s.repo.GetByEmail(email)
	if err != nil {
		return TokenPair{}, ErrBadCredentials
	}
	hash, err := s.repo.GetPasswordHash(u.ID)
	if err != nil {
		return TokenPair{}, ErrBadCredentials
	}
	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)); err != nil {
		return TokenPair{}, ErrBadCredentials
	}
	return s.issuePair(u.ID)
}

// Refresh validates a refresh token and returns a new token pair.
func (s *AuthService) Refresh(refreshToken string) (TokenPair, error) {
	userID, err := s.parseToken(refreshToken, "refresh")
	if err != nil {
		return TokenPair{}, ErrInvalidToken
	}
	return s.issuePair(userID)
}

// ValidateAccessToken parses and validates an access JWT, returning the user ID.
func (s *AuthService) ValidateAccessToken(tokenStr string) (string, error) {
	userID, err := s.parseToken(tokenStr, "access")
	if err != nil {
		return "", ErrInvalidToken
	}
	return userID, nil
}

// --- internal helpers ---

type authClaims struct {
	UserID    string `json:"uid"`
	TokenType string `json:"typ"` // "access" or "refresh"
	jwt.RegisteredClaims
}

func (s *AuthService) issuePair(userID string) (TokenPair, error) {
	now := time.Now()

	accessClaims := authClaims{
		UserID:    userID,
		TokenType: "access",
		RegisteredClaims: jwt.RegisteredClaims{
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(s.accessTTL)),
		},
	}
	accessToken, err := jwt.NewWithClaims(jwt.SigningMethodHS256, accessClaims).SignedString(s.secret)
	if err != nil {
		return TokenPair{}, fmt.Errorf("auth: sign access token: %w", err)
	}

	refreshClaims := authClaims{
		UserID:    userID,
		TokenType: "refresh",
		RegisteredClaims: jwt.RegisteredClaims{
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(s.refreshTTL)),
		},
	}
	refreshToken, err := jwt.NewWithClaims(jwt.SigningMethodHS256, refreshClaims).SignedString(s.secret)
	if err != nil {
		return TokenPair{}, fmt.Errorf("auth: sign refresh token: %w", err)
	}

	return TokenPair{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresIn:    int(s.accessTTL.Seconds()),
	}, nil
}

func (s *AuthService) parseToken(tokenStr, expectedType string) (string, error) {
	token, err := jwt.ParseWithClaims(tokenStr, &authClaims{}, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return s.secret, nil
	})
	if err != nil || !token.Valid {
		return "", ErrInvalidToken
	}
	claims, ok := token.Claims.(*authClaims)
	if !ok || claims.TokenType != expectedType || claims.UserID == "" {
		return "", ErrInvalidToken
	}
	return claims.UserID, nil
}
```

- [ ] 3.3 Run unit tests (no database required).

```bash
go test ./internal/app/ -run TestRegister -run TestLogin -run TestRefresh -run TestValidate -v
```

All tests must pass before proceeding.

- [ ] 3.4 Commit: `feat(app): add AuthService with bcrypt + JWT`

---

## Task 4 — UserRepo Postgres Implementation

**Files:**
- `internal/infra/postgres/user_repo.go` (create)
- `internal/infra/postgres/user_repo_test.go` (create)

### Steps

- [ ] 4.1 Write the integration test first.

```go
// internal/infra/postgres/user_repo_test.go
package postgres_test

import (
	"testing"

	"github.com/cyclist-map/cyclist-map/internal/domain/community"
	"github.com/cyclist-map/cyclist-map/internal/infra/postgres"
)

func TestUserRepo_CreateAndGetByID(t *testing.T) {
	pool := testPool(t)
	repo := postgres.NewUserRepo(pool)

	u, err := community.NewUser("Yuki Tanaka", uniqueEmail(t))
	if err != nil {
		t.Fatalf("NewUser: %v", err)
	}

	if err := repo.Create(u); err != nil {
		t.Fatalf("Create: %v", err)
	}
	t.Cleanup(func() { _ = repo.Delete(u.ID) })

	got, err := repo.GetByID(u.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.DisplayName != u.DisplayName {
		t.Errorf("DisplayName: want %q, got %q", u.DisplayName, got.DisplayName)
	}
	if got.Email != u.Email {
		t.Errorf("Email: want %q, got %q", u.Email, got.Email)
	}
}

func TestUserRepo_GetByEmail(t *testing.T) {
	pool := testPool(t)
	repo := postgres.NewUserRepo(pool)

	email := uniqueEmail(t)
	u, _ := community.NewUser("Kenji Sato", email)
	_ = repo.Create(u)
	t.Cleanup(func() { _ = repo.Delete(u.ID) })

	got, err := repo.GetByEmail(email)
	if err != nil {
		t.Fatalf("GetByEmail: %v", err)
	}
	if got.ID != u.ID {
		t.Errorf("ID: want %q, got %q", u.ID, got.ID)
	}
}

func TestUserRepo_GetByEmail_NotFound(t *testing.T) {
	pool := testPool(t)
	repo := postgres.NewUserRepo(pool)

	_, err := repo.GetByEmail("nobody@nowhere.example")
	if err == nil {
		t.Fatal("expected error for unknown email")
	}
}

func TestUserRepo_DuplicateEmail(t *testing.T) {
	pool := testPool(t)
	repo := postgres.NewUserRepo(pool)

	email := uniqueEmail(t)
	u1, _ := community.NewUser("Alice", email)
	u2, _ := community.NewUser("Bob", email)
	_ = repo.Create(u1)
	t.Cleanup(func() { _ = repo.Delete(u1.ID) })

	err := repo.Create(u2)
	if err == nil {
		t.Fatal("expected duplicate email error")
	}
}

func TestUserRepo_PasswordHash(t *testing.T) {
	pool := testPool(t)
	repo := postgres.NewUserRepo(pool)

	u, _ := community.NewUser("Hana", uniqueEmail(t))
	_ = repo.Create(u)
	t.Cleanup(func() { _ = repo.Delete(u.ID) })

	const hash = "$2a$10$testhashvalue"
	if err := repo.SetPasswordHash(u.ID, hash); err != nil {
		t.Fatalf("SetPasswordHash: %v", err)
	}
	got, err := repo.GetPasswordHash(u.ID)
	if err != nil {
		t.Fatalf("GetPasswordHash: %v", err)
	}
	if got != hash {
		t.Errorf("hash: want %q, got %q", hash, got)
	}
}

func TestUserRepo_Update(t *testing.T) {
	pool := testPool(t)
	repo := postgres.NewUserRepo(pool)

	u, _ := community.NewUser("Original Name", uniqueEmail(t))
	_ = repo.Create(u)
	t.Cleanup(func() { _ = repo.Delete(u.ID) })

	u.DisplayName = "Updated Name"
	u.AvatarURL = "https://example.com/avatar.png"
	if err := repo.Update(u); err != nil {
		t.Fatalf("Update: %v", err)
	}
	got, _ := repo.GetByID(u.ID)
	if got.DisplayName != "Updated Name" {
		t.Errorf("DisplayName after update: %q", got.DisplayName)
	}
	if got.AvatarURL != "https://example.com/avatar.png" {
		t.Errorf("AvatarURL after update: %q", got.AvatarURL)
	}
}

// uniqueEmail generates a test email that won't collide between test runs.
func uniqueEmail(t *testing.T) string {
	t.Helper()
	return "test-" + genUUID() + "@example.com"
}
```

- [ ] 4.2 Create `internal/infra/postgres/user_repo.go`.

```go
// internal/infra/postgres/user_repo.go
package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/cyclist-map/cyclist-map/internal/app"
	"github.com/cyclist-map/cyclist-map/internal/domain/community"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// UserRepo implements app.AuthUserRepo using PostgreSQL.
type UserRepo struct {
	pool *pgxpool.Pool
}

// NewUserRepo creates a new UserRepo.
func NewUserRepo(pool *pgxpool.Pool) *UserRepo {
	return &UserRepo{pool: pool}
}

// Create inserts a new user. Returns app.ErrEmailTaken on UNIQUE violation.
func (r *UserRepo) Create(u *community.User) error {
	ctx := context.Background()
	_, err := r.pool.Exec(ctx, `
		INSERT INTO community.user (id, display_name, email, avatar_url, created_at)
		VALUES ($1::uuid, $2, $3, $4, $5)
	`, u.ID, u.DisplayName, u.Email, nullableStr(u.AvatarURL), u.CreatedAt)
	if err != nil {
		if isUniqueViolation(err) {
			return app.ErrEmailTaken
		}
		return fmt.Errorf("UserRepo.Create: %w", err)
	}
	return nil
}

// GetByID fetches a user by UUID. Returns app.ErrUserNotFound when absent.
func (r *UserRepo) GetByID(id string) (*community.User, error) {
	ctx := context.Background()
	row := r.pool.QueryRow(ctx, `
		SELECT id, display_name, email, COALESCE(avatar_url, ''), created_at
		FROM community.user WHERE id = $1::uuid
	`, id)
	u, err := scanUser(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, app.ErrUserNotFound
		}
		return nil, fmt.Errorf("UserRepo.GetByID: %w", err)
	}
	return u, nil
}

// GetByEmail fetches a user by email address. Returns app.ErrUserNotFound when absent.
func (r *UserRepo) GetByEmail(email string) (*community.User, error) {
	ctx := context.Background()
	row := r.pool.QueryRow(ctx, `
		SELECT id, display_name, email, COALESCE(avatar_url, ''), created_at
		FROM community.user WHERE email = $1
	`, email)
	u, err := scanUser(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, app.ErrUserNotFound
		}
		return nil, fmt.Errorf("UserRepo.GetByEmail: %w", err)
	}
	return u, nil
}

// SetPasswordHash writes (or overwrites) the bcrypt hash for the given user.
func (r *UserRepo) SetPasswordHash(userID, hash string) error {
	ctx := context.Background()
	_, err := r.pool.Exec(ctx, `
		UPDATE community.user SET password_hash = $2 WHERE id = $1::uuid
	`, userID, hash)
	if err != nil {
		return fmt.Errorf("UserRepo.SetPasswordHash: %w", err)
	}
	return nil
}

// GetPasswordHash retrieves the bcrypt hash for the given user.
func (r *UserRepo) GetPasswordHash(userID string) (string, error) {
	ctx := context.Background()
	var hash string
	err := r.pool.QueryRow(ctx, `
		SELECT password_hash FROM community.user WHERE id = $1::uuid
	`, userID).Scan(&hash)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", app.ErrUserNotFound
		}
		return "", fmt.Errorf("UserRepo.GetPasswordHash: %w", err)
	}
	return hash, nil
}

// Update replaces display_name and avatar_url for an existing user.
func (r *UserRepo) Update(u *community.User) error {
	ctx := context.Background()
	_, err := r.pool.Exec(ctx, `
		UPDATE community.user SET display_name = $2, avatar_url = $3
		WHERE id = $1::uuid
	`, u.ID, u.DisplayName, nullableStr(u.AvatarURL))
	if err != nil {
		return fmt.Errorf("UserRepo.Update: %w", err)
	}
	return nil
}

// Delete removes a user by ID (cascades to reviews, ride_logs, contributions).
func (r *UserRepo) Delete(id string) error {
	ctx := context.Background()
	_, err := r.pool.Exec(ctx, `DELETE FROM community.user WHERE id = $1::uuid`, id)
	if err != nil {
		return fmt.Errorf("UserRepo.Delete: %w", err)
	}
	return nil
}

// --- helpers ---

func scanUser(row pgx.Row) (*community.User, error) {
	var u community.User
	return &u, row.Scan(&u.ID, &u.DisplayName, &u.Email, &u.AvatarURL, &u.CreatedAt)
}

func nullableStr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// isUniqueViolation returns true if err is a PostgreSQL unique-constraint violation (SQLSTATE 23505).
func isUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	return contains(err.Error(), "23505") || contains(err.Error(), "unique")
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(s) > 0 && findStr(s, sub))
}

func findStr(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
```

> **Note:** `isUniqueViolation` uses string inspection because `pgconn.PgError` is not exported from pgx in a way that's convenient to import here without adding another direct dependency. If you prefer, replace with a pgconn import: `var pgErr *pgconn.PgError; errors.As(err, &pgErr) && pgErr.Code == "23505"`.

- [ ] 4.3 Run the integration tests.

```bash
DATABASE_URL="postgres://osm_dev:osm_dev@localhost:5432/cyclist_map_dev?sslmode=disable" \
  go test ./internal/infra/postgres/ -run TestUserRepo -v
```

All tests must pass.

- [ ] 4.4 Commit: `feat(postgres): add UserRepo with auth extensions`

---

## Task 5 — Community Postgres Repositories

**Files:**
- `internal/infra/postgres/community_repo.go` (create)
- `internal/infra/postgres/community_repo_test.go` (create)

### Steps

- [ ] 5.1 Write tests first.

```go
// internal/infra/postgres/community_repo_test.go
package postgres_test

import (
	"testing"
	"time"

	"github.com/cyclist-map/cyclist-map/internal/domain/community"
	"github.com/cyclist-map/cyclist-map/internal/infra/postgres"
)

// --- helpers ---

func seedUser(t *testing.T, pool interface{ /* pgxpool */ }) *community.User {
	t.Helper()
	// Use reflection or cast — simplest: accept *pgxpool.Pool directly.
	// For brevity, tests create their own repo instances.
	return nil // see body below
}

// --- ContributionRepo ---

func TestContributionRepo_CreateAndGet(t *testing.T) {
	pool := testPool(t)
	userRepo := postgres.NewUserRepo(pool)
	contribRepo := postgres.NewContributionRepo(pool)

	u, _ := community.NewUser("Rider A", uniqueEmail(t))
	_ = userRepo.Create(u)
	t.Cleanup(func() { _ = userRepo.Delete(u.ID) })

	c := community.NewContribution(u.ID, [][3]float64{
		{139.6917, 35.6895, 10},
		{139.7000, 35.6950, 20},
	}, map[string]any{"surface": "gravel"})

	if err := contribRepo.Create(c); err != nil {
		t.Fatalf("Create: %v", err)
	}
	t.Cleanup(func() { _ = contribRepo.Delete(c.ID) })

	got, err := contribRepo.GetByID(c.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.UserID != u.ID {
		t.Errorf("UserID: want %q, got %q", u.ID, got.UserID)
	}
	if got.Status != community.StatusPending {
		t.Errorf("Status: want pending, got %v", got.Status)
	}
}

func TestContributionRepo_Update(t *testing.T) {
	pool := testPool(t)
	userRepo := postgres.NewUserRepo(pool)
	contribRepo := postgres.NewContributionRepo(pool)

	u, _ := community.NewUser("Rider B", uniqueEmail(t))
	_ = userRepo.Create(u)
	t.Cleanup(func() { _ = userRepo.Delete(u.ID) })

	c := community.NewContribution(u.ID, nil, nil)
	_ = contribRepo.Create(c)
	t.Cleanup(func() { _ = contribRepo.Delete(c.ID) })

	_ = c.Approve("looks good")
	if err := contribRepo.Update(c); err != nil {
		t.Fatalf("Update: %v", err)
	}
	got, _ := contribRepo.GetByID(c.ID)
	if got.Status != community.StatusApproved {
		t.Errorf("Status after approve: %v", got.Status)
	}
}

// --- ReviewRepo ---

func TestReviewRepo_CreateAndList(t *testing.T) {
	pool := testPool(t)
	userRepo := postgres.NewUserRepo(pool)
	reviewRepo := postgres.NewReviewRepo(pool)

	// We need an existing route; use the first published route from seed data.
	routeID := seedRouteID(t, pool)

	u, _ := community.NewUser("Reviewer", uniqueEmail(t))
	_ = userRepo.Create(u)
	t.Cleanup(func() { _ = userRepo.Delete(u.ID) })

	rev, err := community.NewReview(u.ID, routeID, 4, "Great ride!")
	if err != nil {
		t.Fatalf("NewReview: %v", err)
	}
	if err := reviewRepo.Create(rev); err != nil {
		t.Fatalf("Create: %v", err)
	}
	t.Cleanup(func() { _ = reviewRepo.Delete(rev.ID) })

	list, err := reviewRepo.ListByRoute(routeID)
	if err != nil {
		t.Fatalf("ListByRoute: %v", err)
	}
	found := false
	for _, r := range list {
		if r.ID == rev.ID {
			found = true
			if r.Rating != 4 {
				t.Errorf("Rating: want 4, got %d", r.Rating)
			}
			break
		}
	}
	if !found {
		t.Error("review not found in ListByRoute result")
	}
}

// --- RideLogRepo ---

func TestRideLogRepo_CreateAndListByUser(t *testing.T) {
	pool := testPool(t)
	userRepo := postgres.NewUserRepo(pool)
	rideLogRepo := postgres.NewRideLogRepo(pool)

	routeID := seedRouteID(t, pool)

	u, _ := community.NewUser("Rider C", uniqueEmail(t))
	_ = userRepo.Create(u)
	t.Cleanup(func() { _ = userRepo.Delete(u.ID) })

	rl, err := community.NewRideLog(u.ID, routeID, int(time.Now().Unix()), 3600)
	if err != nil {
		t.Fatalf("NewRideLog: %v", err)
	}
	if err := rideLogRepo.Create(rl); err != nil {
		t.Fatalf("Create: %v", err)
	}
	t.Cleanup(func() { _ = rideLogRepo.Delete(rl.ID) })

	list, err := rideLogRepo.ListByUser(u.ID)
	if err != nil {
		t.Fatalf("ListByUser: %v", err)
	}
	if len(list) == 0 {
		t.Fatal("expected at least one ride log")
	}
	if list[0].ID != rl.ID {
		t.Errorf("ID: want %q, got %q", rl.ID, list[0].ID)
	}
}

func TestRideLogRepo_WithGPXTrack(t *testing.T) {
	pool := testPool(t)
	userRepo := postgres.NewUserRepo(pool)
	rideLogRepo := postgres.NewRideLogRepo(pool)

	routeID := seedRouteID(t, pool)
	u, _ := community.NewUser("Rider D", uniqueEmail(t))
	_ = userRepo.Create(u)
	t.Cleanup(func() { _ = userRepo.Delete(u.ID) })

	rl, _ := community.NewRideLog(u.ID, routeID, int(time.Now().Unix()), 1800)
	rl.SetGPXTrack([][3]float64{
		{139.6917, 35.6895, 10},
		{139.7000, 35.6950, 20},
	})
	if err := rideLogRepo.Create(rl); err != nil {
		t.Fatalf("Create with GPX: %v", err)
	}
	t.Cleanup(func() { _ = rideLogRepo.Delete(rl.ID) })

	got, err := rideLogRepo.GetByID(rl.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if len(got.GPXTrack) == 0 {
		t.Error("expected GPX track to be stored and retrieved")
	}
}

// seedRouteID returns the ID of any published route from the seed data.
func seedRouteID(t *testing.T, pool interface{}) string {
	t.Helper()
	// Cast to *pgxpool.Pool — tests always pass this type.
	p := pool.(*pgxpool_Pool)
	var id string
	err := p.QueryRow(context.Background(),
		`SELECT id FROM routes.route WHERE status = 'published' LIMIT 1`).Scan(&id)
	if err != nil {
		t.Skipf("no published route available for FK test: %v", err)
	}
	return id
}
```

> **Compilation note:** The `seedRouteID` helper above uses `pool.(*pgxpool_Pool)` as a placeholder. In the real file, import `"github.com/jackc/pgx/v5/pgxpool"` and use `pool.(*pgxpool.Pool)` with `context` imported. The `testPool` function already returns `*pgxpool.Pool`.

- [ ] 5.2 Create `internal/infra/postgres/community_repo.go`.

```go
// internal/infra/postgres/community_repo.go
package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/cyclist-map/cyclist-map/internal/domain/community"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ============================================================
// ContributionRepo
// ============================================================

// ContributionRepo implements community.ContributionRepository.
type ContributionRepo struct {
	pool *pgxpool.Pool
}

// NewContributionRepo creates a new ContributionRepo.
func NewContributionRepo(pool *pgxpool.Pool) *ContributionRepo {
	return &ContributionRepo{pool: pool}
}

// Create persists a new Contribution.
// Maps domain.RouteGeometry ([][3]float64) → PostGIS LINESTRING Z.
// Maps domain.ModeratorNotes → moderator_notes column (added via migration if absent;
// current schema uses no dedicated column — stored inside metadata JSONB instead).
func (r *ContributionRepo) Create(c *community.Contribution) error {
	ctx := context.Background()
	meta, err := json.Marshal(c.Metadata)
	if err != nil {
		return fmt.Errorf("ContributionRepo.Create marshal metadata: %w", err)
	}
	var geomArg any
	if len(c.RouteGeometry) > 0 {
		geomArg = coordsToWKT(c.RouteGeometry)
	}
	now := time.Now().UTC()
	if geomArg != nil {
		_, err = r.pool.Exec(ctx, `
			INSERT INTO community.contribution
				(id, user_id, route_geometry, metadata, status, created_at, updated_at)
			VALUES ($1::uuid, $2::uuid, ST_GeomFromText($3, 4326), $4, $5::community.contribution_status, $6, $7)
		`, c.ID, c.UserID, geomArg, meta, string(c.Status), now, now)
	} else {
		_, err = r.pool.Exec(ctx, `
			INSERT INTO community.contribution
				(id, user_id, metadata, status, created_at, updated_at)
			VALUES ($1::uuid, $2::uuid, $3, $4::community.contribution_status, $5, $6)
		`, c.ID, c.UserID, meta, string(c.Status), now, now)
	}
	if err != nil {
		return fmt.Errorf("ContributionRepo.Create: %w", err)
	}
	return nil
}

// GetByID retrieves a Contribution by UUID.
func (r *ContributionRepo) GetByID(id string) (*community.Contribution, error) {
	ctx := context.Background()
	row := r.pool.QueryRow(ctx, `
		SELECT id, user_id,
		       CASE WHEN route_geometry IS NOT NULL THEN ST_AsText(route_geometry) ELSE '' END,
		       COALESCE(metadata, '{}'),
		       status,
		       created_at
		FROM community.contribution WHERE id = $1::uuid
	`, id)
	var c community.Contribution
	var geomWKT, metaJSON, statusStr string
	var createdAt time.Time
	if err := row.Scan(&c.ID, &c.UserID, &geomWKT, &metaJSON, &statusStr, &createdAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("ContributionRepo.GetByID: %w", err)
	}
	c.Status = community.ContributionStatus(statusStr)
	c.SubmittedAt = createdAt
	if geomWKT != "" {
		coords, err := wktToCoords(geomWKT)
		if err == nil {
			c.RouteGeometry = coords
		}
	}
	if metaJSON != "" && metaJSON != "{}" {
		_ = json.Unmarshal([]byte(metaJSON), &c.Metadata)
	}
	return &c, nil
}

// Update writes changed status and moderator notes back to the database.
// ModeratorNotes is stored in a moderator_notes JSONB key inside metadata.
func (r *ContributionRepo) Update(c *community.Contribution) error {
	ctx := context.Background()
	// Merge ModeratorNotes into the metadata map before serialising.
	meta := c.Metadata
	if meta == nil {
		meta = make(map[string]any)
	}
	if c.ModeratorNotes != "" {
		meta["moderator_notes"] = c.ModeratorNotes
	}
	metaJSON, err := json.Marshal(meta)
	if err != nil {
		return fmt.Errorf("ContributionRepo.Update marshal: %w", err)
	}
	_, err = r.pool.Exec(ctx, `
		UPDATE community.contribution
		SET status = $2::community.contribution_status,
		    metadata = $3,
		    updated_at = $4
		WHERE id = $1::uuid
	`, c.ID, string(c.Status), metaJSON, time.Now().UTC())
	if err != nil {
		return fmt.Errorf("ContributionRepo.Update: %w", err)
	}
	return nil
}

// Delete removes a contribution by ID.
func (r *ContributionRepo) Delete(id string) error {
	ctx := context.Background()
	_, err := r.pool.Exec(ctx, `DELETE FROM community.contribution WHERE id = $1::uuid`, id)
	return err
}

// ============================================================
// ReviewRepo
// ============================================================

// ReviewRepo implements community.ReviewRepository.
type ReviewRepo struct {
	pool *pgxpool.Pool
}

// NewReviewRepo creates a new ReviewRepo.
func NewReviewRepo(pool *pgxpool.Pool) *ReviewRepo {
	return &ReviewRepo{pool: pool}
}

// Create inserts a new Review.
func (r *ReviewRepo) Create(rev *community.Review) error {
	ctx := context.Background()
	now := time.Now().UTC()
	_, err := r.pool.Exec(ctx, `
		INSERT INTO community.review (id, user_id, route_id, rating, body, created_at, updated_at)
		VALUES ($1::uuid, $2::uuid, $3::uuid, $4, $5, $6, $7)
	`, rev.ID, rev.UserID, rev.RouteID, rev.Rating, rev.Body, now, now)
	if err != nil {
		return fmt.Errorf("ReviewRepo.Create: %w", err)
	}
	return nil
}

// GetByID retrieves a Review by UUID.
func (r *ReviewRepo) GetByID(id string) (*community.Review, error) {
	ctx := context.Background()
	row := r.pool.QueryRow(ctx, `
		SELECT id, user_id, route_id, rating, COALESCE(body,''), created_at
		FROM community.review WHERE id = $1::uuid
	`, id)
	var rev community.Review
	if err := row.Scan(&rev.ID, &rev.UserID, &rev.RouteID, &rev.Rating, &rev.Body, &rev.CreatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("ReviewRepo.GetByID: %w", err)
	}
	return &rev, nil
}

// ListByRoute returns all reviews for the given route, newest first.
func (r *ReviewRepo) ListByRoute(routeID string) ([]*community.Review, error) {
	ctx := context.Background()
	rows, err := r.pool.Query(ctx, `
		SELECT id, user_id, route_id, rating, COALESCE(body,''), created_at
		FROM community.review
		WHERE route_id = $1::uuid
		ORDER BY created_at DESC
	`, routeID)
	if err != nil {
		return nil, fmt.Errorf("ReviewRepo.ListByRoute: %w", err)
	}
	defer rows.Close()
	var reviews []*community.Review
	for rows.Next() {
		var rev community.Review
		if err := rows.Scan(&rev.ID, &rev.UserID, &rev.RouteID, &rev.Rating, &rev.Body, &rev.CreatedAt); err != nil {
			return nil, err
		}
		reviews = append(reviews, &rev)
	}
	if reviews == nil {
		reviews = []*community.Review{}
	}
	return reviews, rows.Err()
}

// Delete removes a Review by ID.
func (r *ReviewRepo) Delete(id string) error {
	ctx := context.Background()
	_, err := r.pool.Exec(ctx, `DELETE FROM community.review WHERE id = $1::uuid`, id)
	return err
}

// ============================================================
// RideLogRepo
// ============================================================

// RideLogRepo implements community.RideLogRepository.
// Schema mapping:
//   domain.RiddenAt  (unix int)  ↔ DB started_at (TIMESTAMPTZ)
//   domain.DurationS (int secs)  ↔ derived: finished_at = started_at + duration_s * interval
type RideLogRepo struct {
	pool *pgxpool.Pool
}

// NewRideLogRepo creates a new RideLogRepo.
func NewRideLogRepo(pool *pgxpool.Pool) *RideLogRepo {
	return &RideLogRepo{pool: pool}
}

// Create inserts a RideLog, converting unix timestamps to TIMESTAMPTZ.
func (r *RideLogRepo) Create(rl *community.RideLog) error {
	ctx := context.Background()
	startedAt := time.Unix(int64(rl.RiddenAt), 0).UTC()
	finishedAt := startedAt.Add(time.Duration(rl.DurationS) * time.Second)
	now := time.Now().UTC()

	var err error
	if len(rl.GPXTrack) > 0 {
		wkt := coordsToWKT(rl.GPXTrack)
		_, err = r.pool.Exec(ctx, `
			INSERT INTO community.ride_log
				(id, user_id, route_id, gpx_track, started_at, finished_at, created_at)
			VALUES ($1::uuid, $2::uuid, $3::uuid, ST_GeomFromText($4, 4326), $5, $6, $7)
		`, rl.ID, rl.UserID, nullableUUID(rl.RouteID), wkt, startedAt, finishedAt, now)
	} else {
		_, err = r.pool.Exec(ctx, `
			INSERT INTO community.ride_log
				(id, user_id, route_id, started_at, finished_at, created_at)
			VALUES ($1::uuid, $2::uuid, $3::uuid, $4, $5, $6)
		`, rl.ID, rl.UserID, nullableUUID(rl.RouteID), startedAt, finishedAt, now)
	}
	if err != nil {
		return fmt.Errorf("RideLogRepo.Create: %w", err)
	}
	return nil
}

// GetByID retrieves a RideLog by UUID.
func (r *RideLogRepo) GetByID(id string) (*community.RideLog, error) {
	ctx := context.Background()
	row := r.pool.QueryRow(ctx, `
		SELECT id, user_id, COALESCE(route_id::text, ''),
		       CASE WHEN gpx_track IS NOT NULL THEN ST_AsText(gpx_track) ELSE '' END,
		       started_at, finished_at, created_at
		FROM community.ride_log WHERE id = $1::uuid
	`, id)
	rl, err := scanRideLog(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("RideLogRepo.GetByID: %w", err)
	}
	return rl, nil
}

// ListByUser returns all ride logs for the given user, newest first.
func (r *RideLogRepo) ListByUser(userID string) ([]*community.RideLog, error) {
	ctx := context.Background()
	rows, err := r.pool.Query(ctx, `
		SELECT id, user_id, COALESCE(route_id::text, ''),
		       CASE WHEN gpx_track IS NOT NULL THEN ST_AsText(gpx_track) ELSE '' END,
		       started_at, finished_at, created_at
		FROM community.ride_log
		WHERE user_id = $1::uuid
		ORDER BY started_at DESC
	`, userID)
	if err != nil {
		return nil, fmt.Errorf("RideLogRepo.ListByUser: %w", err)
	}
	defer rows.Close()
	return collectRideLogs(rows)
}

// ListByRoute returns all ride logs for the given route, newest first.
func (r *RideLogRepo) ListByRoute(routeID string) ([]*community.RideLog, error) {
	ctx := context.Background()
	rows, err := r.pool.Query(ctx, `
		SELECT id, user_id, COALESCE(route_id::text, ''),
		       CASE WHEN gpx_track IS NOT NULL THEN ST_AsText(gpx_track) ELSE '' END,
		       started_at, finished_at, created_at
		FROM community.ride_log
		WHERE route_id = $1::uuid
		ORDER BY started_at DESC
	`, routeID)
	if err != nil {
		return nil, fmt.Errorf("RideLogRepo.ListByRoute: %w", err)
	}
	defer rows.Close()
	return collectRideLogs(rows)
}

// Delete removes a RideLog by ID.
func (r *RideLogRepo) Delete(id string) error {
	ctx := context.Background()
	_, err := r.pool.Exec(ctx, `DELETE FROM community.ride_log WHERE id = $1::uuid`, id)
	return err
}

// --- helpers ---

func scanRideLog(row pgx.Row) (*community.RideLog, error) {
	var rl community.RideLog
	var gpxWKT string
	var startedAt, finishedAt, createdAt time.Time
	if err := row.Scan(&rl.ID, &rl.UserID, &rl.RouteID, &gpxWKT, &startedAt, &finishedAt, &createdAt); err != nil {
		return nil, err
	}
	rl.RiddenAt = int(startedAt.Unix())
	rl.DurationS = int(finishedAt.Sub(startedAt).Seconds())
	rl.CreatedAt = createdAt
	if gpxWKT != "" {
		coords, err := wktToCoords(gpxWKT)
		if err == nil {
			rl.GPXTrack = coords
		}
	}
	// Ensure nil slices are empty slices for consistent JSON encoding.
	if rl.GPXTrack == nil {
		rl.GPXTrack = [][3]float64{}
	}
	return &rl, nil
}

func collectRideLogs(rows pgx.Rows) ([]*community.RideLog, error) {
	var logs []*community.RideLog
	for rows.Next() {
		rl, err := scanRideLog(rows)
		if err != nil {
			return nil, err
		}
		logs = append(logs, rl)
	}
	if logs == nil {
		logs = []*community.RideLog{}
	}
	return logs, rows.Err()
}

// pointToWKT encodes a single lat/lon point as WKT POINT for PostGIS.
// Kept here for future use by other community repos if needed.
func pointToWKT(lon, lat float64) string {
	return fmt.Sprintf("POINT(%f %f)", lon, lat)
}

// ensure coordsToWKT / wktToCoords / nullableUUID / ErrNotFound are available.
// They are defined in route_repo.go (same package), so no re-declaration needed.
var _ = strings.Join // suppress unused import if strings is only used transitively
```

- [ ] 5.3 Run integration tests.

```bash
DATABASE_URL="postgres://osm_dev:osm_dev@localhost:5432/cyclist_map_dev?sslmode=disable" \
  go test ./internal/infra/postgres/ -run TestContribution -run TestReview -run TestRideLog -v
```

- [ ] 5.4 Commit: `feat(postgres): add ContributionRepo, ReviewRepo, RideLogRepo`

---

## Task 6 — Auth HTTP Middleware

**Files:**
- `internal/api/auth_middleware.go` (create)
- `internal/api/auth_middleware_test.go` (create)

### Steps

- [ ] 6.1 Write tests first.

```go
// internal/api/auth_middleware_test.go
package api_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/cyclist-map/cyclist-map/internal/api"
	"github.com/cyclist-map/cyclist-map/internal/app"
)

func makeAuthService(t *testing.T) *app.AuthService {
	t.Helper()
	// Stub repo with one pre-seeded user.
	repo := newStubAuthRepo()
	svc, err := app.NewAuthService(repo, "test-secret-32-chars-padding-here", 15*time.Minute, 7*24*time.Hour)
	if err != nil {
		t.Fatalf("NewAuthService: %v", err)
	}
	return svc
}

func TestAuthMiddleware_NoBearerToken(t *testing.T) {
	svc := makeAuthService(t)
	mw := api.AuthMiddleware(svc)
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rr.Code)
	}
}

func TestAuthMiddleware_InvalidToken(t *testing.T) {
	svc := makeAuthService(t)
	mw := api.AuthMiddleware(svc)
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer not.a.real.token")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rr.Code)
	}
}

func TestAuthMiddleware_ValidToken_InjectsUserID(t *testing.T) {
	svc := makeAuthService(t)
	repo := newStubAuthRepo()
	svc2, _ := app.NewAuthService(repo, "test-secret-32-chars-padding-here", 15*time.Minute, 7*24*time.Hour)

	// Register and login to get a real token.
	u, _ := svc2.Register("Test User", "test@example.com", "password123")
	tokens, _ := svc2.Login("test@example.com", "password123")

	mw := api.AuthMiddleware(svc2)
	var gotUserID string
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUserID = api.UserIDFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+tokens.AccessToken)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}
	if gotUserID != u.ID {
		t.Errorf("userID in context: want %q, got %q", u.ID, gotUserID)
	}
}
```

> **Note:** `newStubAuthRepo()` in the test file mirrors the stub from Task 3 tests — move it to a `testutil_test.go` in `internal/api/` or inline it in the test file.

- [ ] 6.2 Create `internal/api/auth_middleware.go`.

```go
// internal/api/auth_middleware.go
package api

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/cyclist-map/cyclist-map/internal/app"
)

type contextKey string

const ctxUserID contextKey = "userID"

// TokenValidator is the subset of AuthService needed by the middleware.
type TokenValidator interface {
	ValidateAccessToken(token string) (string, error)
}

// AuthMiddleware returns a chi-compatible middleware that validates the
// Authorization: Bearer <token> header, injects the user ID into the request
// context, and returns 401 if the token is absent or invalid.
func AuthMiddleware(v TokenValidator) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
				writeError(w, http.StatusUnauthorized, "authorization required")
				return
			}
			tokenStr := strings.TrimPrefix(authHeader, "Bearer ")
			userID, err := v.ValidateAccessToken(tokenStr)
			if err != nil {
				if errors.Is(err, app.ErrInvalidToken) {
					writeError(w, http.StatusUnauthorized, "invalid or expired token")
					return
				}
				writeError(w, http.StatusUnauthorized, "authorization failed")
				return
			}
			ctx := context.WithValue(r.Context(), ctxUserID, userID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// UserIDFromContext extracts the authenticated user ID from a request context.
// Returns an empty string if not present (for public routes).
func UserIDFromContext(ctx context.Context) string {
	v, _ := ctx.Value(ctxUserID).(string)
	return v
}
```

- [ ] 6.3 Run middleware tests.

```bash
go test ./internal/api/ -run TestAuthMiddleware -v
```

- [ ] 6.4 Commit: `feat(api): add JWT auth middleware`

---

## Task 7 — Auth HTTP Handler

**Files:**
- `internal/api/auth_handler.go` (create)
- `internal/api/auth_handler_test.go` (create)

### Steps

- [ ] 7.1 Write tests first.

```go
// internal/api/auth_handler_test.go
package api_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/cyclist-map/cyclist-map/internal/api"
	"github.com/cyclist-map/cyclist-map/internal/app"
)

func makeAuthHandler(t *testing.T) (*api.AuthHandler, *app.AuthService) {
	t.Helper()
	repo := newStubAuthRepo()
	svc, _ := app.NewAuthService(repo, "test-secret-32-chars-padding-here", 15*time.Minute, 7*24*time.Hour)
	return api.NewAuthHandler(svc), svc
}

func TestAuthHandler_Register_Success(t *testing.T) {
	h, _ := makeAuthHandler(t)

	body := `{"display_name":"Yuki","email":"yuki@test.com","password":"goodpassword"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/register", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	h.Register(rr, req)

	if rr.Code != http.StatusCreated {
		t.Errorf("expected 201, got %d: %s", rr.Code, rr.Body.String())
	}
	var resp map[string]any
	_ = json.NewDecoder(rr.Body).Decode(&resp)
	if resp["id"] == "" {
		t.Error("expected id in response")
	}
}

func TestAuthHandler_Register_EmptyPassword(t *testing.T) {
	h, _ := makeAuthHandler(t)
	body := `{"display_name":"Yuki","email":"yuki2@test.com","password":""}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/register", bytes.NewBufferString(body))
	rr := httptest.NewRecorder()
	h.Register(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rr.Code)
	}
}

func TestAuthHandler_Register_DuplicateEmail(t *testing.T) {
	h, _ := makeAuthHandler(t)
	body := `{"display_name":"Yuki","email":"dup@test.com","password":"password1"}`
	for i := 0; i < 2; i++ {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/register", bytes.NewBufferString(body))
		rr := httptest.NewRecorder()
		h.Register(rr, req)
		if i == 1 && rr.Code != http.StatusConflict {
			t.Errorf("expected 409 on duplicate, got %d", rr.Code)
		}
	}
}

func TestAuthHandler_Login_Success(t *testing.T) {
	h, svc := makeAuthHandler(t)
	_, _ = svc.Register("Yuki", "yuki3@test.com", "pass123")

	body := `{"email":"yuki3@test.com","password":"pass123"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewBufferString(body))
	rr := httptest.NewRecorder()
	h.Login(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	var resp map[string]any
	_ = json.NewDecoder(rr.Body).Decode(&resp)
	if resp["access_token"] == "" {
		t.Error("expected access_token in response")
	}
}

func TestAuthHandler_Login_BadCredentials(t *testing.T) {
	h, _ := makeAuthHandler(t)
	body := `{"email":"nobody@test.com","password":"wrong"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewBufferString(body))
	rr := httptest.NewRecorder()
	h.Login(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rr.Code)
	}
}

func TestAuthHandler_Refresh_Success(t *testing.T) {
	h, svc := makeAuthHandler(t)
	_, _ = svc.Register("Yuki", "yuki4@test.com", "pass123")
	tokens, _ := svc.Login("yuki4@test.com", "pass123")

	body, _ := json.Marshal(map[string]string{"refresh_token": tokens.RefreshToken})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/refresh", bytes.NewReader(body))
	rr := httptest.NewRecorder()
	h.Refresh(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestAuthHandler_Refresh_Invalid(t *testing.T) {
	h, _ := makeAuthHandler(t)
	body := `{"refresh_token":"garbage.token.here"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/refresh", bytes.NewBufferString(body))
	rr := httptest.NewRecorder()
	h.Refresh(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rr.Code)
	}
}
```

- [ ] 7.2 Create `internal/api/auth_handler.go`.

```go
// internal/api/auth_handler.go
package api

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/cyclist-map/cyclist-map/internal/app"
	"github.com/cyclist-map/cyclist-map/internal/domain/community"
)

// AuthHandler handles registration, login, and token refresh.
type AuthHandler struct {
	svc *app.AuthService
}

// NewAuthHandler creates an AuthHandler.
func NewAuthHandler(svc *app.AuthService) *AuthHandler {
	return &AuthHandler{svc: svc}
}

// --- request / response types ---

type registerRequest struct {
	DisplayName string `json:"display_name"`
	Email       string `json:"email"`
	Password    string `json:"password"`
}

type registerResponse struct {
	ID          string `json:"id"`
	DisplayName string `json:"display_name"`
	Email       string `json:"email"`
	CreatedAt   string `json:"created_at"`
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type refreshRequest struct {
	RefreshToken string `json:"refresh_token"`
}

// --- handlers ---

// Register handles POST /api/v1/auth/register.
func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	var req registerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	u, err := h.svc.Register(req.DisplayName, req.Email, req.Password)
	if err != nil {
		switch {
		case errors.Is(err, app.ErrWeakPassword):
			writeError(w, http.StatusBadRequest, err.Error())
		case errors.Is(err, community.ErrEmptyDisplayName):
			writeError(w, http.StatusBadRequest, err.Error())
		case errors.Is(err, app.ErrEmailTaken):
			writeError(w, http.StatusConflict, err.Error())
		default:
			writeError(w, http.StatusInternalServerError, "registration failed")
		}
		return
	}
	writeJSON(w, http.StatusCreated, registerResponse{
		ID:          u.ID,
		DisplayName: u.DisplayName,
		Email:       u.Email,
		CreatedAt:   u.CreatedAt.Format("2006-01-02T15:04:05Z"),
	})
}

// Login handles POST /api/v1/auth/login.
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	tokens, err := h.svc.Login(req.Email, req.Password)
	if err != nil {
		if errors.Is(err, app.ErrBadCredentials) {
			writeError(w, http.StatusUnauthorized, "invalid email or password")
			return
		}
		writeError(w, http.StatusInternalServerError, "login failed")
		return
	}
	writeJSON(w, http.StatusOK, tokens)
}

// Refresh handles POST /api/v1/auth/refresh.
func (h *AuthHandler) Refresh(w http.ResponseWriter, r *http.Request) {
	var req refreshRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	tokens, err := h.svc.Refresh(req.RefreshToken)
	if err != nil {
		if errors.Is(err, app.ErrInvalidToken) {
			writeError(w, http.StatusUnauthorized, "invalid or expired refresh token")
			return
		}
		writeError(w, http.StatusInternalServerError, "refresh failed")
		return
	}
	writeJSON(w, http.StatusOK, tokens)
}
```

- [ ] 7.3 Run handler tests.

```bash
go test ./internal/api/ -run TestAuthHandler -v
```

- [ ] 7.4 Commit: `feat(api): add auth HTTP handler (register, login, refresh)`

---

## Task 8 — Community HTTP Handler

**Files:**
- `internal/api/community_handler.go` (create)
- `internal/api/community_handler_test.go` (create)

### Steps

- [ ] 8.1 Write tests first.

```go
// internal/api/community_handler_test.go
package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/cyclist-map/cyclist-map/internal/api"
	"github.com/cyclist-map/cyclist-map/internal/app"
	"github.com/cyclist-map/cyclist-map/internal/domain/community"
	"github.com/go-chi/chi/v5"
)

// --- stub community service ---

type stubCommunityService struct {
	contributions []*community.Contribution
	reviews       []*community.Review
	rideLogs      []*community.RideLog
}

func (s *stubCommunityService) SubmitContribution(userID string, geometry [][3]float64, metadata map[string]any) (*community.Contribution, error) {
	c := community.NewContribution(userID, geometry, metadata)
	s.contributions = append(s.contributions, c)
	return c, nil
}

func (s *stubCommunityService) AddReview(userID, routeID string, rating int, body string) (*community.Review, error) {
	r, err := community.NewReview(userID, routeID, rating, body)
	if err != nil {
		return nil, err
	}
	s.reviews = append(s.reviews, r)
	return r, nil
}

func (s *stubCommunityService) ListReviews(routeID string) ([]*community.Review, error) {
	var out []*community.Review
	for _, r := range s.reviews {
		if r.RouteID == routeID {
			out = append(out, r)
		}
	}
	return out, nil
}

func (s *stubCommunityService) LogRide(userID, routeID string, riddenAt, durationS int, gpxTrack [][3]float64) (*community.RideLog, error) {
	rl, err := community.NewRideLog(userID, routeID, riddenAt, durationS)
	if err != nil {
		return nil, err
	}
	if len(gpxTrack) > 0 {
		rl.SetGPXTrack(gpxTrack)
	}
	s.rideLogs = append(s.rideLogs, rl)
	return rl, nil
}

func (s *stubCommunityService) ListUserRideLogs(userID string) ([]*community.RideLog, error) {
	var out []*community.RideLog
	for _, rl := range s.rideLogs {
		if rl.UserID == userID {
			out = append(out, rl)
		}
	}
	return out, nil
}

// --- test helpers ---

func ctxWithUser(r *http.Request, userID string) *http.Request {
	ctx := context.WithValue(r.Context(), api.ExportedCtxUserID, userID)
	return r.WithContext(ctx)
}

func chiCtxWithParam(r *http.Request, key, val string) *http.Request {
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add(key, val)
	return r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
}

// --- tests ---

func TestCommunityHandler_SubmitContribution(t *testing.T) {
	stub := &stubCommunityService{}
	h := api.NewCommunityHandler(stub)

	body := `{"geometry":[[139.69,35.69,10],[139.70,35.70,15]],"metadata":{"surface":"gravel"}}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/contributions", bytes.NewBufferString(body))
	req = ctxWithUser(req, "user-123")
	rr := httptest.NewRecorder()
	h.SubmitContribution(rr, req)

	if rr.Code != http.StatusCreated {
		t.Errorf("expected 201, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestCommunityHandler_SubmitContribution_Unauthenticated(t *testing.T) {
	stub := &stubCommunityService{}
	h := api.NewCommunityHandler(stub)

	body := `{"geometry":[[139.69,35.69,10],[139.70,35.70,15]]}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/contributions", bytes.NewBufferString(body))
	// No user in context
	rr := httptest.NewRecorder()
	h.SubmitContribution(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rr.Code)
	}
}

func TestCommunityHandler_AddReview(t *testing.T) {
	stub := &stubCommunityService{}
	h := api.NewCommunityHandler(stub)

	body := `{"rating":4,"body":"Great ride!"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/routes/route-1/reviews", bytes.NewBufferString(body))
	req = ctxWithUser(req, "user-123")
	req = chiCtxWithParam(req, "id", "route-1")
	rr := httptest.NewRecorder()
	h.AddReview(rr, req)

	if rr.Code != http.StatusCreated {
		t.Errorf("expected 201, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestCommunityHandler_AddReview_InvalidRating(t *testing.T) {
	stub := &stubCommunityService{}
	h := api.NewCommunityHandler(stub)

	body := `{"rating":6,"body":"too high"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/routes/route-1/reviews", bytes.NewBufferString(body))
	req = ctxWithUser(req, "user-123")
	req = chiCtxWithParam(req, "id", "route-1")
	rr := httptest.NewRecorder()
	h.AddReview(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rr.Code)
	}
}

func TestCommunityHandler_ListReviews(t *testing.T) {
	stub := &stubCommunityService{}
	h := api.NewCommunityHandler(stub)
	// Pre-seed a review.
	rev, _ := community.NewReview("user-1", "route-1", 5, "Perfect!")
	stub.reviews = append(stub.reviews, rev)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/routes/route-1/reviews", nil)
	req = chiCtxWithParam(req, "id", "route-1")
	rr := httptest.NewRecorder()
	h.ListReviews(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}
	var resp map[string]any
	_ = json.NewDecoder(rr.Body).Decode(&resp)
	reviews, ok := resp["reviews"].([]any)
	if !ok || len(reviews) != 1 {
		t.Errorf("expected 1 review, got %v", resp)
	}
}

func TestCommunityHandler_LogRide(t *testing.T) {
	stub := &stubCommunityService{}
	h := api.NewCommunityHandler(stub)

	now := time.Now().Unix()
	body, _ := json.Marshal(map[string]any{
		"ridden_at":  now,
		"duration_s": 3600,
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/routes/route-1/ride-logs", bytes.NewReader(body))
	req = ctxWithUser(req, "user-123")
	req = chiCtxWithParam(req, "id", "route-1")
	rr := httptest.NewRecorder()
	h.LogRide(rr, req)

	if rr.Code != http.StatusCreated {
		t.Errorf("expected 201, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestCommunityHandler_ListUserRideLogs(t *testing.T) {
	stub := &stubCommunityService{}
	h := api.NewCommunityHandler(stub)
	rl, _ := community.NewRideLog("user-1", "route-1", 1744300000, 7200)
	stub.rideLogs = append(stub.rideLogs, rl)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/users/user-1/ride-logs", nil)
	req = chiCtxWithParam(req, "id", "user-1")
	rr := httptest.NewRecorder()
	h.ListUserRideLogs(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}
	var resp map[string]any
	_ = json.NewDecoder(rr.Body).Decode(&resp)
	logs, ok := resp["ride_logs"].([]any)
	if !ok || len(logs) != 1 {
		t.Errorf("expected 1 ride log, got %v", resp)
	}
}
```

> **Note on `ExportedCtxUserID`:** The test uses `api.ExportedCtxUserID` to inject context values without relying on internal string literals. Export the key from `auth_middleware.go`: `var ExportedCtxUserID = ctxUserID` (only for tests). Alternatively, expose a `WithUserID(ctx, id)` test helper in `auth_middleware.go`.

- [ ] 8.2 Create `internal/api/community_handler.go`.

```go
// internal/api/community_handler.go
package api

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/cyclist-map/cyclist-map/internal/domain/community"
	"github.com/go-chi/chi/v5"
)

// CommunityService is the interface the handler calls. Matches app.CommunityService.
type CommunityService interface {
	SubmitContribution(userID string, geometry [][3]float64, metadata map[string]any) (*community.Contribution, error)
	AddReview(userID, routeID string, rating int, body string) (*community.Review, error)
	ListReviews(routeID string) ([]*community.Review, error)
	LogRide(userID, routeID string, riddenAt, durationS int, gpxTrack [][3]float64) (*community.RideLog, error)
	ListUserRideLogs(userID string) ([]*community.RideLog, error)
}

// CommunityHandler handles community endpoints.
type CommunityHandler struct {
	svc CommunityService
}

// NewCommunityHandler creates a CommunityHandler.
func NewCommunityHandler(svc CommunityService) *CommunityHandler {
	return &CommunityHandler{svc: svc}
}

// --- request / response types ---

type submitContributionRequest struct {
	Geometry [][3]float64   `json:"geometry"`
	Metadata map[string]any `json:"metadata"`
}

type contributionResponse struct {
	ID          string `json:"id"`
	UserID      string `json:"user_id"`
	Status      string `json:"status"`
	SubmittedAt string `json:"submitted_at"`
}

type addReviewRequest struct {
	Rating int    `json:"rating"`
	Body   string `json:"body"`
}

type reviewResponse struct {
	ID        string `json:"id"`
	UserID    string `json:"user_id"`
	RouteID   string `json:"route_id"`
	Rating    int    `json:"rating"`
	Body      string `json:"body"`
	CreatedAt string `json:"created_at"`
}

type listReviewsResponse struct {
	Reviews []reviewResponse `json:"reviews"`
}

type logRideRequest struct {
	RiddenAt  int64        `json:"ridden_at"`  // Unix timestamp
	DurationS int          `json:"duration_s"` // seconds
	GPXTrack  [][3]float64 `json:"gpx_track"`  // optional
}

type rideLogResponse struct {
	ID        string       `json:"id"`
	UserID    string       `json:"user_id"`
	RouteID   string       `json:"route_id"`
	RiddenAt  int64        `json:"ridden_at"`
	DurationS int          `json:"duration_s"`
	GPXTrack  [][3]float64 `json:"gpx_track,omitempty"`
	CreatedAt string       `json:"created_at"`
}

type listRideLogsResponse struct {
	RideLogs []rideLogResponse `json:"ride_logs"`
}

// --- handlers ---

// SubmitContribution handles POST /api/v1/contributions (authenticated).
func (h *CommunityHandler) SubmitContribution(w http.ResponseWriter, r *http.Request) {
	userID := UserIDFromContext(r.Context())
	if userID == "" {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}
	var req submitContributionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	c, err := h.svc.SubmitContribution(userID, req.Geometry, req.Metadata)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to submit contribution")
		return
	}
	writeJSON(w, http.StatusCreated, contributionResponse{
		ID:          c.ID,
		UserID:      c.UserID,
		Status:      string(c.Status),
		SubmittedAt: c.SubmittedAt.Format("2006-01-02T15:04:05Z"),
	})
}

// ListReviews handles GET /api/v1/routes/:id/reviews (public).
func (h *CommunityHandler) ListReviews(w http.ResponseWriter, r *http.Request) {
	routeID := chi.URLParam(r, "id")
	reviews, err := h.svc.ListReviews(routeID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list reviews")
		return
	}
	resp := listReviewsResponse{Reviews: make([]reviewResponse, len(reviews))}
	for i, rev := range reviews {
		resp.Reviews[i] = toReviewResponse(rev)
	}
	writeJSON(w, http.StatusOK, resp)
}

// AddReview handles POST /api/v1/routes/:id/reviews (authenticated).
func (h *CommunityHandler) AddReview(w http.ResponseWriter, r *http.Request) {
	userID := UserIDFromContext(r.Context())
	if userID == "" {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}
	routeID := chi.URLParam(r, "id")
	var req addReviewRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	rev, err := h.svc.AddReview(userID, routeID, req.Rating, req.Body)
	if err != nil {
		if errors.Is(err, community.ErrInvalidRating) {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to add review")
		return
	}
	writeJSON(w, http.StatusCreated, toReviewResponse(rev))
}

// LogRide handles POST /api/v1/routes/:id/ride-logs (authenticated).
func (h *CommunityHandler) LogRide(w http.ResponseWriter, r *http.Request) {
	userID := UserIDFromContext(r.Context())
	if userID == "" {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}
	routeID := chi.URLParam(r, "id")
	var req logRideRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	rl, err := h.svc.LogRide(userID, routeID, int(req.RiddenAt), req.DurationS, req.GPXTrack)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to log ride")
		return
	}
	writeJSON(w, http.StatusCreated, toRideLogResponse(rl))
}

// ListUserRideLogs handles GET /api/v1/users/:id/ride-logs (public).
func (h *CommunityHandler) ListUserRideLogs(w http.ResponseWriter, r *http.Request) {
	userID := chi.URLParam(r, "id")
	logs, err := h.svc.ListUserRideLogs(userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list ride logs")
		return
	}
	resp := listRideLogsResponse{RideLogs: make([]rideLogResponse, len(logs))}
	for i, rl := range logs {
		resp.RideLogs[i] = toRideLogResponse(rl)
	}
	writeJSON(w, http.StatusOK, resp)
}

// --- helpers ---

func toReviewResponse(rev *community.Review) reviewResponse {
	return reviewResponse{
		ID:        rev.ID,
		UserID:    rev.UserID,
		RouteID:   rev.RouteID,
		Rating:    rev.Rating,
		Body:      rev.Body,
		CreatedAt: rev.CreatedAt.Format("2006-01-02T15:04:05Z"),
	}
}

func toRideLogResponse(rl *community.RideLog) rideLogResponse {
	return rideLogResponse{
		ID:        rl.ID,
		UserID:    rl.UserID,
		RouteID:   rl.RouteID,
		RiddenAt:  int64(rl.RiddenAt),
		DurationS: rl.DurationS,
		GPXTrack:  rl.GPXTrack,
		CreatedAt: rl.CreatedAt.Format("2006-01-02T15:04:05Z"),
	}
}
```

- [ ] 8.3 Run handler tests.

```bash
go test ./internal/api/ -run TestCommunityHandler -v
```

- [ ] 8.4 Commit: `feat(api): add community HTTP handler`

---

## Task 9 — Community Application Service

**Files:**
- `internal/app/community_service.go` (create)
- `internal/app/community_service_test.go` (create)

### Steps

- [ ] 9.1 Write tests first.

```go
// internal/app/community_service_test.go
package app_test

import (
	"testing"
	"time"

	"github.com/cyclist-map/cyclist-map/internal/app"
	"github.com/cyclist-map/cyclist-map/internal/domain/community"
)

// --- stub repos ---

type stubContribRepo struct{ items []*community.Contribution }

func (s *stubContribRepo) Create(c *community.Contribution) error {
	s.items = append(s.items, c)
	return nil
}
func (s *stubContribRepo) GetByID(id string) (*community.Contribution, error) {
	for _, c := range s.items {
		if c.ID == id {
			return c, nil
		}
	}
	return nil, app.ErrUserNotFound
}
func (s *stubContribRepo) Update(c *community.Contribution) error { return nil }
func (s *stubContribRepo) Delete(id string) error                 { return nil }

type stubReviewRepo struct{ items []*community.Review }

func (s *stubReviewRepo) Create(r *community.Review) error {
	s.items = append(s.items, r)
	return nil
}
func (s *stubReviewRepo) GetByID(id string) (*community.Review, error) { return nil, nil }
func (s *stubReviewRepo) ListByRoute(routeID string) ([]*community.Review, error) {
	var out []*community.Review
	for _, r := range s.items {
		if r.RouteID == routeID {
			out = append(out, r)
		}
	}
	return out, nil
}
func (s *stubReviewRepo) Delete(id string) error { return nil }

type stubRideLogRepo struct{ items []*community.RideLog }

func (s *stubRideLogRepo) Create(rl *community.RideLog) error {
	s.items = append(s.items, rl)
	return nil
}
func (s *stubRideLogRepo) GetByID(id string) (*community.RideLog, error) { return nil, nil }
func (s *stubRideLogRepo) ListByUser(userID string) ([]*community.RideLog, error) {
	var out []*community.RideLog
	for _, rl := range s.items {
		if rl.UserID == userID {
			out = append(out, rl)
		}
	}
	return out, nil
}
func (s *stubRideLogRepo) ListByRoute(routeID string) ([]*community.RideLog, error) {
	return nil, nil
}
func (s *stubRideLogRepo) Delete(id string) error { return nil }

// --- tests ---

func newTestCommunitySvc() *app.CommunityService {
	return app.NewCommunityService(
		&stubContribRepo{},
		&stubReviewRepo{},
		&stubRideLogRepo{},
	)
}

func TestCommunityService_SubmitContribution(t *testing.T) {
	svc := newTestCommunitySvc()
	c, err := svc.SubmitContribution("user-1", [][3]float64{{139.0, 35.0, 0}}, nil)
	if err != nil {
		t.Fatalf("SubmitContribution: %v", err)
	}
	if c.UserID != "user-1" {
		t.Errorf("UserID: want user-1, got %q", c.UserID)
	}
	if c.Status != community.StatusPending {
		t.Errorf("Status: want pending, got %v", c.Status)
	}
}

func TestCommunityService_AddReview_Valid(t *testing.T) {
	svc := newTestCommunitySvc()
	rev, err := svc.AddReview("user-1", "route-1", 5, "Excellent!")
	if err != nil {
		t.Fatalf("AddReview: %v", err)
	}
	if rev.Rating != 5 {
		t.Errorf("Rating: want 5, got %d", rev.Rating)
	}
}

func TestCommunityService_AddReview_InvalidRating(t *testing.T) {
	svc := newTestCommunitySvc()
	_, err := svc.AddReview("user-1", "route-1", 0, "bad")
	if err == nil {
		t.Fatal("expected error for rating 0")
	}
}

func TestCommunityService_ListReviews(t *testing.T) {
	svc := newTestCommunitySvc()
	_, _ = svc.AddReview("user-1", "route-A", 4, "Good")
	_, _ = svc.AddReview("user-2", "route-A", 3, "OK")
	_, _ = svc.AddReview("user-1", "route-B", 5, "Amazing")

	reviews, err := svc.ListReviews("route-A")
	if err != nil {
		t.Fatalf("ListReviews: %v", err)
	}
	if len(reviews) != 2 {
		t.Errorf("expected 2 reviews for route-A, got %d", len(reviews))
	}
}

func TestCommunityService_LogRide(t *testing.T) {
	svc := newTestCommunitySvc()
	rl, err := svc.LogRide("user-1", "route-1", int(time.Now().Unix()), 3600, nil)
	if err != nil {
		t.Fatalf("LogRide: %v", err)
	}
	if rl.UserID != "user-1" {
		t.Errorf("UserID: want user-1, got %q", rl.UserID)
	}
}

func TestCommunityService_ListUserRideLogs(t *testing.T) {
	svc := newTestCommunitySvc()
	now := int(time.Now().Unix())
	_, _ = svc.LogRide("user-1", "route-1", now, 1800, nil)
	_, _ = svc.LogRide("user-1", "route-2", now, 3600, nil)
	_, _ = svc.LogRide("user-2", "route-1", now, 900, nil)

	logs, err := svc.ListUserRideLogs("user-1")
	if err != nil {
		t.Fatalf("ListUserRideLogs: %v", err)
	}
	if len(logs) != 2 {
		t.Errorf("expected 2 logs for user-1, got %d", len(logs))
	}
}
```

- [ ] 9.2 Create `internal/app/community_service.go`.

```go
// internal/app/community_service.go
package app

import (
	"github.com/cyclist-map/cyclist-map/internal/domain/community"
)

// CommunityService orchestrates community use cases.
type CommunityService struct {
	contributions community.ContributionRepository
	reviews       community.ReviewRepository
	rideLogs      community.RideLogRepository
}

// NewCommunityService creates a CommunityService.
func NewCommunityService(
	contributions community.ContributionRepository,
	reviews community.ReviewRepository,
	rideLogs community.RideLogRepository,
) *CommunityService {
	return &CommunityService{
		contributions: contributions,
		reviews:       reviews,
		rideLogs:      rideLogs,
	}
}

// SubmitContribution creates a new pending Contribution and persists it.
func (s *CommunityService) SubmitContribution(userID string, geometry [][3]float64, metadata map[string]any) (*community.Contribution, error) {
	c := community.NewContribution(userID, geometry, metadata)
	if err := s.contributions.Create(c); err != nil {
		return nil, err
	}
	return c, nil
}

// AddReview creates and persists a new Review.
func (s *CommunityService) AddReview(userID, routeID string, rating int, body string) (*community.Review, error) {
	rev, err := community.NewReview(userID, routeID, rating, body)
	if err != nil {
		return nil, err
	}
	if err := s.reviews.Create(rev); err != nil {
		return nil, err
	}
	return rev, nil
}

// ListReviews returns all reviews for a given route.
func (s *CommunityService) ListReviews(routeID string) ([]*community.Review, error) {
	return s.reviews.ListByRoute(routeID)
}

// LogRide creates and persists a RideLog, attaching a GPX track when provided.
func (s *CommunityService) LogRide(userID, routeID string, riddenAt, durationS int, gpxTrack [][3]float64) (*community.RideLog, error) {
	rl, err := community.NewRideLog(userID, routeID, riddenAt, durationS)
	if err != nil {
		return nil, err
	}
	if len(gpxTrack) > 0 {
		rl.SetGPXTrack(gpxTrack)
	}
	if err := s.rideLogs.Create(rl); err != nil {
		return nil, err
	}
	return rl, nil
}

// ListUserRideLogs returns all ride logs for a given user.
func (s *CommunityService) ListUserRideLogs(userID string) ([]*community.RideLog, error) {
	return s.rideLogs.ListByUser(userID)
}
```

- [ ] 9.3 Run unit tests.

```bash
go test ./internal/app/ -run TestCommunityService -v
```

- [ ] 9.4 Commit: `feat(app): add CommunityService`

---

## Task 10 — Wire Router and main.go

**Files:**
- `internal/api/router.go` (modify)
- `cmd/api/main.go` (modify)

### Steps

- [ ] 10.1 Update `internal/api/router.go` to accept auth and community dependencies and mount all new routes.

The new signature and auth group (replaces the existing `NewRouter` function body):

```go
// internal/api/router.go  — full updated file
package api

import (
	"github.com/cyclist-map/cyclist-map/internal/app"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

// NewRouter creates the chi router with all API routes wired up.
func NewRouter(
	routeSvc *app.RouteService,
	discoverySvc *app.DiscoveryService,
	venueSvc *app.VenueService,
	routingH *RoutingHandler,
	weatherH *WeatherHandler,
	conditionsH *ConditionsHandler,
	previewH *PreviewHandler,
	authSvc *app.AuthService,
	communityH *CommunityHandler,
) *chi.Mux {
	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	rh := &RouteHandler{svc: routeSvc}
	dh := NewDiscoveryHandler(discoverySvc)
	vh := NewVenueHandler(venueSvc)
	authH := NewAuthHandler(authSvc)

	r.Route("/api/v1", func(r chi.Router) {
		// Routes
		r.Get("/routes", rh.List)
		r.Post("/routes", rh.Create)
		r.Get("/routes/{id}", rh.Get)
		r.Patch("/routes/{id}", rh.Update)
		r.Delete("/routes/{id}", rh.Archive)
		r.Get("/routes/{id}/conditions", conditionsH.RouteConditions)

		// Discovery
		r.Get("/discover/nearby", dh.Nearby)
		r.Get("/discover/viewport", dh.Viewport)
		r.Get("/discover/suggested", dh.Suggested)

		// Venues
		r.Get("/venues/along-route", vh.AlongRoute)
		r.Get("/venues/tags", vh.Tags)

		// Routing
		r.Post("/routing/directions", routingH.Directions)
		r.Get("/routing/conditions/preview", previewH.ConditionsPreview)

		// Weather
		r.Get("/weather/point", weatherH.AtPoint)

		// Auth (public)
		r.Post("/auth/register", authH.Register)
		r.Post("/auth/login", authH.Login)
		r.Post("/auth/refresh", authH.Refresh)

		// Community — mixed auth
		// Public reads
		r.Get("/routes/{id}/reviews", communityH.ListReviews)
		r.Get("/users/{id}/ride-logs", communityH.ListUserRideLogs)

		// Protected writes
		r.Group(func(r chi.Router) {
			r.Use(AuthMiddleware(authSvc))
			r.Post("/contributions", communityH.SubmitContribution)
			r.Post("/routes/{id}/reviews", communityH.AddReview)
			r.Post("/routes/{id}/ride-logs", communityH.LogRide)
		})
	})

	return r
}
```

- [ ] 10.2 Update `cmd/api/main.go` to wire `UserRepo`, `AuthService`, community repos, `CommunityService`, and `CommunityHandler`, then pass them to `NewRouter`.

Add after the existing repo wiring (after `weatherRepo`):

```go
// cmd/api/main.go — additions to the wiring section

jwtSecret := os.Getenv("JWT_SECRET")
if jwtSecret == "" {
    log.Fatal("JWT_SECRET environment variable is required")
}

userRepo := postgres.NewUserRepo(pool)
authSvc, err := app.NewAuthService(userRepo, jwtSecret, 15*time.Minute, 7*24*time.Hour)
if err != nil {
    log.Fatalf("failed to create auth service: %v", err)
}

contribRepo := postgres.NewContributionRepo(pool)
reviewRepo := postgres.NewReviewRepo(pool)
rideLogRepo := postgres.NewRideLogRepo(pool)
communitySvc := app.NewCommunityService(contribRepo, reviewRepo, rideLogRepo)
communityHandler := api.NewCommunityHandler(communitySvc)
```

Then update the `NewRouter` call to pass the two new arguments at the end:

```go
router := api.NewRouter(
    routeSvc, discoverySvc, venueSvc,
    routingHandler, weatherHandler, conditionsHandler, previewHandler,
    authSvc, communityHandler,
)
```

- [ ] 10.3 Verify the binary compiles cleanly.

```bash
go build ./cmd/api/
```

- [ ] 10.4 Run the full test suite.

```bash
go test ./...
```

No failing tests (integration tests may be skipped if DB is unreachable — that is fine).

- [ ] 10.5 Commit: `feat(api,cmd): wire auth and community into router and main`

---

## Task 11 — Integration Smoke Test (Manual)

Run the server locally and exercise each endpoint with curl to confirm end-to-end wiring.

### Steps

- [ ] 11.1 Start the server.

```bash
DATABASE_URL="postgres://osm_dev:osm_dev@localhost:5432/cyclist_map_dev?sslmode=disable" \
JWT_SECRET="dev-secret-32-chars-padding-here" \
  go run ./cmd/api/
```

- [ ] 11.2 Register a user.

```bash
curl -s -X POST http://localhost:8080/api/v1/auth/register \
  -H 'Content-Type: application/json' \
  -d '{"display_name":"Test Rider","email":"rider@test.dev","password":"password123"}' | jq .
# Expect: {"id":"...","display_name":"Test Rider","email":"rider@test.dev","created_at":"..."}
```

- [ ] 11.3 Login and capture the access token.

```bash
TOKEN=$(curl -s -X POST http://localhost:8080/api/v1/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"email":"rider@test.dev","password":"password123"}' | jq -r .access_token)
echo "TOKEN=$TOKEN"
```

- [ ] 11.4 Submit a contribution (authenticated).

```bash
curl -s -X POST http://localhost:8080/api/v1/contributions \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"geometry":[[139.6917,35.6895,10],[139.7,35.695,20]],"metadata":{"surface":"paved"}}' | jq .
# Expect: {"id":"...","status":"pending",...}
```

- [ ] 11.5 Add a review (get a route ID from the seed data first).

```bash
ROUTE_ID=$(curl -s http://localhost:8080/api/v1/routes | jq -r '.routes[0].id')
curl -s -X POST "http://localhost:8080/api/v1/routes/$ROUTE_ID/reviews" \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"rating":4,"body":"Great route!"}' | jq .
# Expect: {"id":"...","rating":4,...}
```

- [ ] 11.6 List reviews (public).

```bash
curl -s "http://localhost:8080/api/v1/routes/$ROUTE_ID/reviews" | jq .
# Expect: {"reviews":[{"id":"...","rating":4,...}]}
```

- [ ] 11.7 Log a ride (authenticated).

```bash
curl -s -X POST "http://localhost:8080/api/v1/routes/$ROUTE_ID/ride-logs" \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d "{\"ridden_at\":$(date +%s),\"duration_s\":3600}" | jq .
# Expect: {"id":"...","duration_s":3600,...}
```

- [ ] 11.8 Fetch ride history (public).

```bash
USER_ID=$(curl -s -X POST http://localhost:8080/api/v1/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"email":"rider@test.dev","password":"password123"}' | jq -r .user_id // empty — not in token response)
# user_id is not in the token response; get it from the register response stored earlier.
# For the smoke test, query the DB directly:
psql "$DATABASE_URL" -c "SELECT id FROM community.user WHERE email='rider@test.dev';"
# Then:
curl -s "http://localhost:8080/api/v1/users/<USER_ID>/ride-logs" | jq .
```

- [ ] 11.9 Verify that a request without a token to a protected endpoint returns 401.

```bash
curl -s -X POST http://localhost:8080/api/v1/contributions \
  -H 'Content-Type: application/json' \
  -d '{"geometry":[[139.69,35.69,10]]}' | jq .
# Expect: {"error":"authorization required"} with HTTP 401
```

- [ ] 11.10 Refresh the token.

```bash
REFRESH=$(curl -s -X POST http://localhost:8080/api/v1/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"email":"rider@test.dev","password":"password123"}' | jq -r .refresh_token)
curl -s -X POST http://localhost:8080/api/v1/auth/refresh \
  -H 'Content-Type: application/json' \
  -d "{\"refresh_token\":\"$REFRESH\"}" | jq .
# Expect: {"access_token":"...","refresh_token":"...","expires_in":900}
```

- [ ] 11.11 Commit: `test(smoke): verify auth + community endpoints end-to-end`

---

## Self-Review Checklist

Before marking this phase complete, verify each item:

- [ ] Migration `000017` applied and `password_hash` column confirmed in `community.user`.
- [ ] `go.mod` contains `github.com/golang-jwt/jwt/v5` and `golang.org/x/crypto`.
- [ ] `go test ./internal/domain/community/` — all existing domain tests still pass (no regressions).
- [ ] `go test ./internal/app/` — AuthService and CommunityService unit tests all pass.
- [ ] `go test ./internal/infra/postgres/ -run TestUserRepo` — all pass against dev DB.
- [ ] `go test ./internal/infra/postgres/ -run TestContributionRepo` — all pass.
- [ ] `go test ./internal/infra/postgres/ -run TestReviewRepo` — all pass.
- [ ] `go test ./internal/infra/postgres/ -run TestRideLogRepo` — all pass.
- [ ] `go test ./internal/api/ -run TestAuthMiddleware` — all pass.
- [ ] `go test ./internal/api/ -run TestAuthHandler` — all pass.
- [ ] `go test ./internal/api/ -run TestCommunityHandler` — all pass.
- [ ] `go build ./cmd/api/` — compiles without errors or warnings.
- [ ] `go vet ./...` — clean.
- [ ] All protected endpoints return 401 without a valid Bearer token.
- [ ] Login with wrong password returns 401 (no oracle attack surface).
- [ ] Smoke test steps 11.2–11.10 all produce expected responses.
- [ ] No placeholder strings, TODO comments, or unimplemented stubs left in production code.

---

## Key Design Decisions

**No `GetByEmail` / `SetPasswordHash` on the domain `UserRepository` interface** — These are auth-infrastructure concerns, not domain concerns. The `app.AuthUserRepo` interface extends `community.UserRepository` only in `internal/app/`, keeping the domain clean.

**Schema mismatch bridging without touching domain types** — The `RideLogRepo` converts `started_at`/`finished_at` TIMESTAMPTZ columns to/from `RiddenAt` (unix int) and `DurationS` (int). The `ContributionRepo` stores `ModeratorNotes` inside the `metadata` JSONB column since the current schema has no dedicated column. Both are transparent to the domain layer.

**`isUniqueViolation` string inspection** — Avoids importing `pgconn` at the package level just for one error check. Replace with `errors.As(err, &pgconn.PgError{})` if `pgconn` is already a transitive import (it is, via pgx v5).

**`ExportedCtxUserID` for handler tests** — The context key type is unexported (`contextKey`), which prevents accidental cross-package context key collisions. Tests need to inject a user ID without going through the full JWT stack; exporting the key value (not the type) via a `var ExportedCtxUserID = ctxUserID` in `auth_middleware.go` gives tests a stable handle. An alternative is a `WithTestUserID(ctx, id)` helper gated by a build tag — either is acceptable.

**`JWT_SECRET` required at startup** — Fails fast rather than defaulting to an insecure value. Operators must set this environment variable; the dev secret used in smoke tests must not appear in production.
