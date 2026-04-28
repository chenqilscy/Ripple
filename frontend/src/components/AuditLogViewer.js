import { jsx as _jsx, jsxs as _jsxs } from "react/jsx-runtime";
import { useEffect, useState } from 'react';
import { api } from '../api/client';
/** P11-B：审计日志浏览器 */
export default function AuditLogViewer({ defaultResourceType = '', defaultResourceId = '' }) {
    const [resourceType, setResourceType] = useState(defaultResourceType);
    const [resourceId, setResourceId] = useState(defaultResourceId);
    const [limit, setLimit] = useState(50);
    const [logs, setLogs] = useState([]);
    const [total, setTotal] = useState(0);
    const [loading, setLoading] = useState(false);
    const [err, setErr] = useState(null);
    const [queried, setQueried] = useState(false);
    async function handleQuery() {
        const rt = resourceType.trim();
        const rid = resourceId.trim();
        if (!rt || !rid)
            return;
        setLoading(true);
        setErr(null);
        try {
            const res = await api.listAuditLogs(rt, rid, limit);
            setLogs(res.logs ?? []);
            setTotal(res.total ?? 0);
            setQueried(true);
        }
        catch (e) {
            setErr(e?.message ?? 'query failed');
        }
        finally {
            setLoading(false);
        }
    }
    useEffect(() => {
        const rt = defaultResourceType.trim();
        const rid = defaultResourceId.trim();
        setResourceType(defaultResourceType);
        setResourceId(defaultResourceId);
        if (!rt || !rid)
            return;
        void (async () => {
            setLoading(true);
            setErr(null);
            try {
                const res = await api.listAuditLogs(rt, rid, limit);
                setLogs(res.logs ?? []);
                setTotal(res.total ?? 0);
                setQueried(true);
            }
            catch (e) {
                setErr(e?.message ?? 'query failed');
            }
            finally {
                setLoading(false);
            }
        })();
    }, [defaultResourceType, defaultResourceId, limit]);
    return (_jsxs("div", { style: { padding: '16px', maxWidth: 900 }, children: [_jsx("h3", { style: { margin: '0 0 12px', color: '#cdd6f4' }, children: "\u5BA1\u8BA1\u65E5\u5FD7" }), _jsxs("div", { style: { display: 'flex', gap: 8, marginBottom: 12, flexWrap: 'wrap' }, children: [_jsxs("select", { value: resourceType, onChange: e => setResourceType(e.target.value), style: selectStyle, children: [_jsx("option", { value: "", children: "\u2014 \u8D44\u6E90\u7C7B\u578B \u2014" }), _jsx("option", { value: "node", children: "node" }), _jsx("option", { value: "edge", children: "edge" }), _jsx("option", { value: "lake", children: "lake" }), _jsx("option", { value: "organization", children: "organization" }), _jsx("option", { value: "org_quota", children: "org_quota" }), _jsx("option", { value: "api_key", children: "api_key" })] }), _jsx("input", { value: resourceId, onChange: e => setResourceId(e.target.value), placeholder: "\u8D44\u6E90 ID", style: { ...inputStyle, width: 240 }, onKeyDown: e => e.key === 'Enter' && void handleQuery() }), _jsxs("select", { value: limit, onChange: e => setLimit(Number(e.target.value)), style: { ...selectStyle, width: 90 }, children: [_jsx("option", { value: 20, children: "20 \u6761" }), _jsx("option", { value: 50, children: "50 \u6761" }), _jsx("option", { value: 100, children: "100 \u6761" }), _jsx("option", { value: 200, children: "200 \u6761" })] }), _jsx("button", { onClick: handleQuery, disabled: loading || !resourceType.trim() || !resourceId.trim(), style: btnStyle, children: loading ? '查询中…' : '查询' })] }), err && _jsxs("p", { style: { color: '#f38ba8', margin: '0 0 12px' }, children: ["\u26A0 ", err] }), queried && !loading && (_jsxs("p", { style: { color: '#6c7086', marginBottom: 8, fontSize: 12 }, children: ["\u5171 ", total, " \u6761\u8BB0\u5F55\uFF08\u6700\u591A\u663E\u793A ", limit, " \u6761\uFF09"] })), loading ? (_jsx("p", { style: { color: '#6c7086' }, children: "\u67E5\u8BE2\u4E2D\u2026" })) : queried && logs.length === 0 ? (_jsx("p", { style: { color: '#6c7086' }, children: "\u65E0\u8BB0\u5F55" })) : logs.length > 0 ? (_jsxs("table", { style: { width: '100%', borderCollapse: 'collapse', fontSize: 12 }, children: [_jsx("thead", { children: _jsxs("tr", { style: { color: '#6c7086', textAlign: 'left' }, children: [_jsx("th", { style: thStyle, children: "\u65F6\u95F4" }), _jsx("th", { style: thStyle, children: "\u64CD\u4F5C" }), _jsx("th", { style: thStyle, children: "\u64CD\u4F5C\u4EBA" }), _jsx("th", { style: thStyle, children: "\u8BE6\u60C5" })] }) }), _jsx("tbody", { children: logs.map(l => (_jsxs("tr", { style: { borderBottom: '1px solid #313244' }, children: [_jsx("td", { style: { ...tdStyle, color: '#6c7086', whiteSpace: 'nowrap' }, children: fmtDate(l.created_at) }), _jsx("td", { style: { ...tdStyle, color: '#89dceb' }, children: l.action }), _jsxs("td", { style: { ...tdStyle, fontFamily: 'monospace', fontSize: 11, color: '#bac2de' }, children: [l.actor_id.slice(0, 8), "\u2026"] }), _jsx("td", { style: { ...tdStyle, color: '#a6adc8' }, children: Object.keys(l.detail).length > 0
                                        ? _jsx("code", { style: { fontSize: 11 }, children: JSON.stringify(l.detail) })
                                        : _jsx("span", { style: { color: '#45475a' }, children: "\u2014" }) })] }, l.id))) })] })) : null] }));
}
function fmtDate(s) {
    return new Date(s).toLocaleString('zh-CN', {
        month: '2-digit', day: '2-digit',
        hour: '2-digit', minute: '2-digit', second: '2-digit',
    });
}
const inputStyle = {
    background: '#1e1e2e', border: '1px solid #45475a', borderRadius: 4,
    color: '#cdd6f4', padding: '5px 10px', fontSize: 13,
};
const selectStyle = {
    ...inputStyle, cursor: 'pointer',
};
const btnStyle = {
    background: 'transparent', border: '1px solid #89b4fa', color: '#89b4fa',
    borderRadius: 4, padding: '5px 14px', cursor: 'pointer', fontSize: 13,
};
const thStyle = {
    padding: '6px 8px', fontWeight: 500, borderBottom: '1px solid #313244',
};
const tdStyle = {
    padding: '7px 8px', color: '#cdd6f4', verticalAlign: 'top',
};
