import { jsx as _jsx, jsxs as _jsxs } from "react/jsx-runtime";
import { useEffect, useState } from 'react';
import { api } from '../api/client';
/** P11-A：API Key 管理面板 */
export default function APIKeyManager() {
    const [keys, setKeys] = useState([]);
    const [loading, setLoading] = useState(false);
    const [err, setErr] = useState(null);
    const [creating, setCreating] = useState(false);
    const [newName, setNewName] = useState('');
    const [newKeyResult, setNewKeyResult] = useState(null);
    const [copied, setCopied] = useState(false);
    async function load() {
        setLoading(true);
        setErr(null);
        try {
            const res = await api.listAPIKeys();
            setKeys(res.keys ?? []);
        }
        catch (e) {
            setErr(e?.message ?? 'load failed');
        }
        finally {
            setLoading(false);
        }
    }
    useEffect(() => { void load(); }, []);
    async function handleCreate() {
        const name = newName.trim();
        if (!name)
            return;
        setCreating(true);
        setErr(null);
        try {
            const created = await api.createAPIKey(name);
            setNewKeyResult(created);
            setNewName('');
            void load();
        }
        catch (e) {
            setErr(e?.message ?? 'create failed');
        }
        finally {
            setCreating(false);
        }
    }
    async function handleRevoke(id) {
        if (!window.confirm('确定撤销此 API Key？撤销后不可恢复。'))
            return;
        try {
            await api.revokeAPIKey(id);
            setKeys(prev => prev.filter(k => k.id !== id));
        }
        catch (e) {
            setErr(e?.message ?? 'revoke failed');
        }
    }
    function handleCopy(text) {
        navigator.clipboard.writeText(text).then(() => {
            setCopied(true);
            setTimeout(() => setCopied(false), 2000);
        });
    }
    return (_jsxs("div", { style: { padding: '16px', maxWidth: 720 }, children: [_jsx("h3", { style: { margin: '0 0 12px', color: '#cdd6f4' }, children: "API Key \u7BA1\u7406" }), newKeyResult && (_jsxs("div", { style: {
                    background: '#1e3a2f', border: '1px solid #a6e3a1', borderRadius: 8,
                    padding: '12px 16px', marginBottom: 16,
                }, children: [_jsx("p", { style: { margin: '0 0 8px', color: '#a6e3a1', fontWeight: 600 }, children: "\u2705 API Key \u521B\u5EFA\u6210\u529F \u2014 \u8BF7\u7ACB\u5373\u590D\u5236\uFF0C\u5173\u95ED\u540E\u65E0\u6CD5\u518D\u6B21\u67E5\u770B" }), _jsx("code", { style: {
                            display: 'block', background: '#11111b', borderRadius: 4,
                            padding: '8px 12px', color: '#f9e2af', wordBreak: 'break-all', marginBottom: 8,
                        }, children: newKeyResult.raw_key }), _jsxs("div", { style: { display: 'flex', gap: 8 }, children: [_jsx("button", { onClick: () => handleCopy(newKeyResult.raw_key), style: btnStyle('#89b4fa'), children: copied ? '已复制 ✓' : '复制' }), _jsx("button", { onClick: () => setNewKeyResult(null), style: btnStyle('#6c7086'), children: "\u5173\u95ED" })] })] })), _jsxs("div", { style: { display: 'flex', gap: 8, marginBottom: 16 }, children: [_jsx("input", { value: newName, onChange: e => setNewName(e.target.value), onKeyDown: e => e.key === 'Enter' && void handleCreate(), placeholder: "Key \u540D\u79F0\uFF08\u5982 my-ci-key\uFF09", disabled: creating, style: inputStyle }), _jsx("button", { onClick: handleCreate, disabled: creating || !newName.trim(), style: btnStyle('#a6e3a1'), children: creating ? '创建中…' : '+ 创建' })] }), err && _jsxs("p", { style: { color: '#f38ba8', margin: '0 0 12px' }, children: ["\u26A0 ", err] }), loading ? (_jsx("p", { style: { color: '#6c7086' }, children: "\u52A0\u8F7D\u4E2D\u2026" })) : keys.length === 0 ? (_jsx("p", { style: { color: '#6c7086' }, children: "\u6682\u65E0 API Key" })) : (_jsxs("table", { style: { width: '100%', borderCollapse: 'collapse', fontSize: 13 }, children: [_jsx("thead", { children: _jsxs("tr", { style: { color: '#6c7086', textAlign: 'left' }, children: [_jsx("th", { style: thStyle, children: "\u540D\u79F0" }), _jsx("th", { style: thStyle, children: "\u524D\u7F00" }), _jsx("th", { style: thStyle, children: "\u6743\u9650\u57DF" }), _jsx("th", { style: thStyle, children: "\u6700\u540E\u4F7F\u7528" }), _jsx("th", { style: thStyle, children: "\u521B\u5EFA\u65F6\u95F4" }), _jsx("th", { style: thStyle })] }) }), _jsx("tbody", { children: keys.map(k => (_jsxs("tr", { style: { borderBottom: '1px solid #313244' }, children: [_jsx("td", { style: tdStyle, children: k.name }), _jsx("td", { style: { ...tdStyle, fontFamily: 'monospace', color: '#89dceb' }, children: k.key_prefix }), _jsx("td", { style: tdStyle, children: k.scopes.join(', ') }), _jsx("td", { style: { ...tdStyle, color: '#6c7086' }, children: k.last_used_at ? fmtDate(k.last_used_at) : '—' }), _jsx("td", { style: { ...tdStyle, color: '#6c7086' }, children: fmtDate(k.created_at) }), _jsx("td", { style: tdStyle, children: _jsx("button", { onClick: () => handleRevoke(k.id), style: btnStyle('#f38ba8', true), children: "\u64A4\u9500" }) })] }, k.id))) })] }))] }));
}
function fmtDate(s) {
    return new Date(s).toLocaleString('zh-CN', { month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit' });
}
function btnStyle(color, small = false) {
    return {
        background: 'transparent', border: `1px solid ${color}`, color,
        borderRadius: 4, padding: small ? '2px 8px' : '5px 12px',
        cursor: 'pointer', fontSize: small ? 12 : 13,
    };
}
const inputStyle = {
    flex: 1, background: '#1e1e2e', border: '1px solid #45475a', borderRadius: 4,
    color: '#cdd6f4', padding: '5px 10px', fontSize: 13,
};
const thStyle = {
    padding: '6px 8px', fontWeight: 500, borderBottom: '1px solid #313244',
};
const tdStyle = {
    padding: '8px 8px', color: '#cdd6f4',
};
