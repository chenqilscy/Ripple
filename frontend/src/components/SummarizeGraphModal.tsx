import { useState, type CSSProperties } from 'react'
import { Button } from './ui'
import { api } from '../api/client'
import type { SummarizeGraphResult } from '../api/types'

interface SummarizeGraphModalProps {
  lakeId: string
  nodeIds: string[]
  onClose: () => void
  onSuccess?: () => void
}

export default function SummarizeGraphModal({ lakeId, nodeIds, onClose, onSuccess }: SummarizeGraphModalProps) {
  const [titleHint, setTitleHint] = useState('')
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState('')
  const [result, setResult] = useState<SummarizeGraphResult | null>(null)

  const handleSubmit = async () => {
    setLoading(true)
    setError('')
    try {
      const data = await api.summarizeGraph(lakeId, nodeIds, titleHint.trim())
      setResult({
        ...data,
        sources: data.sources ?? [],
        edges: data.edges ?? [],
        edge_failures: data.edge_failures ?? [],
        edge_kind: data.edge_kind ?? 'summarizes',
        complete: data.complete ?? ((data.edge_failures?.length ?? 0) === 0),
      })
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : '请求失败')
    } finally {
      setLoading(false)
    }
  }

  const handleClose = () => {
    if (result) onSuccess?.()
    onClose()
  }

  return (
    <div style={{
      position: 'fixed', inset: 0, zIndex: 1000,
      background: 'rgba(0,0,0,0.6)',
      display: 'flex', alignItems: 'center', justifyContent: 'center',
    }} onClick={handleClose}>
      <div
        style={{
          background: '#0e1f3a', border: '1px solid #2a4a7e',
          borderRadius: 10, padding: '24px 28px', width: 720, maxWidth: '92vw', maxHeight: '88vh', overflowY: 'auto',
        }}
        onClick={e => e.stopPropagation()}
      >
        <h3 style={{ margin: '0 0 16px', color: '#9ec5ee', fontSize: 16 }}>
          多节点 AI 整理
        </h3>
        <p style={{ margin: '0 0 14px', color: '#7a9ab0', fontSize: 13 }}>
          已选 <b style={{ color: '#9ec5ee' }}>{nodeIds.length}</b> 个节点，AI 将归纳共同主题，生成一个摘要节点，并创建 <b style={{ color: '#9ec5ee' }}>summarizes</b> 摘要关联。
        </p>
        {!result ? (
          <>
            <label style={{ display: 'block', color: '#7a9ab0', fontSize: 12, marginBottom: 6 }}>
              方向提示（可选）
            </label>
            <input
              value={titleHint}
              onChange={e => setTitleHint(e.target.value)}
              placeholder={`让 AI 聚焦于某个角度，如"分析技术可行性"（可留空）`}
              disabled={loading}
              maxLength={200}
              style={{
                width: '100%', boxSizing: 'border-box',
                background: '#060d1a', border: '1px solid #2a4a7e',
                borderRadius: 6, color: '#c0d8f0', fontSize: 13,
                padding: '8px 12px', marginBottom: 16,
                outline: 'none', opacity: loading ? 0.6 : 1,
              }}
            />
            {error && (
              <div style={{ color: '#ff6b6b', fontSize: 12, marginBottom: 12 }}>
                ⚠ {error}
              </div>
            )}
            <div style={{ display: 'flex', gap: 10, justifyContent: 'flex-end' }}>
              <Button
                variant="secondary"
                size="sm"
                onClick={handleClose}
                disabled={loading}
              >
                取消
              </Button>
              <Button
                variant="primary"
                size="sm"
                onClick={handleSubmit}
                disabled={loading}
                icon={loading ? <span style={{ display: 'inline-block', width: 12, height: 12, border: '2px solid #4a8eff', borderTopColor: 'transparent', borderRadius: '50%', animation: 'spin 0.7s linear infinite' }} /> : undefined}
              >
                {loading ? `AI 分析 ${nodeIds.length} 个节点…` : `生成整理结果 (${nodeIds.length} 节点)`}
              </Button>
            </div>
          </>
        ) : (
          <>
            <div style={{ color: result.complete ? '#4ecdc4' : '#f9e2af', fontSize: 13, marginBottom: 12 }}>
              {result.complete ? '✓' : '⚠'} 摘要节点已生成（{result.source_count} 个源节点 → {result.edges.length} 条 summarizes 边）
              {!result.complete && result.edge_failures.length > 0 && `，${result.edge_failures.length} 条摘要关联创建失败`}
            </div>
            <div style={{ display: 'grid', gridTemplateColumns: 'minmax(0, 1fr) minmax(0, 1fr)', gap: 12, marginBottom: 14 }}>
              <section style={previewPanelStyle}>
                <div style={previewTitleStyle}>整理前：源节点预览</div>
                <div style={{ display: 'grid', gap: 8, maxHeight: 220, overflowY: 'auto' }}>
                  {result.sources.length === 0 ? (
                    <div style={{ color: '#7a9ab0', fontSize: 12 }}>后端未返回源节点预览，但摘要节点已创建。</div>
                  ) : result.sources.map((source, index) => (
                    <div key={source.id} style={sourceCardStyle}>
                      <div style={{ color: '#89b4fa', fontSize: 11, marginBottom: 4 }}>
                        #{index + 1} · {shortId(source.id)} · {source.content_length} 字符
                      </div>
                      <div style={{ color: '#c0d8f0', fontSize: 12, lineHeight: 1.6, whiteSpace: 'pre-wrap', wordBreak: 'break-word' }}>
                        {source.content_snippet || '（空内容）'}
                      </div>
                    </div>
                  ))}
                </div>
              </section>

              <section style={previewPanelStyle}>
                <div style={previewTitleStyle}>整理后：新摘要节点</div>
                <div style={{
                  background: '#060d1a', border: '1px solid #1a3a6a',
                  borderRadius: 6, padding: '10px 14px', minHeight: 132,
                  color: '#c0d8f0', fontSize: 13, lineHeight: 1.7,
                  whiteSpace: 'pre-wrap', wordBreak: 'break-word',
                }}>
                  {result.summary_node.content}
                </div>
                <div style={{ color: '#4a6a8e', fontSize: 11, marginTop: 8 }}>
                  摘要节点：{shortId(result.summary_node.id)}
                </div>
              </section>
            </div>

            <div style={{ background: '#081426', border: '1px solid #1a3a6a', borderRadius: 6, padding: '10px 12px', marginBottom: 12 }}>
              <div style={{ color: '#9ec5ee', fontSize: 12, marginBottom: 8 }}>摘要关联预览</div>
              {result.edges.length === 0 ? (
                <div style={{ color: '#f9e2af', fontSize: 12 }}>尚未成功创建 summarizes 关联，请检查后端日志或重试。</div>
              ) : (
                <div style={{ display: 'flex', flexWrap: 'wrap', gap: 6 }}>
                  {result.edges.map(edge => (
                    <span key={`${edge.source_id}-${edge.target_id}`} style={edgePillStyle}>
                      {shortId(edge.source_id)} → {shortId(edge.target_id)} · {edge.kind}
                    </span>
                  ))}
                </div>
              )}
            </div>

            {result.edge_failures.length > 0 && (
              <div style={{ color: '#f9e2af', background: '#2a210d', border: '1px solid #5c4818', borderRadius: 6, padding: '8px 10px', fontSize: 12, marginBottom: 16 }}>
                {result.edge_failures.map(failure => (
                  <div key={`${failure.source_id}-${failure.target_id}`}>⚠ {shortId(failure.target_id)}：{failure.reason}</div>
                ))}
              </div>
            )}

            <div style={{ textAlign: 'right' }}>
              <Button
                variant="primary"
                size="sm"
                onClick={handleClose}
              >
                关闭并刷新图谱
              </Button>
            </div>
          </>
        )}
      </div>
    </div>
  )
}

function shortId(id: string) {
  return id.length > 8 ? id.slice(0, 8) : id
}

const previewPanelStyle: CSSProperties = {
  background: '#081426',
  border: '1px solid #1a3a6a',
  borderRadius: 8,
  padding: 12,
  minWidth: 0,
}

const previewTitleStyle: CSSProperties = {
  color: '#9ec5ee',
  fontSize: 12,
  fontWeight: 600,
  marginBottom: 8,
}

const sourceCardStyle: CSSProperties = {
  background: '#060d1a',
  border: '1px solid #183052',
  borderRadius: 6,
  padding: '8px 10px',
}

const edgePillStyle: CSSProperties = {
  color: '#94e2d5',
  border: '1px solid #2f6f6d',
  borderRadius: 999,
  padding: '3px 8px',
  fontSize: 11,
  background: '#062323',
}
