package metrics

import (
	"bytes"
	"strings"
	"sync"
	"testing"
)

func TestCounter_IncAdd(t *testing.T) {
	r := NewRegistry()
	c := r.CounterVec("foo_total", "test", nil)
	c.Inc()
	c.Inc()
	c.Add(5)

	var buf bytes.Buffer
	if err := r.WriteText(&buf); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, "# TYPE foo_total counter") {
		t.Errorf("missing TYPE line: %s", out)
	}
	if !strings.Contains(out, "foo_total 7") {
		t.Errorf("want value 7, got: %s", out)
	}
}

func TestCounter_Labels(t *testing.T) {
	r := NewRegistry()
	a := r.CounterVec("calls_total", "h", map[string]string{"provider": "zhipu"})
	b := r.CounterVec("calls_total", "h", map[string]string{"provider": "openai"})
	a.Inc()
	a.Inc()
	b.Inc()

	var buf bytes.Buffer
	_ = r.WriteText(&buf)
	out := buf.String()
	if !strings.Contains(out, `calls_total{provider="zhipu"} 2`) {
		t.Errorf("missing zhipu label: %s", out)
	}
	if !strings.Contains(out, `calls_total{provider="openai"} 1`) {
		t.Errorf("missing openai label: %s", out)
	}
}

func TestGauge(t *testing.T) {
	r := NewRegistry()
	g := r.GaugeVec("ws_active", "h", nil)
	g.Inc()
	g.Inc()
	g.Inc()
	g.Dec()
	g.Set(42)
	var buf bytes.Buffer
	_ = r.WriteText(&buf)
	if !strings.Contains(buf.String(), "ws_active 42") {
		t.Errorf("want 42: %s", buf.String())
	}
}

func TestHistogram(t *testing.T) {
	r := NewRegistry()
	h := r.HistogramVec("dur_ms", "h", []float64{10, 100, 1000}, nil)
	h.Observe(5)    // ≤10
	h.Observe(50)   // ≤100
	h.Observe(500)  // ≤1000
	h.Observe(5000) // +Inf
	var buf bytes.Buffer
	_ = r.WriteText(&buf)
	out := buf.String()
	if !strings.Contains(out, `dur_ms_bucket{le="10"} 1`) {
		t.Errorf("bucket10 wrong: %s", out)
	}
	if !strings.Contains(out, `dur_ms_bucket{le="100"} 2`) {
		t.Errorf("bucket100 wrong: %s", out)
	}
	if !strings.Contains(out, `dur_ms_bucket{le="1000"} 3`) {
		t.Errorf("bucket1000 wrong: %s", out)
	}
	if !strings.Contains(out, `dur_ms_bucket{le="+Inf"} 4`) {
		t.Errorf("bucketInf wrong: %s", out)
	}
	if !strings.Contains(out, "dur_ms_count 4") {
		t.Errorf("count wrong: %s", out)
	}
}

func TestConcurrent(t *testing.T) {
	r := NewRegistry()
	c := r.CounterVec("ct", "h", nil)
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 1000; j++ {
				c.Inc()
			}
		}()
	}
	wg.Wait()
	var buf bytes.Buffer
	_ = r.WriteText(&buf)
	if !strings.Contains(buf.String(), "ct 100000") {
		t.Errorf("concurrency lost: %s", buf.String())
	}
}

func TestDefaultVarsExist(t *testing.T) {
	HTTPRequests.Inc()
	WSConnections.Inc()
	WSConnections.Dec()
	LLMCallsBy("zhipu").Inc()
	LLMErrorsBy("zhipu").Inc()
	LLMDurationBy("zhipu").Observe(123)
	HTTPDuration.Observe(45)

	var buf bytes.Buffer
	_ = Default.WriteText(&buf)
	out := buf.String()
	for _, want := range []string{
		"ripple_http_requests_total",
		"ripple_http_request_duration_ms",
		"ripple_ws_connections",
		`ripple_llm_calls_total{provider="zhipu"}`,
		`ripple_llm_errors_total{provider="zhipu"}`,
		`ripple_llm_call_duration_ms_count{provider="zhipu"}`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing metric %q in output:\n%s", want, out)
		}
	}
}
