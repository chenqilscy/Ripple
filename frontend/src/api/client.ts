// apiClient · 与 Ripple Go 后端通信的轻量封装。
// 持久化 access_token 到 localStorage（key=ripple.token），
// 401 时清除 token 并触发 onUnauthorized 回调（由上层 UI 路由到登录页）。

import type {
  APIKeyCreated, APIKeyItem, AuditLogItem,
  ApiError, AuthTokens, CloudTask, EdgeItem, EdgeKind, InviteItem, InvitePreview,
  Lake, NodeItem, NodeRevision, NodeType, PermaNode, Space, SpaceMember, SummarizeGraphResult, User,
} from './types'

const BASE = (import.meta.env.VITE_API_BASE as string | undefined) ?? ''
const TOKEN_KEY = 'ripple.token'

let onUnauthorizedCb: (() => void) | null = null
export function onUnauthorized(cb: () => void) { onUnauthorizedCb = cb }

const PLAN_DESCRIPTIONS: Record<string, string> = {
  free: '适合个人试用与轻量协作。',
  pro: '适合稳定使用 AI Workflow 的个人团队。',
  team: '适合多人协作、配额更高的正式团队。',
}

export function getToken(): string | null { return localStorage.getItem(TOKEN_KEY) }
export function setToken(tok: string | null) {
  if (tok) localStorage.setItem(TOKEN_KEY, tok)
  else localStorage.removeItem(TOKEN_KEY)
}

async function request<T>(method: string, path: string, body?: unknown, opts?: { signal?: AbortSignal }): Promise<T> {
  const headers: Record<string, string> = { 'Content-Type': 'application/json' }
  const tok = getToken()
  if (tok) headers.Authorization = `Bearer ${tok}`
  const res = await fetch(`${BASE}${path}`, {
    method, headers, body: body !== undefined ? JSON.stringify(body) : undefined, signal: opts?.signal,
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
      (data as { message?: string; error?: string })?.message ?? (data as { error?: string })?.error ?? `HTTP ${res.status}`,
    ), { status: res.status, code: (data as { code?: string })?.code })
    throw err
  }
  return data as T
}

function normalizeBillingCycle(cycle: string | undefined): import('./types').BillingCycle {
  return cycle === 'annual' ? 'yearly' : 'monthly'
}

function normalizePlan(plan: {
  id: string
  name?: string
  name_zh?: string
  price_cny_monthly: number
  price_cny_yearly?: number
  quotas: import('./types').PlanQuotas
}): import('./types').SubscriptionPlan {
  return {
    id: plan.id,
    name: plan.name ?? plan.name_zh ?? plan.id,
    description: PLAN_DESCRIPTIONS[plan.id] ?? '',
    price_cny_monthly: plan.price_cny_monthly,
    price_cny_yearly: plan.price_cny_yearly,
    quotas: plan.quotas,
  }
}

