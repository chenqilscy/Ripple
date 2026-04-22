import { jsx as _jsx, jsxs as _jsxs } from "react/jsx-runtime";
// AttachmentBar · M5-T1 节点附件上传 UI（最小可用版本）
//
// 用法：在任意页面引入 <AttachmentBar nodeId={...} /> 即可。
// 支持：drag-drop / 点击选择；上传成功后展示缩略图与下载链接。
import { useCallback, useEffect, useRef, useState } from 'react';
import { api } from '../api/client';
import { getToken } from '../api/client';
const ALLOWED = ['image/png', 'image/jpeg', 'image/gif', 'image/webp'];
// 用 fetch+blob 拉取受鉴权保护的图片，避免直接用 <img src> 漏认证。
function useBlobURL(id) {
    const [url, setUrl] = useState(null);
    useEffect(() => {
        let cancelled = false;
        let createdURL = null;
        const tok = getToken();
        fetch(api.attachmentURL(id), { headers: tok ? { Authorization: `Bearer ${tok}` } : {} })
            .then(r => r.ok ? r.blob() : Promise.reject(new Error('fetch fail')))
            .then(b => {
            if (cancelled)
                return;
            createdURL = URL.createObjectURL(b);
            setUrl(createdURL);
        })
            .catch(() => { });
        return () => {
            cancelled = true;
            if (createdURL)
                URL.revokeObjectURL(createdURL);
        };
    }, [id]);
    return url;
}
function Thumb({ id, mime, size }) {
    const url = useBlobURL(id);
    return (_jsx("a", { href: url ?? '#', target: "_blank", rel: "noreferrer", title: `${mime} · ${(size / 1024).toFixed(1)} KB`, style: { display: 'block', width: 56, height: 56, borderRadius: 4, overflow: 'hidden', background: '#161616' }, children: url ? (_jsx("img", { src: url, alt: "", style: { width: '100%', height: '100%', objectFit: 'cover' } })) : (_jsx("div", { style: { width: '100%', height: '100%', display: 'flex', alignItems: 'center', justifyContent: 'center', fontSize: 10, opacity: 0.5 }, children: "\u2026" })) }));
}
export default function AttachmentBar({ nodeId, onUploaded, compact }) {
    const [items, setItems] = useState([]);
    const [dragOver, setDragOver] = useState(false);
    const [busy, setBusy] = useState(false);
    const [err, setErr] = useState(null);
    const inputRef = useRef(null);
    const handleFiles = useCallback(async (files) => {
        if (!files || files.length === 0)
            return;
        setErr(null);
        setBusy(true);
        try {
            for (const f of Array.from(files)) {
                if (!ALLOWED.includes(f.type)) {
                    setErr(`不支持的类型：${f.type || '未知'}`);
                    continue;
                }
                if (f.size > 5 * 1024 * 1024) {
                    setErr('文件超过 5MB');
                    continue;
                }
                const a = await api.uploadAttachment(f, nodeId);
                setItems(prev => [a, ...prev].slice(0, 12));
                onUploaded?.(a);
            }
        }
        catch (e) {
            setErr(e.message);
        }
        finally {
            setBusy(false);
        }
    }, [nodeId, onUploaded]);
    const onDrop = useCallback((e) => {
        e.preventDefault();
        setDragOver(false);
        void handleFiles(e.dataTransfer.files);
    }, [handleFiles]);
    return (_jsxs("div", { onDragOver: (e) => { e.preventDefault(); setDragOver(true); }, onDragLeave: () => setDragOver(false), onDrop: onDrop, style: {
            border: `1px dashed ${dragOver ? '#4a8eff' : '#3a4555'}`,
            borderRadius: 6,
            padding: compact ? '6px 8px' : '12px',
            background: dragOver ? '#1d2433' : '#0e1218',
            transition: 'all 120ms',
        }, children: [_jsxs("div", { style: { display: 'flex', alignItems: 'center', gap: 8 }, children: [_jsx("input", { ref: inputRef, type: "file", accept: ALLOWED.join(','), multiple: true, style: { display: 'none' }, onChange: (e) => { void handleFiles(e.target.files); if (inputRef.current)
                            inputRef.current.value = ''; } }), _jsxs("button", { onClick: () => inputRef.current?.click(), disabled: busy, style: {
                            background: '#4a8eff', color: '#fff', border: 'none',
                            padding: '4px 10px', borderRadius: 4, fontSize: 12, cursor: 'pointer',
                            opacity: busy ? 0.5 : 1,
                        }, children: ["\uD83D\uDCCE ", busy ? '上传中…' : '附件'] }), _jsx("span", { style: { fontSize: 11, opacity: 0.6, flex: 1 }, children: "\u62D6\u62FD\u56FE\u7247\u5230\u6B64 \u00B7 png/jpg/gif/webp \u00B7 \u22645MB" }), nodeId && _jsxs("span", { style: { fontSize: 10, opacity: 0.4 }, children: ["node ", nodeId.slice(0, 8)] })] }), err && (_jsxs("div", { style: { marginTop: 6, color: '#d24343', fontSize: 11 }, children: ["\u26A0 ", err] })), items.length > 0 && (_jsx("div", { style: { marginTop: 8, display: 'flex', flexWrap: 'wrap', gap: 6 }, children: items.map(a => (_jsx(Thumb, { id: a.id, mime: a.mime, size: a.size_bytes }, a.id))) }))] }));
}
