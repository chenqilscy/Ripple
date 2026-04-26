// apiClient · 与 Ripple Go 后端通信的轻量封装。
// 持久化 access_token 到 localStorage（key=ripple.token），
// 401 时清除 token 并触发 onUnauthorized 回调（由上层 UI 路由到登录页）。
const BASE = import.meta.env.VITE_API_BASE ?? 'http://localhost:8000';
const TOKEN_KEY = 'ripple.token';
let onUnauthorizedCb = null;
export function onUnauthorized(cb) { onUnauthorizedCb = cb; }
export function getToken() { return localStorage.getItem(TOKEN_KEY); }
export function setToken(tok) {
    if (tok)
        localStorage.setItem(TOKEN_KEY, tok);
    else
        localStorage.removeItem(TOKEN_KEY);
}
async function request(method, path, body) {
    const headers = { 'Content-Type': 'application/json' };
    const tok = getToken();
    if (tok)
        headers.Authorization = `Bearer ${tok}`;
    const res = await fetch(`${BASE}${path}`, {
        method, headers, body: body !== undefined ? JSON.stringify(body) : undefined,
    });
    if (res.status === 401) {
        setToken(null);
        onUnauthorizedCb?.();
    }
    const text = await res.text();
    let data = null;
    if (text) {
        try {
            data = JSON.parse(text);
        }
        catch { /* non-json */ }
    }
    if (!res.ok) {
        const err = Object.assign(new Error(data?.message ?? `HTTP ${res.status}`), { status: res.status, code: data?.code });
        throw err;
    }
    return data;
}
export const api = {
    // ---- Auth ----
    register(email, password, display_name) {
        return request('POST', '/api/v1/auth/register', { email, password, display_name });
    },
    login(email, password) {
        return request('POST', '/api/v1/auth/login', { email, password })
            .then(t => { setToken(t.access_token); return t; });
    },
    logout() { setToken(null); },
    me() {
        return request('GET', '/api/v1/auth/me');
    },
    // ---- Lakes ----
    listLakes(spaceId) {
        const q = spaceId ? `?space_id=${encodeURIComponent(spaceId)}` : '';
        return request('GET', `/api/v1/lakes${q}`);
    },
    createLake(name, description, is_public = false, space_id) {
        return request('POST', '/api/v1/lakes', { name, description, is_public, space_id });
    },
    getLake(id) {
        return request('GET', `/api/v1/lakes/${id}`);
    },
    listNodes(lakeId, includeVapor = false) {
        return request('GET', `/api/v1/lakes/${lakeId}/nodes?include_vapor=${includeVapor}`);
    },
    // ---- Nodes ----
    createNode(lake_id, content, type = 'TEXT') {
        return request('POST', '/api/v1/nodes', { lake_id, content, type });
    },
    getNode(id) {
        return request('GET', `/api/v1/nodes/${id}`);
    },
    evaporateNode(id) {
        return request('POST', `/api/v1/nodes/${id}/evaporate`);
    },
    restoreNode(id) {
        return request('POST', `/api/v1/nodes/${id}/restore`);
    },
    condenseNode(id, lake_id) {
        return request('POST', `/api/v1/nodes/${id}/condense`, lake_id ? { lake_id } : {});
    },
    // ---- Clouds ----
    generateCloud(lake_id, prompt, n = 5, type = 'TEXT') {
        return request('POST', '/api/v1/clouds', { lake_id, prompt, n, type });
    },
    getCloud(id) {
        return request('GET', `/api/v1/clouds/${id}`);
    },
    listClouds() {
        return request('GET', '/api/v1/clouds');
    },
    // ---- Edges ----
    listEdges(lakeId, includeDeleted = false) {
        return request('GET', `/api/v1/lakes/${lakeId}/edges?include_deleted=${includeDeleted}`);
    },
    createEdge(src_node_id, dst_node_id, kind, label) {
        return request('POST', '/api/v1/edges', { src_node_id, dst_node_id, kind, label });
    },
    deleteEdge(id) {
        return request('DELETE', `/api/v1/edges/${id}`);
    },
    // ---- Invites ----
    createInvite(lakeId, role, max_uses, ttl_seconds) {
        return request('POST', `/api/v1/lakes/${lakeId}/invites`, { role, max_uses, ttl_seconds });
    },
    listInvites(lakeId, includeInactive = false) {
        return request('GET', `/api/v1/lakes/${lakeId}/invites?include_inactive=${includeInactive}`);
    },
    revokeInvite(inviteId) {
        return request('DELETE', `/api/v1/lake-invites/${inviteId}`);
    },
    previewInvite(token) {
        return request('GET', `/api/v1/invites/preview?token=${encodeURIComponent(token)}`);
    },
    acceptInvite(token) {
        return request('POST', '/api/v1/invites/accept', { token });
    },
    // ---- Presence ----
    listPresence(lakeId) {
        return request('GET', `/api/v1/lakes/${lakeId}/presence`);
    },
    // ---- Node revisions (F3) ----
    updateNodeContent(id, content, edit_reason) {
        return request('PUT', `/api/v1/nodes/${id}/content`, { content, edit_reason: edit_reason ?? '' });
    },
    listNodeRevisions(id, limit = 50) {
        return request('GET', `/api/v1/nodes/${id}/revisions?limit=${limit}`);
    },
    rollbackNode(id, target_rev_number) {
        return request('POST', `/api/v1/nodes/${id}/rollback`, { target_rev_number });
    },
    // ---- Spaces (M3-S1) ----
    listSpaces() {
        return request('GET', '/api/v1/spaces');
    },
    createSpace(name, description = '') {
        return request('POST', '/api/v1/spaces', { name, description });
    },
    getSpace(id) {
        return request('GET', `/api/v1/spaces/${id}`);
    },
    updateSpace(id, name, description = '') {
        return request('PATCH', `/api/v1/spaces/${id}`, { name, description });
    },
    deleteSpace(id) {
        return request('DELETE', `/api/v1/spaces/${id}`);
    },
    listSpaceMembers(id) {
        return request('GET', `/api/v1/spaces/${id}/members`);
    },
    addSpaceMember(id, user_id, role) {
        return request('POST', `/api/v1/spaces/${id}/members`, { user_id, role });
    },
    removeSpaceMember(id, userId) {
        return request('DELETE', `/api/v1/spaces/${id}/members/${userId}`);
    },
    // ---- Crystallize (M3-S2) ----
    crystallize(lake_id, source_node_ids, title_hint = '') {
        return request('POST', '/api/v1/perma_nodes', { lake_id, source_node_ids, title_hint });
    },
    // ---- Lake Move (M3 T7) ----
    moveLake(lakeID, target_space_id) {
        return request('PATCH', `/api/v1/lakes/${lakeID}/space`, { space_id: target_space_id });
    },
    // ---- Recommendations / Feedback (M3-S3) ----
    recommend(target_type, limit = 10) {
        return request('GET', `/api/v1/recommendations?target_type=${encodeURIComponent(target_type)}&limit=${limit}`);
    },
    sendFeedback(target_type, target_id, event_type, payload = '') {
        return request('POST', '/api/v1/feedback', { target_type, target_id, event_type, payload });
    },
    // ---- Attachments (M4-B 本地 FS) ----
    async uploadAttachment(file, nodeId) {
        const fd = new FormData();
        fd.append('file', file);
        if (nodeId)
            fd.append('node_id', nodeId);
        const tok = getToken();
        const resp = await fetch(`${BASE}/api/v1/attachments`, {
            method: 'POST',
            headers: tok ? { Authorization: `Bearer ${tok}` } : {},
            body: fd,
        });
        if (!resp.ok)
            throw new Error(`HTTP ${resp.status}: ${await resp.text()}`);
        return resp.json();
    },
    attachmentURL(id) {
        return `${BASE}/api/v1/attachments/${id}`;
    },
    // ---- API Keys (P10-A) ----
    createAPIKey(name, scopes) {
        return request('POST', '/api/v1/api_keys', { name, scopes: scopes ?? ['read_lake', 'read_node'] });
    },
    listAPIKeys() {
        return request('GET', '/api/v1/api_keys');
    },
    revokeAPIKey(id) {
        return request('DELETE', `/api/v1/api_keys/${id}`);
    },
    // ---- Audit Logs (P10-B) ----
    listAuditLogs(resourceType, resourceId, limit = 50) {
        const q = new URLSearchParams({ resource_type: resourceType, resource_id: resourceId, limit: String(limit) });
        return request('GET', `/api/v1/audit_logs?${q}`);
    },
    // ---- Lake Members (P11-C) ----
    listLakeMembers(lakeId) {
        return request('GET', `/api/v1/lakes/${lakeId}/members`);
    },
    updateMemberRole(lakeId, userId, role) {
        return request('PUT', `/api/v1/lakes/${lakeId}/members/${userId}/role`, { role });
    },
    removeLakeMember(lakeId, userId) {
        return request('DELETE', `/api/v1/lakes/${lakeId}/members/${userId}`);
    },
    // ---- Full-text Search (P12-D) ----
    searchNodes(q, lakeId, limit = 20) {
        const params = new URLSearchParams({ q, lake_id: lakeId, limit: String(limit) });
        return request('GET', `/api/v1/search?${params}`);
    },
    // ---- Batch Import (P12-A) ----
    batchImportNodes(lakeId, nodes) {
        return request('POST', `/api/v1/lakes/${lakeId}/nodes/batch`, { nodes });
    },
    // ---- Organizations (P12-C) ----
    createOrg(name, slug, description) {
        return request('POST', '/api/v1/organizations', { name, slug, description: description ?? '' });
    },
    listOrgs() {
        return request('GET', '/api/v1/organizations');
    },
    getOrg(id) {
        return request('GET', `/api/v1/organizations/${id}`);
    },
    listOrgMembers(orgId) {
        return request('GET', `/api/v1/organizations/${orgId}/members`);
    },
    addOrgMember(orgId, userId, role) {
        return request('POST', `/api/v1/organizations/${orgId}/members`, { user_id: userId, role });
    },
    // P12-C：按 email 邀请已注册用户加入组织
    addOrgMemberByEmail(orgId, email, role) {
        return request('POST', `/api/v1/organizations/${orgId}/members/by_email`, { email, role });
    },
    updateOrgMemberRole(orgId, userId, role) {
        return request('PATCH', `/api/v1/organizations/${orgId}/members/${userId}/role`, { role });
    },
    removeOrgMember(orgId, userId) {
        return request('DELETE', `/api/v1/organizations/${orgId}/members/${userId}`);
    },
    // P13-A：湖归属组织
    setLakeOrg(lakeId, orgId) {
        return request('PATCH', `/api/v1/lakes/${lakeId}/org`, { org_id: orgId ?? '' });
    },
    listOrgLakes(orgId) {
        return request('GET', `/api/v1/organizations/${orgId}/lakes`);
    },
    // P13-C：标签系统
    getLakeTags(lakeId) {
        return request('GET', `/api/v1/lakes/${lakeId}/tags`);
    },
    getNodeTags(nodeId) {
        return request('GET', `/api/v1/nodes/${nodeId}/tags`);
    },
    setNodeTags(nodeId, tags) {
        return request('PUT', `/api/v1/nodes/${nodeId}/tags`, { tags });
    },
    listNodesByTag(lakeId, tag) {
        return request('GET', `/api/v1/lakes/${lakeId}/nodes/by_tag?tag=${encodeURIComponent(tag)}`);
    },
    // P13-D：内容导出
    async exportLake(lakeId, format) {
        const tok = getToken();
        const resp = await fetch(`${BASE}/api/v1/lakes/${lakeId}/export?format=${format}`, {
            headers: tok ? { Authorization: `Bearer ${tok}` } : {},
        });
        if (!resp.ok)
            throw new Error(`HTTP ${resp.status}`);
        const blob = await resp.blob();
        const url = URL.createObjectURL(blob);
        const a = document.createElement('a');
        a.href = url;
        a.download = `lake-${lakeId}.${format === 'json' ? 'json' : 'md'}`;
        document.body.appendChild(a);
        a.click();
        a.remove();
        URL.revokeObjectURL(url);
    },
    // P13-E：导入外部内容
    async importLake(lakeId, file) {
        const tok = getToken();
        const form = new FormData();
        form.append('file', file);
        const resp = await fetch(`${BASE}/api/v1/lakes/${lakeId}/import`, {
            method: 'POST',
            headers: tok ? { Authorization: `Bearer ${tok}` } : {},
            body: form,
        });
        if (!resp.ok) {
            const text = await resp.text().catch(() => '');
            throw new Error(`HTTP ${resp.status}: ${text}`);
        }
        return resp.json();
    },
    // P14-C：节点批量操作
    batchOperateNodes(lakeId, action, nodeIds) {
        return request('POST', `/api/v1/lakes/${lakeId}/nodes/batch_op`, { action, node_ids: nodeIds });
    },
    // P16-B：AI 节点摘要
    aiSummaryNode(nodeId) {
        return request('POST', `/api/v1/nodes/${nodeId}/ai_summary`);
    },
    // P13-B：通知系统
    listNotifications(limit = 20, before) {
        const q = new URLSearchParams({ limit: String(limit) });
        if (before !== undefined)
            q.set('before', String(before));
        return request('GET', `/api/v1/notifications?${q}`);
    },
    markNotificationRead(id) {
        return request('POST', `/api/v1/notifications/${id}/read`);
    },
    markAllNotificationsRead() {
        return request('POST', '/api/v1/notifications/read_all');
    },
    getUnreadNotificationCount() {
        return request('GET', '/api/v1/notifications/unread_count');
    },
    // ---- Weave Stream (SSE / M3 T4) ----
    // onEvent(eventName, payload) 回调；返回 abort 函数。
    // ---- P18-A：节点关联推荐 ----
    getRelatedNodes(nodeId, limit = 5) {
        return request('GET', `/api/v1/nodes/${nodeId}/related?limit=${limit}`);
    },
    // ---- P18-C：节点模板库 ----
    listTemplates() {
        return request('GET', '/api/v1/templates');
    },
    createTemplate(name, content, description, tags) {
        return request('POST', '/api/v1/templates', { name, content, description: description ?? '', tags: tags ?? [] });
    },
    deleteTemplate(id) {
        return request('DELETE', `/api/v1/templates/${id}`);
    },
    createNodeFromTemplate(lakeId, template_id) {
        return request('POST', `/api/v1/lakes/${lakeId}/nodes/from_template`, { template_id });
    },
    // ---- P18-D：图谱快照 ----
    createSnapshot(lakeId, name, layout) {
        return request('POST', `/api/v1/lakes/${lakeId}/snapshots`, { name, layout });
    },
    listSnapshots(lakeId) {
        return request('GET', `/api/v1/lakes/${lakeId}/snapshots`);
    },
    deleteSnapshot(lakeId, snapshotId) {
        return request('DELETE', `/api/v1/lakes/${lakeId}/snapshots/${snapshotId}`);
    },
    // ---- P18-B：节点外链分享 ----
    createNodeShare(nodeId, ttl_hours) {
        return request('POST', `/api/v1/nodes/${nodeId}/share`, ttl_hours ? { ttl_hours } : {});
    },
    listNodeShares(nodeId) {
        return request('GET', `/api/v1/nodes/${nodeId}/shares`);
    },
    revokeNodeShare(id) {
        return request('DELETE', `/api/v1/shares/${id}`);
    },
    // 公开访问（无鉴权）
    getSharedNode(token) {
        return request('GET', `/api/v1/share/${token}`);
    },
    streamWeave(lakeID, prompt, onEvent) {
        const ctrl = new AbortController();
        const tok = getToken();
        const url = `${BASE}/api/v1/lakes/${lakeID}/weave/stream?prompt=${encodeURIComponent(prompt)}`;
        fetch(url, {
            headers: tok ? { Authorization: `Bearer ${tok}` } : {},
            signal: ctrl.signal,
        }).then(async (resp) => {
            if (!resp.ok || !resp.body) {
                onEvent('error', { message: `HTTP ${resp.status}` });
                return;
            }
            const reader = resp.body.getReader();
            const dec = new TextDecoder();
            let buf = '';
            while (true) {
                const { done, value } = await reader.read();
                if (done)
                    break;
                buf += dec.decode(value, { stream: true });
                // 按 SSE 分隔符（双换行）切分
                let idx;
                while ((idx = buf.indexOf('\n\n')) !== -1) {
                    const raw = buf.slice(0, idx);
                    buf = buf.slice(idx + 2);
                    let event = 'message';
                    let dataStr = '';
                    for (const line of raw.split('\n')) {
                        if (line.startsWith('event: '))
                            event = line.slice(7).trim();
                        else if (line.startsWith('data: '))
                            dataStr += line.slice(6);
                    }
                    if (!dataStr)
                        continue;
                    try {
                        const data = JSON.parse(dataStr);
                        onEvent(event, data);
                    }
                    catch {
                        onEvent('error', { message: 'invalid sse json' });
                    }
                }
            }
        }).catch((e) => {
            if (e?.name !== 'AbortError')
                onEvent('error', { message: String(e?.message || e) });
        });
        return () => ctrl.abort();
    },
};
// 重新导出 types
export * from './types';
