package llm

import (
	"context"
	"errors"
	"sync"
	"testing"
)

type stubProvider struct {
	name      string
	supports  Modality
	cands     []Candidate
	err       error
	calls     int
	mu        sync.Mutex
}

func (p *stubProvider) Name() string { return p.name }
func (p *stubProvider) Supports(m Modality) bool {
	return p.supports == "" || p.supports == m
}
func (p *stubProvider) Generate(_ context.Context, _ GenerateRequest) ([]Candidate, error) {
	p.mu.Lock()
	p.calls++
	p.mu.Unlock()
	if p.err != nil {
		return nil, p.err
	}
	return p.cands, nil
}

type captureRecorder struct {
	mu   sync.Mutex
	recs []CallRecord
}

func (r *captureRecorder) Record(rec CallRecord) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.recs = append(r.recs, rec)
}

func TestRouter_PicksFirstSupporting(t *testing.T) {
	p1 := &stubProvider{name: "img-only", supports: ModalityImage}
	p2 := &stubProvider{name: "txt", supports: ModalityText, cands: []Candidate{{Text: "hi"}}}
	r := NewDefaultRouter([]Provider{p1, p2}, Policy{}, nil)
	out, err := r.Generate(context.Background(), GenerateRequest{Modality: ModalityText, N: 1})
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 1 || out[0].Text != "hi" {
		t.Fatalf("unexpected: %+v", out)
	}
	if p1.calls != 0 || p2.calls != 1 {
		t.Fatalf("expected only p2 to be called, p1=%d p2=%d", p1.calls, p2.calls)
	}
}

func TestRouter_FallbackOff_FailsImmediately(t *testing.T) {
	p1 := &stubProvider{name: "a", err: errors.New("boom")}
	p2 := &stubProvider{name: "b", cands: []Candidate{{Text: "x"}}}
	r := NewDefaultRouter([]Provider{p1, p2}, Policy{EnableFallback: false}, nil)
	_, err := r.Generate(context.Background(), GenerateRequest{Modality: ModalityText})
	if err == nil {
		t.Fatal("expected error")
	}
	if p2.calls != 0 {
		t.Fatalf("p2 should not be called when fallback off")
	}
}

func TestRouter_FallbackOn_TriesNext(t *testing.T) {
	p1 := &stubProvider{name: "a", err: errors.New("boom")}
	p2 := &stubProvider{name: "b", cands: []Candidate{{Text: "ok"}}}
	r := NewDefaultRouter([]Provider{p1, p2}, Policy{EnableFallback: true}, nil)
	out, err := r.Generate(context.Background(), GenerateRequest{Modality: ModalityText})
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 1 || out[0].Text != "ok" {
		t.Fatalf("unexpected: %+v", out)
	}
}

func TestRouter_RecordsBothOkAndError(t *testing.T) {
	p1 := &stubProvider{name: "a", err: errors.New("boom")}
	p2 := &stubProvider{name: "b", cands: []Candidate{{Text: "ok"}}}
	rec := &captureRecorder{}
	r := NewDefaultRouter([]Provider{p1, p2}, Policy{EnableFallback: true}, rec)
	_, err := r.Generate(context.Background(), GenerateRequest{Modality: ModalityText, Prompt: "hello"})
	if err != nil {
		t.Fatal(err)
	}
	if len(rec.recs) != 2 {
		t.Fatalf("want 2 records, got %d", len(rec.recs))
	}
	if rec.recs[0].Status != "error" || rec.recs[1].Status != "ok" {
		t.Fatalf("unexpected statuses: %+v", rec.recs)
	}
	if rec.recs[0].PromptHash == "" || rec.recs[0].PromptHash != rec.recs[1].PromptHash {
		t.Fatalf("prompt hash should be stable + non-empty")
	}
}

func TestRouter_NoSupportingProvider(t *testing.T) {
	p1 := &stubProvider{name: "img", supports: ModalityImage}
	r := NewDefaultRouter([]Provider{p1}, Policy{}, nil)
	_, err := r.Generate(context.Background(), GenerateRequest{Modality: ModalityText})
	if err == nil {
		t.Fatal("expected error")
	}
}
