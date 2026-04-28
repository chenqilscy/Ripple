import { jsx as _jsx, jsxs as _jsxs } from "react/jsx-runtime";
import { useEffect, useState } from 'react';
import { api } from '../api/client';
export default function PlatformAdminManager() {
    const [admins, setAdmins] = useState([]);
    const [loading, setLoading] = useState(false);
    const [err, setErr] = useState(null);
    const [forbidden, setForbidden] = useState(false);
    const [saving, setSaving] = useState(false);
    const [deletingId, setDeletingId] = useState(null);
    const [target, setTarget] = useState('');
    const [role, setRole] = useState('ADMIN');
    const [note, setNote] = useState('');
    async function load() {
        setLoading(true);
        setErr(null);
        try {
            const res = await api.listPlatformAdmins();
            setAdmins((res.admins ?? []).slice().sort((left, right) => new Date(right.created_at).getTime() - new Date(left.created_at).getTime()));
            setForbidden(false);
        }
        catch (e) {
            if (e?.status === 403) {
                setForbidden(true);
                setAdmins([]);
                setErr('仅平台 OWNER 可管理平台管理员');
            }
            else {
                setErr(e?.message ?? 'load failed');
            }
        }
        finally {
            setLoading(false);
        }
    }
    useEffect(() => { void load(); }, []);
    async function handleGrant() {
        const value = target.trim();
        if (!value || forbidden || saving || deletingId)
            return;
        if (role === 'OWNER' && !window.confirm(`确定授予 ${value} 平台 OWNER？OWNER 可继续授权或撤销平台管理员。`))
            return;
        setSaving(true);
        setErr(null);
        try {
            const input = platformAdminGrantInput(value, role, note);
            await api.grantPlatformAdmin(input);
            setTarget('');
            setNote('');
            await load();
        }
        catch (e) {
            setErr(e?.message ?? 'grant failed');
        }
        finally {
            setSaving(false);
        }
    }
    async function handleRevoke(admin) {
        if (forbidden || saving || deletingId)
            return;
        const message = platformAdminRevokeMessage(admin);
        if (!window.confirm(message))
            return;
        setDeletingId(admin.user_id);
        setErr(null);
        try {
            await api.revokePlatformAdmin(admin.user_id);
            setAdmins(prev => prev.filter(item => item.user_id !== admin.user_id));
        }
        catch (e) {
            setErr(e?.message ?? 'revoke failed');
        }
        finally {
            setDeletingId(null);
        }
    }
    return (_jsxs("div", { style: { padding: 16, maxWidth: 820, minWidth: 420, flex: '1 1 520px' }, children: [_jsx("h3", { style: { margin: '0 0 12px', color: '#cdd6f4' }, children: "\u5E73\u53F0\u7BA1\u7406\u5458 RBAC" }), _jsx("p", { style: { margin: '0 0 12px', color: '#6c7086', fontSize: 12, lineHeight: 1.5 }, children: "\u4EC5 OWNER \u53EF\u6388\u6743\u6216\u64A4\u9500\u5E73\u53F0\u7BA1\u7406\u5458\u3002\u73AF\u5883\u53D8\u91CF\u767D\u540D\u5355\u4ECD\u4F5C\u4E3A bootstrap OWNER \u515C\u5E95\uFF1BAPI Key \u4E0D\u53EF\u8C03\u7528\u6B64\u9762\u677F\u3002" }), _jsxs("div", { style: { display: 'flex', gap: 8, marginBottom: 16, flexWrap: 'wrap' }, children: [_jsx("input", { value: target, onChange: e => setTarget(e.target.value), onKeyDown: e => e.key === 'Enter' && void handleGrant(), placeholder: "\u7528\u6237 ID \u6216\u90AE\u7BB1", disabled: saving || !!deletingId || forbidden, style: { ...inputStyle, minWidth: 240, flex: '1 1 240px' } }), _jsxs("select", { value: role, onChange: e => setRole(e.target.value), disabled: saving || !!deletingId || forbidden, style: selectStyle, children: [_jsx("option", { value: "ADMIN", children: "ADMIN" }), _jsx("option", { value: "OWNER", children: "OWNER" })] }), _jsx("input", { value: note, onChange: e => setNote(e.target.value), onKeyDown: e => e.key === 'Enter' && void handleGrant(), placeholder: "\u6388\u6743\u5907\u6CE8", disabled: saving || !!deletingId || forbidden, style: { ...inputStyle, minWidth: 180, flex: '1 1 180px' } }), _jsx("button", { onClick: () => void handleGrant(), disabled: saving || !!deletingId || forbidden || !target.trim(), style: btnStyle('#a6e3a1'), children: saving ? '授权中…' : '+ 授权' })] }), err && _jsxs("p", { style: { color: forbidden ? '#f9e2af' : '#f38ba8', margin: '0 0 12px' }, children: ["\u26A0 ", err] }), forbidden ? (_jsx("p", { style: { color: '#6c7086', margin: 0 }, children: "\u5F53\u524D\u8D26\u53F7\u4E0D\u662F\u5E73\u53F0 OWNER\uFF0C\u53EA\u80FD\u67E5\u770B\u6B64\u8BF4\u660E\u3002" })) : loading ? (_jsx("p", { style: { color: '#6c7086' }, children: "\u52A0\u8F7D\u4E2D\u2026" })) : admins.length === 0 ? (_jsx("p", { style: { color: '#6c7086' }, children: "\u6682\u65E0\u6570\u636E\u5E93\u5E73\u53F0\u7BA1\u7406\u5458\uFF1B\u5F53\u524D\u53EF\u80FD\u4EC5\u4F9D\u8D56\u73AF\u5883\u53D8\u91CF bootstrap OWNER\u3002" })) : (_jsxs("table", { style: { width: '100%', borderCollapse: 'collapse', fontSize: 13 }, children: [_jsx("thead", { children: _jsxs("tr", { style: { color: '#6c7086', textAlign: 'left' }, children: [_jsx("th", { style: thStyle, children: "\u7528\u6237" }), _jsx("th", { style: thStyle, children: "\u89D2\u8272" }), _jsx("th", { style: thStyle, children: "\u5907\u6CE8" }), _jsx("th", { style: thStyle, children: "\u6388\u6743\u4EBA" }), _jsx("th", { style: thStyle, children: "\u6388\u6743\u65F6\u95F4" }), _jsx("th", { style: thStyle })] }) }), _jsx("tbody", { children: admins.map(admin => (_jsxs("tr", { style: { borderBottom: '1px solid #313244' }, children: [_jsx("td", { style: { ...tdStyle, color: '#89dceb' }, title: admin.user_id, children: admin.email || shortID(admin.user_id) }), _jsx("td", { style: { ...tdStyle, color: admin.role === 'OWNER' ? '#f9e2af' : '#a6e3a1' }, children: admin.role }), _jsx("td", { style: tdStyle, children: admin.note || '—' }), _jsx("td", { style: { ...tdStyle, color: '#6c7086' }, children: shortID(admin.created_by) }), _jsx("td", { style: { ...tdStyle, color: '#6c7086' }, children: fmtDate(admin.created_at) }), _jsx("td", { style: tdStyle, children: _jsx("button", { onClick: () => void handleRevoke(admin), disabled: saving || !!deletingId, style: btnStyle('#f38ba8', true), children: deletingId === admin.user_id ? '撤销中…' : '撤销' }) })] }, admin.user_id))) })] }))] }));
}
function fmtDate(s) {
    return new Date(s).toLocaleString('zh-CN', { month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit' });
}
export function platformAdminGrantInput(value, role, note) {
    const normalized = value.trim();
    return normalized.includes('@')
        ? { email: normalized.toLowerCase(), role, note: note.trim() }
        : { user_id: normalized, role, note: note.trim() };
}
export function platformAdminRevokeMessage(admin) {
    const label = admin.email || admin.user_id;
    return admin.role === 'OWNER'
        ? `确定撤销平台 OWNER ${label}？这会移除其平台管理员授权能力。`
        : `确定撤销平台管理员 ${label}？`;
}
function shortID(s) {
    return s ? `${s.slice(0, 8)}…` : '—';
}
function btnStyle(color, small = false) {
    return {
        background: 'transparent', border: `1px solid ${color}`, color,
        borderRadius: 4, padding: small ? '2px 8px' : '5px 12px',
        cursor: 'pointer', fontSize: small ? 12 : 13,
    };
}
const inputStyle = {
    background: '#1e1e2e', border: '1px solid #45475a', borderRadius: 4,
    color: '#cdd6f4', padding: '5px 10px', fontSize: 13,
};
const selectStyle = {
    background: '#1e1e2e', border: '1px solid #45475a', borderRadius: 4,
    color: '#cdd6f4', padding: '5px 10px', fontSize: 13, minWidth: 120,
};
const thStyle = {
    padding: '6px 8px', fontWeight: 500, borderBottom: '1px solid #313244',
};
const tdStyle = {
    padding: '8px 8px', color: '#cdd6f4',
};
