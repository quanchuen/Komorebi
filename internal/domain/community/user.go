package community

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"time"
)

// ErrEmptyDisplayName is returned when a display name is blank.
var ErrEmptyDisplayName = errors.New("community: display name must not be empty")

// User represents a registered cyclist.
type User struct {
	ID          string
	DisplayName string
	Email       string
	AvatarURL   string
	CreatedAt   time.Time
}

// NewUser creates a new User with a generated ID.
func NewUser(displayName, email string) (*User, error) {
	if displayName == "" {
		return nil, ErrEmptyDisplayName
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
