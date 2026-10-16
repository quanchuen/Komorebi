package community

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"strings"
	"time"
)

// Validation errors for NewUser.
var (
	ErrEmptyDisplayName = errors.New("community: display name must not be empty")
	ErrEmptyEmail       = errors.New("community: email must not be empty")
)

// User represents a registered cyclist.
type User struct {
	ID          string
	DisplayName string
	Email       string
	AvatarURL   string
	CreatedAt   time.Time
}

// NewUser creates a new User with a generated ID. Email is normalised
// (trimmed, lower-cased) so the unique index catches case variants.
func NewUser(displayName, email string) (*User, error) {
	if displayName == "" {
		return nil, ErrEmptyDisplayName
	}
	email = strings.ToLower(strings.TrimSpace(email))
	if email == "" {
		return nil, ErrEmptyEmail
	}
	return &User{
		ID:          newID(),
		DisplayName: displayName,
		Email:       email,
		CreatedAt:   time.Now().UTC(),
	}, nil
}

// newID generates a random UUID-like string using crypto/rand.
func newID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return hex.EncodeToString(b[:4]) + "-" +
		hex.EncodeToString(b[4:6]) + "-" +
		hex.EncodeToString(b[6:8]) + "-" +
		hex.EncodeToString(b[8:10]) + "-" +
		hex.EncodeToString(b[10:])
}
