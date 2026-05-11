package postgres

import (
	"context"
	"errors"
	"fmt"

	"komorebi/internal/app"
	"komorebi/internal/domain/community"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
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
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code == "23505"
	}
	return false
}
