// Package metrics · 极简 Prometheus 文本格式埋点。
//
// 设计取舍：
//   - 不引入 prometheus/client_golang（额外 ~3MB 依赖 + 反射开销）
//   - 仅支持 Counter / Gauge / Histogram（固定 bucket）
//   - 内部用 sync/atomic + RWMutex；标签维度通过拼接 key
//   - 输出符合 Prometheus 0.0.4 文本格式，可直接被 prometheus scrape
//
// 提供给 HTTP / WS / LLM / DB 等场景的标准计数器在 [vars.go](vars.go)。
package metrics

import (
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
)

// Counter 单调递增计数器。并发安全。
type Counter struct {
	name   string
	help   string
	val    uint64
	labels map[string]string
}

// Inc +1。
func (c *Counter) Inc() { atomic.AddUint64(&c.val, 1) }

// Add 增加任意正值（负值会 panic）。
func (c *Counter) Add(n uint64) { atomic.AddUint64(&c.val, n) }

// Gauge 可任意增减的瞬时值。并发安全。
type Gauge struct {
	name   string
	help   string
	val    int64
	labels map[string]string
}

// Set 直接设值。
func (g *Gauge) Set(v int64) { atomic.StoreInt64(&g.val, v) }

// Inc +1。
func (g *Gauge) Inc() { atomic.AddInt64(&g.val, 1) }

// Dec -1。
func (g *Gauge) Dec() { atomic.AddInt64(&g.val, -1) }

// Histogram 观察分布。bucket 上限固定。
type Histogram struct {
	name    string
	help    string
	buckets []float64 // 升序，最后隐含 +Inf
	counts  []uint64  // len = len(buckets)+1，对应 ≤bucket 与 +Inf
	sum     uint64    // 累计值 * 1000（毫秒精度，用 uint64 存储微秒；这里直接按调用方传入的单位累计）
	count   uint64
	mu      sync.Mutex
	labels  map[string]string
}

// Observe 记录一次观察值（单位由调用方决定，如毫秒）。
func (h *Histogram) Observe(v float64) {
	h.mu.Lock()
	defer h.mu.Unlock()
	atomic.AddUint64(&h.count, 1)
	// sum 用 float 累加；这里简化为 *1000 后取 uint64（精度到 1/1000 单位）
	atomic.AddUint64(&h.sum, uint64(v*1000))
	for i, b := range h.buckets {
		if v <= b {
			h.counts[i]++
			return
		}
	}
	h.counts[len(h.buckets)]++ // +Inf
}

// Registry 集中保存所有指标。
type Registry struct {
	mu         sync.RWMutex
	counters   map[string]*Counter
	gauges     map[string]*Gauge
	histograms map[string]*Histogram
}

// NewRegistry 构造空 Registry。
func NewRegistry() *Registry {
	return &Registry{
		counters:   map[string]*Counter{},
		gauges:     map[string]*Gauge{},
		histograms: map[string]*Histogram{},
	}
}

