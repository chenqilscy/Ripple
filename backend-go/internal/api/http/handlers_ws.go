package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/chenqilscy/ripple/backend-go/internal/metrics"
	"github.com/chenqilscy/ripple/backend-go/internal/presence"
	"github.com/chenqilscy/ripple/backend-go/internal/realtime"
	"github.com/chenqilscy/ripple/backend-go/internal/service"
	"github.com/go-chi/chi/v5"
	"golang.org/x/time/rate"
	"nhooyr.io/websocket"
)

// WSHandlers WebSocket 升级与广播。
type WSHandlers struct {
	Lakes    *service.LakeService
	Broker   realtime.Broker
	Presence *presence.Service // 可空
	Origins  []string          // CORS 白名单（用于 ws Origin 校验）
}

// LakeWS GET /api/v1/lakes/{id}/ws (鉴权后升级)
//
// 流程：
//   1. JWT 已在 AuthMiddleware 校验，CurrentUser 取出。
//   2. 校验 lake 可读（私有湖必须是成员）。
//   3. 升级到 WS，订阅 broker，把消息推送给客户端。
//   4. 同时读取客户端发来的消息（暂仅心跳/订阅控制）。
func (h *WSHandlers) LakeWS(w http.ResponseWriter, r *http.Request) {
	user, ok := CurrentUser(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "no user")
		return
	}
	lakeID := chi.URLParam(r, "id")

	// 权限：尝试读，不读取数据，只验证可见。
	if _, _, err := h.Lakes.Get(r.Context(), user, lakeID); err != nil {
		writeError(w, mapDomainError(err), err.Error())
		return
	}

	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		OriginPatterns:     websocketOriginPatterns(h.Origins),
		InsecureSkipVerify: len(h.Origins) == 0,
	})
	if err != nil {
		return // websocket.Accept 已写过响应
	}
	defer conn.Close(websocket.StatusInternalError, "server closed")

	metrics.WSConnections.Inc()
	defer metrics.WSConnections.Dec()

	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	// 注册到 presence（广播 presence.joined），断开时 Leave。
	if h.Presence != nil {
		if err := h.Presence.Join(ctx, lakeID, user.ID); err == nil {
			defer func() {
				// 用独立 ctx，避免 cancel 后 Leave 失败。
				leaveCtx, c := context.WithTimeout(context.Background(), 2*time.Second)
				defer c()
				_ = h.Presence.Leave(leaveCtx, lakeID, user.ID)
			}()
		}
	}

	topic := realtime.LakeTopic(lakeID)
	ch, err := h.Broker.Subscribe(ctx, topic)
	if err != nil {
		conn.Close(websocket.StatusInternalError, "broker subscribe failed")
		return
	}

	// P14-A：同时订阅用户个人通知 topic，实现实时推送。
	userTopic := realtime.UserTopic(user.ID)
	notifCh, err := h.Broker.Subscribe(ctx, userTopic)
	if err != nil {
		// 通知订阅失败不影响主流程，降级为空 channel。
		notifCh = make(chan realtime.Message)
	}

	// 合并两个 channel 到一个 send goroutine。
	merged := make(chan realtime.Message, 32)
	go func() {
		defer close(merged)
		for {
			select {
			case msg, ok := <-ch:
				if !ok {
					return
				}
				merged <- msg
			case msg, ok := <-notifCh:
				if !ok {
					return
				}
				merged <- msg
			case <-ctx.Done():
				return
			}
		}
	}()

	// 写出 goroutine：从 merged channel 推到客户端。
	go func() {
		for msg := range merged {
			b, err := json.Marshal(msg)
			if err != nil {
				continue
			}
			wctx, wcancel := context.WithTimeout(ctx, 5*time.Second)
			err = conn.Write(wctx, websocket.MessageText, b)
			wcancel()
			if err != nil {
				cancel()
				return
			}
			metrics.WSMessagesOut.Inc()
		}
	}()

	// 写一个 hello 让客户端确认建立成功。
	hello := realtime.Message{Type: "hello", Payload: map[string]any{
		"user_id": user.ID, "lake_id": lakeID, "ts": time.Now().Unix(),
	}}
	if b, err := json.Marshal(hello); err == nil {
		_ = conn.Write(ctx, websocket.MessageText, b)
	}

	// P19-C：连接级速率限制，防止恶意客户端绕过前端 throttle 高频广播（20 msg/s，burst=5）。
	cursorLimiter := rate.NewLimiter(rate.Every(50*time.Millisecond), 5)

	// 读循环：消费客户端消息（ping/cursor.move 等）。客户端断开时退出。
	for {
		_, raw, err := conn.Read(ctx)
		if err != nil {
			cancel()
			conn.Close(websocket.StatusNormalClosure, "client gone")
			return
		}
		// P19-C：先做大小检查，超限消息不计入心跳，避免被利用维持 presence。
		if len(raw) > 256 {
			continue
		}
		metrics.WSMessagesIn.Inc()
		// 客户端任意合法消息都视为心跳；刷新 presence score。
		if h.Presence != nil {
			_ = h.Presence.Heartbeat(ctx, lakeID, user.ID)
		}
		// P19-C：cursor.move 消息转发给同湖其他在线用户。
		var clientMsg struct {
			Type    string         `json:"type"`
			Payload map[string]any `json:"payload"`
		}
		if json.Unmarshal(raw, &clientMsg) != nil {
			continue
		}
		if clientMsg.Type == "cursor.move" {
			// 速率限制：超限直接丢弃，不广播。
			if !cursorLimiter.Allow() {
				continue
			}
			// 校验 x/y 在 [0,1] 范围，防止越界垃圾数据广播。
			if x, ok := clientMsg.Payload["x"].(float64); !ok || x < 0 || x > 1 {
				continue
			}
			if y, ok := clientMsg.Payload["y"].(float64); !ok || y < 0 || y > 1 {
				continue
			}
			// 后端注入 user_id，防止客户端伪造他人 ID。
			clientMsg.Payload["user_id"] = user.ID
			broadcast := realtime.Message{
				Type:    "cursor.move",
				Payload: clientMsg.Payload,
			}
			_ = h.Broker.Publish(ctx, topic, broadcast)
		}
	}
}

// websocketOriginPatterns converts configured CORS origins into nhooyr host
// patterns. chi/cors expects full origins such as "http://fn.cky:14173", while
// nhooyr websocket AcceptOptions.OriginPatterns expects only host patterns such
// as "fn.cky:14173". Passing full URLs makes valid browser WS requests fail
// with 403 on staging.
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
