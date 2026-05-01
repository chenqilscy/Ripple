// SearchModal · P12-D 全文搜索浮层 + P20-C 语义搜索
// 快捷键 Cmd+K / Ctrl+K 触发；在当前激活的湖内搜索节点。
// 修复：scroll lock + 键盘导航 + 响应式 + 统一样式
import React, { useCallback, useEffect, useRef, useState } from 'react'
import { api } from '../api/client'
import { Button } from './ui'
import type { SearchHit } from '../api/types'

interface Props {
  lakeId: string
  lakeName?: string
  onClose: () => void
  onSelect?: (hit: SearchHit) => void
}

export default function SearchModal({ lakeId, lakeName, onClose, onSelect }: Props) {
  const [q, setQ] = useState('')
  const [results, setResults] = useState<SearchHit[]>([])
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [semantic, setSemantic] = useState(false)
  const [stateFilter, setStateFilter] = useState('')
  const [typeFilter, setTypeFilter] = useState('')
  const [focusedIndex, setFocusedIndex] = useState(-1)
  const inputRef = useRef<HTMLInputElement>(null)
  const debounceRef = useRef<ReturnType<typeof setTimeout> | null>(null)

  // Scroll lock: prevent body scroll while modal is open
  useEffect(() => {
    const prev = document.body.style.overflow
    document.body.style.overflow = 'hidden'
    return () => { document.body.style.overflow = prev }
  }, [])

  // Focus input on open
  useEffect(() => {
    inputRef.current?.focus()
  }, [])

  // Close on Escape
  useEffect(() => {
    const handler = (e: KeyboardEvent) => {
      if (e.key === 'Escape') onClose()
    }
    window.addEventListener('keydown', handler)
    return () => window.removeEventListener('keydown', handler)
  }, [onClose])

  const doSearch = useCallback(async (query: string, isSemantic: boolean, st: string, tp: string) => {
    if (!query.trim()) { setResults([]); setFocusedIndex(-1); return }
    setLoading(true)
    setError(null)
    setFocusedIndex(-1)
    try {
      if (isSemantic) {
        const { results: hits } = await api.semanticSearchNodes(query.trim(), lakeId)
        setResults(hits)
      } else {
        const { results: hits } = await api.searchNodes(query.trim(), lakeId, 20, st || undefined, tp || undefined)
        setResults(hits)
      }
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : 'Search failed')
    } finally {
      setLoading(false)
    }
  }, [lakeId])

  const handleChange = (val: string) => {
    setQ(val)
    if (debounceRef.current) clearTimeout(debounceRef.current)
    debounceRef.current = setTimeout(() => void doSearch(val, semantic, stateFilter, typeFilter), 300)
  }

  const handleModeToggle = () => {
    const next = !semantic
    setSemantic(next)
    if (q.trim()) void doSearch(q, next, stateFilter, typeFilter)
  }

  const handleFilterChange = (newState: string, newType: string) => {
    if (q.trim()) void doSearch(q, semantic, newState, newType)
  }

  // Keyboard navigation: ArrowUp / ArrowDown / Enter
  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === 'ArrowDown') {
      e.preventDefault()
      setFocusedIndex(i => Math.min(i + 1, results.length - 1))
    } else if (e.key === 'ArrowUp') {
      e.preventDefault()
      setFocusedIndex(i => Math.max(i - 1, -1))
    } else if (e.key === 'Enter' && focusedIndex >= 0 && results[focusedIndex]) {
      e.preventDefault()
      onSelect?.(results[focusedIndex])
      onClose()
    }
  }

  return (
    // Backdrop
    <div
      onClick={onClose}
      style={{
        position: 'fixed', inset: 0,
        background: 'var(--bg-overlay)',
        zIndex: 9000,
        display: 'flex', alignItems: 'flex-start', justifyContent: 'center',
        paddingTop: '10vh',
      }}
    >
      {/* Modal panel */}
      <div
        onClick={e => e.stopPropagation()}
        onKeyDown={handleKeyDown}
        role="dialog"
        aria-modal="true"
        aria-label={`搜索 ${lakeName ?? '当前湖'} 中的节点`}
        style={{
          width: 560, maxWidth: '92vw',
          background: 'var(--bg-primary)',
          border: '1px solid var(--border)',
          borderRadius: 'var(--radius-xl)',
          boxShadow: 'var(--shadow-overlay)',
          overflow: 'hidden',
        }}
      >
        {/* Header */}
        <div style={{ padding: 'var(--space-lg) var(--space-lg) 0', display: 'flex', alignItems: 'center', gap: 'var(--space-sm)' }}>
          <span style={{ color: 'var(--accent)', fontSize: 'var(--font-xl)' }}>🔍</span>
          <input
            ref={inputRef}
            value={q}
            onChange={e => handleChange(e.target.value)}
            placeholder={`搜索「${lakeName ?? '当前湖'}」中的节点…`}
            aria-label="搜索关键词"
            style={{
              flex: 1, background: 'transparent', border: 'none', outline: 'none',
              color: 'var(--text-primary)', fontSize: 'var(--font-xl)', padding: 'var(--space-sm) 0',
            }}
          />
          {/* 语义搜索切换 */}
          <Button
            variant={semantic ? 'primary' : 'ghost'}
            size="sm"
            onClick={handleModeToggle}
            title={semantic ? '当前：语义搜索（点击切换为关键词）' : '当前：关键词搜索（点击切换为语义）'}
            aria-pressed={semantic}
          >
            {semantic ? '✦ 语义' : '关键词'}
          </Button>
          {loading && (
            <span style={{ color: semantic ? 'var(--accent)' : 'var(--text-tertiary)', fontSize: 'var(--font-md)' }}>
              {semantic ? '✦ AI 理解中…' : '搜索中…'}
            </span>
          )}
        </div>

        <div style={{ height: 1, background: 'var(--border)', margin: 'var(--space-lg) 0 0' }} />

        {/* 过滤器行（仅关键词搜索可用） */}
        {!semantic && (
          <div style={{ padding: 'var(--space-sm) var(--space-lg)', display: 'flex', gap: 'var(--space-sm)', borderBottom: '1px solid var(--border-subtle)' }}>
            <select
              value={stateFilter}
              onChange={e => { setStateFilter(e.target.value); handleFilterChange(e.target.value, typeFilter) }}
              aria-label="按状态过滤"
              style={{
                background: 'var(--bg-secondary)', border: '1px solid var(--border-input)', borderRadius: 'var(--radius-sm)',
                color: stateFilter ? 'var(--text-primary)' : 'var(--text-tertiary)', fontSize: 'var(--font-sm)', padding: '3px 6px', cursor: 'pointer',
              }}
            >
              <option value="">所有状态</option>
              <option value="MIST">雾态</option>
              <option value="DROP">水滴</option>
              <option value="FROZEN">冻结</option>
              <option value="VAPOR">蒸发</option>
              <option value="GHOST">幽灵</option>
            </select>
            <select
              value={typeFilter}
              onChange={e => { setTypeFilter(e.target.value); handleFilterChange(stateFilter, e.target.value) }}
              aria-label="按类型过滤"
              style={{
                background: 'var(--bg-secondary)', border: '1px solid var(--border-input)', borderRadius: 'var(--radius-sm)',
                color: typeFilter ? 'var(--text-primary)' : 'var(--text-tertiary)', fontSize: 'var(--font-sm)', padding: '3px 6px', cursor: 'pointer',
              }}
            >
              <option value="">所有类型</option>
              <option value="TEXT">文本</option>
              <option value="IMAGE">图片</option>
              <option value="LINK">链接</option>
              <option value="AUDIO">音频</option>
            </select>
            {(stateFilter || typeFilter) && (
              <Button
                variant="ghost"
                size="sm"
                onClick={() => { setStateFilter(''); setTypeFilter(''); handleFilterChange('', '') }}
                aria-label="清除过滤"
              >✕ 清除</Button>
            )}
          </div>
        )}

        {/* Results */}
        <div style={{ maxHeight: 400, overflowY: 'auto', padding: 'var(--space-xs) 0 var(--space-md)' }}
          role="listbox"
          aria-label="搜索结果"
        >
          {error && (
            <div style={{ padding: 'var(--space-lg)', color: 'var(--status-danger)', fontSize: 'var(--font-md)' }}>{error}</div>
          )}
          {!loading && !error && q && results.length === 0 && (
            <div style={{ padding: 'var(--space-lg)', color: 'var(--text-tertiary)', fontSize: 'var(--font-md)' }}>未找到相关节点</div>
          )}
          {!q && (
            <div style={{ padding: 'var(--space-lg)', color: 'var(--text-tertiary)', fontSize: 'var(--font-md)' }}>
              {semantic
                ? '✦ 语义搜索模式 · 输入自然语言描述查找相关节点 · Esc 关闭'
                : '输入关键词搜索节点内容 · ↑↓ 键导航 · Esc 关闭'}
            </div>
          )}
          {results.map((hit, idx) => (
            <Button
              key={hit.node_id}
              variant="ghost"
              onClick={() => { onSelect?.(hit); onClose() }}
              style={{
                display: 'flex', flexDirection: 'column', gap: 'var(--space-xs)',
                width: '100%', textAlign: 'left',
                background: focusedIndex === idx ? 'var(--accent-subtle)' : 'transparent',
                padding: 'var(--space-md) var(--space-lg)',
                borderRadius: 0,
              }}
              onMouseEnter={e => { if (focusedIndex !== idx) e.currentTarget.style.background = 'var(--bg-secondary)' }}
              onMouseLeave={e => { if (focusedIndex !== idx) e.currentTarget.style.background = 'transparent' }}
            >
              <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', gap: 'var(--space-sm)' }}>
                <code style={{ fontSize: 'var(--font-sm)', color: 'var(--accent)', fontFamily: 'var(--font-mono)' }}>
                  {hit.node_id.slice(0, 8)}…
                </code>
                <span style={{ fontSize: 'var(--font-xs)', color: 'var(--text-tertiary)' }}>
                  score: {hit.score.toFixed(3)}
                </span>
              </div>
              <div style={{ fontSize: 'var(--font-base)', lineHeight: 1.5, whiteSpace: 'pre-wrap', wordBreak: 'break-word' }}>
                {hit.snippet || '（无内容）'}
              </div>
            </Button>
          ))}
        </div>
      </div>
    </div>
  )
}
