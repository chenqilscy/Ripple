// Package main · M4-A Yjs Spike 桥接服务（独立二进制 :7790）。
//
// 功能：
//   - 按 node_id 维度聚合 WebSocket 连接（P8-D 升级，原按 lake_id）；
//   - 接收任意二进制（Yjs y-protocol 帧），原样广播给同 node 其他对端；
//   - P8-D：首个对端接入时从 Ripple REST API 加载 Y.Doc 快照并推送；
//     快照写回由前端负责（PUT /api/v1/nodes/{id}/doc_state）。
//
// 路由：GET /yjs?lake=<lake_id>&node=<node_id>&token=<jwt>
//
// 鉴权（P6-B 起强制；可用 YJS_BRIDGE_REQUIRE_AUTH=false 关闭）：
//   - JWT：由 RIPPLE_JWT_SECRET 配置，复用 Ripple 主服务签发的 Token
//     P7-B 起要求 token purpose=="ws"（短期 ws-only token）
//   - Origin：YJS_BRIDGE_ALLOWED_ORIGINS（逗号分隔）；
//     P7-A：鉴权开启时白名单为空则 fail-closed（403），不回落 InsecureSkipVerify
//
// 环境变量（P8-D 新增）：
//   - YJS_BRIDGE_API_URL：Ripple 后端 URL（如 http://localhost:8000），
//     非空时启用快照加载功能。
//
// 运行：go run ./cmd/yjs-bridge  (默认监听 :7790)
package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/chenqilscy/ripple/backend-go/internal/platform"
	"github.com/redis/go-redis/v9"
	"nhooyr.io/websocket"
)

// newRandomID 生成 16 字节随机十六进制字符串（32 字符）。
func newRandomID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		panic("yjs-bridge: failed to generate random ID: " + err.Error())
	}
	return hex.EncodeToString(b)
}

type peer struct {
	conn   *websocket.Conn
	id     string // 对端唯一 ID（hex），用于 Redis 消息去重
	nodeID string // P8-D：使用 node_id 作为房间键
	lakeID string
	userID string // P6-B 鉴权后写入
	token  string // P8-D：ws token，用于首次加载快照
}

type hub struct {
	mu             sync.RWMutex
	rooms          map[string]map[*peer]struct{} // nodeID -> peers
	roomSubs       map[string]context.CancelFunc // P9-A：nodeID -> 订阅 goroutine cancel 函数
	jwt            *platform.JWTSigner           // nil 表示禁用鉴权
	allowedOrigins map[string]struct{}           // empty 表示不限 Origin（仅当 jwt==nil 时安全）
	originsRawList []string                      // 用于 AcceptOptions
	apiURL         string                        // P8-D：Ripple 后端 URL（如 http://localhost:8000）
	httpClient     *http.Client                  // P8-D：用于加载快照的 HTTP 客户端
	// P9-A：Redis Pub/Sub（nil = 单实例模式，禁用 Redis 广播）
	rdb        *redis.Client
	instanceID string // 当前实例唯一 ID，用于消息去重（防止本实例收到自己发出的广播）
	peerSeq    uint64 // 简单递增 ID（配合 sync/atomic）
}

// redisChannel 返回指定 nodeID 的 Redis Pub/Sub channel 名。
func redisChannel(nodeID string) string {
	return "yjs:room:" + nodeID
}

// encodeRedisMsg 将 Yjs 帧编码为 Redis 消息：<instanceID>\n<peerID>\n<payload>。
// 格式保证 instanceID 和 peerID 不含换行符（均为十六进制字符串）。
func encodeRedisMsg(instanceID, peerID string, payload []byte) []byte {
	var buf bytes.Buffer
	buf.WriteString(instanceID)
	buf.WriteByte('\n')
	buf.WriteString(peerID)
	buf.WriteByte('\n')
	buf.Write(payload)
	return buf.Bytes()
}

