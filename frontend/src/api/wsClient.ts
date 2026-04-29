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
  private deadlineTimer: ReturnType<typeof setTimeout> | null = null
  private static readonly HEARTBEAT_INTERVAL = 20_000 // 每20s发ping
  private static readonly DEADLINE_MS = 60_000        // 60s无消息→重连

  constructor(
    private lakeId: string,
    private token: string,
    private onMessage: Handler,
    private onStatusChange?: (online: boolean) => void,
  ) {}

  connect() {
    if (this.closed) return
    const proto = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
    // 与 client.ts 保持一致：默认走同源 /api，交给 Vite/nginx 代理。
    const host = (import.meta.env.VITE_API_HOST as string | undefined) ?? window.location.host
    const url = `${proto}//${host}/api/v1/lakes/${encodeURIComponent(this.lakeId)}/ws?access_token=${encodeURIComponent(this.token)}`
    const ws = new WebSocket(url)
    this.ws = ws

    ws.onopen = () => {
      this.retry = 0
      this.onStatusChange?.(true)
      this.startHeartbeat()
      this.resetDeadline()
    }
    ws.onmessage = ev => {
      this.resetDeadline() // 任意消息都重置超时计时器
      try {
        const msg = JSON.parse(ev.data) as LakeMessage
        if (msg.type === 'pong') return // pong 仅用于心跳确认，不上传给业务层
        this.onMessage(msg)
      } catch {
        // 忽略非 JSON 帧
      }
    }
    ws.onclose = () => {
      this.onStatusChange?.(false)
      this.stopHeartbeat()
      this.stopDeadline()
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
    this.heartbeatTimer = setInterval(() => this.send({ type: 'ping' }), LakeWS.HEARTBEAT_INTERVAL)
  }

  private stopHeartbeat() {
    if (this.heartbeatTimer) {
      clearInterval(this.heartbeatTimer)
      this.heartbeatTimer = null
    }
  }

  private resetDeadline() {
    this.stopDeadline()
    this.deadlineTimer = setTimeout(() => {
      // 60s 无任何服务端消息，主动关闭触发重连
      if (this.ws) {
        try { this.ws.close(4000, 'heartbeat timeout') } catch { /* ignore */ }
        this.ws = null
      }
      if (!this.closed) this.scheduleReconnect()
    }, LakeWS.DEADLINE_MS)
  }

  private stopDeadline() {
    if (this.deadlineTimer) {
      clearTimeout(this.deadlineTimer)
      this.deadlineTimer = null
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
    this.stopDeadline()
    if (this.reconnectTimer) clearTimeout(this.reconnectTimer)
    if (this.ws) {
      try { this.ws.close() } catch { /* ignore */ }
      this.ws = null
    }
  }
}
