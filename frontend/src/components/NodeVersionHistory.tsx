/**
 * P17-A: Node version history timeline.
 * Displays revisions in reverse-chronological order with content preview,
 * expand-on-click full view, and one-click rollback.
 */
import { useState } from 'react'
import { api } from '../api/client'
import type { NodeItem, NodeRevision } from '../api/types'

interface Props {
  node: NodeItem
  revisions: NodeRevision[]
  onClose: () => void
  /** Called with the updated node after a successful rollback */
  onRolledBack: (node: NodeItem) => void
}

export default function NodeVersionHistory({ node, revisions, onClose, onRolledBack }: Props) {
  const [expanded, setExpanded] = useState<number | null>(null)
  const [rolling, setRolling] = useState<number | null>(null)
  const [err, setErr] = useState<string | null>(null)

  async function handleRollback(rev: NodeRevision) {
    if (!window.confirm(`确认回滚到 rev ${rev.rev_number}？当前内容将被替换。`)) return
    setRolling(rev.rev_number)
    setErr(null)
    try {
      const updated = await api.rollbackNode(node.id, rev.rev_number)
      onRolledBack(updated)
      onClose()
    } catch (e: unknown) {
      setErr(e instanceof Error ? e.message : '回滚失败')
    } finally {
      setRolling(null)
    }
  }

  return (
    <div style={{
      position: 'fixed', inset: 0, zIndex: 2000,
      background: 'rgba(0,0,0,0.7)',
      display: 'flex', alignItems: 'center', justifyContent: 'center',
      padding: 24,
    }} onClick={e => { if (e.target === e.currentTarget) onClose() }}>
      <div style={{
        background: '#0d1526', border: '1px solid #1e3050', borderRadius: 10,
        padding: 24, width: '100%', maxWidth: 540, maxHeight: '80vh',
        display: 'flex', flexDirection: 'column', gap: 12,
      }}>
        {/* header */}
        <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
          <span style={{ fontWeight: 600, fontSize: 14, color: '#c8d8e8' }}>
            版本历史 — {node.id.slice(0, 8)}
          </span>
          <button onClick={onClose} style={{
            background: 'none', border: 'none', color: '#9ec5ee',
            fontSize: 20, cursor: 'pointer', lineHeight: 1,
          }}>✕</button>
        </div>

        {err && (
          <div style={{ color: '#ff6b7a', fontSize: 12, padding: '4px 8px', background: 'rgba(220,53,69,0.1)', borderRadius: 4 }}>
            {err}
          </div>
        )}

        {revisions.length === 0 && (
          <div style={{ opacity: 0.5, fontSize: 12, textAlign: 'center', padding: 24 }}>暂无历史记录</div>
        )}

        {/* timeline */}
        <div style={{ overflowY: 'auto', flex: 1, display: 'flex', flexDirection: 'column', gap: 8 }}>
          {revisions.map((rev, idx) => {
            const isLatest = idx === 0
            const isExpanded = expanded === rev.rev_number
            return (
              <div key={rev.rev_number} style={{
                display: 'flex', gap: 12, position: 'relative',
              }}>
                {/* timeline line */}
                <div style={{ display: 'flex', flexDirection: 'column', alignItems: 'center', minWidth: 20 }}>
                  <div style={{
                    width: 10, height: 10, borderRadius: '50%', flexShrink: 0,
                    background: isLatest ? '#4a8eff' : '#2a4a6e',
                    border: `2px solid ${isLatest ? '#9ec5ee' : '#4a6a8e'}`,
                    marginTop: 4,
                  }} />
                  {idx < revisions.length - 1 && (
                    <div style={{ width: 2, flex: 1, background: '#1e3050', marginTop: 2 }} />
                  )}
                </div>

                {/* card */}
                <div style={{
                  flex: 1, background: '#0a1828', border: '1px solid #1e3050', borderRadius: 6,
                  padding: '8px 10px', marginBottom: 4,
                }}>
                  <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start' }}>
                    <div>
                      <span style={{ color: '#4a8eff', fontSize: 12, fontWeight: 600 }}>
                        rev {rev.rev_number}
                      </span>
                      {isLatest && (
                        <span style={{ marginLeft: 6, fontSize: 10, color: '#52c41a', background: 'rgba(82,196,26,0.12)', padding: '1px 5px', borderRadius: 3 }}>当前</span>
                      )}
                      {rev.edit_reason && (
                        <span style={{ marginLeft: 6, fontSize: 10, color: '#9ec5ee', opacity: 0.7 }}>
                          {rev.edit_reason}
                        </span>
                      )}
                    </div>
                    <span style={{ fontSize: 10, color: '#4a6a8e', whiteSpace: 'nowrap', marginLeft: 8 }}>
                      {new Date(rev.created_at).toLocaleString('zh-CN')}
                    </span>
                  </div>

                  {/* preview / expanded content */}
                  <div
                    onClick={() => setExpanded(isExpanded ? null : rev.rev_number)}
                    style={{ cursor: 'pointer', marginTop: 6 }}
                  >
                    {isExpanded ? (
                      <pre style={{
                        margin: 0, whiteSpace: 'pre-wrap', wordBreak: 'break-all',
                        fontSize: 11, color: '#c0d8f0', lineHeight: 1.6,
                        maxHeight: 200, overflowY: 'auto',
                        background: '#061020', padding: '6px 8px', borderRadius: 4,
                      }}>
                        {rev.content}
                      </pre>
                    ) : (
                      <div style={{ fontSize: 11, color: '#9ec5ee', opacity: 0.8, lineHeight: 1.5 }}>
                        {rev.content.slice(0, 100)}{rev.content.length > 100 ? '…' : ''}
                      </div>
                    )}
                    <div style={{ fontSize: 10, color: '#4a6a8e', marginTop: 2 }}>
                      {isExpanded ? '▲ 折叠' : '▼ 展开'}
                    </div>
                  </div>

                  {/* rollback action — only for non-latest revs */}
                  {!isLatest && (
                    <div style={{ marginTop: 8 }}>
                      <button
                        onClick={() => void handleRollback(rev)}
                        disabled={rolling !== null}
                        style={{
                          fontSize: 11, padding: '3px 10px',
                          background: 'rgba(74,144,226,0.15)', color: '#4a8eff',
                          border: '1px solid #2a4a7e', borderRadius: 4, cursor: 'pointer',
                        }}
                      >
                        {rolling === rev.rev_number ? '回滚中…' : '⟲ 回滚到此版本'}
                      </button>
                    </div>
                  )}
                </div>
              </div>
            )
          })}
        </div>
      </div>
    </div>
  )
}
