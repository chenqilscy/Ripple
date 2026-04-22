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
    // ---- Lakes ----
    listLakes() {
        return request('GET', '/api/v1/lakes');
    },
    createLake(name, description, is_public = false) {
        return request('POST', '/api/v1/lakes', { name, description, is_public });
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
};
// 重新导出 types
export * from './types';
