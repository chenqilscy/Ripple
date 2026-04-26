import { jsx as _jsx, jsxs as _jsxs } from "react/jsx-runtime";
import { useState, useMemo } from 'react';
/** 最长公共子序列（LCS）长度矩阵 */
function lcsTable(a, b) {
    const m = a.length;
    const n = b.length;
    const dp = Array.from({ length: m + 1 }, () => new Array(n + 1).fill(0));
    for (let i = 1; i <= m; i++) {
        for (let j = 1; j <= n; j++) {
            dp[i][j] = a[i - 1] === b[j - 1] ? dp[i - 1][j - 1] + 1 : Math.max(dp[i - 1][j], dp[i][j - 1]);
        }
    }
    return dp;
}
/** 根据 LCS 表回溯生成 diff 序列 */
function buildDiff(a, b, dp, i, j, out) {
    if (i > 0 && j > 0 && a[i - 1] === b[j - 1]) {
        buildDiff(a, b, dp, i - 1, j - 1, out);
        out.push({ kind: 'equal', text: a[i - 1] });
    }
    else if (j > 0 && (i === 0 || dp[i][j - 1] >= dp[i - 1][j])) {
        buildDiff(a, b, dp, i, j - 1, out);
        out.push({ kind: 'added', text: b[j - 1] });
    }
    else if (i > 0) {
        buildDiff(a, b, dp, i - 1, j, out);
        out.push({ kind: 'removed', text: a[i - 1] });
    }
}
/** 对两段文本做逐行 diff */
export function diffLines(oldText, newText) {
    const a = oldText.split('\n');
    const b = newText.split('\n');
    const dp = lcsTable(a, b);
    const out = [];
    buildDiff(a, b, dp, a.length, b.length, out);
    return out;
}
export function NodeDiffViewer({ nodeId: _nodeId, revisions, onClose }) {
    const sortedRevs = useMemo(() => [...revisions].sort((a, b) => a.rev_number - b.rev_number), [revisions]);
    const [baseRev, setBaseRev] = useState(sortedRevs[0]?.rev_number ?? 1);
    const [compareRev, setCompareRev] = useState(sortedRevs[sortedRevs.length - 1]?.rev_number ?? 1);
    const baseContent = useMemo(() => sortedRevs.find(r => r.rev_number === baseRev)?.content ?? '', [sortedRevs, baseRev]);
    const compareContent = useMemo(() => sortedRevs.find(r => r.rev_number === compareRev)?.content ?? '', [sortedRevs, compareRev]);
    const diff = useMemo(() => diffLines(baseContent, compareContent), [baseContent, compareContent]);
    const addedCount = diff.filter(l => l.kind === 'added').length;
    const removedCount = diff.filter(l => l.kind === 'removed').length;
    return (_jsx("div", { role: "dialog", "aria-modal": "true", style: {
            position: 'fixed', inset: 0, zIndex: 9000,
            background: 'rgba(0,0,0,0.65)', display: 'flex',
            alignItems: 'center', justifyContent: 'center',
        }, onClick: e => { if (e.target === e.currentTarget)
            onClose(); }, children: _jsxs("div", { style: {
                background: '#1a2332', borderRadius: 10, padding: 24,
                width: 'min(800px, 95vw)', maxHeight: '85vh',
                display: 'flex', flexDirection: 'column', gap: 12,
                boxShadow: '0 8px 40px rgba(0,0,0,0.5)',
            }, children: [_jsxs("div", { style: { display: 'flex', justifyContent: 'space-between', alignItems: 'center' }, children: [_jsx("span", { style: { fontWeight: 600, color: '#c8d8e8' }, children: "\u7248\u672C\u5BF9\u6BD4" }), _jsx("button", { onClick: onClose, style: {
                                background: 'none', border: 'none', color: '#9ec5ee',
                                fontSize: 18, cursor: 'pointer', padding: '0 4px',
                            }, children: "\u2715" })] }), _jsxs("div", { style: { display: 'flex', gap: 16, alignItems: 'center', flexWrap: 'wrap' }, children: [_jsxs("label", { style: { color: '#9ec5ee', fontSize: 13 }, children: ["\u57FA\u51C6\u7248\u672C\uFF1A", _jsx("select", { value: baseRev, onChange: e => setBaseRev(Number(e.target.value)), style: { marginLeft: 6, background: '#253447', color: '#c8d8e8', border: '1px solid #3a5068', borderRadius: 4, padding: '2px 6px' }, children: sortedRevs.map(r => (_jsxs("option", { value: r.rev_number, children: ["rev ", r.rev_number, " \u2014 ", new Date(r.created_at).toLocaleString(), " ", r.edit_reason ? `(${r.edit_reason})` : ''] }, r.rev_number))) })] }), _jsx("span", { style: { color: '#4a7aaa' }, children: "\u2192" }), _jsxs("label", { style: { color: '#9ec5ee', fontSize: 13 }, children: ["\u5BF9\u6BD4\u7248\u672C\uFF1A", _jsx("select", { value: compareRev, onChange: e => setCompareRev(Number(e.target.value)), style: { marginLeft: 6, background: '#253447', color: '#c8d8e8', border: '1px solid #3a5068', borderRadius: 4, padding: '2px 6px' }, children: sortedRevs.map(r => (_jsxs("option", { value: r.rev_number, children: ["rev ", r.rev_number, " \u2014 ", new Date(r.created_at).toLocaleString(), " ", r.edit_reason ? `(${r.edit_reason})` : ''] }, r.rev_number))) })] }), _jsxs("span", { style: { fontSize: 12, color: '#9ec5ee', marginLeft: 'auto' }, children: [_jsxs("span", { style: { color: '#4caf50' }, children: ["+", addedCount] }), ' / ', _jsxs("span", { style: { color: '#f44336' }, children: ["-", removedCount] }), ' 行'] })] }), _jsxs("div", { style: {
                        flex: 1, overflow: 'auto',
                        background: '#111c2a', borderRadius: 6,
                        padding: '10px 0', fontFamily: 'monospace', fontSize: 13,
                    }, children: [diff.map((line, i) => (_jsxs("div", { style: {
                                padding: '1px 12px',
                                background: line.kind === 'added' ? 'rgba(76,175,80,0.12)' :
                                    line.kind === 'removed' ? 'rgba(244,67,54,0.12)' :
                                        'transparent',
                                color: line.kind === 'added' ? '#81c784' :
                                    line.kind === 'removed' ? '#e57373' :
                                        '#8aa8c0',
                                borderLeft: `3px solid ${line.kind === 'added' ? '#4caf50' :
                                    line.kind === 'removed' ? '#f44336' :
                                        'transparent'}`,
                                whiteSpace: 'pre-wrap',
                                wordBreak: 'break-word',
                            }, children: [_jsx("span", { style: { userSelect: 'none', color: 'inherit', opacity: 0.7, marginRight: 8 }, children: line.kind === 'added' ? '+' : line.kind === 'removed' ? '-' : ' ' }), line.text || '\u00a0'] }, i))), diff.length === 0 && (_jsx("div", { style: { padding: '20px 12px', color: '#4a7aaa', textAlign: 'center' }, children: "\u4E24\u7248\u672C\u5185\u5BB9\u5B8C\u5168\u76F8\u540C" }))] })] }) }));
}
