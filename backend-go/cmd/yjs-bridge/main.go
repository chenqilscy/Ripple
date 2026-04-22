// Package main · M4-A Yjs Spike 桥接服务（独立二进制 :7790）。
//
// 功能：
//   - 按 lake_id 维度聚合 WebSocket 连接；
//   - 接收任意二进制（Yjs y-protocol 帧），原样广播给同 lake 其他对端；
//   - 不解析协议、不持久化，仅做"广播 hub"（M4-A Spike 阶段）。
//
// 路由：GET /yjs?lake=<lake_id>&token=<jwt>
//
// 鉴权（P6-B 起强制；可用 YJS_BRIDGE_REQUIRE_AUTH=false 关闭）：
//   - JWT：由 RIPPLE_JWT_SECRET 配置，复用 Ripple 主服务签发的 Token
//     P7-B 起要求 token purpose=="ws"（短期 ws-only token）
//   - Origin：YJS_BRIDGE_ALLOWED_ORIGINS（逗号分隔）；
//     P7-A：鉴权开启时白名单为空则 fail-closed（403），不回落 InsecureSkipVerify
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
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/chenqilscy/ripple/backend-go/internal/platform"
	"nhooyr.io/websocket"
)

type peer struct {
	conn   *websocket.Conn
	lakeID string
	userID string // P6-B 鉴权后写入
}

type hub struct {
	mu              sync.RWMutex
	rooms           map[string]map[*peer]struct{} // lakeID -> peers
	jwt             *platform.JWTSigner           // nil 表示禁用鉴权
	allowedOrigins  map[string]struct{}           // empty 表示不限 Origin（仅当 jwt==nil 时安全）
	originsRawList  []string                      // 用于 AcceptOptions
}

func newHub(jwtSigner *platform.JWTSigner, originsList []string) *hub {
	allowed := map[string]struct{}{}
	for _, o := range originsList {
		o = strings.TrimSpace(o)
		if o != "" {
			allowed[o] = struct{}{}
		}
	}
	return &hub{
		rooms:          map[string]map[*peer]struct{}{},
		jwt:            jwtSigner,
		allowedOrigins: allowed,
		originsRawList: originsList,
	}
}

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

	// P6-B 鉴权：JWT 必须有效；user 来自 token claims
	var userID string
	if h.jwt != nil {
		tok := r.URL.Query().Get("token")
		if tok == "" {
			// 兼容 Authorization 头
			if h := r.Header.Get("Authorization"); strings.HasPrefix(h, "Bearer ") {
				tok = strings.TrimPrefix(h, "Bearer ")
			}
		}
		if tok == "" {
			http.Error(w, "token required", http.StatusUnauthorized)
			return
		}
		claims, err := h.jwt.Parse(tok)
		if err != nil {
			http.Error(w, "invalid token", http.StatusUnauthorized)
			return
		}
		// P7-B：要求 ws-only token（purpose=="ws"），拒绝主 token 直连
		if claims.Purpose != "ws" {
			http.Error(w, "token purpose must be 'ws': use POST /api/v1/ws_token to obtain a ws-only token", http.StatusUnauthorized)
			return
		}
		userID = claims.UserID
	}

	// P7-A fail-closed：鉴权开启时，必须显式配置白名单；
	// 若白名单为空则拒绝所有跨域请求，防止 InsecureSkipVerify 回落导致无鉴权。
	acceptOpts := &websocket.AcceptOptions{Subprotocols: []string{"y-protocol"}}
	if len(h.allowedOrigins) > 0 {
		acceptOpts.OriginPatterns = h.originsRawList
	} else if h.jwt != nil {
		// 鉴权已启用但白名单为空 → fail-closed：拒绝所有带 Origin 头的请求
		http.Error(w, "websocket origin not allowed", http.StatusForbidden)
		return
	} else {
		// 鉴权已关闭（spike 模式）：允许所有来源
		acceptOpts.InsecureSkipVerify = true
	}

	c, err := websocket.Accept(w, r, acceptOpts)
	if err != nil {
		return
	}
	defer c.Close(websocket.StatusInternalError, "")

	p := &peer{conn: c, lakeID: lakeID, userID: userID}
	h.join(p)
	defer h.leave(p)
	log.Printf("yjs-bridge: peer joined lake=%s user=%s", lakeID, userID)

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

	// P6-B 鉴权（默认开启；YJS_BRIDGE_REQUIRE_AUTH=false 关闭，仅 spike 用）
	var jwtSigner *platform.JWTSigner
	if strings.ToLower(os.Getenv("YJS_BRIDGE_REQUIRE_AUTH")) != "false" {
		secret := os.Getenv("RIPPLE_JWT_SECRET")
		if secret == "" {
			log.Fatal("yjs-bridge: RIPPLE_JWT_SECRET required when auth enabled (set YJS_BRIDGE_REQUIRE_AUTH=false to disable for spike)")
		}
		jwtSigner = platform.NewJWTSigner(secret, 24*time.Hour)
	}

	// P6-B Origin 白名单
	originsList := []string{}
	if v := os.Getenv("YJS_BRIDGE_ALLOWED_ORIGINS"); v != "" {
		for _, o := range strings.Split(v, ",") {
			if o = strings.TrimSpace(o); o != "" {
				originsList = append(originsList, o)
			}
		}
	}

	h := newHub(jwtSigner, originsList)
	mux := http.NewServeMux()
	mux.HandleFunc("/yjs", h.handleWS)
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("ok")) })
	mux.HandleFunc("/stats", h.stats)

	srv := &http.Server{Addr: addr, Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	authStatus := "DISABLED"
	if jwtSigner != nil {
		authStatus = "ENABLED"
	}
	log.Printf("yjs-bridge listening on %s (auth=%s, origins=%v)", addr, authStatus, originsList)

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