function normalizeSubscription(subscription: {
  id: string
  org_id: string
  plan_id: string
  status: import('./types').SubscriptionStatus
  billing_cycle?: string
  started_at?: string
  expires_at?: string | null
  created_at: string
} | null): import('./types').OrgSubscription | null {
  if (!subscription) return null
  return {
    id: subscription.id,
    org_id: subscription.org_id,
    plan_id: subscription.plan_id,
    status: subscription.status,
    billing_cycle: normalizeBillingCycle(subscription.billing_cycle),
    current_period_start: subscription.started_at ?? subscription.created_at,
    current_period_end: subscription.expires_at ?? subscription.started_at ?? subscription.created_at,
    created_at: subscription.created_at,
  }
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
  me(): Promise<User> {
    return request('GET', '/api/v1/auth/me')
  },

  // ---- Lakes ----
  listLakes(spaceId?: string): Promise<{ lakes: Lake[] }> {
    const q = spaceId ? `?space_id=${encodeURIComponent(spaceId)}` : ''
    return request('GET', `/api/v1/lakes${q}`)
  },
  createLake(name: string, description: string, is_public = false, space_id?: string): Promise<Lake> {
    return request('POST', '/api/v1/lakes', { name, description, is_public, space_id })
  },
  getLake(id: string): Promise<Lake> {
    return request('GET', `/api/v1/lakes/${id}`)
  },
  listNodes(lakeId: string, includeVapor = false, signal?: AbortSignal): Promise<{ nodes: NodeItem[] }> {
    return request('GET', `/api/v1/lakes/${lakeId}/nodes?include_vapor=${includeVapor}`, undefined, { signal })
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
  listEdges(lakeId: string, includeDeleted = false, signal?: AbortSignal): Promise<{ edges: EdgeItem[] }> {
    return request('GET', `/api/v1/lakes/${lakeId}/edges?include_deleted=${includeDeleted}`, undefined, { signal })
  },
  createEdge(src_node_id: string, dst_node_id: string, kind: EdgeKind, label?: string, signal?: AbortSignal, strength?: number): Promise<EdgeItem> {
    return request('POST', '/api/v1/edges', { src_node_id, dst_node_id, kind, label, strength }, { signal })
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

  // ---- Presence ----
  listPresence(lakeId: string): Promise<{ users: string[] }> {
    return request('GET', `/api/v1/lakes/${lakeId}/presence`)
  },

  // ---- Node revisions (F3) ----
  updateNodeContent(id: string, content: string, edit_reason?: string, version?: number): Promise<NodeItem> {
    return request('PUT', `/api/v1/nodes/${id}/content`, { content, edit_reason: edit_reason ?? '', version: version ?? 0 })
  },
  listNodeRevisions(id: string, limit = 50): Promise<{ revisions: NodeRevision[] }> {
    return request('GET', `/api/v1/nodes/${id}/revisions?limit=${limit}`)
  },
  rollbackNode(id: string, target_rev_number: number): Promise<NodeItem> {
    return request('POST', `/api/v1/nodes/${id}/rollback`, { target_rev_number })
  },

  // ---- Spaces (M3-S1) ----
  listSpaces(): Promise<{ spaces: Space[] }> {
    return request('GET', '/api/v1/spaces')
  },
  createSpace(name: string, description = ''): Promise<Space> {
    return request('POST', '/api/v1/spaces', { name, description })
  },
  getSpace(id: string): Promise<Space> {
    return request('GET', `/api/v1/spaces/${id}`)
  },
  updateSpace(id: string, name: string, description = ''): Promise<Space> {
    return request('PATCH', `/api/v1/spaces/${id}`, { name, description })
  },
  deleteSpace(id: string): Promise<void> {
    return request('DELETE', `/api/v1/spaces/${id}`)
  },
  listSpaceMembers(id: string): Promise<{ members: SpaceMember[] }> {
    return request('GET', `/api/v1/spaces/${id}/members`)
  },
  addSpaceMember(id: string, user_id: string, role: 'EDITOR' | 'VIEWER'): Promise<void> {
    return request('POST', `/api/v1/spaces/${id}/members`, { user_id, role })
  },
  removeSpaceMember(id: string, userId: string): Promise<void> {
    return request('DELETE', `/api/v1/spaces/${id}/members/${userId}`)
  },

  // ---- Crystallize (M3-S2) ----
  crystallize(lake_id: string, source_node_ids: string[], title_hint = ''): Promise<PermaNode> {
    return request('POST', '/api/v1/perma_nodes', { lake_id, source_node_ids, title_hint })
  },

  // ---- Lake Move (M3 T7) ----
  moveLake(lakeID: string, target_space_id: string): Promise<Lake> {
    return request('PATCH', `/api/v1/lakes/${lakeID}/space`, { space_id: target_space_id })
  },

  // ---- Recommendations / Feedback (M3-S3) ----
  recommend(target_type: string, limit = 10): Promise<{ recommendations: { target_id: string; score: number }[] }> {
    return request('GET', `/api/v1/recommendations?target_type=${encodeURIComponent(target_type)}&limit=${limit}`)
  },
  sendFeedback(target_type: string, target_id: string, event_type: string, payload = ''): Promise<{ id: string }> {
    return request('POST', '/api/v1/feedback', { target_type, target_id, event_type, payload })
  },

  // ---- Attachments (M4-B 本地 FS) ----
  async uploadAttachment(file: File, nodeId?: string): Promise<{ id: string; url: string; mime: string; size_bytes: number }> {
    const fd = new FormData()
    fd.append('file', file)
    if (nodeId) fd.append('node_id', nodeId)
    const tok = getToken()
    const resp = await fetch(`${BASE}/api/v1/attachments`, {
      method: 'POST',
      headers: tok ? { Authorization: `Bearer ${tok}` } : {},
      body: fd,
    })
    if (!resp.ok) throw new Error(`HTTP ${resp.status}: ${await resp.text()}`)
    return resp.json()
  },
  attachmentURL(id: string): string {
    return `${BASE}/api/v1/attachments/${id}`
  },

  // ---- API Keys (P10-A) ----
  createAPIKey(name: string, scopes?: string[], orgId?: string): Promise<APIKeyCreated> {
    return request('POST', '/api/v1/api_keys', { name, scopes: scopes ?? ['read_lake', 'read_node'], org_id: orgId ?? '' })
  },
  listAPIKeys(): Promise<{ keys: APIKeyItem[] }> {
    return request('GET', '/api/v1/api_keys')
  },
  revokeAPIKey(id: string): Promise<void> {
    return request('DELETE', `/api/v1/api_keys/${id}`)
  },

  // ---- Audit Logs (P10-B) ----
  listAuditLogs(resourceType: string, resourceId: string, limit = 50): Promise<{ logs: AuditLogItem[]; total: number }> {
    const q = new URLSearchParams({ resource_type: resourceType, resource_id: resourceId, limit: String(limit) })
    return request('GET', `/api/v1/audit_logs?${q}`)
  },

  // ---- Graylist (P14.3) ----
  listGraylist(): Promise<{ entries: import('./types').GraylistEntry[] }> {
    return request('GET', '/api/v1/admin/graylist')
  },
  getAdminOverview(): Promise<import('./types').AdminOverview> {
    return request('GET', '/api/v1/admin/overview')
  },
  upsertGraylist(email: string, note = ''): Promise<import('./types').GraylistEntry> {
    return request('POST', '/api/v1/admin/graylist', { email, note })
  },
  deleteGraylist(id: string): Promise<void> {
    return request('DELETE', `/api/v1/admin/graylist/${id}`)
  },

  // ---- Platform Admins (P14.5) ----
  listPlatformAdmins(): Promise<{ admins: import('./types').PlatformAdmin[] }> {
    return request('GET', '/api/v1/admin/platform_admins')
  },
  grantPlatformAdmin(input: { user_id?: string; email?: string; role?: import('./types').PlatformAdminRole; note?: string }): Promise<import('./types').PlatformAdmin> {
    return request('POST', '/api/v1/admin/platform_admins', input)
  },
  revokePlatformAdmin(userId: string): Promise<void> {
    return request('DELETE', `/api/v1/admin/platform_admins/${encodeURIComponent(userId)}`)
  },

  // ---- Lake Members (P11-C) ----
  listLakeMembers(lakeId: string): Promise<{ members: import('./types').LakeMember[] }> {
    return request('GET', `/api/v1/lakes/${lakeId}/members`)
  },
  updateMemberRole(lakeId: string, userId: string, role: import('./types').LakeRole): Promise<void> {
    return request('PUT', `/api/v1/lakes/${lakeId}/members/${userId}/role`, { role })
  },
  removeLakeMember(lakeId: string, userId: string): Promise<void> {
    return request('DELETE', `/api/v1/lakes/${lakeId}/members/${userId}`)
  },

  // ---- Full-text Search (P12-D / P22) ----
  searchNodes(q: string, lakeId: string, limit = 20, state?: string, type?: string): Promise<{ results: import('./types').SearchHit[] }> {
    const params = new URLSearchParams({ q, lake_id: lakeId, limit: String(limit) })
    if (state) params.set('state', state)
    if (type) params.set('type', type)
    return request('GET', `/api/v1/search?${params}`)
  },

  // ---- Semantic Search (P20-C) ----
  semanticSearchNodes(q: string, lakeId: string, limit = 8): Promise<{ results: import('./types').SearchHit[] }> {
    const params = new URLSearchParams({ q, lake_id: lakeId, limit: String(limit), mode: 'semantic' })
    return request('GET', `/api/v1/semantic-search?${params}`)
  },

  // ---- Batch Import (P12-A) ----
  batchImportNodes(lakeId: string, nodes: { content: string; type?: string }[]): Promise<{ created: number; nodes: NodeItem[] }> {
    return request('POST', `/api/v1/lakes/${lakeId}/nodes/batch`, { nodes })
  },

  // ---- Organizations (P12-C) ----
  createOrg(name: string, slug: string, description?: string): Promise<import('./types').Organization> {
    return request('POST', '/api/v1/organizations', { name, slug, description: description ?? '' })
  },
  listOrgs(): Promise<{ organizations: import('./types').Organization[] }> {
    return request('GET', '/api/v1/organizations')
  },
  listOrgOverviews(): Promise<{ organizations: import('./types').OrgOverview[] }> {
    return request('GET', '/api/v1/organizations/overview')
  },
  getOrg(id: string): Promise<import('./types').Organization> {
    return request('GET', `/api/v1/organizations/${id}`)
  },
  getOrgOverview(orgId: string): Promise<import('./types').OrgOverview> {
    return request('GET', `/api/v1/organizations/${orgId}/overview`)
  },
  listOrgMembers(orgId: string): Promise<{ members: import('./types').OrgMember[] }> {
    return request('GET', `/api/v1/organizations/${orgId}/members`)
  },
  addOrgMember(orgId: string, userId: string, role: import('./types').OrgRole): Promise<void> {
    return request('POST', `/api/v1/organizations/${orgId}/members`, { user_id: userId, role })
  },
  // P12-C：按 email 邀请已注册用户加入组织
  addOrgMemberByEmail(orgId: string, email: string, role: import('./types').OrgRole): Promise<void> {
    return request('POST', `/api/v1/organizations/${orgId}/members/by_email`, { email, role })
  },
  updateOrgMemberRole(orgId: string, userId: string, role: import('./types').OrgRole): Promise<void> {
    return request('PATCH', `/api/v1/organizations/${orgId}/members/${userId}/role`, { role })
  },
  removeOrgMember(orgId: string, userId: string): Promise<void> {
    return request('DELETE', `/api/v1/organizations/${orgId}/members/${userId}`)
  },
  // P13-A：湖归属组织
  setLakeOrg(lakeId: string, orgId: string | null): Promise<Lake> {
    return request('PATCH', `/api/v1/lakes/${lakeId}/org`, { org_id: orgId ?? '' })
  },
  listOrgLakes(orgId: string): Promise<{ lakes: Lake[] }> {
    return request('GET', `/api/v1/organizations/${orgId}/lakes`)
  },
  getOrgQuota(orgId: string): Promise<import('./types').OrgQuota> {
    return request('GET', `/api/v1/organizations/${orgId}/quota`)
  },
  updateOrgQuota(orgId: string, patch: import('./types').OrgQuotaPatch): Promise<import('./types').OrgQuota> {
    return request('PATCH', `/api/v1/organizations/${orgId}/quota`, patch)
  },

  // P13-C：标签系统
  getLakeTags(lakeId: string): Promise<{ tags: string[] }> {
    return request('GET', `/api/v1/lakes/${lakeId}/tags`)
  },
  getNodeTags(nodeId: string): Promise<{ tags: string[] }> {
    return request('GET', `/api/v1/nodes/${nodeId}/tags`)
  },
  setNodeTags(nodeId: string, tags: string[]): Promise<{ tags: string[] }> {
    return request('PUT', `/api/v1/nodes/${nodeId}/tags`, { tags })
  },
  listNodesByTag(lakeId: string, tag: string): Promise<{ node_ids: string[] }> {
    return request('GET', `/api/v1/lakes/${lakeId}/nodes/by_tag?tag=${encodeURIComponent(tag)}`)
  },

  // P13-D：内容导出
  async exportLake(lakeId: string, format: 'json' | 'markdown'): Promise<void> {
    const tok = getToken()
    const resp = await fetch(`${BASE}/api/v1/lakes/${lakeId}/export?format=${format}`, {
      headers: tok ? { Authorization: `Bearer ${tok}` } : {},
    })
    if (!resp.ok) throw new Error(`HTTP ${resp.status}`)
    const blob = await resp.blob()
    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url
    a.download = `lake-${lakeId}.${format === 'json' ? 'json' : 'md'}`
    document.body.appendChild(a)
    a.click()
    a.remove()
    URL.revokeObjectURL(url)
  },

  // P13-E：导入外部内容
  async importLake(lakeId: string, file: File): Promise<{ imported: number; skipped: number }> {
    const tok = getToken()
    const form = new FormData()
    form.append('file', file)
    const resp = await fetch(`${BASE}/api/v1/lakes/${lakeId}/import`, {
      method: 'POST',
      headers: tok ? { Authorization: `Bearer ${tok}` } : {},
      body: form,
    })
    if (!resp.ok) {
      const text = await resp.text().catch(() => '')
      throw new Error(`HTTP ${resp.status}: ${text}`)
    }
    return resp.json() as Promise<{ imported: number; skipped: number }>
  },

  // P14-C：节点批量操作
  batchOperateNodes(lakeId: string, action: 'evaporate' | 'condense' | 'erase', nodeIds: string[]): Promise<{ succeeded: number; failed: number }> {
    return request('POST', `/api/v1/lakes/${lakeId}/nodes/batch_op`, { action, node_ids: nodeIds })
  },

  // P16-B：AI 节点摘要
  aiSummaryNode(nodeId: string): Promise<{ node_id: string; summary: string; created_at: string }> {
    return request('POST', `/api/v1/nodes/${nodeId}/ai_summary`)
  },

  // P13-B：通知系统
  listNotifications(limit = 20, before?: number): Promise<{ notifications: import('./types').Notification[] }> {
    const q = new URLSearchParams({ limit: String(limit) })
    if (before !== undefined) q.set('before', String(before))
    return request('GET', `/api/v1/notifications?${q}`)
  },
  markNotificationRead(id: number): Promise<void> {
    return request('POST', `/api/v1/notifications/${id}/read`)
  },
  markAllNotificationsRead(): Promise<void> {
    return request('POST', '/api/v1/notifications/read_all')
  },
  getUnreadNotificationCount(): Promise<{ count: number }> {
    return request('GET', '/api/v1/notifications/unread_count')
  },

  // ---- Weave Stream (SSE / M3 T4) ----
  // onEvent(eventName, payload) 回调；返回 abort 函数。
  // ---- P18-A：节点关联推荐 ----
  getRelatedNodes(nodeId: string, limit = 5, signal?: AbortSignal): Promise<{ related: import('./types').NodeSearchResult[] }> {
    return request('GET', `/api/v1/nodes/${nodeId}/related?limit=${limit}`, undefined, { signal })
  },

  // ---- P18-C：节点模板库 ----
  listTemplates(): Promise<{ templates: import('./types').NodeTemplate[] }> {
    return request('GET', '/api/v1/templates')
  },
  createTemplate(name: string, content: string, description?: string, tags?: string[]): Promise<import('./types').NodeTemplate> {
    return request('POST', '/api/v1/templates', { name, content, description: description ?? '', tags: tags ?? [] })
  },
  deleteTemplate(id: string): Promise<void> {
    return request('DELETE', `/api/v1/templates/${id}`)
  },
  createNodeFromTemplate(lakeId: string, template_id: string): Promise<import('./types').NodeItem> {
    return request('POST', `/api/v1/lakes/${lakeId}/nodes/from_template`, { template_id })
  },

  // ---- P18-D/P21：图谱快照 ----
  createSnapshot(lakeId: string, name: string, layout: Record<string, { x: number; y: number }>, graphState?: import('./types').SnapshotGraphState): Promise<import('./types').LakeSnapshot> {
    return request('POST', `/api/v1/lakes/${lakeId}/snapshots`, { name, layout, ...(graphState ? { graph_state: graphState } : {}) })
  },
  listSnapshots(lakeId: string): Promise<{ snapshots: import('./types').LakeSnapshot[] }> {
    return request('GET', `/api/v1/lakes/${lakeId}/snapshots`)
  },
  getSnapshot(lakeId: string, snapshotId: string): Promise<import('./types').LakeSnapshot> {
    return request('GET', `/api/v1/lakes/${lakeId}/snapshots/${snapshotId}`)
  },
  deleteSnapshot(lakeId: string, snapshotId: string): Promise<void> {
    return request('DELETE', `/api/v1/lakes/${lakeId}/snapshots/${snapshotId}`)
  },

  // ---- P18-B：节点外链分享 ----
  createNodeShare(nodeId: string, ttl_hours?: number): Promise<import('./types').NodeShare> {
    return request('POST', `/api/v1/nodes/${nodeId}/share`, ttl_hours ? { ttl_hours } : {})
  },
  listNodeShares(nodeId: string): Promise<{ shares: import('./types').NodeShare[] }> {
    return request('GET', `/api/v1/nodes/${nodeId}/shares`)
  },
  revokeNodeShare(id: string): Promise<void> {
    return request('DELETE', `/api/v1/shares/${id}`)
  },
  // 公开访问（无鉴权）
  getSharedNode(token: string): Promise<{ node: import('./types').NodeItem; share_id: string; expires_at: string | null }> {
    return request('GET', `/api/v1/share/${token}`)
  },

  // P19-A：AI 图谱探索
  exploreGraph(lakeId: string, query: string, maxNodes = 20): Promise<{
    relevant_nodes: Array<{ node_id: string; content: string; score: number }>
    summary: string
  }> {
    return request('POST', `/api/v1/lakes/${lakeId}/explore`, { query, max_nodes: maxNodes })
  },

  streamWeave(
    lakeID: string,
    prompt: string,
    onEvent: (event: 'delta' | 'done' | 'error', data: any) => void,
  ): () => void {
    const ctrl = new AbortController()
    const tok = getToken()
    const url = `${BASE}/api/v1/lakes/${lakeID}/weave/stream?prompt=${encodeURIComponent(prompt)}`
    fetch(url, {
      headers: tok ? { Authorization: `Bearer ${tok}` } : {},
      signal: ctrl.signal,
    }).then(async (resp) => {
      if (!resp.ok || !resp.body) {
        onEvent('error', { message: `HTTP ${resp.status}` })
        return
      }
      const reader = resp.body.getReader()
      const dec = new TextDecoder()
      let buf = ''
      while (true) {
        const { done, value } = await reader.read()
        if (done) break
        buf += dec.decode(value, { stream: true })
        // 按 SSE 分隔符（双换行）切分
        let idx
        while ((idx = buf.indexOf('\n\n')) !== -1) {
          const raw = buf.slice(0, idx)
          buf = buf.slice(idx + 2)
          let event = 'message'
          let dataStr = ''
          for (const line of raw.split('\n')) {
            if (line.startsWith('event: ')) event = line.slice(7).trim()
            else if (line.startsWith('data: ')) dataStr += line.slice(6)
          }
          if (!dataStr) continue
          try {
            const data = JSON.parse(dataStr)
            onEvent(event as any, data)
          } catch {
            onEvent('error', { message: 'invalid sse json' })
          }
        }
      }
    }).catch((e) => {
      if (e?.name !== 'AbortError') onEvent('error', { message: String(e?.message || e) })
    })
    return () => ctrl.abort()
  },

  // ---- Phase 15-A：Prompt 模板 ----
  listPromptTemplates(): Promise<{ items: import('./types').PromptTemplate[]; total: number }> {
    return request('GET', '/api/v1/prompt_templates')
  },
  createPromptTemplate(input: {
    name: string
    description?: string
    template: string
    scope?: import('./types').PromptScope
    org_id?: string
  }): Promise<import('./types').PromptTemplate> {
    return request('POST', '/api/v1/prompt_templates', {
      name: input.name,
      description: input.description ?? '',
      template: input.template,
      scope: input.scope ?? 'private',
      org_id: input.org_id ?? '',
    })
  },
  getPromptTemplate(id: string): Promise<import('./types').PromptTemplate> {
    return request('GET', `/api/v1/prompt_templates/${id}`)
  },
  updatePromptTemplate(id: string, patch: { name?: string; description?: string; template?: string }): Promise<import('./types').PromptTemplate> {
    return request('PATCH', `/api/v1/prompt_templates/${id}`, patch)
  },
  deletePromptTemplate(id: string): Promise<void> {
    return request('DELETE', `/api/v1/prompt_templates/${id}`)
  },

  // ---- Phase 15-B：订阅套餐 ----
  listSubscriptionPlans(): Promise<{ plans: import('./types').SubscriptionPlan[] }> {
    return request<{ plans: Array<{
      id: string
      name?: string
      name_zh?: string
      price_cny_monthly: number
      price_cny_yearly?: number
      quotas: import('./types').PlanQuotas
    }> }>('GET', '/api/v1/subscriptions/plans')
      .then(res => ({ plans: (res.plans ?? []).map(normalizePlan) }))
  },
  getOrgSubscription(orgId: string): Promise<{ subscription: import('./types').OrgSubscription | null }> {
    return request<{ subscription: {
      id: string
      org_id: string
      plan_id: string
      status: import('./types').SubscriptionStatus
      billing_cycle?: string
      started_at?: string
      expires_at?: string | null
      created_at: string
    } | null }>('GET', `/api/v1/organizations/${orgId}/subscription`)
      .then(res => ({ subscription: normalizeSubscription(res.subscription) }))
  },
  createOrgSubscription(orgId: string, plan_id: string, billing_cycle: import('./types').BillingCycle, stub_confirm = false): Promise<{ subscription: import('./types').OrgSubscription | null }> {
    return request<{ subscription: {
      id: string
      org_id: string
      plan_id: string
      status: import('./types').SubscriptionStatus
      billing_cycle?: string
      started_at?: string
      expires_at?: string | null
      created_at: string
    } | null }>('POST', `/api/v1/organizations/${orgId}/subscription`, {
      plan_id,
      billing_cycle: billing_cycle === 'yearly' ? 'annual' : 'monthly',
      stub_confirm,
    }).then(res => ({ subscription: normalizeSubscription(res.subscription) }))
  },
  // Phase 16: 真实用量
  getOrgUsage(orgId: string): Promise<{ usage: import('./types').OrgUsage }> {
    return request('GET', `/api/v1/organizations/${orgId}/usage`)
  },
  // Phase 15-D: AI 用量账单
  getOrgLLMUsage(orgId: string, days = 30): Promise<import('./types').OrgLLMUsage> {
    return request('GET', `/api/v1/organizations/${orgId}/llm_usage?days=${days}`)
  },

  // ---- Phase 15-C：AI Job 触发 ----
  aiTrigger(lakeId: string, nodeId: string, opts?: { prompt_template_id?: string; input_node_ids?: string[]; override_vars?: Record<string, string> }): Promise<import('./types').AiJob> {
    return request('POST', `/api/v1/lakes/${lakeId}/nodes/${nodeId}/ai_trigger`, opts ?? {})
  },
  aiStatus(lakeId: string, nodeId: string): Promise<import('./types').AiJob> {
    return request('GET', `/api/v1/lakes/${lakeId}/nodes/${nodeId}/ai_status`)
  },

  // ---- Phase 15-C.3：多节点 AI 整理 ----
  summarizeGraph(lakeId: string, nodeIds: string[], titleHint = ''): Promise<SummarizeGraphResult> {
    return request('POST', `/api/v1/lakes/${lakeId}/nodes/summarize`, { node_ids: nodeIds, title_hint: titleHint })
  },

  // ---- P20-A：自由文本一键转图谱 ----
  importText(lakeId: string, text: string, maxNodes = 20): Promise<import('./types').ImportTextResult> {
    return request('POST', `/api/v1/lakes/${lakeId}/import/text`, { text, max_nodes: maxNodes })
  },

  // ---- 图谱价值增强 ----
  // 路径追溯
  getPath(sourceId: string, targetId: string): Promise<import('./types').PathResult> {
    return request('POST', '/api/v1/graph/path', { source_id: sourceId, target_id: targetId })
  },

  // 聚类分析
  getClusters(lakeId: string): Promise<{ clusters: import('./types').Cluster[] }> {
    return request('GET', `/api/v1/lakes/${lakeId}/clusters`)
  },

  // 规划建议
  getPlanningSuggestions(lakeId: string): Promise<{ suggestions: import('./types').PlanningSuggestion[] }> {
    return request('GET', `/api/v1/lakes/${lakeId}/planning`)
  },
  acceptPlanningSuggestion(id: string): Promise<void> {
    return request('POST', `/api/v1/planning/${id}/accept`)
  },

  // 推荐列表
  getRecommendations(lakeId: string): Promise<{ recommendations: import('./types').Recommendation[] }> {
    return request('GET', `/api/v1/lakes/${lakeId}/recommendations`)
  },

  // 热度趋势
  getHeatTrend(lakeId: string, limit = 10): Promise<{
    heat_nodes: import('./types').HeatNode[]
    window_days: number
    computed_at: string
  }> {
    return request('GET', `/api/v1/lakes/${lakeId}/heat-trend?limit=${limit}`)
  },

  // 推荐操作
  acceptRecommendation(id: string, sourceNodeId: string, targetNodeId: string): Promise<{ id: string; status: string }> {
    return request('POST', `/api/v1/recommendations/${id}/accept`, {
      source_node_id: sourceNodeId,
      target_node_id: targetNodeId,
    })
  },
  rejectRecommendation(id: string): Promise<{ id: string; status: string }> {
    return request('POST', `/api/v1/recommendations/${id}/reject`)
  },
  ignoreRecommendation(id: string): Promise<{ id: string; status: string }> {
    return request('POST', `/api/v1/recommendations/${id}/ignore`)
  },
}

// 重新导出 types
export * from './types'

