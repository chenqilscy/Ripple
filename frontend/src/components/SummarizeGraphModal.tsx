import { useState } from 'react'
import { getToken } from '../api/client'

const BASE = (import.meta.env.VITE_API_BASE as string | undefined) ?? ''

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
  const [done, setDone] = useState(false)

  const handleSubmit = async () => {
    setLoading(true)
    setError('')
    try {
      const token = getToken() ?? ''
      const res = await fetch(`${BASE}/api/v1/lakes/${lakeId}/nodes/summarize`, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          ...(token ? { Authorization: `Bearer ${token}` } : {}),
        },
        body: JSON.stringify({ node_ids: nodeIds, title_hint: titleHint }),
      })
      if (!res.ok) {
        const body = await res.json().catch(() => ({}))
        throw new Error(body.error ?? `HTTP ${res.status}`)
      }
      setDone(true)
      onSuccess?.()
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : '请求失败')
    } finally {
      setLoading(false)
    }
  }

  return (
    <div style={{
      position: 'fixed', inset: 0, zIndex: 1000,
      background: 'rgba(0,0,0,0.6)',
      display: 'flex', alignItems: 'center', justifyContent: 'center',
    }} onClick={onClose}>
      <div
        style={{
          background: '#0e1f3a', border: '1px solid #2a4a7e',
          borderRadius: 10, padding: '24px 28px', width: 420, maxWidth: '90vw',
        }}
        onClick={e => e.stopPropagation()}
      >
        <h3 style={{ margin: '0 0 16px', color: '#9ec5ee', fontSize: 16 }}>
          摘要所选节点
        </h3>
        <p style={{ margin: '0 0 14px', color: '#7a9ab0', fontSize: 13 }}>
          已选 <b style={{ color: '#9ec5ee' }}>{nodeIds.length}</b> 个节点，LLM 将生成摘要节点并自动关联。
        </p>
        {!done ? (
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
              <button
                onClick={onClose}
                disabled={loading}
                style={{
                  background: 'transparent', border: '1px solid #2a4a7e',
                  color: '#7a9ab0', borderRadius: 6, padding: '7px 18px',
                  fontSize: 13, cursor: 'pointer',
                }}
              >
                取消
              </button>
              <button
                onClick={handleSubmit}
                disabled={loading}
                style={{
                  background: loading ? '#1a3a6a' : '#1e4d9e',
                  border: 'none', color: '#9ec5ee', borderRadius: 6,
                  padding: '7px 18px', fontSize: 13, cursor: loading ? 'not-allowed' : 'pointer',
                  display: 'flex', alignItems: 'center', gap: 6,
                }}
              >
                {loading ? (
                  <>
                    <span style={{ display: 'inline-block', width: 12, height: 12, border: '2px solid #4a8eff', borderTopColor: 'transparent', borderRadius: '50%', animation: 'spin 0.7s linear infinite' }} />
                    AI 分析 {nodeIds.length} 个节点…
                  </>
                ) : `生成摘要 (${nodeIds.length} 节点)`}
              </button>
            </div>
          </>
        ) : (
          <>
            <div style={{ color: '#4ecdc4', fontSize: 14, marginBottom: 16 }}>
              ✓ 摘要节点已生成，图谱将自动更新。
            </div>
            <div style={{ textAlign: 'right' }}>
              <button
                onClick={onClose}
                style={{
                  background: '#1e4d9e', border: 'none', color: '#9ec5ee',
                  borderRadius: 6, padding: '7px 18px', fontSize: 13, cursor: 'pointer',
                }}
              >
                关闭
              </button>
            </div>
          </>
        )}
      </div>
    </div>
  )
}