// decodeRedisMsg 从 Redis 消息中解析 instanceID、peerID 和原始 payload。
func decodeRedisMsg(msg []byte) (instanceID, peerID string, payload []byte, ok bool) {
	idx1 := bytes.IndexByte(msg, '\n')
	if idx1 < 0 {
		return
	}
	idx2 := bytes.IndexByte(msg[idx1+1:], '\n')
	if idx2 < 0 {
		return
	}
	instanceID = string(msg[:idx1])
	peerID = string(msg[idx1+1 : idx1+1+idx2])
	payload = msg[idx1+1+idx2+1:]
	ok = true
	return
}

func newHub(jwtSigner *platform.JWTSigner, originsList []string, apiURL string, rdb *redis.Client, instanceID string) *hub {
	allowed := map[string]struct{}{}
	for _, o := range originsList {
		o = strings.TrimSpace(o)
		if o != "" {
			allowed[o] = struct{}{}
		}
	}
	return &hub{
		rooms:          map[string]map[*peer]struct{}{},
		roomSubs:       map[string]context.CancelFunc{},
		jwt:            jwtSigner,
		allowedOrigins: allowed,
		originsRawList: websocketOriginPatterns(originsList),
		apiURL:         apiURL,
		httpClient:     &http.Client{Timeout: 5 * time.Second},
		rdb:            rdb,
		instanceID:     instanceID,
	}
}

// joinResult 表示对端加入后的房间状态。
type joinResult struct {
	isFirst bool // 加入前房间为空（此对端是首个加入者）
}

func (h *hub) join(p *peer) joinResult {
	h.mu.Lock()
	defer h.mu.Unlock()
	room, ok := h.rooms[p.nodeID]
	isFirst := !ok || len(room) == 0
	if !ok {
		room = map[*peer]struct{}{}
		h.rooms[p.nodeID] = room
	}
	room[p] = struct{}{}

	// P9-A：首个对端加入时启动 Redis 订阅 goroutine
	if isFirst && h.rdb != nil {
		ctx, cancel := context.WithCancel(context.Background())
		h.roomSubs[p.nodeID] = cancel
		go h.subscribeRoom(ctx, p.nodeID)
	}
	return joinResult{isFirst: isFirst}
}

func (h *hub) leave(p *peer) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if room, ok := h.rooms[p.nodeID]; ok {
		delete(room, p)
		if len(room) == 0 {
			delete(h.rooms, p.nodeID)
			// P9-A：最后一个对端离开时取消 Redis 订阅
			if cancel, ok := h.roomSubs[p.nodeID]; ok {
				cancel()
				delete(h.roomSubs, p.nodeID)
			}
		}
	}
}

// broadcast 将消息广播给同 room 的本地对端（排除 sender）。
// P9-A：同时 publish 到 Redis，供其他实例转发。
func (h *hub) broadcast(ctx context.Context, sender *peer, msgType websocket.MessageType, data []byte) {
	h.mu.RLock()
	room := h.rooms[sender.nodeID]
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

	// P9-A：publish 到 Redis（其他实例的对端会收到并转发）
	if h.rdb != nil && msgType == websocket.MessageBinary {
		msg := encodeRedisMsg(h.instanceID, sender.id, data)
		// 使用独立的 background context 避免因 WS 请求 context 取消导致 publish 失败
		pubCtx, pubCancel := context.WithTimeout(context.Background(), 2*time.Second)
		if err := h.rdb.Publish(pubCtx, redisChannel(sender.nodeID), msg).Err(); err != nil {
			log.Printf("yjs-bridge: redis publish node=%s err=%v", sender.nodeID, err)
		}
		pubCancel()
	}
}

