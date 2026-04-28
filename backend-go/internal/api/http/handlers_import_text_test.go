package httpapi

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/chenqilscy/ripple/backend-go/internal/domain"
	"github.com/chenqilscy/ripple/backend-go/internal/llm"
	"github.com/go-chi/chi/v5"
)

// ── stub LLM router ──────────────────────────────────────────────────────────

type itStubRouter struct {
	resp string
	err  error
}

func (s *itStubRouter) Generate(_ context.Context, _ llm.GenerateRequest) ([]llm.Candidate, error) {
	if s.err != nil {
		return nil, s.err
	}
	return []llm.Candidate{{Modality: llm.ModalityText, Text: s.resp}}, nil
}

// capturePromptRouter 代理 llm.Router，记录每次 Generate 请求。
type capturePromptRouter struct {
	inner    llm.Router
	captured llm.GenerateRequest
}

func (c *capturePromptRouter) Generate(ctx context.Context, req llm.GenerateRequest) ([]llm.Candidate, error) {
	c.captured = req
	return c.inner.Generate(ctx, req)
}

// ── chi context helper ─────────────────────────────────────────────────────

func itWithLakeID(r *http.Request, id string) *http.Request {
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", id)
	ctx := context.WithValue(r.Context(), chi.RouteCtxKey, rctx)
	ctx = context.WithValue(ctx, ctxUserKey, &domain.User{ID: "u-test"})
	return r.WithContext(ctx)
}

// ── unit tests ────────────────────────────────────────────────────────────────

func TestStripMarkdownCodeFence(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{`{"nodes":[]}`, `{"nodes":[]}`},
		{"```json\n{\"nodes\":[]}\n```", `{"nodes":[]}`},
		{"```\n{\"nodes\":[]}\n```", `{"nodes":[]}`},
	}
	for _, c := range cases {
		got := stripMarkdownCodeFence(c.in)
		if got != c.want {
			t.Errorf("stripMarkdownCodeFence(%q) = %q; want %q", c.in, got, c.want)
		}
	}
}

func TestParseEdgeKind(t *testing.T) {
	cases := map[string]domain.EdgeKind{
		"relates": domain.EdgeKindRelates,
		"derives": domain.EdgeKindDerives,
		"opposes": domain.EdgeKindOpposes,
		"refines": domain.EdgeKindRefines,
		"groups":  domain.EdgeKindGroups,
		"unknown": domain.EdgeKindRelates,
		"DERIVES": domain.EdgeKindDerives,
		"":        domain.EdgeKindRelates,
	}
	for input, want := range cases {
		got := parseEdgeKind(input)
		if got != want {
			t.Errorf("parseEdgeKind(%q) = %q; want %q", input, got, want)
		}
	}
}

func TestImportText_MissingText(t *testing.T) {
	h := &ImportTextHandlers{
		Router: &itStubRouter{resp: `{"nodes":[],"edges":[]}`},
	}
	r := httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString(`{"text":"  "}`))
	r = itWithLakeID(r, "lake-1")
	w := httptest.NewRecorder()
	h.ImportText(w, r)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d body=%s", w.Code, w.Body.String())
	}
}

func TestImportText_LLMError(t *testing.T) {
	h := &ImportTextHandlers{
		Router: &itStubRouter{err: errors.New("llm down")},
	}
	r := httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString(`{"text":"hello world","max_nodes":5}`))
	r = itWithLakeID(r, "lake-1")
	w := httptest.NewRecorder()
	h.ImportText(w, r)
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d body=%s", w.Code, w.Body.String())
	}
}

func TestImportText_InvalidJSON(t *testing.T) {
	h := &ImportTextHandlers{
		Router: &itStubRouter{resp: "not json at all"},
	}
	r := httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString(`{"text":"hello","max_nodes":5}`))
	r = itWithLakeID(r, "lake-1")
	w := httptest.NewRecorder()
	h.ImportText(w, r)
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d body=%s", w.Code, w.Body.String())
	}
}

func TestImportText_MaxNodesCap(t *testing.T) {
	cp := &capturePromptRouter{inner: &itStubRouter{resp: `{"nodes":[],"edges":[]}`}}
	h := &ImportTextHandlers{Router: cp}
	r := httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString(`{"text":"some content","max_nodes":999}`))
	r = itWithLakeID(r, "lake-1")
	w := httptest.NewRecorder()
	h.ImportText(w, r)
	if !strings.Contains(cp.captured.Prompt, "50") {
		t.Errorf("expected cap 50 in prompt, got: %.200s", cp.captured.Prompt)
	}
	if strings.Contains(cp.captured.Prompt, "999") {
		t.Error("prompt should not contain uncapped max_nodes 999")
	}
}

func TestImportText_TextTruncation(t *testing.T) {
	longText := strings.Repeat("a", maxImportTextRunes+100)
	cp := &capturePromptRouter{inner: &itStubRouter{resp: `{"nodes":[],"edges":[]}`}}
	h := &ImportTextHandlers{Router: cp}
	body := `{"text":"` + longText + `","max_nodes":5}`
	r := httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString(body))
	r = itWithLakeID(r, "lake-1")
	w := httptest.NewRecorder()
	h.ImportText(w, r)
	userText := itExtractBetween(cp.captured.Prompt, "<text>", "</text>")
	if len([]rune(userText)) > maxImportTextRunes {
		t.Errorf("prompt text not truncated: rune count = %d (max %d)", len([]rune(userText)), maxImportTextRunes)
	}
}

func itExtractBetween(s, open, close string) string {
	start := strings.LastIndex(s, open)
	if start == -1 {
		return ""
	}
	start += len(open)
	end := strings.Index(s[start:], close)
	if end == -1 {
		return s[start:]
	}
	return s[start : start+end]
}
