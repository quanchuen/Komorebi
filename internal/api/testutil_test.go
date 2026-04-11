package api_test

import (
	"github.com/cyclist-map/cyclist-map/internal/app"
	"github.com/cyclist-map/cyclist-map/internal/domain/community"
)

// stubAuthRepo is an in-memory implementation of app.AuthUserRepo for tests.
type stubAuthRepo struct {
	users   map[string]*community.User
	byEmail map[string]*community.User
	hashes  map[string]string
}

func newStubAuthRepo() *stubAuthRepo {
	return &stubAuthRepo{
		users:   make(map[string]*community.User),
		byEmail: make(map[string]*community.User),
		hashes:  make(map[string]string),
	}
}

func (s *stubAuthRepo) Create(u *community.User) error {
	if _, exists := s.byEmail[u.Email]; exists {
		return app.ErrEmailTaken
	}
	s.users[u.ID] = u
	s.byEmail[u.Email] = u
	return nil
}

func (s *stubAuthRepo) GetByID(id string) (*community.User, error) {
	u, ok := s.users[id]
	if !ok {
		return nil, app.ErrUserNotFound
	}
	return u, nil
}

func (s *stubAuthRepo) GetByEmail(email string) (*community.User, error) {
	u, ok := s.byEmail[email]
	if !ok {
		return nil, app.ErrUserNotFound
	}
	return u, nil
}

func (s *stubAuthRepo) SetPasswordHash(userID, hash string) error {
	s.hashes[userID] = hash
	return nil
}

func (s *stubAuthRepo) GetPasswordHash(userID string) (string, error) {
	h, ok := s.hashes[userID]
	if !ok {
		return "", app.ErrUserNotFound
	}
	return h, nil
}

func (s *stubAuthRepo) Update(u *community.User) error {
	s.users[u.ID] = u
	return nil
}

func (s *stubAuthRepo) Delete(id string) error {
	u, ok := s.users[id]
	if !ok {
		return app.ErrUserNotFound
	}
	delete(s.byEmail, u.Email)
	delete(s.users, id)
	return nil
}