// subscribeRoom 订阅 Redis channel，将远端消息广播给本实例的本地对端（P9-A）。
// 调用方须保证 nodeID 对应 room 存在；ctx cancel 时退出。
// 内置重连逻辑：Redis 断开后每 2s 重试，直到 ctx 取消。
func (h *hub) subscribeRoom(ctx context.Context, nodeID string) {
	ch := redisChannel(nodeID)
	for {
		if ctx.Err() != nil {
			return
		}
		ps := h.rdb.Subscribe(ctx, ch)
		msgCh := ps.Channel()
		exited := false
		for !exited {
			select {
			case <-ctx.Done():
				_ = ps.Unsubscribe(context.Background(), ch)
				_ = ps.Close()
				return
			case msg, ok := <-msgCh:
				if !ok {
					// Redis 连接断开，退出内层循环以触发重连
					_ = ps.Close()
					exited = true
					break
				}
				instanceID, _, payload, valid := decodeRedisMsg([]byte(msg.Payload))
				if !valid {
					continue
				}
				// 跳过本实例自己发出的消息（避免回环广播）
				if instanceID == h.instanceID {
					continue
				}
				// 广播给本地 room 的所有对端
				h.mu.RLock()
				room := h.rooms[nodeID]
				peers := make([]*peer, 0, len(room))
				for p := range room {
					peers = append(peers, p)
				}
				h.mu.RUnlock()
				for _, p := range peers {
					wCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
					_ = p.conn.Write(wCtx, websocket.MessageBinary, payload)
					cancel()
				}
			}
		}
		log.Printf("yjs-bridge: redis subscription lost for node=%s, retrying in 2s", nodeID)
		select {
		case <-ctx.Done():
			return
		case <-time.After(2 * time.Second):
		}
	}
}

// loadSnapshot 从 Ripple REST API 读取节点 Y.Doc 快照（P8-D）。
// 成功时返回快照字节；无快照时返回 nil, nil；错误时返回 nil, err。
func (h *hub) loadSnapshot(ctx context.Context, nodeID, token string) ([]byte, error) {
	if h.apiURL == "" || nodeID == "" || token == "" {
		return nil, nil
	}
	apiPath := fmt.Sprintf("%s/api/v1/nodes/%s/doc_state", h.apiURL, url.PathEscape(nodeID))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiPath, nil)
	if err != nil {
		return nil, fmt.Errorf("load snapshot: build request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := h.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("load snapshot: http: %w", err)
	}
	defer resp.Body.Close()
	switch resp.StatusCode {
	case http.StatusOK:
		data, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20)) // 最多读 2 MiB
		if err != nil {
			return nil, fmt.Errorf("load snapshot: read body: %w", err)
		}
		return data, nil
	case http.StatusNoContent, http.StatusNotFound:
		return nil, nil // 无快照，正常
	default:
		return nil, fmt.Errorf("load snapshot: unexpected status %d", resp.StatusCode)
	}
}

// wrapSyncStep2 将原始 Y.Doc update 字节包装为 y-websocket SyncStep2 帧（P8-D）。
//
// y-websocket wire format（little-endian varint，lib0 编码）：
//
//	[0x00] = messageSync
//	[0x01] = messageSyncStep2
//	[varint: len(update)] + [update bytes]
func wrapSyncStep2(update []byte) []byte {
	lenBuf := appendVarUint(nil, uint64(len(update)))
	frame := make([]byte, 0, 2+len(lenBuf)+len(update))
	frame = append(frame, 0x00, 0x01) // messageSync + messageSyncStep2
	frame = append(frame, lenBuf...)
	frame = append(frame, update...)
	return frame
}

// appendVarUint 追加无符号变长整数（lib0/encoding 兼容）。
func appendVarUint(dst []byte, v uint64) []byte {
	for v >= 0x80 {
		dst = append(dst, byte(v)|0x80)
		v >>= 7
	}
	return append(dst, byte(v))
}

