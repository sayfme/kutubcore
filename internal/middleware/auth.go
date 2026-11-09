package middleware

import (
	"context"
	"net/http"

	"kutubcore/internal/domain/user"
)

// Authenticator resolves a session token to a user.
type Authenticator interface {
	Authenticate(ctx context.Context, token string) (*user.User, error)
}

type ctxKey struct{}

// WithUser stores the user in the request context.
func WithUser(ctx context.Context, u *user.User) context.Context {
	return context.WithValue(ctx, ctxKey{}, u)
}

// UserFrom returns the authenticated user, or nil.
func UserFrom(ctx context.Context) *user.User {
	u, _ := ctx.Value(ctxKey{}).(*user.User)
	return u
}

const cookieName = "kutubcore_session"

func sessionToken(r *http.Request) string {
	c, err := r.Cookie(cookieName)
	if err != nil {
		return ""
	}
	return c.Value
}

// SessionAuth is an optional-auth middleware: it sets the user in context
// when a valid cookie is present, but never rejects (guests allowed).
func SessionAuth(auth Authenticator) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if tok := sessionToken(r); tok != "" {
				if u, err := auth.Authenticate(r.Context(), tok); err == nil {
					r = r.WithContext(WithUser(r.Context(), u))
				}
			}
			next.ServeHTTP(w, r)
		})
	}
}

// RequireAuth rejects unauthenticated requests with 401 (Blueprint §9).
func RequireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if UserFrom(r.Context()) == nil {
			writeError(w, http.StatusUnauthorized, "UNAUTHENTICATED", "Authentication required.")
			return
		}
		next.ServeHTTP(w, r)
	})
}

// RequireOrganizer rejects non-organizers with 403.
func RequireOrganizer(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u := UserFrom(r.Context())
		if u == nil {
			writeError(w, http.StatusUnauthorized, "UNAUTHENTICATED", "Authentication required.")
			return
		}
		for _, role := range u.Roles {
			if role == user.RoleOrganizer {
				next.ServeHTTP(w, r)
				return
			}
		}
		writeError(w, http.StatusForbidden, "FORBIDDEN", "Organizer role required.")
	})
}

// Cookie helpers (Blueprint §14: HttpOnly, SameSite, Secure in prod).
func SetSessionCookie(w http.ResponseWriter, token string, maxAge int, secure bool) {
	http.SetCookie(w, &http.Cookie{
		Name:     cookieName,
		Value:    token,
		Path:     "/",
		MaxAge:   maxAge,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   secure,
	})
}

func ClearSessionCookie(w http.ResponseWriter, secure bool) {
	SetSessionCookie(w, "", -1, secure)
}
