import { jsxs as _jsxs, jsx as _jsx } from "react/jsx-runtime";
/**
 * P17-A: Node version history timeline.
 * Displays revisions in reverse-chronological order with content preview,
 * expand-on-click full view, and one-click rollback.
 */
import { useState } from 'react';
import { api } from '../api/client';
export default function NodeVersionHistory({ node, revisions, onClose, onRolledBack }) {
    const [expanded, setExpanded] = useState(null);
    const [rolling, setRolling] = useState(null);
    const [err, setErr] = useState(null);
    async function handleRollback(rev) {
        if (!window.confirm(`确认回滚到 rev ${rev.rev_number}？当前内容将被替换。`))
            return;
        setRolling(rev.rev_number);
        setErr(null);
        try {
            const updated = await api.rollbackNode(node.id, rev.rev_number);
            onRolledBack(updated);
            onClose();
        }
        catch (e) {
            setErr(e instanceof Error ? e.message : '回滚失败');
        }
        finally {
            setRolling(null);
        }
    }
    return (_jsx("div", { style: {
            position: 'fixed', inset: 0, zIndex: 2000,
            background: 'rgba(0,0,0,0.7)',
            display: 'flex', alignItems: 'center', justifyContent: 'center',
            padding: 24,
        }, onClick: e => { if (e.target === e.currentTarget)
            onClose(); }, children: _jsxs("div", { style: {
                background: '#0d1526', border: '1px solid #1e3050', borderRadius: 10,
                padding: 24, width: '100%', maxWidth: 540, maxHeight: '80vh',
                display: 'flex', flexDirection: 'column', gap: 12,
            }, children: [_jsxs("div", { style: { display: 'flex', justifyContent: 'space-between', alignItems: 'center' }, children: [_jsxs("span", { style: { fontWeight: 600, fontSize: 14, color: '#c8d8e8' }, children: ["\u7248\u672C\u5386\u53F2 \u2014 ", node.id.slice(0, 8)] }), _jsx("button", { onClick: onClose, style: {
                                background: 'none', border: 'none', color: '#9ec5ee',
                                fontSize: 20, cursor: 'pointer', lineHeight: 1,
                            }, children: "\u2715" })] }), err && (_jsx("div", { style: { color: '#ff6b7a', fontSize: 12, padding: '4px 8px', background: 'rgba(220,53,69,0.1)', borderRadius: 4 }, children: err })), revisions.length === 0 && (_jsx("div", { style: { opacity: 0.5, fontSize: 12, textAlign: 'center', padding: 24 }, children: "\u6682\u65E0\u5386\u53F2\u8BB0\u5F55" })), _jsx("div", { style: { overflowY: 'auto', flex: 1, display: 'flex', flexDirection: 'column', gap: 8 }, children: revisions.map((rev, idx) => {
                        const isLatest = idx === 0;
                        const isExpanded = expanded === rev.rev_number;
                        return (_jsxs("div", { style: {
                                display: 'flex', gap: 12, position: 'relative',
                            }, children: [_jsxs("div", { style: { display: 'flex', flexDirection: 'column', alignItems: 'center', minWidth: 20 }, children: [_jsx("div", { style: {
                                                width: 10, height: 10, borderRadius: '50%', flexShrink: 0,
                                                background: isLatest ? '#4a8eff' : '#2a4a6e',
                                                border: `2px solid ${isLatest ? '#9ec5ee' : '#4a6a8e'}`,
                                                marginTop: 4,
                                            } }), idx < revisions.length - 1 && (_jsx("div", { style: { width: 2, flex: 1, background: '#1e3050', marginTop: 2 } }))] }), _jsxs("div", { style: {
                                        flex: 1, background: '#0a1828', border: '1px solid #1e3050', borderRadius: 6,
                                        padding: '8px 10px', marginBottom: 4,
                                    }, children: [_jsxs("div", { style: { display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start' }, children: [_jsxs("div", { children: [_jsxs("span", { style: { color: '#4a8eff', fontSize: 12, fontWeight: 600 }, children: ["rev ", rev.rev_number] }), isLatest && (_jsx("span", { style: { marginLeft: 6, fontSize: 10, color: '#52c41a', background: 'rgba(82,196,26,0.12)', padding: '1px 5px', borderRadius: 3 }, children: "\u5F53\u524D" })), rev.edit_reason && (_jsx("span", { style: { marginLeft: 6, fontSize: 10, color: '#9ec5ee', opacity: 0.7 }, children: rev.edit_reason }))] }), _jsx("span", { style: { fontSize: 10, color: '#4a6a8e', whiteSpace: 'nowrap', marginLeft: 8 }, children: new Date(rev.created_at).toLocaleString('zh-CN') })] }), _jsxs("div", { onClick: () => setExpanded(isExpanded ? null : rev.rev_number), style: { cursor: 'pointer', marginTop: 6 }, children: [isExpanded ? (_jsx("pre", { style: {
                                                        margin: 0, whiteSpace: 'pre-wrap', wordBreak: 'break-all',
                                                        fontSize: 11, color: '#c0d8f0', lineHeight: 1.6,
                                                        maxHeight: 200, overflowY: 'auto',
                                                        background: '#061020', padding: '6px 8px', borderRadius: 4,
                                                    }, children: rev.content })) : (_jsxs("div", { style: { fontSize: 11, color: '#9ec5ee', opacity: 0.8, lineHeight: 1.5 }, children: [rev.content.slice(0, 100), rev.content.length > 100 ? '…' : ''] })), _jsx("div", { style: { fontSize: 10, color: '#4a6a8e', marginTop: 2 }, children: isExpanded ? '▲ 折叠' : '▼ 展开' })] }), !isLatest && (_jsx("div", { style: { marginTop: 8 }, children: _jsx("button", { onClick: () => void handleRollback(rev), disabled: rolling !== null, style: {
                                                    fontSize: 11, padding: '3px 10px',
                                                    background: 'rgba(74,144,226,0.15)', color: '#4a8eff',
                                                    border: '1px solid #2a4a7e', borderRadius: 4, cursor: 'pointer',
                                                }, children: rolling === rev.rev_number ? '回滚中…' : '⟲ 回滚到此版本' }) }))] })] }, rev.rev_number));
                    }) })] }) }));
}
