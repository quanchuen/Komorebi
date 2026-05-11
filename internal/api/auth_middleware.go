package api

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"komorebi/internal/app"
)

type contextKey string

const ctxUserID contextKey = "userID"

// ExportedCtxUserID is the context key for the authenticated user ID.
// Exported so test packages can inject user IDs directly into context.
var ExportedCtxUserID = ctxUserID

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
