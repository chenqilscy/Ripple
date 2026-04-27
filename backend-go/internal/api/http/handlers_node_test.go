package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/chenqilscy/ripple/backend-go/internal/domain"
)

func TestNodeHandlers_Create_RejectsOversizedBody(t *testing.T) {
	h := &NodeHandlers{}

	bigContent := strings.Repeat("x", 300*1024)
	body := `{"lake_id":"lake-1","content":"` + bigContent + `","type":"TEXT"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/nodes", strings.NewReader(body))
	req = req.WithContext(context.WithValue(req.Context(), ctxUserKey, &domain.User{ID: "u-1"}))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	h.Create(rr, req)

	if rr.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("want 413, got %d", rr.Code)
	}
	var out map[string]string
	if err := json.Unmarshal(rr.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if out["error"] != "request body too large" {
		t.Fatalf("unexpected error message: %q", out["error"])
	}
}
