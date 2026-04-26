import { jsx as _jsx, jsxs as _jsxs } from "react/jsx-runtime";
import { useEffect, useState } from 'react';
import { api } from '../api/client';
/** P18-B 公开分享节点页面 — 无需登录 */
export function SharedNode() {
    const token = window.location.pathname.split('/')[2] ?? '';
    const [node, setNode] = useState(null);
    const [loading, setLoading] = useState(true);
    const [err, setErr] = useState(null);
    useEffect(() => {
        if (!token) {
            setErr('无效的分享链接');
            setLoading(false);
            return;
        }
        api.getSharedNode(token)
            .then(r => { setNode(r.node); })
            .catch(e => { setErr(e.message ?? '加载失败'); })
            .finally(() => setLoading(false));
    }, [token]);
    return (_jsxs("div", { style: {
            minHeight: '100vh', background: '#0a1929', color: '#e0f0ff',
            fontFamily: 'system-ui, -apple-system, sans-serif',
            display: 'flex', flexDirection: 'column', alignItems: 'center',
            padding: '60px 24px',
        }, children: [_jsxs("div", { style: { marginBottom: 40, textAlign: 'center' }, children: [_jsx("div", { style: { fontSize: 22, fontWeight: 600, color: '#9ec5ee', letterSpacing: 3 }, children: "\u9752\u840D Ripple" }), _jsx("div", { style: { fontSize: 12, color: '#6c7086', marginTop: 6 }, children: "\u77E5\u8BC6\u8282\u70B9\u5171\u4EAB" })] }), loading && (_jsx("div", { style: { color: '#6c7086', fontSize: 14 }, children: "\u52A0\u8F7D\u4E2D\u2026" })), !loading && err && (_jsxs("div", { style: {
                    padding: '20px 28px', maxWidth: 480, width: '100%', textAlign: 'center',
                    background: 'rgba(255,80,80,0.08)', border: '1px solid rgba(255,80,80,0.3)',
                    borderRadius: 10, color: '#ff9898', fontSize: 14,
                }, children: [_jsx("div", { style: { fontSize: 24, marginBottom: 10 }, children: "\uD83D\uDD17" }), err === '404 Not Found' || err.includes('404')
                        ? '该分享链接不存在或已失效'
                        : err] })), !loading && node && (_jsxs("div", { style: { maxWidth: 660, width: '100%' }, children: [_jsxs("div", { style: {
                            background: 'rgba(255,255,255,0.04)',
                            border: '1px solid rgba(255,255,255,0.1)',
                            borderRadius: 12, padding: '24px 28px',
                        }, children: [_jsxs("div", { style: { display: 'flex', gap: 8, marginBottom: 16, alignItems: 'center', flexWrap: 'wrap' }, children: [_jsx("span", { style: { background: stateColor(node.state), color: '#001020', fontSize: 10, padding: '2px 10px', borderRadius: 10, letterSpacing: 1, fontWeight: 600 }, children: node.state }), _jsx("span", { style: { fontSize: 11, color: '#6c7086', background: 'rgba(255,255,255,0.06)', padding: '2px 8px', borderRadius: 6 }, children: node.type }), _jsx("span", { style: { flex: 1 } }), _jsx("span", { style: { fontSize: 11, color: '#6c7086' }, children: new Date(node.updated_at).toLocaleString('zh-CN', { year: 'numeric', month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit' }) })] }), _jsx("div", { style: {
                                    fontSize: 15, lineHeight: 1.8, whiteSpace: 'pre-wrap',
                                    color: '#e0f0ff', wordBreak: 'break-word',
                                }, children: node.content })] }), _jsxs("div", { style: { textAlign: 'center', marginTop: 28, fontSize: 12, color: '#45475a' }, children: ["\u901A\u8FC7", ' ', _jsx("a", { href: "/", style: { color: '#9ec5ee', textDecoration: 'none' }, children: "\u9752\u840D Ripple" }), ' ', "\u5206\u4EAB \u00B7 \u8282\u70B9 ID: ", _jsx("code", { style: { fontSize: 10, color: '#6c7086' }, children: node.id.slice(0, 8) })] })] }))] }));
}
function stateColor(s) {
    const m = {
        MIST: '#7fbfff', DROP: '#89b4fa', FROZEN: '#a5d8ff',
        VAPOR: '#313244', WAVE: '#89dceb', STONE: '#cba6f7',
    };
    return m[s] ?? '#45475a';
}
