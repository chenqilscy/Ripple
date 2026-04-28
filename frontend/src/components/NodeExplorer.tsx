/**
 * P19-A: AI 图谱探索面板。
 * 支持用户输入查询词，后端基于 TF 打分 + 单次 LLM 摘要返回相关节点列表。
 */
import { useState, useRef } from 'react'
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
    if (e.key === 'Enter') handleExplore()
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
      width: 360, background: '#1e1e2e', color: '#cdd6f4',
      borderLeft: '1px solid #313244', zIndex: 1000,
      display: 'flex', flexDirection: 'column', padding: '16px',
      fontFamily: 'system-ui, sans-serif', fontSize: 14,
    }}>
      {/* 标题栏 */}
      <div style={{ display: 'flex', alignItems: 'center', marginBottom: 16 }}>
        <span style={{ fontWeight: 700, fontSize: 16, flex: 1 }}>🔍 AI 图谱探索</span>
        <button
          onClick={onClose}
          style={{
            background: 'none', border: 'none', color: '#6c7086',
            cursor: 'pointer', fontSize: 18, lineHeight: 1,
          }}
          aria-label="关闭"
        >×</button>
      </div>

      {/* 搜索框 */}
      <div style={{ display: 'flex', gap: 6, marginBottom: 12 }}>
        <input
          ref={inputRef}
          value={query}
          onChange={e => setQuery(e.target.value)}
          onKeyDown={handleKeyDown}
          placeholder="输入探索词，如「产品设计」"
          style={{
            flex: 1, padding: '7px 10px', borderRadius: 6,
            border: '1px solid #45475a', background: '#181825',
            color: '#cdd6f4', outline: 'none', fontSize: 14,
          }}
        />
        <button
          onClick={handleExplore}
          disabled={loading || !query.trim()}
          style={{
            padding: '7px 14px', borderRadius: 6,
            border: 'none', background: loading ? '#45475a' : '#7c3aed',
            color: '#fff', cursor: loading || !query.trim() ? 'not-allowed' : 'pointer',
            fontWeight: 600, fontSize: 14, whiteSpace: 'nowrap',
          }}
        >
          {loading ? '探索中…' : '探索'}
        </button>
      </div>

      {/* 错误提示 */}
      {err && (
        <div style={{
          background: '#45213a', border: '1px solid #f38ba8',
          borderRadius: 6, padding: '8px 10px', marginBottom: 10,
          color: '#f38ba8', fontSize: 13,
        }}>
          {err}
        </div>
      )}

      {/* LLM 摘要 */}
      {summary && (
        <div style={{
          background: '#181825', border: '1px solid #45475a',
          borderRadius: 8, padding: '10px 12px', marginBottom: 12,
          color: '#a6e3a1', fontSize: 13, lineHeight: 1.6,
        }}>
          <div style={{ fontWeight: 600, marginBottom: 4, color: '#89dceb' }}>AI 摘要</div>
          {summary}
        </div>
      )}

      {/* 结果列表 */}
      <div style={{ flex: 1, overflowY: 'auto' }}>
        {searched && results.length === 0 && !loading && (
          <div style={{ color: '#6c7086', textAlign: 'center', marginTop: 32, fontSize: 13 }}>
            未找到与「{query}」相关的节点
          </div>
        )}

        {results.map((node, idx) => (
          <div
            key={node.node_id}
            style={{
              background: '#181825',
              border: '1px solid #313244',
              borderRadius: 8, padding: '10px 12px',
              marginBottom: 8, cursor: 'pointer',
              transition: 'border-color 0.15s',
            }}
            onMouseEnter={e => (e.currentTarget.style.borderColor = '#7c3aed')}
            onMouseLeave={e => (e.currentTarget.style.borderColor = '#313244')}
            onClick={() => onHighlight(new Set([node.node_id]))}
          >
            <div style={{
              display: 'flex', alignItems: 'flex-start', gap: 8, marginBottom: 4,
            }}>
              <span style={{
                background: '#313244', color: '#89b4fa',
                borderRadius: 4, padding: '1px 6px', fontSize: 12, flexShrink: 0,
              }}>#{idx + 1}</span>
              <span style={{
                fontSize: 11, color: '#6c7086', marginLeft: 'auto', flexShrink: 0,
              }}>
                相关度 {node.score.toFixed(2)}
              </span>
            </div>
            <p style={{
              margin: 0, color: '#cdd6f4', fontSize: 13,
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
          paddingTop: 10, borderTop: '1px solid #313244',
          display: 'flex', justifyContent: 'space-between', alignItems: 'center',
        }}>
          <span style={{ fontSize: 12, color: '#6c7086' }}>
            共 {results.length} 个相关节点已高亮
          </span>
          <button
            onClick={handleClear}
            style={{
              background: 'none', border: '1px solid #45475a',
              borderRadius: 6, padding: '4px 10px',
              color: '#6c7086', cursor: 'pointer', fontSize: 12,
            }}
          >
            清除
          </button>
        </div>
      )}
    </div>
  )
}
