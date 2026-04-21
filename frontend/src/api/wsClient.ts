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

export interface LakeMessage {
  type: string
  payload?: any
}

type Handler = (msg: LakeMessage) => void

export class LakeWS {
  private ws: WebSocket | null = null
  private retry = 0
  private closed = false
  private reconnectTimer: ReturnType<typeof setTimeout> | null = null
  private heartbeatTimer: ReturnType<typeof setInterval> | null = null

  constructor(
    private lakeId: string,
    private token: string,
    private onMessage: Handler,
    private onStatusChange?: (online: boolean) => void,
  ) {}

  connect() {
    if (this.closed) return
    const proto = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
    // 与 client.ts 保持一致：默认 dev 走 8000
    const host = (import.meta.env.VITE_API_HOST as string | undefined) ?? `${window.location.hostname}:8000`
    const url = `${proto}//${host}/api/v1/lakes/${encodeURIComponent(this.lakeId)}/ws?access_token=${encodeURIComponent(this.token)}`
    const ws = new WebSocket(url)
    this.ws = ws

    ws.onopen = () => {
      this.retry = 0
      this.onStatusChange?.(true)
      // 每 30s 发送心跳（后端任意帧都视为 heartbeat，用于续期 presence TTL=60s）。
      this.startHeartbeat()
    }
    ws.onmessage = ev => {
      try {
        const msg = JSON.parse(ev.data) as LakeMessage
        this.onMessage(msg)
      } catch {
        // 忽略非 JSON 帧
      }
    }
    ws.onclose = () => {
      this.onStatusChange?.(false)
      this.stopHeartbeat()
      this.ws = null
      if (!this.closed) this.scheduleReconnect()
    }
    ws.onerror = () => {
      // onclose 会跟着触发，统一在那里处理重连
    }
  }

  send(msg: LakeMessage) {
    if (this.ws && this.ws.readyState === WebSocket.OPEN) {
      try { this.ws.send(JSON.stringify(msg)) } catch { /* ignore */ }
    }
  }

  private startHeartbeat() {
    this.stopHeartbeat()
    this.heartbeatTimer = setInterval(() => this.send({ type: 'ping' }), 30_000)
  }

  private stopHeartbeat() {
    if (this.heartbeatTimer) {
      clearInterval(this.heartbeatTimer)
      this.heartbeatTimer = null
    }
  }

  private scheduleReconnect() {
    if (this.closed) return
    this.retry++
    // 指数退避，封顶 10s
    const delay = Math.min(1000 * 2 ** Math.min(this.retry, 4), 10_000)
    this.reconnectTimer = setTimeout(() => this.connect(), delay)
  }

  close() {
    this.closed = true
    this.stopHeartbeat()
    if (this.reconnectTimer) clearTimeout(this.reconnectTimer)
    if (this.ws) {
      try { this.ws.close() } catch { /* ignore */ }
      this.ws = null
    }
  }
}
