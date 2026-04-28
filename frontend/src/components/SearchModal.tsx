// SearchModal · P12-D 全文搜索浮层 + P20-C 语义搜索
// 快捷键 Cmd+K / Ctrl+K 触发；在当前激活的湖内搜索节点。
import { useCallback, useEffect, useRef, useState } from 'react'
import { api } from '../api/client'
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
  // P22：过滤参数
  const [stateFilter, setStateFilter] = useState('')
  const [typeFilter, setTypeFilter] = useState('')
  const inputRef = useRef<HTMLInputElement>(null)
  const debounceRef = useRef<ReturnType<typeof setTimeout> | null>(null)

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
    if (!query.trim()) { setResults([]); return }
    setLoading(true)
    setError(null)
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

  return (
    // Backdrop
    <div
      onClick={onClose}
      style={{
        position: 'fixed', inset: 0,
        background: 'rgba(0,0,0,0.55)',
        zIndex: 9000,
        display: 'flex', alignItems: 'flex-start', justifyContent: 'center',
        paddingTop: '12vh',
      }}
    >
      {/* Modal panel */}
      <div
        onClick={e => e.stopPropagation()}
        style={{
          width: 560, maxWidth: '92vw',
          background: '#0d1526',
          border: '1px solid #1e3050',
          borderRadius: 12,
          boxShadow: '0 16px 48px rgba(0,0,0,0.6)',
          overflow: 'hidden',
        }}
      >
        {/* Header */}
        <div style={{ padding: '10px 16px 0', display: 'flex', alignItems: 'center', gap: 8 }}>
          <span style={{ color: '#4a8eff', fontSize: 16 }}>🔍</span>
          <input
            ref={inputRef}
            value={q}
            onChange={e => handleChange(e.target.value)}
            placeholder={`搜索「${lakeName ?? '当前湖'}」中的节点…`}
            style={{
              flex: 1, background: 'transparent', border: 'none', outline: 'none',
              color: '#e0f0ff', fontSize: 15, padding: '6px 0',
            }}
          />
          {/* P20-C: 语义搜索切换 */}
          <button
            onClick={handleModeToggle}
            title={semantic ? '当前：语义搜索（点击切换为关键词）' : '当前：关键词搜索（点击切换为语义）'}
            style={{
              background: semantic ? '#1e4d9e' : 'transparent',
              border: `1px solid ${semantic ? '#4a8eff' : '#2a3e5c'}`,
              borderRadius: 5, color: semantic ? '#9ec5ee' : '#6c7086',
              fontSize: 11, padding: '3px 8px', cursor: 'pointer', whiteSpace: 'nowrap',
            }}
          >
            {semantic ? '✦ 语义' : '关键词'}
          </button>
          {loading && (
            <span style={{ color: semantic ? '#4a8eff' : '#6c7086', fontSize: 12 }}>
              {semantic ? '✦ AI 理解中…' : '搜索中…'}
            </span>
          )}
        </div>

        <div style={{ height: 1, background: '#1e3050', margin: '10px 0 0' }} />

        {/* P22：过滤器行（仅关键词搜索可用） */}
        {!semantic && (
          <div style={{ padding: '6px 16px', display: 'flex', gap: 8, borderBottom: '1px solid #1a2840' }}>
            <select
              value={stateFilter}
              onChange={e => { setStateFilter(e.target.value); handleFilterChange(e.target.value, typeFilter) }}
              style={{
                background: '#1a2840', border: '1px solid #2a3e5c', borderRadius: 4,
                color: stateFilter ? '#c8d8e8' : '#5a7a9e', fontSize: 11, padding: '3px 6px', cursor: 'pointer',
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
              style={{
                background: '#1a2840', border: '1px solid #2a3e5c', borderRadius: 4,
                color: typeFilter ? '#c8d8e8' : '#5a7a9e', fontSize: 11, padding: '3px 6px', cursor: 'pointer',
              }}
            >
              <option value="">所有类型</option>
              <option value="TEXT">文本</option>
              <option value="IMAGE">图片</option>
              <option value="LINK">链接</option>
              <option value="AUDIO">音频</option>
            </select>
            {(stateFilter || typeFilter) && (
              <button
                onClick={() => { setStateFilter(''); setTypeFilter(''); handleFilterChange('', '') }}
                style={{
                  background: 'none', border: 'none', color: '#f38ba8',
                  fontSize: 11, cursor: 'pointer', padding: '2px 4px',
                }}
              >✕ 清除过滤</button>
            )}
          </div>
        )}

        {/* Results */}
        <div style={{ maxHeight: 400, overflowY: 'auto', padding: '4px 0 8px' }}>
          {error && (
            <div style={{ padding: '12px 16px', color: '#ff6b6b', fontSize: 13 }}>{error}</div>
          )}
          {!loading && !error && q && results.length === 0 && (
            <div style={{ padding: '12px 16px', color: '#6c7086', fontSize: 13 }}>未找到相关节点</div>
          )}
          {!q && (
            <div style={{ padding: '12px 16px', color: '#6c7086', fontSize: 13 }}>
              {semantic ? '✦ 语义搜索模式 · 输入自然语言描述查找相关节点 · Esc 关闭' : '输入关键词搜索节点内容 · Esc 关闭'}
            </div>
          )}
          {results.map(hit => (
            <button
              key={hit.node_id}
              onClick={() => { onSelect?.(hit); onClose() }}
              style={{
                display: 'block', width: '100%', textAlign: 'left',
                background: 'transparent', border: 'none', cursor: 'pointer',
                padding: '10px 16px',
                color: '#c0d4f5',
                borderBottom: '1px solid rgba(255,255,255,0.05)',
              }}
              onMouseEnter={e => { (e.currentTarget as HTMLElement).style.background = '#1a2a44' }}
              onMouseLeave={e => { (e.currentTarget as HTMLElement).style.background = 'transparent' }}
            >
              <div style={{ fontSize: 12, color: '#4a8eff', marginBottom: 3, fontFamily: 'monospace' }}>
                {hit.node_id.slice(0, 8)}…
              </div>
              <div style={{ fontSize: 13, lineHeight: 1.5, whiteSpace: 'pre-wrap', wordBreak: 'break-word' }}>
                {hit.snippet || '（无内容）'}
              </div>
              <div style={{ fontSize: 11, color: '#6c7086', marginTop: 2 }}>
                score: {hit.score.toFixed(3)}
              </div>
            </button>
          ))}
        </div>
      </div>
    </div>
  )
}
