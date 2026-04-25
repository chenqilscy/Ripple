import { jsxs as _jsxs, jsx as _jsx, Fragment as _Fragment } from "react/jsx-runtime";
import React, { useState, useRef, useCallback } from 'react';
import { api } from '../api/client';
// 简单 CSV 解析：支持带引号字段、\n 分隔行
function parseCsv(text) {
    const rows = [];
    const lines = text.split(/\r?\n/);
    for (const line of lines) {
        if (!line.trim())
            continue;
        const cols = [];
        let cur = '';
        let inQ = false;
        for (let i = 0; i < line.length; i++) {
            const c = line[i];
            if (inQ) {
                if (c === '"' && line[i + 1] === '"') {
                    cur += '"';
                    i++;
                }
                else if (c === '"') {
                    inQ = false;
                }
                else {
                    cur += c;
                }
            }
            else {
                if (c === '"') {
                    inQ = true;
                }
                else if (c === ',') {
                    cols.push(cur);
                    cur = '';
                }
                else {
                    cur += c;
                }
            }
        }
        cols.push(cur);
        rows.push(cols);
    }
    return rows;
}
function csvToItems(text) {
    const rows = parseCsv(text);
    if (rows.length === 0)
        return [];
    // 检测 header 行
    const header = rows[0].map(h => h.trim().toLowerCase());
    const contentCol = header.findIndex(h => h === 'content' || h === 'text' || h === '内容');
    const typeCol = header.findIndex(h => h === 'type' || h === 'kind' || h === '类型');
    const dataRows = contentCol >= 0 ? rows.slice(1) : rows;
    return dataRows.map(row => ({
        content: contentCol >= 0 ? (row[contentCol] ?? '').trim() : row.join(' ').trim(),
        type: typeCol >= 0 ? (row[typeCol] ?? 'TEXT').trim().toUpperCase() : 'TEXT',
    })).filter(it => it.content);
}
const overlay = {
    position: 'fixed', inset: 0,
    background: 'rgba(0,0,0,0.6)',
    display: 'flex', alignItems: 'center', justifyContent: 'center',
    zIndex: 1100,
};
const modal = {
    background: '#0d1526',
    border: '1px solid #1e3050',
    borderRadius: 12,
    padding: '24px 28px',
    width: 560,
    maxHeight: '80vh',
    overflowY: 'auto',
    display: 'flex',
    flexDirection: 'column',
    gap: 16,
    color: '#cdd6f4',
    fontFamily: 'sans-serif',
};
const textarea = {
    width: '100%',
    minHeight: 180,
    background: '#0a0f1e',
    border: '1px solid #1e3050',
    borderRadius: 8,
    color: '#cdd6f4',
    fontFamily: 'monospace',
    fontSize: 13,
    padding: '10px 12px',
    resize: 'vertical',
    boxSizing: 'border-box',
};
const btnPrimary = {
    background: '#89b4fa',
    color: '#0d1526',
    border: 'none',
    borderRadius: 8,
    padding: '8px 20px',
    cursor: 'pointer',
    fontWeight: 700,
};
const btnSecondary = {
    background: 'transparent',
    color: '#6c7086',
    border: '1px solid #313244',
    borderRadius: 8,
    padding: '8px 20px',
    cursor: 'pointer',
};
const previewRow = {
    background: '#0a0f1e',
    border: '1px solid #1e3050',
    borderRadius: 6,
    padding: '8px 12px',
    fontSize: 13,
    display: 'flex',
    gap: 8,
    alignItems: 'flex-start',
};
const badge = {
    background: '#1e3050',
    borderRadius: 4,
    padding: '2px 6px',
    fontSize: 11,
    color: '#89b4fa',
    flexShrink: 0,
};
export default function ImportModal({ lakeId, lakeName, onClose, onImported }) {
    const [tab, setTab] = useState('json');
    const [raw, setRaw] = useState('');
    const [preview, setPreview] = useState(null);
    const [parseError, setParseError] = useState('');
    const [loading, setLoading] = useState(false);
    const [done, setDone] = useState(false);
    const [doneCount, setDoneCount] = useState(0);
    const fileRef = useRef(null);
    const handleParse = useCallback(() => {
        setParseError('');
        try {
            if (tab === 'json') {
                const parsed = JSON.parse(raw);
                if (!Array.isArray(parsed))
                    throw new Error('JSON 必须是数组');
                const items = parsed.map((it, i) => {
                    if (typeof it !== 'object' || it === null)
                        throw new Error(`第 ${i + 1} 项不是对象`);
                    const obj = it;
                    const content = String(obj.content ?? obj.text ?? '');
                    if (!content.trim())
                        return null;
                    return { content: content.trim(), type: String(obj.type ?? 'TEXT').toUpperCase() };
                }).filter(Boolean);
                if (items.length === 0)
                    throw new Error('解析出 0 个节点');
                setPreview(items);
            }
            else {
                const items = csvToItems(raw);
                if (items.length === 0)
                    throw new Error('解析出 0 个节点');
                setPreview(items);
            }
        }
        catch (e) {
            setParseError(e.message);
            setPreview(null);
        }
    }, [tab, raw]);
    const handleFileUpload = useCallback((e) => {
        const file = e.target.files?.[0];
        if (!file)
            return;
        const reader = new FileReader();
        reader.onload = () => setRaw(reader.result);
        reader.readAsText(file, 'utf-8');
        e.target.value = '';
    }, []);
    const handleConfirm = useCallback(async () => {
        if (!preview || preview.length === 0)
            return;
        setLoading(true);
        try {
            const result = await api.batchImportNodes(lakeId, preview);
            setDone(true);
            setDoneCount(result.created);
            onImported?.(result.created);
        }
        catch (e) {
            setParseError(e.message);
        }
        finally {
            setLoading(false);
        }
    }, [lakeId, preview, onImported]);
    // Escape to close
    React.useEffect(() => {
        const handler = (e) => { if (e.key === 'Escape')
            onClose(); };
        window.addEventListener('keydown', handler);
        return () => window.removeEventListener('keydown', handler);
    }, [onClose]);
    return (_jsx("div", { style: overlay, onClick: e => { if (e.target === e.currentTarget)
            onClose(); }, children: _jsxs("div", { style: modal, children: [_jsxs("div", { style: { display: 'flex', justifyContent: 'space-between', alignItems: 'center' }, children: [_jsxs("strong", { style: { fontSize: 16 }, children: ["\u6279\u91CF\u5BFC\u5165\u8282\u70B9", lakeName ? ` · ${lakeName}` : ''] }), _jsx("button", { onClick: onClose, style: { ...btnSecondary, padding: '4px 10px' }, children: "\u2715" })] }), done ? (_jsxs("div", { style: { textAlign: 'center', padding: '24px 0' }, children: [_jsx("div", { style: { fontSize: 32, marginBottom: 8 }, children: "\u2705" }), _jsxs("div", { style: { color: '#a6e3a1', fontSize: 15 }, children: ["\u6210\u529F\u5BFC\u5165 ", doneCount, " \u4E2A\u8282\u70B9"] }), _jsx("button", { onClick: onClose, style: { ...btnPrimary, marginTop: 16 }, children: "\u5173\u95ED" })] })) : (_jsxs(_Fragment, { children: [_jsxs("div", { style: { display: 'flex', gap: 8 }, children: [['json', 'csv'].map(t => (_jsx("button", { onClick: () => { setTab(t); setPreview(null); setParseError(''); }, style: tab === t ? { ...btnPrimary, padding: '6px 16px' } : { ...btnSecondary, padding: '6px 16px' }, children: t.toUpperCase() }, t))), _jsx("button", { onClick: () => fileRef.current?.click(), style: { ...btnSecondary, marginLeft: 'auto', padding: '6px 14px', fontSize: 12 }, title: "\u9009\u62E9\u6587\u4EF6\uFF08.json/.csv\uFF09", children: "\uD83D\uDCC2 \u9009\u62E9\u6587\u4EF6" }), _jsx("input", { ref: fileRef, type: "file", accept: ".json,.csv,.txt", style: { display: 'none' }, onChange: handleFileUpload })] }), _jsx("div", { style: { fontSize: 12, color: '#6c7086' }, children: tab === 'json'
                                ? '粘贴 JSON 数组，每项须包含 content 字段，type 字段可选（默认 TEXT）。最多 100 个节点。'
                                : '粘贴 CSV 数据，第一行为列标题（content/text, type）。' }), _jsx("textarea", { style: textarea, placeholder: tab === 'json'
                                ? '[{"content": "第一个节点", "type": "TEXT"}, ...]'
                                : 'content,type\n第一个节点,TEXT\n第二个节点,TEXT', value: raw, onChange: e => { setRaw(e.target.value); setPreview(null); setParseError(''); }, spellCheck: false }), parseError && (_jsxs("div", { style: { color: '#f38ba8', fontSize: 13 }, children: ["\u26A0 ", parseError] })), !preview && (_jsxs("div", { style: { display: 'flex', justifyContent: 'flex-end', gap: 8 }, children: [_jsx("button", { onClick: onClose, style: btnSecondary, children: "\u53D6\u6D88" }), _jsx("button", { onClick: handleParse, style: btnPrimary, disabled: !raw.trim(), children: "\u89E3\u6790\u9884\u89C8" })] })), preview && (_jsxs(_Fragment, { children: [_jsxs("div", { style: { fontSize: 13, color: '#a6e3a1' }, children: ["\u89E3\u6790\u6210\u529F ", preview.length, " \u4E2A\u8282\u70B9\uFF08\u5171 ", preview.length, " \u9879\uFF0C\u7A7A\u5185\u5BB9\u5DF2\u8DF3\u8FC7\uFF09", preview.length > 100 && (_jsx("span", { style: { color: '#f38ba8' }, children: " \u00B7 \u8D85\u51FA 100 \u4E0A\u9650\uFF0C\u5C06\u62D2\u7EDD\u63D0\u4EA4" }))] }), _jsxs("div", { style: { display: 'flex', flexDirection: 'column', gap: 6, maxHeight: 240, overflowY: 'auto' }, children: [preview.slice(0, 20).map((it, i) => (_jsxs("div", { style: previewRow, children: [_jsx("span", { style: badge, children: it.type }), _jsx("span", { style: { flex: 1, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }, children: it.content })] }, i))), preview.length > 20 && (_jsxs("div", { style: { textAlign: 'center', color: '#6c7086', fontSize: 12 }, children: ["... \u8FD8\u6709 ", preview.length - 20, " \u4E2A\u8282\u70B9\u672A\u663E\u793A"] }))] }), _jsxs("div", { style: { display: 'flex', justifyContent: 'flex-end', gap: 8 }, children: [_jsx("button", { onClick: () => { setPreview(null); setParseError(''); }, style: btnSecondary, children: "\u91CD\u65B0\u7F16\u8F91" }), _jsx("button", { onClick: handleConfirm, style: btnPrimary, disabled: loading || preview.length > 100, children: loading ? '导入中…' : `确认导入 ${preview.length} 个节点` })] })] }))] }))] }) }));
}
