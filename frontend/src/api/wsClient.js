// 青萍 · WebSocket 客户端
//
// 与后端 /api/v1/lakes/{id}/ws 建连，自动重连，回调消息。
// 鉴权：浏览器 WS 不支持自定义 header，token 走 ?access_token= query。
//
// 用法：
//   const ws = new LakeWS(lakeId, token, msg => { ... })
//   ws.connect()
//   ...
//   ws.close()
export class LakeWS {
    lakeId;
    token;
    onMessage;
    onStatusChange;
    ws = null;
    retry = 0;
    closed = false;
    reconnectTimer = null;
    heartbeatTimer = null;
    constructor(lakeId, token, onMessage, onStatusChange) {
        this.lakeId = lakeId;
        this.token = token;
        this.onMessage = onMessage;
        this.onStatusChange = onStatusChange;
    }
    connect() {
        if (this.closed)
            return;
        const proto = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
        // 与 client.ts 保持一致：默认走同源 /api，交给 Vite/nginx 代理。
        const host = import.meta.env.VITE_API_HOST ?? window.location.host;
        const url = `${proto}//${host}/api/v1/lakes/${encodeURIComponent(this.lakeId)}/ws?access_token=${encodeURIComponent(this.token)}`;
        const ws = new WebSocket(url);
        this.ws = ws;
        ws.onopen = () => {
            this.retry = 0;
            this.onStatusChange?.(true);
            // 每 30s 发送心跳（后端任意帧都视为 heartbeat，用于续期 presence TTL=60s）。
            this.startHeartbeat();
        };
        ws.onmessage = ev => {
            try {
                const msg = JSON.parse(ev.data);
                this.onMessage(msg);
            }
            catch {
                // 忽略非 JSON 帧
            }
        };
        ws.onclose = () => {
            this.onStatusChange?.(false);
            this.stopHeartbeat();
            this.ws = null;
            if (!this.closed)
                this.scheduleReconnect();
        };
        ws.onerror = () => {
            // onclose 会跟着触发，统一在那里处理重连
        };
    }
    send(msg) {
        if (this.ws && this.ws.readyState === WebSocket.OPEN) {
            try {
                this.ws.send(JSON.stringify(msg));
            }
            catch { /* ignore */ }
        }
    }
    startHeartbeat() {
        this.stopHeartbeat();
        this.heartbeatTimer = setInterval(() => this.send({ type: 'ping' }), 30_000);
    }
    stopHeartbeat() {
        if (this.heartbeatTimer) {
            clearInterval(this.heartbeatTimer);
            this.heartbeatTimer = null;
        }
    }
    scheduleReconnect() {
        if (this.closed)
            return;
        this.retry++;
        // 指数退避，封顶 10s
        const delay = Math.min(1000 * 2 ** Math.min(this.retry, 4), 10_000);
        this.reconnectTimer = setTimeout(() => this.connect(), delay);
    }
    close() {
        this.closed = true;
        this.stopHeartbeat();
        if (this.reconnectTimer)
            clearTimeout(this.reconnectTimer);
        if (this.ws) {
            try {
                this.ws.close();
            }
            catch { /* ignore */ }
            this.ws = null;
        }
    }
}
