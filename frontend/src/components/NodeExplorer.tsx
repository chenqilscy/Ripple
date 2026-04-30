/**
 * P19-A: AI 图谱探索面板。
 * 支持用户输入查询词，后端基于 TF 打分 + 单次 LLM 摘要返回相关节点列表。
 * 修复：scroll lock + CSS 变量（Deep Ocean Dark 主题）
 */
import React, { useEffect, useRef, useState } from 'react'
import { api } from '../api/client'

interface ExploreNode {
  node_id: string
  content: string
  score: number
}

interface Props {
  lakeId: string
  /** 高亮回调：将相关节点 id 集合传给父组件（如 LakeGraph） */
  onHighlight: (ids: Set<string>) => void
  onClose: () => void
}

export default function NodeExplorer({ lakeId, onHighlight, onClose }: Props) {
  const [query, setQuery] = useState('')
  const [loading, setLoading] = useState(false)
  const [results, setResults] = useState<ExploreNode[]>([])
  const [summary, setSummary] = useState('')
  const [err, setErr] = useState<string | null>(null)
  const [searched, setSearched] = useState(false)
  const inputRef = useRef<HTMLInputElement>(null)

  // Scroll lock
  useEffect(() => {
    const prev = document.body.style.overflow
    document.body.style.overflow = 'hidden'
    return () => { document.body.style.overflow = prev }
  }, [])

  // Escape to close
  useEffect(() => {
    const handler = (e: KeyboardEvent) => { if (e.key === 'Escape') onClose() }
    window.addEventListener('keydown', handler)
    return () => window.removeEventListener('keydown', handler)
  }, [onClose])

  async function handleExplore() {
    const q = query.trim()
    if (!q) return
    setLoading(true)
    setErr(null)
    try {
      const res = await api.exploreGraph(lakeId, q)
      setResults(res.relevant_nodes)
      setSummary(res.summary)
      setSearched(true)
      onHighlight(new Set(res.relevant_nodes.map(n => n.node_id)))
    } catch (e: unknown) {
      setErr(e instanceof Error ? e.message : 'AI 探索失败，请稍后重试')
    } finally {
      setLoading(false)
    }
  }

  function handleKeyDown(e: React.KeyboardEvent) {
    if (e.key === 'Enter') void handleExplore()
  }

  function handleClear() {
    setQuery('')
    setResults([])
    setSummary('')
    setErr(null)
    setSearched(false)
    onHighlight(new Set())
    inputRef.current?.focus()
  }

  return (
    <div style={{
      position: 'fixed', top: 0, right: 0, bottom: 0,
      width: 360, background: 'var(--bg-primary)', color: 'var(--text-primary)',
      borderLeft: '1px solid var(--border)', zIndex: 1000,
      display: 'flex', flexDirection: 'column', padding: 'var(--space-lg)',
      fontFamily: 'var(--font-body)', fontSize: 'var(--font-base)',
    }}>
      {/* 标题栏 */}
      <div style={{ display: 'flex', alignItems: 'center', marginBottom: 'var(--space-lg)' }}>
        <span style={{ fontWeight: 700, fontSize: 'var(--font-lg)', flex: 1, color: 'var(--accent)' }}>🔍 AI 图谱探索</span>
        <button
          onClick={onClose}
          style={{
            background: 'none', border: 'none', color: 'var(--text-secondary)',
            cursor: 'pointer', fontSize: 'var(--font-lg)', lineHeight: 1,
          }}
          aria-label="关闭"
        >×</button>
      </div>

      {/* 搜索框 */}
      <div style={{ display: 'flex', gap: 'var(--space-sm)', marginBottom: 'var(--space-md)' }}>
        <input
          ref={inputRef}
          value={query}
          onChange={e => setQuery(e.target.value)}
          onKeyDown={handleKeyDown}
          placeholder="输入探索词，如「产品设计」"
          style={{
            flex: 1, padding: 'var(--space-sm) var(--space-md)',
            borderRadius: 'var(--radius-md)',
            border: '1px solid var(--border-input)', background: 'var(--bg-input)',
            color: 'var(--text-primary)', outline: 'none', fontSize: 'var(--font-base)',
          }}
        />
        <button
          onClick={() => void handleExplore()}
          disabled={loading || !query.trim()}
          style={{
            padding: 'var(--space-sm) var(--space-lg)', borderRadius: 'var(--radius-md)',
            border: 'none',
            background: loading ? 'var(--bg-tertiary)' : 'var(--accent)',
            color: 'var(--text-inverse)', cursor: loading || !query.trim() ? 'not-allowed' : 'pointer',
            fontWeight: 600, fontSize: 'var(--font-base)', whiteSpace: 'nowrap',
          }}
        >
          {loading ? '探索中…' : '探索'}
        </button>
      </div>

      {/* 错误提示 */}
      {err && (
        <div style={{
          background: 'var(--status-danger-subtle)',
          border: '1px solid var(--status-danger)',
          borderRadius: 'var(--radius-md)',
          padding: 'var(--space-sm) var(--space-md)', marginBottom: 'var(--space-md)',
          color: 'var(--status-danger)', fontSize: 'var(--font-md)',
        }}>
          {err}
        </div>
      )}

      {/* LLM 摘要 */}
      {summary && (
        <div style={{
          background: 'var(--bg-surface)',
          border: '1px solid var(--border)',
          borderRadius: 'var(--radius-lg)',
          padding: 'var(--space-md)',
          marginBottom: 'var(--space-md)',
          color: 'var(--status-success)', fontSize: 'var(--font-md)', lineHeight: 1.6,
        }}>
          <div style={{ fontWeight: 600, marginBottom: 'var(--space-xs)', color: 'var(--accent)' }}>AI 摘要</div>
          {summary}
        </div>
      )}

      {/* 结果列表 */}
      <div style={{ flex: 1, overflowY: 'auto' }}>
        {searched && results.length === 0 && !loading && (
          <div style={{ color: 'var(--text-tertiary)', textAlign: 'center', marginTop: 'var(--space-xl)', fontSize: 'var(--font-md)' }}>
            未找到与「{query}」相关的节点
          </div>
        )}

        {results.map((node, idx) => (
          <div
            key={node.node_id}
            style={{
              background: 'var(--bg-surface)',
              border: '1px solid var(--border)',
              borderRadius: 'var(--radius-lg)',
              padding: 'var(--space-md)',
              marginBottom: 'var(--space-sm)', cursor: 'pointer',
              transition: 'border-color 0.15s',
            }}
            onMouseEnter={e => (e.currentTarget.style.borderColor = 'var(--accent)')}
            onMouseLeave={e => (e.currentTarget.style.borderColor = 'var(--border)')}
            onClick={() => onHighlight(new Set([node.node_id]))}
          >
            <div style={{
              display: 'flex', alignItems: 'flex-start', gap: 'var(--space-sm)', marginBottom: 'var(--space-xs)',
            }}>
              <span style={{
                background: 'var(--bg-tertiary)', color: 'var(--accent)',
                borderRadius: 'var(--radius-sm)', padding: '1px var(--space-sm)', fontSize: 'var(--font-sm)', flexShrink: 0,
              }}>#{idx + 1}</span>
              <span style={{
                fontSize: 'var(--font-sm)', color: 'var(--text-tertiary)', marginLeft: 'auto', flexShrink: 0,
              }}>
                相关度 {node.score.toFixed(2)}
              </span>
            </div>
            <p style={{
              margin: 0, color: 'var(--text-primary)', fontSize: 'var(--font-md)',
              lineHeight: 1.5, wordBreak: 'break-all',
            }}>
              {node.content}
            </p>
          </div>
        ))}
      </div>

      {/* 底部操作栏 */}
      {searched && (
        <div style={{
          paddingTop: 'var(--space-md)', borderTop: '1px solid var(--border)',
          display: 'flex', justifyContent: 'space-between', alignItems: 'center',
        }}>
          <span style={{ fontSize: 'var(--font-sm)', color: 'var(--text-tertiary)' }}>
            共 {results.length} 个相关节点已高亮
          </span>
          <button
            onClick={handleClear}
            style={{
              background: 'none', border: '1px solid var(--border)',
              borderRadius: 'var(--radius-md)', padding: 'var(--space-xs) var(--space-md)',
              color: 'var(--text-secondary)', cursor: 'pointer', fontSize: 'var(--font-sm)',
            }}
          >
            清除
          </button>
        </div>
      )}
    </div>
  )
}
