package llm

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestOpenAICompatStream_HappyPath(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-key" {
			http.Error(w, "auth", http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		flusher, _ := w.(http.Flusher)
		frames := []string{
			`{"choices":[{"delta":{"content":"Hello"}}]}`,
			`{"choices":[{"delta":{"content":", "}}]}`,
			`{"choices":[{"delta":{"content":"world!"}}]}`,
			`{"choices":[],"usage":{"total_tokens":42}}`,
		}
		for _, f := range frames {
			fmt.Fprintf(w, "data: %s\n\n", f)
			flusher.Flush()
		}
		fmt.Fprint(w, "data: [DONE]\n\n")
		flusher.Flush()
	}))
	defer srv.Close()

	c := NewOpenAICompatClient(OpenAICompatConfig{
		Name: "test", APIKey: "test-key", Model: "x", Endpoint: srv.URL,
		Timeout: 5 * time.Second,
	})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	ch, err := c.GenerateStream(ctx, GenerateRequest{Prompt: "hi", Modality: ModalityText})
	if err != nil {
		t.Fatalf("GenerateStream: %v", err)
	}
	var got strings.Builder
	var done bool
	var cost int64
	for chunk := range ch {
		if chunk.Err != nil {
			t.Fatalf("chunk err: %v", chunk.Err)
		}
		if chunk.Done {
			done = true
			cost = chunk.CostTokens
			continue
		}
		got.WriteString(chunk.Delta)
	}
	if got.String() != "Hello, world!" {
		t.Fatalf("want %q, got %q", "Hello, world!", got.String())
	}
	if !done {
		t.Fatal("missing Done chunk")
	}
	if cost != 42 {
		t.Fatalf("want CostTokens=42, got %d", cost)
	}
}

func TestOpenAICompatStream_HTTP4xx(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "bad", http.StatusBadRequest)
	}))
	defer srv.Close()

	c := NewOpenAICompatClient(OpenAICompatConfig{
		Name: "test", APIKey: "k", Model: "x", Endpoint: srv.URL,
	})
	_, err := c.GenerateStream(context.Background(), GenerateRequest{Prompt: "x", Modality: ModalityText})
	if err == nil {
		t.Fatal("want error on 4xx")
	}
}

func TestOpenAICompatStream_RejectsNonText(t *testing.T) {
	c := NewOpenAICompatClient(OpenAICompatConfig{Name: "test", APIKey: "k", Model: "x", Endpoint: "http://localhost"})
	_, err := c.GenerateStream(context.Background(), GenerateRequest{Modality: ModalityImage})
	if err == nil {
		t.Fatal("want error on non-text modality")
	}
}
