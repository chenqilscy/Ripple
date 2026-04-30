/**
 * Modal 通用组件 + useInputModal Hook（M2 收官 / Task-5 第一刀）。
 *
 * 设计取舍：
 *  - 不引入第三方 UI 库（保持 Vite + React 18 最小依赖）
 *  - 提供一个 Promise 风格的输入模态：调用方 `await prompt({...})` 拿字符串或 null
 *  - 多行输入（textarea）+ 单行输入两种模式
 *  - 取代 window.prompt：可输入长文本、UTF-8 友好、ESC/取消按钮、Enter 提交（多行用 Ctrl+Enter）
 *
 * 修复：scroll lock + CSS 变量
 */
import React, { useEffect, useRef, useState } from 'react'
import { Button } from './ui'

export type PromptOptions = {
    title: string
    label?: string
    initial?: string
    placeholder?: string
    multiline?: boolean
    confirmText?: string
    cancelText?: string
    /** 校验返回错误信息字符串则禁用提交；返回 null/undefined 通过 */
    validate?: (val: string) => string | null | undefined
    /**
     * 'prompt' = 显示输入框（默认）
     * 'confirm' = 只显示 label + 确认/取消（无输入框）
     * 'alert' = 只显示 label + 单个确认按钮
     */
    kind?: 'prompt' | 'confirm' | 'alert'
    /** danger=true 时确认按钮变红色（删除等危险操作） */
    danger?: boolean
}

type Resolver = (val: string | null) => void

let activeOpen: ((opts: PromptOptions, resolve: Resolver) => void) | null = null

/** 命令式 prompt：在任意业务代码里 `await prompt(...)`。 */
export function prompt(opts: PromptOptions): Promise<string | null> {
    return new Promise((resolve) => {
        if (!activeOpen) {
            // 未挂载 host —— 退化到 window 原生
            const kind = opts.kind ?? 'prompt'
            if (kind === 'confirm') {
                resolve(window.confirm(opts.label ?? opts.title) ? '' : null)
                return
            }
            if (kind === 'alert') {
                window.alert(opts.label ?? opts.title)
                resolve('')
                return
            }
            resolve(window.prompt(opts.title, opts.initial ?? ''))
            return
        }
        activeOpen(opts, resolve)
    })
}

/**
 * 命令式 confirm：取代 window.confirm，返回 Promise<boolean>。
 * 渲染只有 label 没有输入框；点确认 → true，取消/ESC/点遮罩 → false。
 */
export function confirm(message: string, opts?: { title?: string; confirmText?: string; cancelText?: string; danger?: boolean }): Promise<boolean> {
    return prompt({
        kind: 'confirm',
        title: opts?.title ?? '确认',
        label: message,
        confirmText: opts?.confirmText ?? (opts?.danger ? '删除' : '确定'),
        cancelText: opts?.cancelText ?? '取消',
        danger: opts?.danger,
    }).then(v => v !== null)
}

/** 命令式 alert：单按钮确认，返回 Promise<void>。 */
export function alert(message: string, opts?: { title?: string; confirmText?: string }): Promise<void> {
    return prompt({
        kind: 'alert',
        title: opts?.title ?? '提示',
        label: message,
        confirmText: opts?.confirmText ?? '知道了',
    }).then(() => undefined)
}

