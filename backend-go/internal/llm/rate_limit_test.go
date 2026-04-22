package llm

import (
	"context"
	"testing"
	"time"
)

type rlStub struct{ calls int }

func (s *rlStub) Name() string             { return "rl-stub" }
func (s *rlStub) Supports(_ Modality) bool { return true }
func (s *rlStub) Generate(_ context.Context, _ GenerateRequest) ([]Candidate, error) {
	s.calls++
	return []Candidate{{Modality: ModalityText, Text: "ok"}}, nil
}

func TestRateLimitedProvider_NoLimitWhenRPSZero(t *testing.T) {
	stub := &rlStub{}
	p := NewRateLimitedProvider(stub, 0, 0)
	if _, ok := p.(*RateLimitedProvider); ok {
		t.Fatalf("rps=0 should bypass limiter, got wrapped")
	}
}

func TestRateLimitedProvider_AllowsBurst(t *testing.T) {
	stub := &rlStub{}
	p := NewRateLimitedProvider(stub, 1, 3) // 1 rps, burst 3

	start := time.Now()
	for i := 0; i < 3; i++ {
		if _, err := p.Generate(context.Background(), GenerateRequest{Modality: ModalityText}); err != nil {
			t.Fatalf("call %d: %v", i, err)
		}
	}
	if d := time.Since(start); d > 100*time.Millisecond {
		t.Fatalf("burst calls should not block, took %s", d)
	}
}

func TestRateLimitedProvider_BlocksOnExhaust(t *testing.T) {
	stub := &rlStub{}
	p := NewRateLimitedProvider(stub, 50, 1) // 50 rps, burst 1（每 20ms 一个令牌）

	// 消耗 burst
	if _, err := p.Generate(context.Background(), GenerateRequest{Modality: ModalityText}); err != nil {
		t.Fatalf("first: %v", err)
	}
	// 第二次应等 ~20ms
	start := time.Now()
	if _, err := p.Generate(context.Background(), GenerateRequest{Modality: ModalityText}); err != nil {
		t.Fatalf("second: %v", err)
	}
	if d := time.Since(start); d < 10*time.Millisecond {
		t.Fatalf("second call should wait, took %s", d)
	}
}

func TestRateLimitedProvider_CtxCancelReturnsError(t *testing.T) {
	stub := &rlStub{}
	p := NewRateLimitedProvider(stub, 0.01, 1) // 100s/token
	// 消耗 burst
	_, _ = p.Generate(context.Background(), GenerateRequest{Modality: ModalityText})

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	_, err := p.Generate(ctx, GenerateRequest{Modality: ModalityText})
	if err == nil {
		t.Fatal("expected ctx cancel error")
	}
}
