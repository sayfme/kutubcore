package handler_test

import (
	"context"
	"encoding/json"
	"testing"

	"kutubcore/internal/database/sqlite"
	"kutubcore/internal/service"
	"kutubcore/internal/testutil"
)

// Forgot -> reset with a real token from the service layer, single-use,
// sessions invalidated. Unknown identifier still returns 200 (no enumeration).
func TestAccountFlow_ForgotResetVerify(t *testing.T) {
	db := testutil.NewDB(t)
	defer db.Close()
	application := testutil.NewApp(db)
	h := application.Handler

	rec := doJSON(t, h, "POST", "/api/v1/auth/register", map[string]any{
		"display_name": "Acc", "email": "acc@example.com", "password": "oldpassword1",
	}, "")
	if rec.Code != 201 {
		t.Fatalf("register: %d %s", rec.Code, rec.Body.String())
	}
	cookie := cookieOf(rec)

	// Forgot for existing: 200 + sent:true.
	rec = doJSON(t, h, "POST", "/api/v1/auth/forgot-password", map[string]any{"email": "acc@example.com"}, "")
	if rec.Code != 200 {
		t.Fatalf("forgot: %d %s", rec.Code, rec.Body.String())
	}
	// Forgot for unknown: still 200.
	rec = doJSON(t, h, "POST", "/api/v1/auth/forgot-password", map[string]any{"email": "nobody@example.com"}, "")
	if rec.Code != 200 {
		t.Fatalf("forgot unknown: %d %s, want 200 (no enumeration)", rec.Code, rec.Body.String())
	}

	// Get a real token via the service (delivery channel in prod).
	acct := &service.AccountService{
		Users:    sqlite.NewUserRepo(db),
		Tokens:   sqlite.NewTokenRepo(db),
		Sessions: sqlite.NewSessionRepo(db),
	}
	token, err := acct.ForgotPassword(context.Background(), "acc@example.com")
	if err != nil || token == "" {
		t.Fatalf("service forgot: token=%q err=%v", token, err)
	}
	rec = doJSON(t, h, "POST", "/api/v1/auth/reset-password", map[string]any{"token": token, "new_password": "newpassword2"}, "")
	if rec.Code != 200 {
		t.Fatalf("reset: %d %s", rec.Code, rec.Body.String())
	}
	// Token single-use.
	rec = doJSON(t, h, "POST", "/api/v1/auth/reset-password", map[string]any{"token": token, "new_password": "anotherpass1"}, "")
	if rec.Code != 422 {
		t.Fatalf("reset reuse: %d %s, want 422", rec.Code, rec.Body.String())
	}
	// Old session invalidated by reset: /auth/me now 401.
	rec = doJSON(t, h, "GET", "/api/v1/auth/me", nil, cookie)
	if rec.Code != 401 {
		t.Fatalf("old session after reset: %d %s, want 401", rec.Code, rec.Body.String())
	}
	// Login with new password works, old fails.
	rec = doJSON(t, h, "POST", "/api/v1/auth/login", map[string]any{"email": "acc@example.com", "password": "oldpassword1"}, "")
	if rec.Code != 401 {
		t.Fatalf("old password login: %d %s, want 401", rec.Code, rec.Body.String())
	}
	rec = doJSON(t, h, "POST", "/api/v1/auth/login", map[string]any{"email": "acc@example.com", "password": "newpassword2"}, "")
	if rec.Code != 200 {
		t.Fatalf("new password login: %d %s", rec.Code, rec.Body.String())
	}

	// Email verification.
	u, err := sqlite.NewUserRepo(db).GetByEmail(context.Background(), "acc@example.com")
	if err != nil {
		t.Fatal(err)
	}
	vt, err := acct.CreateVerification(context.Background(), u.ID)
	if err != nil || vt == "" {
		t.Fatalf("create verification: token=%q err=%v", vt, err)
	}
	rec = doJSON(t, h, "POST", "/api/v1/auth/verify-email", map[string]any{"token": vt}, "")
	if rec.Code != 200 {
		t.Fatalf("verify: %d %s", rec.Code, rec.Body.String())
	}
	// Fresh login sees email_verified=true.
	rec = doJSON(t, h, "POST", "/api/v1/auth/login", map[string]any{"email": "acc@example.com", "password": "newpassword2"}, "")
	cookie2 := cookieOf(rec)
	var me struct {
		Data struct {
			EmailVerified bool `json:"email_verified"`
		} `json:"data"`
	}
	rec = doJSON(t, h, "GET", "/api/v1/auth/me", nil, cookie2)
	_ = json.Unmarshal(rec.Body.Bytes(), &me)
	if !me.Data.EmailVerified {
		t.Fatalf("/me email_verified = false, want true: %s", rec.Body.String())
	}
}
