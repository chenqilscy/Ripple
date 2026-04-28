import { jsx as _jsx, jsxs as _jsxs } from "react/jsx-runtime";
import { useEffect, useState } from 'react';
import { api } from '../api/client';
export default function GraylistManager() {
    const [entries, setEntries] = useState([]);
    const [loading, setLoading] = useState(false);
    const [err, setErr] = useState(null);
    const [forbidden, setForbidden] = useState(false);
    const [saving, setSaving] = useState(false);
    const [deletingId, setDeletingId] = useState(null);
    const [email, setEmail] = useState('');
    const [note, setNote] = useState('');
    async function load() {
        setLoading(true);
        setErr(null);
        try {
            const res = await api.listGraylist();
            setEntries((res.entries ?? []).slice().sort((left, right) => new Date(right.created_at).getTime() - new Date(left.created_at).getTime()));
            setForbidden(false);
        }
        catch (e) {
            if (e?.status === 403) {
                setForbidden(true);
                setEntries([]);
                setErr('仅平台管理员可管理灰度名单');
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
    async function handleSave() {
        const normalized = email.trim().toLowerCase();
        if (!normalized || forbidden)
            return;
        setSaving(true);
        setErr(null);
        try {
            await api.upsertGraylist(normalized, note.trim());
            setEmail('');
            setNote('');
            await load();
        }
        catch (e) {
            setErr(e?.message ?? 'save failed');
        }
        finally {
            setSaving(false);
        }
    }
    async function handleDelete(entry) {
        if (forbidden)
            return;
        if (!window.confirm(`确定移除灰度邮箱 ${entry.email}？`))
            return;
        setDeletingId(entry.id);
        setErr(null);
        try {
            await api.deleteGraylist(entry.id);
            setEntries(prev => prev.filter(item => item.id !== entry.id));
        }
        catch (e) {
            setErr(e?.message ?? 'delete failed');
        }
        finally {
            setDeletingId(null);
        }
    }
    return (_jsxs("div", { style: { padding: 16, maxWidth: 760, minWidth: 360, flex: '1 1 420px' }, children: [_jsx("h3", { style: { margin: '0 0 12px', color: '#cdd6f4' }, children: "\u7070\u5EA6\u540D\u5355" }), _jsx("p", { style: { margin: '0 0 12px', color: '#6c7086', fontSize: 12, lineHeight: 1.5 }, children: "\u4EC5\u5E73\u53F0\u7BA1\u7406\u5458\u53EF\u7F16\u8F91\u3002\u53EA\u6709\u5F53\u540E\u7AEF\u5F00\u542F\u6CE8\u518C\u7070\u5EA6\u5F00\u5173\u65F6\uFF0C\u8FD9\u91CC\u7684\u90AE\u7BB1\u624D\u5141\u8BB8\u6CE8\u518C\u3002" }), _jsxs("div", { style: { display: 'flex', gap: 8, marginBottom: 16, flexWrap: 'wrap' }, children: [_jsx("input", { value: email, onChange: e => setEmail(e.target.value), onKeyDown: e => e.key === 'Enter' && void handleSave(), placeholder: "\u5141\u8BB8\u6CE8\u518C\u7684\u90AE\u7BB1", disabled: saving || forbidden, style: { ...inputStyle, minWidth: 220, flex: '1 1 220px' } }), _jsx("input", { value: note, onChange: e => setNote(e.target.value), onKeyDown: e => e.key === 'Enter' && void handleSave(), placeholder: "\u5907\u6CE8\uFF08\u53EF\u9009\uFF09", disabled: saving || forbidden, style: { ...inputStyle, minWidth: 180, flex: '1 1 180px' } }), _jsx("button", { onClick: () => void handleSave(), disabled: saving || forbidden || !email.trim(), style: btnStyle('#f9e2af'), children: saving ? '保存中…' : '+ 添加 / 更新' })] }), err && _jsxs("p", { style: { color: forbidden ? '#f9e2af' : '#f38ba8', margin: '0 0 12px' }, children: ["\u26A0 ", err] }), forbidden ? (_jsx("p", { style: { color: '#6c7086', margin: 0 }, children: "\u5F53\u524D\u8D26\u53F7\u4E0D\u662F\u5E73\u53F0\u7BA1\u7406\u5458\uFF0C\u53EA\u80FD\u67E5\u770B\u6B64\u8BF4\u660E\u3002" })) : loading ? (_jsx("p", { style: { color: '#6c7086' }, children: "\u52A0\u8F7D\u4E2D\u2026" })) : entries.length === 0 ? (_jsx("p", { style: { color: '#6c7086' }, children: "\u6682\u65E0\u7070\u5EA6\u90AE\u7BB1\uFF0C\u5F00\u542F\u7070\u5EA6\u5F00\u5173\u540E\u5C06\u62D2\u7EDD\u6240\u6709\u672A\u5217\u5165\u90AE\u7BB1\u7684\u6CE8\u518C\u3002" })) : (_jsxs("table", { style: { width: '100%', borderCollapse: 'collapse', fontSize: 13 }, children: [_jsx("thead", { children: _jsxs("tr", { style: { color: '#6c7086', textAlign: 'left' }, children: [_jsx("th", { style: thStyle, children: "\u90AE\u7BB1" }), _jsx("th", { style: thStyle, children: "\u5907\u6CE8" }), _jsx("th", { style: thStyle, children: "\u521B\u5EFA\u4EBA" }), _jsx("th", { style: thStyle, children: "\u521B\u5EFA\u65F6\u95F4" }), _jsx("th", { style: thStyle })] }) }), _jsx("tbody", { children: entries.map(entry => (_jsxs("tr", { style: { borderBottom: '1px solid #313244' }, children: [_jsx("td", { style: { ...tdStyle, color: '#89dceb' }, children: entry.email }), _jsx("td", { style: tdStyle, children: entry.note || '—' }), _jsx("td", { style: { ...tdStyle, color: '#6c7086' }, children: shortID(entry.created_by) }), _jsx("td", { style: { ...tdStyle, color: '#6c7086' }, children: fmtDate(entry.created_at) }), _jsx("td", { style: tdStyle, children: _jsx("button", { onClick: () => void handleDelete(entry), disabled: deletingId === entry.id, style: btnStyle('#f38ba8', true), children: deletingId === entry.id ? '移除中…' : '移除' }) })] }, entry.id))) })] }))] }));
}
function fmtDate(s) {
    return new Date(s).toLocaleString('zh-CN', { month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit' });
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
const thStyle = {
    padding: '6px 8px', fontWeight: 500, borderBottom: '1px solid #313244',
};
const tdStyle = {
    padding: '8px 8px', color: '#cdd6f4',
};
