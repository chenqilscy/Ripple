/**
 * P17-A: Node version history timeline.
 * Displays revisions in reverse-chronological order with content preview,
 * expand-on-click full view, and one-click rollback.
 * P27: Added "版本对比" button to open side-by-side diff.
 * 修复：scroll lock + CSS 变量
 */
import { lazy, Suspense, useEffect, useState } from 'react'
import { api } from '../api/client'
import type { NodeItem, NodeRevision } from '../api/types'

const NodeVersionDiff = lazy(() => import('./NodeVersionDiff'))

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
  const [diffOpen, setDiffOpen] = useState(false)

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
    <>
    <div style={{
      position: 'fixed', inset: 0, zIndex: 2000,
      background: 'var(--bg-overlay)',
      display: 'flex', alignItems: 'center', justifyContent: 'center',
      padding: 'var(--space-xl)',
    }} onClick={e => { if (e.target === e.currentTarget) onClose() }}>
      <div style={{
        background: 'var(--bg-primary)',
        border: '1px solid var(--border)',
        borderRadius: 'var(--radius-xl)',
        padding: 'var(--space-xl)',
        width: '100%', maxWidth: 540, maxHeight: '80vh',
        display: 'flex', flexDirection: 'column', gap: 'var(--space-md)',
      }}>
        {/* header */}
        <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
          <span style={{ fontWeight: 600, fontSize: 'var(--font-lg)', color: 'var(--accent)' }}>
            版本历史 — {node.id.slice(0, 8)}
          </span>
          <div style={{ display: 'flex', gap: 'var(--space-sm)', alignItems: 'center' }}>
            {revisions.length >= 2 && (
              <button
                onClick={() => setDiffOpen(true)}
                style={{
                  background: 'var(--accent-subtle)', border: '1px solid var(--accent)',
                  color: 'var(--accent)', borderRadius: 'var(--radius-sm)',
                  padding: '3px var(--space-md)', fontSize: 'var(--font-sm)', cursor: 'pointer',
                }}
                title="左右版本对比"
              >⇄ 版本对比</button>
            )}
            <button onClick={onClose} style={{
              background: 'none', border: 'none', color: 'var(--text-secondary)',
              fontSize: 20, cursor: 'pointer', lineHeight: 1,
            }} aria-label="关闭">✕</button>
          </div>
        </div>

        {err && (
          <div style={{ color: 'var(--status-danger)', fontSize: 'var(--font-sm)', padding: 'var(--space-sm)', background: 'var(--status-danger-subtle)', borderRadius: 'var(--radius-sm)' }}>
            {err}
          </div>
        )}

        {revisions.length === 0 && (
          <div style={{ opacity: 0.5, fontSize: 'var(--font-sm)', textAlign: 'center', padding: 'var(--space-xl)' }}>暂无历史记录</div>
        )}

        {/* timeline */}
        <div style={{ overflowY: 'auto', flex: 1, display: 'flex', flexDirection: 'column', gap: 'var(--space-sm)' }}>
          {revisions.map((rev, idx) => {
            const isLatest = idx === 0
            const isExpanded = expanded === rev.rev_number
            return (
              <div key={rev.rev_number} style={{
                display: 'flex', gap: 'var(--space-md)', position: 'relative',
              }}>
                {/* timeline line */}
                <div style={{ display: 'flex', flexDirection: 'column', alignItems: 'center', minWidth: 20 }}>
                  <div style={{
                    width: 10, height: 10, borderRadius: '50%', flexShrink: 0,
                    background: isLatest ? 'var(--accent)' : 'var(--border)',
                    border: `2px solid ${isLatest ? 'var(--accent)' : 'var(--border-subtle)'}`,
                    marginTop: 4,
                  }} />
                  {idx < revisions.length - 1 && (
                    <div style={{ width: 2, flex: 1, background: 'var(--border)', marginTop: 2 }} />
                  )}
                </div>

                {/* card */}
                <div style={{
                  flex: 1,
                  background: 'var(--bg-surface)',
                  border: '1px solid var(--border)',
                  borderRadius: 'var(--radius-md)',
                  padding: 'var(--space-sm) var(--space-md)', marginBottom: 'var(--space-xs)',
                }}>
                  <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start' }}>
                    <div>
                      <span style={{ color: 'var(--accent)', fontSize: 'var(--font-sm)', fontWeight: 600 }}>
                        rev {rev.rev_number}
                      </span>
                      {isLatest && (
                        <span style={{ marginLeft: 'var(--space-sm)', fontSize: 'var(--font-xs)', color: 'var(--status-success)', background: 'var(--status-success-subtle)', padding: '1px var(--space-sm)', borderRadius: 'var(--radius-sm)' }}>当前</span>
                      )}
                      {rev.edit_reason && (
                        <span style={{ marginLeft: 'var(--space-sm)', fontSize: 'var(--font-xs)', color: 'var(--text-tertiary)' }}>
                          {rev.edit_reason}
                        </span>
                      )}
                    </div>
                    <span style={{ fontSize: 'var(--font-xs)', color: 'var(--text-tertiary)', whiteSpace: 'nowrap', marginLeft: 'var(--space-sm)' }}>
                      {new Date(rev.created_at).toLocaleString('zh-CN')}
                    </span>
                  </div>

                  {/* preview / expanded content */}
                  <div
                    onClick={() => setExpanded(isExpanded ? null : rev.rev_number)}
                    style={{ cursor: 'pointer', marginTop: 'var(--space-sm)' }}
                  >
                    {isExpanded ? (
                      <pre style={{
                        margin: 0, whiteSpace: 'pre-wrap', wordBreak: 'break-all',
                        fontSize: 'var(--font-sm)', color: 'var(--text-primary)', lineHeight: 1.6,
                        maxHeight: 200, overflowY: 'auto',
                        background: 'var(--bg-input)',
                        border: '1px solid var(--border-input)',
                        borderRadius: 'var(--radius-md)',
                        padding: 'var(--space-sm) var(--space-md)',
                      }}>
                        {rev.content}
                      </pre>
                    ) : (
                      <div style={{ fontSize: 'var(--font-sm)', color: 'var(--text-secondary)', lineHeight: 1.5 }}>
                        {rev.content.slice(0, 100)}{rev.content.length > 100 ? '…' : ''}
                      </div>
                    )}
                    <div style={{ fontSize: 'var(--font-xs)', color: 'var(--text-tertiary)', marginTop: 2 }}>
                      {isExpanded ? '▲ 折叠' : '▼ 展开'}
                    </div>
                  </div>

                  {/* rollback action — only for non-latest revs */}
                  {!isLatest && (
                    <div style={{ marginTop: 'var(--space-sm)' }}>
                      <button
                        onClick={() => void handleRollback(rev)}
                        disabled={rolling !== null}
                        style={{
                          fontSize: 'var(--font-sm)', padding: '3px var(--space-md)',
                          background: 'var(--accent-subtle)', color: 'var(--accent)',
                          border: '1px solid var(--accent)', borderRadius: 'var(--radius-sm)', cursor: 'pointer',
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
    {diffOpen && revisions.length >= 2 && (
      <Suspense fallback={null}>
        <NodeVersionDiff revisions={revisions} onClose={() => setDiffOpen(false)} />
      </Suspense>
    )}
    </>
  )
}
