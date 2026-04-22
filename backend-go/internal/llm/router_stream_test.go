package llm

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

// rsStreamStub 实现 Provider + StreamProvider
type rsStreamStub struct {
	name      string
	supports  Modality
	chunks    []string
	startErr  error
	chunkErr  error
	cost      int64
	streamRet <-chan StreamChunk
}

func (s *rsStreamStub) Name() string                { return s.name }
func (s *rsStreamStub) Supports(m Modality) bool    { return m == s.supports }
func (s *rsStreamStub) Generate(_ context.Context, _ GenerateRequest) ([]Candidate, error) {
	return []Candidate{{Modality: s.supports, Text: strings.Join(s.chunks, ""), CostTokens: s.cost}}, nil
}
func (s *rsStreamStub) GenerateStream(_ context.Context, _ GenerateRequest) (<-chan StreamChunk, error) {
	if s.startErr != nil {
		return nil, s.startErr
	}
	if s.streamRet != nil {
		return s.streamRet, nil
	}
	out := make(chan StreamChunk, len(s.chunks)+2)
	go func() {
		defer close(out)
		for _, c := range s.chunks {
			out <- StreamChunk{Delta: c}
		}
		if s.chunkErr != nil {
			out <- StreamChunk{Err: s.chunkErr}
			return
		}
		if s.cost > 0 {
			out <- StreamChunk{CostTokens: s.cost}
		}
		out <- StreamChunk{Done: true}
	}()
	return out, nil
}

// rsNoStreamStub 只实现 Provider（没有 GenerateStream）
type rsNoStreamStub struct {
	name     string
	supports Modality
	text     string
	err      error
}

func (s *rsNoStreamStub) Name() string             { return s.name }
func (s *rsNoStreamStub) Supports(m Modality) bool { return m == s.supports }
func (s *rsNoStreamStub) Generate(_ context.Context, _ GenerateRequest) ([]Candidate, error) {
	if s.err != nil {
		return nil, s.err
	}
	return []Candidate{{Modality: s.supports, Text: s.text}}, nil
}

func TestRouterStream_HappyPath(t *testing.T) {
	p := &rsStreamStub{name: "p1", supports: ModalityText, chunks: []string{"Hel", "lo"}, cost: 12}
	r := NewDefaultRouter([]Provider{p}, Policy{}, nil)
	ch, err := r.GenerateStream(context.Background(), GenerateRequest{Prompt: "x", Modality: ModalityText, N: 1})
	if err != nil {
		t.Fatal(err)
	}
	var sb strings.Builder
	var cost int64
	done := false
	for c := range ch {
		if c.Err != nil {
			t.Fatalf("unexpected err: %v", c.Err)
		}
		sb.WriteString(c.Delta)
		if c.CostTokens > 0 {
			cost = c.CostTokens
		}
		if c.Done {
			done = true
		}
	}
	if sb.String() != "Hello" || cost != 12 || !done {
		t.Fatalf("got %q cost=%d done=%v", sb.String(), cost, done)
	}
}

func TestRouterStream_FallbackToNonStream(t *testing.T) {
	// 唯一 provider 不实现 StreamProvider；应触发降级 Generate
	p := &rsNoStreamStub{name: "ns", supports: ModalityText, text: "FullAnswer"}
	r := NewDefaultRouter([]Provider{p}, Policy{}, nil)
	ch, err := r.GenerateStream(context.Background(), GenerateRequest{Prompt: "x", Modality: ModalityText, N: 1})
	if err != nil {
		t.Fatal(err)
	}
	var sb strings.Builder
	done := false
	for c := range ch {
		sb.WriteString(c.Delta)
		if c.Done {
			done = true
		}
	}
	if sb.String() != "FullAnswer" || !done {
		t.Fatalf("got %q done=%v", sb.String(), done)
	}
}

func TestRouterStream_NoProviderSupports(t *testing.T) {
	p := &rsStreamStub{name: "p1", supports: ModalityImage, chunks: []string{"x"}}
	r := NewDefaultRouter([]Provider{p}, Policy{}, nil)
	ch, err := r.GenerateStream(context.Background(), GenerateRequest{Prompt: "x", Modality: ModalityText, N: 1})
	if err != nil {
		t.Fatal(err)
	}
	// 走降级路径，应该最终发出 Err
	var sawErr error
	for c := range ch {
		if c.Err != nil {
			sawErr = c.Err
		}
	}
	if sawErr == nil {
		t.Fatal("expected err for unsupported modality")
	}
}

func TestRouterStream_FallbackProviderOnInitErr(t *testing.T) {
	p1 := &rsStreamStub{name: "p1", supports: ModalityText, startErr: errors.New("boom")}
	p2 := &rsStreamStub{name: "p2", supports: ModalityText, chunks: []string{"OK"}}
	r := NewDefaultRouter([]Provider{p1, p2}, Policy{EnableFallback: true}, nil)
	ch, err := r.GenerateStream(context.Background(), GenerateRequest{Prompt: "x", Modality: ModalityText, N: 1})
	if err != nil {
		t.Fatal(err)
	}
	var sb strings.Builder
	for c := range ch {
		sb.WriteString(c.Delta)
	}
	if sb.String() != "OK" {
		t.Fatalf("got %q", sb.String())
	}
}

func TestRouterStream_NoFallbackByDefault(t *testing.T) {
	p1 := &rsStreamStub{name: "p1", supports: ModalityText, startErr: errors.New("boom")}
	p2 := &rsStreamStub{name: "p2", supports: ModalityText, chunks: []string{"OK"}}
	r := NewDefaultRouter([]Provider{p1, p2}, Policy{EnableFallback: false}, nil)
	_, err := r.GenerateStream(context.Background(), GenerateRequest{Prompt: "x", Modality: ModalityText, N: 1})
	if err == nil {
		t.Fatal("expected error without fallback")
	}
}

// fallback 路径下 provider 返回空文本应当 emit Err 而不是空 Done
func TestRouterStream_FallbackEmptyOutputEmitsErr(t *testing.T) {
	p := &rsNoStreamStub{name: "ns", supports: ModalityText, text: ""}
	r := NewDefaultRouter([]Provider{p}, Policy{}, nil)
	ch, err := r.GenerateStream(context.Background(), GenerateRequest{Prompt: "x", Modality: ModalityText, N: 1})
	if err != nil {
		t.Fatal(err)
	}
	var sawErr error
	sawDone := false
	for c := range ch {
		if c.Err != nil {
			sawErr = c.Err
		}
		if c.Done {
			sawDone = true
		}
	}
	if sawErr == nil {
		t.Fatal("expected Err for empty output")
	}
	if sawDone {
		t.Error("Done should not be emitted on empty-output failure")
	}
}

// 防止 time 包未使用警告
var _ = time.Second
