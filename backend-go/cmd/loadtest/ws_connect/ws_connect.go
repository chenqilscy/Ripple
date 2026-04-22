// ws_connect.go · M4-T5 WebSocket 连接压测
//
// 仅测试"建立 + 维持"连接的容量，不发送任何业务消息。
//
//	go run scripts/loadtest/ws_connect.go -url ws://localhost:8000/api/v1/lakes/<lakeID>/ws \
//	    -token <jwt> -conc 300 -hold 30s
//
// 输出：成功建立的连接数、失败数、握手 p50/p95、维持 X 秒后的存活率。
//
// P7-C 注意（Windows 默认动态端口范围 49152–65535，约 16383 个端口）：
//
//	测试客户端与服务端同机运行时每条连接占用 1 个本机端口，1000 并发会耗尽端口
//	并导致大量 dial timeout（非后端逻辑问题）。
//	解决方案：
//	  a) 扩展端口范围（需管理员权限，重启生效）：
//	     netsh int ipv4 set dynamicport tcp start=10000 num=55535
//	  b) 控制客户端并发数 ≤300（本工具默认值）确保 CI 稳定通过。
//	  c) 使用独立客户端机器做 1000+ 压测。
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"nhooyr.io/websocket"
)

func main() {
	urlStr := flag.String("url", "", "WebSocket URL (含 /api/v1/lakes/<id>/ws)")
	token := flag.String("token", "", "JWT bearer")
	// P7-C：默认 300（Windows 同机测试安全上限；详见文件头注释）
	conc    := flag.Int("conc", 300, "并发连接数（Windows 同机推荐 ≤300；跨机或调参后可提至 1000+）")
	hold    := flag.Duration("hold", 30*time.Second, "建连后保持时间")
	timeout := flag.Duration("dial-timeout", 10*time.Second, "握手超时")
	flag.Parse()
	if *urlStr == "" {
		log.Fatal("-url required")
	}

	var (
		ok        atomic.Int64
		failDial  atomic.Int64
		alive     atomic.Int64
		latencies = make([]time.Duration, 0, *conc)
		mu        sync.Mutex
	)
	hdr := http.Header{}
	if *token != "" {
		hdr.Set("Authorization", "Bearer "+*token)
	}
	wg := sync.WaitGroup{}
	start := time.Now()
	for i := 0; i < *conc; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ctx, cancel := context.WithTimeout(context.Background(), *timeout)
			defer cancel()
			t0 := time.Now()
			c, _, err := websocket.Dial(ctx, *urlStr, &websocket.DialOptions{HTTPHeader: hdr})
			if err != nil {
				failDial.Add(1)
				return
			}
			ok.Add(1)
			alive.Add(1)
			mu.Lock()
			latencies = append(latencies, time.Since(t0))
			mu.Unlock()
			defer func() {
				alive.Add(-1)
				_ = c.Close(websocket.StatusNormalClosure, "bye")
			}()
			// 仅维持，不读不写（除了底层 ping/pong 由 lib 处理）
			holdCtx, holdCancel := context.WithTimeout(context.Background(), *hold)
			defer holdCancel()
			<-holdCtx.Done()
		}()
	}
	// 启动后等待 dial 完成（取 hold 之前的中点采样）
	time.Sleep(*hold / 2)
	midAlive := alive.Load()
	wg.Wait()
	dur := time.Since(start)

	// 计算握手延迟分位
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

	fmt.Println("=== WS Connect Stress ===")
	fmt.Printf("URL:           %s\n", *urlStr)
	fmt.Printf("Concurrent:    %d\n", *conc)
	fmt.Printf("Hold:          %s\n", *hold)
	fmt.Printf("Total time:    %s\n", dur.Round(time.Millisecond))
	fmt.Printf("Dial OK:       %d\n", ok.Load())
	fmt.Printf("Dial failed:   %d\n", failDial.Load())
	fmt.Printf("Alive @ mid:   %d (%.1f%% of OK)\n", midAlive, 100.0*float64(midAlive)/float64(max1(int(ok.Load()))))
	fmt.Printf("Handshake p50: %s\n", p(0.50).Round(time.Microsecond))
	fmt.Printf("Handshake p95: %s\n", p(0.95).Round(time.Microsecond))
	fmt.Printf("Handshake p99: %s\n", p(0.99).Round(time.Microsecond))
}

func max1(n int) int {
	if n < 1 {
		return 1
	}
	return n
}
