// 简易 Go 原生压测工具（无需 k6/vegeta 安装）
//
// 用法：
//   go run scripts/loadtest/baseline.go -url http://localhost:8000/healthz -dur 30s -conc 50
//
// 输出：QPS / p50 / p95 / p99 / 错误率
package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"net/http"
	"sort"
	"sync"
	"sync/atomic"
	"time"
)

func main() {
	url := flag.String("url", "http://localhost:8000/healthz", "目标 URL")
	dur := flag.Duration("dur", 30*time.Second, "压测时长")
	conc := flag.Int("conc", 50, "并发协程数")
	token := flag.String("token", "", "Bearer token（可选）")
	flag.Parse()

	ctx, cancel := context.WithTimeout(context.Background(), *dur)
	defer cancel()

	var (
		ok, fail int64
		mu       sync.Mutex
		samples  []float64 // ms
	)

	client := &http.Client{
		Timeout: 10 * time.Second,
		Transport: &http.Transport{
			MaxIdleConns:        *conc * 2,
			MaxIdleConnsPerHost: *conc * 2,
			MaxConnsPerHost:     *conc * 2,
			IdleConnTimeout:     90 * time.Second,
			DisableCompression:  true,
		},
	}
	var wg sync.WaitGroup
	start := time.Now()

	for i := 0; i < *conc; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				if ctx.Err() != nil {
					return
				}
				t0 := time.Now()
				req, _ := http.NewRequestWithContext(ctx, "GET", *url, nil)
				if *token != "" {
					req.Header.Set("Authorization", "Bearer "+*token)
				}
				resp, err := client.Do(req)
				ms := float64(time.Since(t0).Microseconds()) / 1000.0
				if err != nil || resp.StatusCode >= 400 {
					atomic.AddInt64(&fail, 1)
				} else {
					atomic.AddInt64(&ok, 1)
				}
				if resp != nil {
					_, _ = io.Copy(io.Discard, resp.Body)
					_ = resp.Body.Close()
				}
				mu.Lock()
				samples = append(samples, ms)
				mu.Unlock()
			}
		}()
	}
	wg.Wait()

	elapsed := time.Since(start).Seconds()
	total := ok + fail
	sort.Float64s(samples)
	pct := func(p float64) float64 {
		if len(samples) == 0 {
			return 0
		}
		idx := int(float64(len(samples)) * p)
		if idx >= len(samples) {
			idx = len(samples) - 1
		}
		return samples[idx]
	}

	fmt.Printf("URL          : %s\n", *url)
	fmt.Printf("Duration     : %.2fs\n", elapsed)
	fmt.Printf("Concurrency  : %d\n", *conc)
	fmt.Printf("Requests     : %d (ok=%d, fail=%d)\n", total, ok, fail)
	fmt.Printf("QPS          : %.2f\n", float64(total)/elapsed)
	fmt.Printf("Error rate   : %.3f%%\n", float64(fail)/float64(total)*100)
	fmt.Printf("Latency p50  : %.2f ms\n", pct(0.50))
	fmt.Printf("Latency p95  : %.2f ms\n", pct(0.95))
	fmt.Printf("Latency p99  : %.2f ms\n", pct(0.99))
}
