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

	// ErrNotFound is a generic "resource not found" sentinel used by repos.
	ErrNotFound = errors.New("not found")
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
	repo       AuthUserRepo
	secret     []byte
	accessTTL  time.Duration
	refreshTTL time.Duration
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
