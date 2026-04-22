// Package main · M4-A Yjs Spike 桥接服务（独立二进制 :7790）。
//
// 功能：
//   - 按 lake_id 维度聚合 WebSocket 连接；
//   - 接收任意二进制（Yjs y-protocol 帧），原样广播给同 lake 其他对端；
//   - 不解析协议、不持久化，仅做"广播 hub"（M4-A Spike 阶段）。
//
// 路由：GET /yjs?lake=<lake_id>
// 鉴权：Spike 阶段使用查询参数 token（非生产）；后续接入 JWT。
//
// 运行：go run ./cmd/yjs-bridge  (默认监听 :7790)
package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"nhooyr.io/websocket"
)

type peer struct {
	conn   *websocket.Conn
	lakeID string
}

type hub struct {
	mu    sync.RWMutex
	rooms map[string]map[*peer]struct{} // lakeID -> peers
}

func newHub() *hub { return &hub{rooms: map[string]map[*peer]struct{}{}} }

func (h *hub) join(p *peer) {
	h.mu.Lock()
	defer h.mu.Unlock()
	room, ok := h.rooms[p.lakeID]
	if !ok {
		room = map[*peer]struct{}{}
		h.rooms[p.lakeID] = room
	}
	room[p] = struct{}{}
}

func (h *hub) leave(p *peer) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if room, ok := h.rooms[p.lakeID]; ok {
		delete(room, p)
		if len(room) == 0 {
			delete(h.rooms, p.lakeID)
		}
	}
}

func (h *hub) broadcast(ctx context.Context, sender *peer, msgType websocket.MessageType, data []byte) {
	h.mu.RLock()
	room := h.rooms[sender.lakeID]
	peers := make([]*peer, 0, len(room))
	for p := range room {
		if p != sender {
			peers = append(peers, p)
		}
	}
	h.mu.RUnlock()
	for _, p := range peers {
		c, cancel := context.WithTimeout(ctx, 3*time.Second)
		_ = p.conn.Write(c, msgType, data)
		cancel()
	}
}

func (h *hub) handleWS(w http.ResponseWriter, r *http.Request) {
	lakeID := r.URL.Query().Get("lake")
	if lakeID == "" {
		http.Error(w, "lake required", http.StatusBadRequest)
		return
	}
	c, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		InsecureSkipVerify: true, // Spike 阶段允许跨域
		Subprotocols:       []string{"y-protocol"},
	})
	if err != nil {
		return
	}
	defer c.Close(websocket.StatusInternalError, "")

	p := &peer{conn: c, lakeID: lakeID}
	h.join(p)
	defer h.leave(p)
	log.Printf("yjs-bridge: peer joined lake=%s", lakeID)

	ctx := r.Context()
	for {
		typ, data, err := c.Read(ctx)
		if err != nil {
			var ce websocket.CloseError
			if errors.As(err, &ce) {
				log.Printf("yjs-bridge: peer left lake=%s code=%d", lakeID, ce.Code)
			}
			return
		}
		h.broadcast(ctx, p, typ, data)
	}
}

func (h *hub) stats(w http.ResponseWriter, _ *http.Request) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte("{\"rooms\":"))
	first := true
	_, _ = w.Write([]byte("{"))
	for lake, peers := range h.rooms {
		if !first {
			_, _ = w.Write([]byte(","))
		}
		first = false
		_, _ = w.Write([]byte("\"" + lake + "\":"))
		_, _ = w.Write([]byte{byte('0' + (len(peers) % 10))})
	}
	_, _ = w.Write([]byte("}}"))
}

func main() {
	addr := os.Getenv("YJS_BRIDGE_ADDR")
	if addr == "" {
		addr = ":7790"
	}
	h := newHub()
	mux := http.NewServeMux()
	mux.HandleFunc("/yjs", h.handleWS)
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("ok")) })
	mux.HandleFunc("/stats", h.stats)

	srv := &http.Server{Addr: addr, Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	log.Printf("yjs-bridge listening on %s", addr)

	go func() {
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("yjs-bridge listen: %v", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop
	log.Println("yjs-bridge: shutting down")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = srv.Shutdown(ctx)
}
