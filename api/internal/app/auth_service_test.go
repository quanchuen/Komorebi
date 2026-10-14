package app_test

import (
	"errors"
	"testing"
	"time"

	"komorebi/internal/app"
	"komorebi/internal/domain/community"
)

// --- stub repo ---

type stubUserRepo struct {
	users   map[string]*community.User
	byEmail map[string]*community.User
	hashes  map[string]string // userID → password_hash
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
