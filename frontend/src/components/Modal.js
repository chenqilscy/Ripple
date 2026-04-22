import { jsx as _jsx, jsxs as _jsxs } from "react/jsx-runtime";
/**
 * Modal 通用组件 + useInputModal Hook（M2 收官 / Task-5 第一刀）。
 *
 * 设计取舍：
 *  - 不引入第三方 UI 库（保持 Vite + React 18 最小依赖）
 *  - 提供一个 Promise 风格的输入模态：调用方 `await prompt({...})` 拿字符串或 null
 *  - 多行输入（textarea）+ 单行输入两种模式
 *  - 取代 window.prompt：可输入长文本、UTF-8 友好、ESC/取消按钮、Enter 提交（多行用 Ctrl+Enter）
 *
 * 替换范围（本轮）：节点内容编辑 + 变更说明。其它 prompt（邀请/边标签/历史回滚）
 * 留 TD-010 后续迭代。
 */
import { useEffect, useRef, useState } from 'react';
let activeOpen = null;
/** 命令式 prompt：在任意业务代码里 `await prompt(...)`。 */
export function prompt(opts) {
    return new Promise((resolve) => {
        if (!activeOpen) {
            // 未挂载 host —— 退化到 window.prompt 兼容老逻辑
            const v = window.prompt(opts.title, opts.initial ?? '');
            resolve(v);
            return;
        }
        activeOpen(opts, resolve);
    });
}
/** 在 App 根挂载一次，提供命令式 prompt host。 */
export function ModalHost() {
    const [opts, setOpts] = useState(null);
    const [val, setVal] = useState('');
    const [err, setErr] = useState(null);
    const resolverRef = useRef(null);
    const inputRef = useRef(null);
    useEffect(() => {
        activeOpen = (o, resolve) => {
            setOpts(o);
            setVal(o.initial ?? '');
            setErr(null);
            resolverRef.current = resolve;
        };
        return () => {
            activeOpen = null;
        };
    }, []);
    useEffect(() => {
        if (opts && inputRef.current) {
            inputRef.current.focus();
            if ('select' in inputRef.current) {
                try {
                    inputRef.current.select();
                }
                catch {
                    /* noop */
                }
            }
        }
    }, [opts]);
    if (!opts)
        return null;
    const close = (out) => {
        const r = resolverRef.current;
        resolverRef.current = null;
        setOpts(null);
        if (r)
            r(out);
    };
    const submit = () => {
        if (opts.validate) {
            const e = opts.validate(val);
            if (e) {
                setErr(e);
                return;
            }
        }
        close(val);
    };
    const onKey = (e) => {
        if (e.key === 'Escape') {
            e.preventDefault();
            close(null);
        }
        else if (e.key === 'Enter') {
            if (opts.multiline && !(e.ctrlKey || e.metaKey))
                return;
            e.preventDefault();
            submit();
        }
    };
    return (_jsx("div", { role: "dialog", "aria-modal": "true", "aria-label": opts.title, onKeyDown: onKey, style: {
            position: 'fixed', inset: 0, background: 'rgba(0,0,0,0.45)',
            display: 'flex', alignItems: 'center', justifyContent: 'center', zIndex: 9999,
        }, onClick: (e) => { if (e.target === e.currentTarget)
            close(null); }, children: _jsxs("div", { style: {
                background: '#1f2330', color: '#e8ecf3', borderRadius: 10,
                padding: 20, minWidth: 380, maxWidth: '90vw', maxHeight: '90vh',
                boxShadow: '0 18px 48px rgba(0,0,0,0.4)', display: 'flex', flexDirection: 'column', gap: 10,
            }, children: [_jsx("div", { style: { fontSize: 16, fontWeight: 600 }, children: opts.title }), opts.label ? _jsx("div", { style: { fontSize: 13, opacity: 0.8 }, children: opts.label }) : null, opts.multiline ? (_jsx("textarea", { ref: (el) => { inputRef.current = el; }, value: val, onChange: e => { setVal(e.target.value); if (err)
                        setErr(null); }, placeholder: opts.placeholder, rows: 6, style: {
                        resize: 'vertical', minHeight: 96, maxHeight: '50vh',
                        padding: 10, borderRadius: 6, border: '1px solid #3b4358',
                        background: '#12141c', color: '#e8ecf3', fontSize: 14, lineHeight: 1.5,
                    } })) : (_jsx("input", { ref: (el) => { inputRef.current = el; }, value: val, onChange: e => { setVal(e.target.value); if (err)
                        setErr(null); }, placeholder: opts.placeholder, style: {
                        padding: '8px 10px', borderRadius: 6, border: '1px solid #3b4358',
                        background: '#12141c', color: '#e8ecf3', fontSize: 14,
                    } })), err ? _jsx("div", { style: { color: '#ff8a8a', fontSize: 12 }, children: err }) : null, _jsxs("div", { style: { display: 'flex', justifyContent: 'flex-end', gap: 8, marginTop: 4 }, children: [_jsx("button", { onClick: () => close(null), style: btnStyle(false), children: opts.cancelText ?? '取消' }), _jsx("button", { onClick: submit, style: btnStyle(true), children: opts.confirmText ?? '确定' })] }), opts.multiline ? (_jsx("div", { style: { fontSize: 11, opacity: 0.5, marginTop: -4 }, children: "Ctrl+Enter \u63D0\u4EA4 \u00B7 Esc \u53D6\u6D88" })) : null] }) }));
}
function btnStyle(primary) {
    return {
        padding: '6px 14px', borderRadius: 6, border: 0, cursor: 'pointer',
        background: primary ? '#4f7cff' : '#2c3142', color: '#fff', fontSize: 13,
    };
}
