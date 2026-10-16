package postgres

import (
	"errors"
	"fmt"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"komorebi/internal/app"
)

func TestIsNotFound(t *testing.T) {
	if !errors.Is(ErrNotFound, app.ErrNotFound) {
		t.Fatal("postgres.ErrNotFound must be app.ErrNotFound so services map it to 404")
	}
	cases := map[error]bool{
		pgx.ErrNoRows:                            true,
		fmt.Errorf("wrapped: %w", pgx.ErrNoRows): true,
		&pgconn.PgError{Code: "22P02"}:           true, // non-UUID id
		&pgconn.PgError{Code: "23505"}:           false,
		errors.New("other"):                      false,
	}
	for err, want := range cases {
		if got := isNotFound(err); got != want {
			t.Errorf("isNotFound(%v) = %v, want %v", err, got, want)
		}
	}
}
