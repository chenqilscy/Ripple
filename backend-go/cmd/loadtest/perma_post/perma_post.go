// perma_post.go · M4-T5 凝结接口压测（POST /api/v1/perma_nodes）
//
// 强烈建议先启动后端：RIPPLE_LLM_FAKE=true（避免真实 LLM 计费）。
//
//	go run scripts/loadtest/perma_post.go -base http://localhost:8000 -token <jwt> \
//	    -lake <lake_id> -nodes "<id1>,<id2>" -dur 30s -conc 10
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

func main() {
	base := flag.String("base", "http://localhost:8000", "API base URL")
	token := flag.String("token", "", "JWT bearer")
	lake := flag.String("lake", "", "lake_id")
	nodesCSV := flag.String("nodes", "", "node id 列表（逗号分隔，至少 2 个）")
	dur := flag.Duration("dur", 30*time.Second, "压测时长")
	conc := flag.Int("conc", 10, "并发数")
	flag.Parse()
	if *token == "" || *lake == "" || *nodesCSV == "" {
		log.Fatal("-token -lake -nodes required")
	}
	nodes := strings.Split(*nodesCSV, ",")
	if len(nodes) < 2 {
		log.Fatal("need >= 2 node ids")
	}

	body, _ := json.Marshal(map[string]any{
		"lake_id":  *lake,
		"node_ids": nodes,
		"title":    "loadtest perma",
	})
	tr := &http.Transport{
		MaxIdleConns:        2048,
		MaxIdleConnsPerHost: 1024,
		MaxConnsPerHost:     1024,
		IdleConnTimeout:     90 * time.Second,
		DisableCompression:  true,
	}
	client := &http.Client{Transport: tr, Timeout: 30 * time.Second}

	var (
		ok        atomic.Int64
		fail      atomic.Int64
		latencies = make([]time.Duration, 0, 4096)
		mu        sync.Mutex
	)
	stop := time.After(*dur)
	wg := sync.WaitGroup{}
	for w := 0; w < *conc; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-stop:
					return
				default:
				}
				req, _ := http.NewRequest("POST", *base+"/api/v1/perma_nodes", bytes.NewReader(body))
				req.Header.Set("Authorization", "Bearer "+*token)
				req.Header.Set("Content-Type", "application/json")
				t0 := time.Now()
				resp, err := client.Do(req)
				lat := time.Since(t0)
				if err != nil || resp.StatusCode >= 400 {
					fail.Add(1)
					if resp != nil {
						_, _ = io.Copy(io.Discard, resp.Body)
						_ = resp.Body.Close()
					}
					continue
				}
				_, _ = io.Copy(io.Discard, resp.Body)
				_ = resp.Body.Close()
				ok.Add(1)
				mu.Lock()
				latencies = append(latencies, lat)
				mu.Unlock()
			}
		}()
	}
	wg.Wait()

	mu.Lock()
	sort.Slice(latencies, func(i, j int) bool { return latencies[i] < latencies[j] })
	mu.Unlock()
	p := func(q float64) time.Duration {
		if len(latencies) == 0 {
			return 0
		}
		idx := int(float64(len(latencies)) * q)
		if idx >= len(latencies) {
			idx = len(latencies) - 1
		}
		return latencies[idx]
	}
	total := ok.Load() + fail.Load()
	qps := float64(ok.Load()) / dur.Seconds()
	fmt.Println("=== POST /api/v1/perma_nodes ===")
	fmt.Printf("Conc:     %d\n", *conc)
	fmt.Printf("Dur:      %s\n", *dur)
	fmt.Printf("OK:       %d\n", ok.Load())
	fmt.Printf("Fail:     %d (%.2f%%)\n", fail.Load(), 100*float64(fail.Load())/float64(maxInt(int(total))))
	fmt.Printf("QPS:      %.1f\n", qps)
	fmt.Printf("p50:      %s\n", p(0.50).Round(time.Microsecond))
	fmt.Printf("p95:      %s\n", p(0.95).Round(time.Microsecond))
	fmt.Printf("p99:      %s\n", p(0.99).Round(time.Microsecond))
}

func maxInt(n int) int {
	if n < 1 {
		return 1
	}
	return n
}
