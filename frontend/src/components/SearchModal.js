import { jsx as _jsx, jsxs as _jsxs } from "react/jsx-runtime";
// SearchModal · P12-D 全文搜索浮层
// 快捷键 Cmd+K / Ctrl+K 触发；在当前激活的湖内搜索节点。
import { useCallback, useEffect, useRef, useState } from 'react';
import { api } from '../api/client';
export default function SearchModal({ lakeId, lakeName, onClose, onSelect }) {
    const [q, setQ] = useState('');
    const [results, setResults] = useState([]);
    const [loading, setLoading] = useState(false);
    const [error, setError] = useState(null);
    const inputRef = useRef(null);
    const debounceRef = useRef(null);
    // Focus input on open
    useEffect(() => {
        inputRef.current?.focus();
    }, []);
    // Close on Escape
    useEffect(() => {
        const handler = (e) => {
            if (e.key === 'Escape')
                onClose();
        };
        window.addEventListener('keydown', handler);
        return () => window.removeEventListener('keydown', handler);
    }, [onClose]);
    const doSearch = useCallback(async (query) => {
        if (!query.trim()) {
            setResults([]);
            return;
        }
        setLoading(true);
        setError(null);
        try {
            const { results: hits } = await api.searchNodes(query.trim(), lakeId);
            setResults(hits);
        }
        catch (e) {
            setError(e instanceof Error ? e.message : 'Search failed');
        }
        finally {
            setLoading(false);
        }
    }, [lakeId]);
    const handleChange = (val) => {
        setQ(val);
        if (debounceRef.current)
            clearTimeout(debounceRef.current);
        debounceRef.current = setTimeout(() => void doSearch(val), 300);
    };
    return (
    // Backdrop
    _jsx("div", { onClick: onClose, style: {
            position: 'fixed', inset: 0,
            background: 'rgba(0,0,0,0.55)',
            zIndex: 9000,
            display: 'flex', alignItems: 'flex-start', justifyContent: 'center',
            paddingTop: '12vh',
        }, children: _jsxs("div", { onClick: e => e.stopPropagation(), style: {
                width: 560, maxWidth: '92vw',
                background: '#0d1526',
                border: '1px solid #1e3050',
                borderRadius: 12,
                boxShadow: '0 16px 48px rgba(0,0,0,0.6)',
                overflow: 'hidden',
            }, children: [_jsxs("div", { style: { padding: '10px 16px 0', display: 'flex', alignItems: 'center', gap: 8 }, children: [_jsx("span", { style: { color: '#4a8eff', fontSize: 16 }, children: "\uD83D\uDD0D" }), _jsx("input", { ref: inputRef, value: q, onChange: e => handleChange(e.target.value), placeholder: `搜索「${lakeName ?? '当前湖'}」中的节点…`, style: {
                                flex: 1, background: 'transparent', border: 'none', outline: 'none',
                                color: '#e0f0ff', fontSize: 15, padding: '6px 0',
                            } }), loading && (_jsx("span", { style: { color: '#6c7086', fontSize: 12 }, children: "\u641C\u7D22\u4E2D\u2026" }))] }), _jsx("div", { style: { height: 1, background: '#1e3050', margin: '10px 0 0' } }), _jsxs("div", { style: { maxHeight: 400, overflowY: 'auto', padding: '4px 0 8px' }, children: [error && (_jsx("div", { style: { padding: '12px 16px', color: '#ff6b6b', fontSize: 13 }, children: error })), !loading && !error && q && results.length === 0 && (_jsx("div", { style: { padding: '12px 16px', color: '#6c7086', fontSize: 13 }, children: "\u672A\u627E\u5230\u76F8\u5173\u8282\u70B9" })), !q && (_jsx("div", { style: { padding: '12px 16px', color: '#6c7086', fontSize: 13 }, children: "\u8F93\u5165\u5173\u952E\u8BCD\u641C\u7D22\u8282\u70B9\u5185\u5BB9 \u00B7 Esc \u5173\u95ED" })), results.map(hit => (_jsxs("button", { onClick: () => { onSelect?.(hit); onClose(); }, style: {
                                display: 'block', width: '100%', textAlign: 'left',
                                background: 'transparent', border: 'none', cursor: 'pointer',
                                padding: '10px 16px',
                                color: '#c0d4f5',
                                borderBottom: '1px solid rgba(255,255,255,0.05)',
                            }, onMouseEnter: e => { e.currentTarget.style.background = '#1a2a44'; }, onMouseLeave: e => { e.currentTarget.style.background = 'transparent'; }, children: [_jsxs("div", { style: { fontSize: 12, color: '#4a8eff', marginBottom: 3, fontFamily: 'monospace' }, children: [hit.node_id.slice(0, 8), "\u2026"] }), _jsx("div", { style: { fontSize: 13, lineHeight: 1.5, whiteSpace: 'pre-wrap', wordBreak: 'break-word' }, children: hit.snippet || '（无内容）' }), _jsxs("div", { style: { fontSize: 11, color: '#6c7086', marginTop: 2 }, children: ["score: ", hit.score.toFixed(3)] })] }, hit.node_id)))] })] }) }));
}
