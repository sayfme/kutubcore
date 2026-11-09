package handler

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHealth(t *testing.T) {
	h := &Handler{}
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()
	h.Health(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, `"success":true`) {
		t.Fatalf("body = %s, want success:true", body)
	}
	if !strings.Contains(body, `"status":"ok"`) {
		t.Fatalf("body = %s, want status ok", body)
	}
}