func (h *hub) handleWS(w http.ResponseWriter, r *http.Request) {
	lakeID := r.URL.Query().Get("lake")
	if lakeID == "" {
		http.Error(w, "lake required", http.StatusBadRequest)
		return
	}
	nodeID := r.URL.Query().Get("node") // P8-D：可选，空时退化为 lake 作为房间键

	// 房间键优先用 nodeID；兼容旧版 lake-only 模式
	roomKey := nodeID
	if roomKey == "" {
		roomKey = lakeID
	}

	// P6-B 鉴权：JWT 必须有效；user 来自 token claims
	var userID, rawToken string
	if h.jwt != nil {
		tok := r.URL.Query().Get("token")
		if tok == "" {
			// 兼容 Authorization 头
			if ah := r.Header.Get("Authorization"); strings.HasPrefix(ah, "Bearer ") {
				tok = strings.TrimPrefix(ah, "Bearer ")
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
		rawToken = tok
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

	p := &peer{conn: c, id: newRandomID(), nodeID: roomKey, lakeID: lakeID, userID: userID, token: rawToken}
	result := h.join(p)
	defer h.leave(p)
	log.Printf("yjs-bridge: peer joined node=%s lake=%s user=%s first=%v", roomKey, lakeID, userID, result.isFirst)

	// P8-D：首个对端接入时加载快照并推送
	if result.isFirst && nodeID != "" && rawToken != "" {
		// 使用带超时的子 context，避免快照加载阻塞 WS 握手
		snapCtx, snapCancel := context.WithTimeout(r.Context(), 5*time.Second)
		snap, err := h.loadSnapshot(snapCtx, nodeID, rawToken)
		snapCancel()
		if err != nil {
			log.Printf("yjs-bridge: load snapshot node=%s err=%v (ignored)", nodeID, err)
		} else if len(snap) > 0 {
			frame := wrapSyncStep2(snap)
			ctx2, cancel := context.WithTimeout(r.Context(), 3*time.Second)
			if werr := c.Write(ctx2, websocket.MessageBinary, frame); werr != nil {
				log.Printf("yjs-bridge: send snapshot node=%s err=%v", nodeID, werr)
			}
			cancel()
		}
	}

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

// websocketOriginPatterns converts configured CORS origins into nhooyr host
// patterns. nhooyr websocket AcceptOptions.OriginPatterns matches only origin
// hosts (for example "fn.cky:14173"), not full URLs.
func websocketOriginPatterns(origins []string) []string {
	patterns := make([]string, 0, len(origins))
	for _, origin := range origins {
		origin = strings.TrimSpace(origin)
		if origin == "" {
			continue
		}
		if u, err := url.Parse(origin); err == nil && u.Host != "" {
			patterns = append(patterns, u.Host)
			continue
		}
		patterns = append(patterns, origin)
	}
	return patterns
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

	// P8-D：Ripple 后端 URL（用于加载 Y.Doc 快照）
	apiURL := strings.TrimRight(os.Getenv("YJS_BRIDGE_API_URL"), "/")

	// P9-A：Redis Pub/Sub（多实例广播）
	var rdb *redis.Client
	instanceID := newRandomID()
	if redisURL := os.Getenv("YJS_BRIDGE_REDIS_URL"); redisURL != "" {
		opt, err := redis.ParseURL(redisURL)
		if err != nil {
			log.Fatalf("yjs-bridge: invalid YJS_BRIDGE_REDIS_URL: %v", err)
		}
		rdb = redis.NewClient(opt)
		pingCtx, pingCancel := context.WithTimeout(context.Background(), 3*time.Second)
		if err := rdb.Ping(pingCtx).Err(); err != nil {
			log.Fatalf("yjs-bridge: redis ping failed: %v", err)
		}
		pingCancel()
		log.Printf("yjs-bridge: redis pub/sub enabled (instance=%s)", instanceID)
	}

	h := newHub(jwtSigner, originsList, apiURL, rdb, instanceID)
	mux := http.NewServeMux()
	mux.HandleFunc("/yjs", h.handleWS)
	mux.HandleFunc("/yjs/", h.handleWS) // P8-E：y-websocket params 选项会追加 /<roomName> 到 URL 路径
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("ok")) })
	mux.HandleFunc("/stats", h.stats)

	srv := &http.Server{Addr: addr, Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	authStatus := "DISABLED"
	if jwtSigner != nil {
		authStatus = "ENABLED"
	}
	apiStatus := "DISABLED"
	if apiURL != "" {
		apiStatus = apiURL
	}
	redisStatus := "DISABLED"
	if rdb != nil {
		redisStatus = "ENABLED (instance=" + instanceID + ")"
	}
	log.Printf("yjs-bridge listening on %s (auth=%s, origins=%v, api=%s, redis=%s)", addr, authStatus, originsList, apiStatus, redisStatus)

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
	if rdb != nil {
		_ = rdb.Close()
	}
}
