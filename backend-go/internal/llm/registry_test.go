package llm

import (
	"context"
	"errors"
	"testing"
)

func TestBuildProviders_DefaultOrder(t *testing.T) {
	ps := BuildProviders(ProviderConfig{
		ZhipuKey: "k", ZhipuModel: "m",
		OpenAIKey: "k2", OpenAIModel: "gpt",
		DeepSeekKey: "k3", DeepSeekModel: "ds",
	})
	if len(ps) != 3 {
		t.Fatalf("want 3 providers, got %d", len(ps))
	}
	want := []string{"zhipu", "deepseek", "openai"}
	for i, n := range want {
		if ps[i].Name() != n {
			t.Errorf("idx %d: want %s got %s", i, n, ps[i].Name())
		}
	}
}

func TestBuildProviders_CustomOrder(t *testing.T) {
	ps := BuildProviders(ProviderConfig{
		ZhipuKey:    "k",
		OpenAIKey:   "k2",
		DeepSeekKey: "k3",
		Order:       "openai,zhipu,deepseek",
	})
	if len(ps) != 3 {
		t.Fatalf("want 3, got %d", len(ps))
	}
	if ps[0].Name() != "openai" || ps[1].Name() != "zhipu" || ps[2].Name() != "deepseek" {
		t.Errorf("order wrong: %s,%s,%s", ps[0].Name(), ps[1].Name(), ps[2].Name())
	}
}

func TestBuildProviders_SkipsMissingKeys(t *testing.T) {
	ps := BuildProviders(ProviderConfig{
		ZhipuKey: "k",
		// 其他全空
	})
	if len(ps) != 1 || ps[0].Name() != "zhipu" {
		t.Fatalf("want only zhipu, got %d", len(ps))
	}
}

func TestBuildProviders_OpenAICompatRequiresAll3(t *testing.T) {
	// 只有 key 没 endpoint/model：跳过
	ps := BuildProviders(ProviderConfig{
		OpenAICompatKey: "k",
		Order:           "openai-compat",
	})
	if len(ps) != 0 {
		t.Fatalf("openai-compat needs key+model+endpoint, got %d providers", len(ps))
	}
	// 全配
	ps = BuildProviders(ProviderConfig{
		OpenAICompatKey:      "k",
		OpenAICompatModel:    "llama3",
		OpenAICompatEndpoint: "http://localhost:11434/v1/chat/completions",
		OpenAICompatName:     "ollama",
		Order:                "openai-compat",
	})
	if len(ps) != 1 || ps[0].Name() != "ollama" {
		t.Fatalf("want ollama, got %d %v", len(ps), ps)
	}
}

// TestRouter_FallbackToSecondProvider 验证当首选 provider 失败时
// EnableFallback=true 会顺位下一个 provider 尝试。
func TestRouter_FallbackToSecondProvider(t *testing.T) {
	failing := &stubProvider{name: "zhipu", supports: ModalityText, err: errors.New("503")}
	healthy := &stubProvider{name: "openai", supports: ModalityText, cands: []Candidate{{Text: "a"}, {Text: "b"}}}
	router := NewDefaultRouter([]Provider{failing, healthy}, Policy{EnableFallback: true}, NoopRecorder{})
	out, err := router.Generate(context.Background(), GenerateRequest{
		Prompt: "x", N: 2, Modality: ModalityText,
	})
	if err != nil {
		t.Fatalf("fallback failed: %v", err)
	}
	if len(out) != 2 || out[0].Text != "a" {
		t.Fatalf("want fallback to openai, got %v", out)
	}
	if failing.calls != 1 || healthy.calls != 1 {
		t.Fatalf("both providers should be called once: failing=%d healthy=%d", failing.calls, healthy.calls)
	}
}

// TestRouter_NoFallback_FailsFast 默认不 fallback，第一个失败就返。
func TestRouter_NoFallback_FailsFast(t *testing.T) {
	failing := &stubProvider{name: "zhipu", supports: ModalityText, err: errors.New("401")}
	healthy := &stubProvider{name: "openai", supports: ModalityText, cands: []Candidate{{Text: "a"}}}
	router := NewDefaultRouter([]Provider{failing, healthy}, Policy{EnableFallback: false}, NoopRecorder{})
	_, err := router.Generate(context.Background(), GenerateRequest{
		Prompt: "x", N: 1, Modality: ModalityText,
	})
	if err == nil {
		t.Fatal("want error from first provider, got nil")
	}
	if healthy.calls != 0 {
		t.Fatalf("healthy must not be called when fallback disabled, calls=%d", healthy.calls)
	}
}
