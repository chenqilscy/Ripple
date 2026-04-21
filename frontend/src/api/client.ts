// apiClient · 与 Ripple Go 后端通信的轻量封装。
// 持久化 access_token 到 localStorage（key=ripple.token），
// 401 时清除 token 并触发 onUnauthorized 回调（由上层 UI 路由到登录页）。

import type {
  ApiError, AuthTokens, CloudTask, EdgeItem, EdgeKind, InviteItem, InvitePreview,
  Lake, NodeItem, NodeType, User,
} from './types'

const BASE = (import.meta.env.VITE_API_BASE as string | undefined) ?? 'http://localhost:8000'
const TOKEN_KEY = 'ripple.token'

let onUnauthorizedCb: (() => void) | null = null
export function onUnauthorized(cb: () => void) { onUnauthorizedCb = cb }

export function getToken(): string | null { return localStorage.getItem(TOKEN_KEY) }
export function setToken(tok: string | null) {
  if (tok) localStorage.setItem(TOKEN_KEY, tok)
  else localStorage.removeItem(TOKEN_KEY)
}

async function request<T>(method: string, path: string, body?: unknown): Promise<T> {
  const headers: Record<string, string> = { 'Content-Type': 'application/json' }
  const tok = getToken()
  if (tok) headers.Authorization = `Bearer ${tok}`
  const res = await fetch(`${BASE}${path}`, {
    method, headers, body: body !== undefined ? JSON.stringify(body) : undefined,
  })
  if (res.status === 401) {
    setToken(null)
    onUnauthorizedCb?.()
  }
  const text = await res.text()
  let data: unknown = null
  if (text) {
    try { data = JSON.parse(text) } catch { /* non-json */ }
  }
  if (!res.ok) {
    const err: ApiError = Object.assign(new Error(
      (data as { message?: string })?.message ?? `HTTP ${res.status}`,
    ), { status: res.status, code: (data as { code?: string })?.code })
    throw err
  }
  return data as T
}

export const api = {
  // ---- Auth ----
  register(email: string, password: string, display_name: string): Promise<User> {
    return request('POST', '/api/v1/auth/register', { email, password, display_name })
  },
  login(email: string, password: string): Promise<AuthTokens> {
    return request<AuthTokens>('POST', '/api/v1/auth/login', { email, password })
      .then(t => { setToken(t.access_token); return t })
  },
  logout() { setToken(null) },

  // ---- Lakes ----
  listLakes(): Promise<{ lakes: Lake[] }> {
    return request('GET', '/api/v1/lakes')
  },
  createLake(name: string, description: string, is_public = false): Promise<Lake> {
    return request('POST', '/api/v1/lakes', { name, description, is_public })
  },
  getLake(id: string): Promise<Lake> {
    return request('GET', `/api/v1/lakes/${id}`)
  },
  listNodes(lakeId: string, includeVapor = false): Promise<{ nodes: NodeItem[] }> {
    return request('GET', `/api/v1/lakes/${lakeId}/nodes?include_vapor=${includeVapor}`)
  },

  // ---- Nodes ----
  createNode(lake_id: string, content: string, type: NodeType = 'TEXT'): Promise<NodeItem> {
    return request('POST', '/api/v1/nodes', { lake_id, content, type })
  },
  getNode(id: string): Promise<NodeItem> {
    return request('GET', `/api/v1/nodes/${id}`)
  },
  evaporateNode(id: string): Promise<NodeItem> {
    return request('POST', `/api/v1/nodes/${id}/evaporate`)
  },
  restoreNode(id: string): Promise<NodeItem> {
    return request('POST', `/api/v1/nodes/${id}/restore`)
  },
  condenseNode(id: string, lake_id?: string): Promise<NodeItem> {
    return request('POST', `/api/v1/nodes/${id}/condense`, lake_id ? { lake_id } : {})
  },

  // ---- Clouds ----
  generateCloud(lake_id: string, prompt: string, n = 5, type: NodeType = 'TEXT'): Promise<CloudTask> {
    return request('POST', '/api/v1/clouds', { lake_id, prompt, n, type })
  },
  getCloud(id: string): Promise<CloudTask> {
    return request('GET', `/api/v1/clouds/${id}`)
  },
  listClouds(): Promise<{ tasks: CloudTask[] }> {
    return request('GET', '/api/v1/clouds')
  },

  // ---- Edges ----
  listEdges(lakeId: string, includeDeleted = false): Promise<{ edges: EdgeItem[] }> {
    return request('GET', `/api/v1/lakes/${lakeId}/edges?include_deleted=${includeDeleted}`)
  },
  createEdge(src_node_id: string, dst_node_id: string, kind: EdgeKind, label?: string): Promise<EdgeItem> {
    return request('POST', '/api/v1/edges', { src_node_id, dst_node_id, kind, label })
  },
  deleteEdge(id: string): Promise<void> {
    return request('DELETE', `/api/v1/edges/${id}`)
  },

  // ---- Invites ----
  createInvite(lakeId: string, role: 'NAVIGATOR' | 'PASSENGER' | 'OBSERVER', max_uses: number, ttl_seconds: number): Promise<InviteItem> {
    return request('POST', `/api/v1/lakes/${lakeId}/invites`, { role, max_uses, ttl_seconds })
  },
  listInvites(lakeId: string, includeInactive = false): Promise<{ invites: InviteItem[] }> {
    return request('GET', `/api/v1/lakes/${lakeId}/invites?include_inactive=${includeInactive}`)
  },
  revokeInvite(inviteId: string): Promise<void> {
    return request('DELETE', `/api/v1/lake-invites/${inviteId}`)
  },
  previewInvite(token: string): Promise<InvitePreview> {
    return request('GET', `/api/v1/invites/preview?token=${encodeURIComponent(token)}`)
  },
  acceptInvite(token: string): Promise<{ lake_id: string; role: string; already_member: boolean }> {
    return request('POST', '/api/v1/invites/accept', { token })
  },
}

// 重新导出 types
export * from './types'
