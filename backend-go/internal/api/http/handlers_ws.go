package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/chenqilscy/ripple/backend-go/internal/presence"
	"github.com/chenqilscy/ripple/backend-go/internal/realtime"
	"github.com/chenqilscy/ripple/backend-go/internal/service"
	"github.com/go-chi/chi/v5"
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
		OriginPatterns:     h.Origins,
		InsecureSkipVerify: len(h.Origins) == 0,
	})
	if err != nil {
		return // websocket.Accept 已写过响应
	}
	defer conn.Close(websocket.StatusInternalError, "server closed")

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

	// 写出 goroutine：从 broker 推到客户端。
	go func() {
		for msg := range ch {
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
		}
	}()

	// 写一个 hello 让客户端确认建立成功。
	hello := realtime.Message{Type: "hello", Payload: map[string]any{
		"user_id": user.ID, "lake_id": lakeID, "ts": time.Now().Unix(),
	}}
	if b, err := json.Marshal(hello); err == nil {
		_ = conn.Write(ctx, websocket.MessageText, b)
	}

	// 读循环：消费 ping/控制帧。客户端断开时退出。
	for {
		_, _, err := conn.Read(ctx)
		if err != nil {
			cancel()
			conn.Close(websocket.StatusNormalClosure, "client gone")
			return
		}
		// 客户端任意消息都视为心跳；刷新 presence score。
		if h.Presence != nil {
			_ = h.Presence.Heartbeat(ctx, lakeID, user.ID)
		}
	}
}