/** 在 App 根挂载一次，提供命令式 prompt host。 */
export function ModalHost(): React.ReactElement | null {
    const [opts, setOpts] = useState<PromptOptions | null>(null)
    const [val, setVal] = useState('')
    const [err, setErr] = useState<string | null>(null)
    const resolverRef = useRef<Resolver | null>(null)
    const inputRef = useRef<HTMLInputElement | HTMLTextAreaElement | null>(null)

    useEffect(() => {
        activeOpen = (o, resolve) => {
            setOpts(o)
            setVal(o.initial ?? '')
            setErr(null)
            resolverRef.current = resolve
        }
        return () => {
            activeOpen = null
        }
    }, [])

    useEffect(() => {
        if (opts && (opts.kind ?? 'prompt') === 'prompt' && inputRef.current) {
            inputRef.current.focus()
            if ('select' in inputRef.current) {
                try {
                    (inputRef.current as HTMLInputElement).select()
                } catch {
                    /* noop */
                }
            }
        }
    }, [opts])

    // Scroll lock: prevent body scroll while modal is open
    useEffect(() => {
        if (!opts) return
        const prev = document.body.style.overflow
        document.body.style.overflow = 'hidden'
        return () => { document.body.style.overflow = prev }
    }, [opts])

    if (!opts) return null

    const close = (out: string | null) => {
        const r = resolverRef.current
        resolverRef.current = null
        setOpts(null)
        if (r) r(out)
    }

    const submit = () => {
        if ((opts.kind ?? 'prompt') === 'prompt' && opts.validate) {
            const e = opts.validate(val)
            if (e) {
                setErr(e)
                return
            }
        }
        close((opts.kind ?? 'prompt') === 'prompt' ? val : '')
    }

    const onKey = (e: React.KeyboardEvent) => {
        if (e.key === 'Escape') {
            e.preventDefault()
            close(null)
        } else if (e.key === 'Enter') {
            if ((opts.kind ?? 'prompt') !== 'prompt') {
                e.preventDefault()
                submit()
                return
            }
            if (opts.multiline && !(e.ctrlKey || e.metaKey)) return
            e.preventDefault()
            submit()
        }
    }

    const kind = opts.kind ?? 'prompt'
    const showInput = kind === 'prompt'
    const showCancel = kind !== 'alert'

    return (
        <div
            role="dialog"
            aria-modal="true"
            aria-label={opts.title}
            onKeyDown={onKey}
            style={{
                position: 'fixed', inset: 0, background: 'var(--bg-overlay)',
                display: 'flex', alignItems: 'center', justifyContent: 'center', zIndex: 9999,
            }}
            onClick={(e) => { if (e.target === e.currentTarget) close(null) }}
        >
            <div style={{
                background: 'var(--bg-primary)',
                color: 'var(--text-primary)',
                borderRadius: 'var(--radius-xl)',
                padding: 'var(--space-xl)',
                minWidth: 380, maxWidth: '90vw', maxHeight: '90vh',
                boxShadow: 'var(--shadow-overlay)',
                display: 'flex', flexDirection: 'column', gap: 'var(--space-md)',
                border: '1px solid var(--border)',
            }}>
                <div style={{ fontSize: 'var(--font-lg)', fontWeight: 600, color: 'var(--text-primary)' }}>{opts.title}</div>
                {opts.label ? <div style={{ fontSize: 'var(--font-md)', color: 'var(--text-secondary)', whiteSpace: 'pre-wrap' }}>{opts.label}</div> : null}
                {showInput && (opts.multiline ? (
                    <textarea
                        ref={(el) => { inputRef.current = el }}
                        value={val}
                        onChange={e => { setVal(e.target.value); if (err) setErr(null) }}
                        placeholder={opts.placeholder}
                        rows={6}
                        style={{
                            resize: 'vertical', minHeight: 96, maxHeight: '50vh',
                            padding: 'var(--space-md)',
                            borderRadius: 'var(--radius-md)',
                            border: '1px solid var(--border-input)',
                            background: 'var(--bg-input)',
                            color: 'var(--text-primary)',
                            fontSize: 'var(--font-base)', lineHeight: 1.5,
                        }}
                    />
                ) : (
                    <input
                        ref={(el) => { inputRef.current = el }}
                        value={val}
                        onChange={e => { setVal(e.target.value); if (err) setErr(null) }}
                        placeholder={opts.placeholder}
                        style={{
                            padding: 'var(--space-sm) var(--space-md)',
                            borderRadius: 'var(--radius-md)',
                            border: '1px solid var(--border-input)',
                            background: 'var(--bg-input)',
                            color: 'var(--text-primary)',
                            fontSize: 'var(--font-base)',
                        }}
                    />
                ))}
                {err ? <div style={{ color: 'var(--status-danger)', fontSize: 'var(--font-sm)' }}>{err}</div> : null}
                <div style={{ display: 'flex', justifyContent: 'flex-end', gap: 'var(--space-sm)', marginTop: 'var(--space-xs)' }}>
                    {showCancel && (
                        <Button variant="secondary" onClick={() => close(null)}>
                            {opts.cancelText ?? '取消'}
                        </Button>
                    )}
                    <Button variant={opts.danger ? 'danger' : 'primary'} onClick={submit}>
                        {opts.confirmText ?? '确定'}
                    </Button>
                </div>
                {showInput && opts.multiline ? (
                    <div style={{ fontSize: 'var(--font-xs)', color: 'var(--text-tertiary)', marginTop: -4 }}>
                        Ctrl+Enter 提交 · Esc 取消
                    </div>
                ) : null}
            </div>
        </div>
    )
}