func metricKey(name string, labels map[string]string) string {
	if len(labels) == 0 {
		return name
	}
	keys := make([]string, 0, len(labels))
	for k := range labels {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var sb strings.Builder
	sb.WriteString(name)
	sb.WriteByte('{')
	for i, k := range keys {
		if i > 0 {
			sb.WriteByte(',')
		}
		sb.WriteString(k)
		sb.WriteByte('=')
		sb.WriteString(strconv.Quote(labels[k]))
	}
	sb.WriteByte('}')
	return sb.String()
}

// CounterVec 提供 Counter(labels...) 形式访问。
func (r *Registry) CounterVec(name, help string, labels map[string]string) *Counter {
	key := metricKey(name, labels)
	r.mu.RLock()
	if c, ok := r.counters[key]; ok {
		r.mu.RUnlock()
		return c
	}
	r.mu.RUnlock()
	r.mu.Lock()
	defer r.mu.Unlock()
	if c, ok := r.counters[key]; ok {
		return c
	}
	c := &Counter{name: name, help: help, labels: labels}
	r.counters[key] = c
	return c
}

// GaugeVec 同 CounterVec。
func (r *Registry) GaugeVec(name, help string, labels map[string]string) *Gauge {
	key := metricKey(name, labels)
	r.mu.RLock()
	if g, ok := r.gauges[key]; ok {
		r.mu.RUnlock()
		return g
	}
	r.mu.RUnlock()
	r.mu.Lock()
	defer r.mu.Unlock()
	if g, ok := r.gauges[key]; ok {
		return g
	}
	g := &Gauge{name: name, help: help, labels: labels}
	r.gauges[key] = g
	return g
}

// HistogramVec 注册或获取直方图。buckets 必须升序。
func (r *Registry) HistogramVec(name, help string, buckets []float64, labels map[string]string) *Histogram {
	key := metricKey(name, labels)
	r.mu.RLock()
	if h, ok := r.histograms[key]; ok {
		r.mu.RUnlock()
		return h
	}
	r.mu.RUnlock()
	r.mu.Lock()
	defer r.mu.Unlock()
	if h, ok := r.histograms[key]; ok {
		return h
	}
	h := &Histogram{
		name:    name,
		help:    help,
		buckets: buckets,
		counts:  make([]uint64, len(buckets)+1),
		labels:  labels,
	}
	r.histograms[key] = h
	return h
}

// WriteText 输出 Prometheus 文本格式。
func (r *Registry) WriteText(w io.Writer) error {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// 同名指标合并 HELP/TYPE 头
	type counterEntry struct{ c *Counter }
	type gaugeEntry struct{ g *Gauge }
	type histEntry struct{ h *Histogram }

	cByName := map[string][]*Counter{}
	for _, c := range r.counters {
		cByName[c.name] = append(cByName[c.name], c)
	}
	gByName := map[string][]*Gauge{}
	for _, g := range r.gauges {
		gByName[g.name] = append(gByName[g.name], g)
	}
	hByName := map[string][]*Histogram{}
	for _, h := range r.histograms {
		hByName[h.name] = append(hByName[h.name], h)
	}

	cNames := sortedKeys(cByName)
	for _, name := range cNames {
		list := cByName[name]
		fmt.Fprintf(w, "# HELP %s %s\n", name, list[0].help)
		fmt.Fprintf(w, "# TYPE %s counter\n", name)
		for _, c := range list {
			fmt.Fprintf(w, "%s%s %d\n", name, formatLabels(c.labels), atomic.LoadUint64(&c.val))
		}
	}
	gNames := sortedKeys(gByName)
	for _, name := range gNames {
		list := gByName[name]
		fmt.Fprintf(w, "# HELP %s %s\n", name, list[0].help)
		fmt.Fprintf(w, "# TYPE %s gauge\n", name)
		for _, g := range list {
			fmt.Fprintf(w, "%s%s %d\n", name, formatLabels(g.labels), atomic.LoadInt64(&g.val))
		}
	}
	hNames := sortedKeys(hByName)
	for _, name := range hNames {
		list := hByName[name]
		fmt.Fprintf(w, "# HELP %s %s\n", name, list[0].help)
		fmt.Fprintf(w, "# TYPE %s histogram\n", name)
		for _, h := range list {
			h.mu.Lock()
			cum := uint64(0)
			for i, b := range h.buckets {
				cum += h.counts[i]
				lbls := mergeLabels(h.labels, "le", strconv.FormatFloat(b, 'g', -1, 64))
				fmt.Fprintf(w, "%s_bucket%s %d\n", name, formatLabels(lbls), cum)
			}
			cum += h.counts[len(h.buckets)]
			lbls := mergeLabels(h.labels, "le", "+Inf")
			fmt.Fprintf(w, "%s_bucket%s %d\n", name, formatLabels(lbls), cum)
			fmt.Fprintf(w, "%s_sum%s %g\n", name, formatLabels(h.labels), float64(atomic.LoadUint64(&h.sum))/1000)
			fmt.Fprintf(w, "%s_count%s %d\n", name, formatLabels(h.labels), atomic.LoadUint64(&h.count))
			h.mu.Unlock()
		}
	}
	_ = counterEntry{}
	_ = gaugeEntry{}
	_ = histEntry{}
	return nil
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func mergeLabels(base map[string]string, k, v string) map[string]string {
	out := make(map[string]string, len(base)+1)
	for kk, vv := range base {
		out[kk] = vv
	}
	out[k] = v
	return out
}

func formatLabels(labels map[string]string) string {
	if len(labels) == 0 {
		return ""
	}
	keys := make([]string, 0, len(labels))
	for k := range labels {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var sb strings.Builder
	sb.WriteByte('{')
	for i, k := range keys {
		if i > 0 {
			sb.WriteByte(',')
		}
		sb.WriteString(k)
		sb.WriteByte('=')
		sb.WriteString(strconv.Quote(labels[k]))
	}
	sb.WriteByte('}')
	return sb.String()
}
