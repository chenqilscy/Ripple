/**
 * P18-D / P21: 图谱布局快照面板。
 * 允许用户在湖泊画布中保存当前布局为命名快照，并查看历史快照列表。
 * P21 扩展：保存时附带图谱内容快照，支持与历史快照进行内容 diff。
 */
import { useState, useEffect, useCallback } from 'react'
import { api } from '../api/client'
import type { LakeSnapshot, SnapshotGraphState, SnapshotNodeEntry, SnapshotEdgeEntry } from '../api/types'

interface Props {
  lakeId: string
  /** 当前画布布局（nodeId → 坐标），由父组件提供 */
  currentLayout: Record<string, { x: number; y: number }>
  /** 当前图谱内容（P21 diff 用），可选 */
  currentGraphState?: SnapshotGraphState
  onClose: () => void
  /** 用户点击"恢复快照"时，父组件应用该布局 */
  onRestore?: (layout: Record<string, { x: number; y: number }>) => void
}

export default function SnapshotPanel({ lakeId, currentLayout, currentGraphState, onClose, onRestore }: Props) {
  const [snapshots, setSnapshots] = useState<LakeSnapshot[]>([])
  const [loading, setLoading] = useState(true)
  const [saving, setSaving] = useState(false)
  const [name, setName] = useState('')
  const [err, setErr] = useState<string | null>(null)
  // P21 diff 状态
  const [diffLoading, setDiffLoading] = useState(false)
  const [diffSnap, setDiffSnap] = useState<LakeSnapshot | null>(null)
  const [diffResult, setDiffResult] = useState<{
    addedNodes: SnapshotNodeEntry[]
    removedNodes: SnapshotNodeEntry[]
    addedEdges: SnapshotEdgeEntry[]
    removedEdges: SnapshotEdgeEntry[]
  } | null>(null)

  const loadSnapshots = useCallback(async () => {
    setLoading(true)
    try {
      const res = await api.listSnapshots(lakeId)
      setSnapshots(res.snapshots)
    } catch (e: unknown) {
      setErr(e instanceof Error ? e.message : '加载快照失败')
    } finally {
      setLoading(false)
    }
  }, [lakeId])

  useEffect(() => {
    loadSnapshots()
  }, [loadSnapshots])

  async function handleSave() {
    const trimmed = name.trim()
    if (!trimmed) {
      setErr('快照名称不能为空')
      return
    }
    setSaving(true)
    setErr(null)
    try {
      const snap = await api.createSnapshot(lakeId, trimmed, currentLayout, currentGraphState)
      setSnapshots(prev => [snap, ...prev])
      setName('')
    } catch (e: unknown) {
      setErr(e instanceof Error ? e.message : '保存快照失败')
    } finally {
      setSaving(false)
    }
  }

  async function handleDelete(snap: LakeSnapshot) {
    if (!window.confirm(`确认删除快照「${snap.name}」？`)) return
    try {
      await api.deleteSnapshot(lakeId, snap.id)
      setSnapshots(prev => prev.filter(s => s.id !== snap.id))
      if (diffSnap?.id === snap.id) { setDiffSnap(null); setDiffResult(null) }
    } catch (e: unknown) {
      setErr(e instanceof Error ? e.message : '删除失败')
    }
  }

  async function handleDiff(snap: LakeSnapshot) {
    if (!currentGraphState) {
      setErr('当前图谱无内容快照，无法 diff（请重新保存一次快照以记录内容）')
      return
    }
    setDiffLoading(true)
    setDiffSnap(snap)
    setDiffResult(null)
    setErr(null)
    try {
      const full = await api.getSnapshot(lakeId, snap.id)
      const old = full.graph_state
      if (!old) {
        setErr(`快照「${snap.name}」无内容记录（创建于 P21 上线前），无法对比`)
        setDiffSnap(null)
        return
      }
      const oldNodeIds = new Set(old.nodes.map(n => n.id))
      const curNodeIds = new Set(currentGraphState.nodes.map(n => n.id))
      const addedNodes = currentGraphState.nodes.filter(n => !oldNodeIds.has(n.id))
      const removedNodes = old.nodes.filter(n => !curNodeIds.has(n.id))
      const oldEdgeIds = new Set(old.edges.map(e => e.id))
      const curEdgeIds = new Set(currentGraphState.edges.map(e => e.id))
      const addedEdges = currentGraphState.edges.filter(e => !oldEdgeIds.has(e.id))
      const removedEdges = old.edges.filter(e => !curEdgeIds.has(e.id))
      setDiffResult({ addedNodes, removedNodes, addedEdges, removedEdges })
    } catch (e: unknown) {
      setErr(e instanceof Error ? e.message : '对比失败')
      setDiffSnap(null)
    } finally {
      setDiffLoading(false)
    }
  }

  function handleRestore(snap: LakeSnapshot) {
    if (!onRestore) return
    if (!window.confirm(`确认恢复快照「${snap.name}」？当前布局将被覆盖。`)) return
    onRestore(snap.layout)
    onClose()
  }

  function formatDate(iso: string): string {
    const d = new Date(iso)
    return d.toLocaleString('zh-CN', {
      month: 'short', day: 'numeric',
      hour: '2-digit', minute: '2-digit',
    })
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
        padding: 24, width: '100%', maxWidth: 480, maxHeight: '80vh',
        display: 'flex', flexDirection: 'column', gap: 16,
      }}>
        {/* Header */}
        <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
          <span style={{ fontWeight: 600, fontSize: 14, color: '#c8d8e8' }}>图谱版本快照</span>
          <button onClick={onClose} style={{
            background: 'none', border: 'none', color: '#9ec5ee',
            fontSize: 20, cursor: 'pointer', lineHeight: 1,
          }}>✕</button>
        </div>

        {err && (
          <div style={{ color: '#ff6b7a', fontSize: 12, padding: '6px 10px', background: 'rgba(220,53,69,0.1)', borderRadius: 4 }}>
            {err}
          </div>
        )}

        {/* 保存当前布局 */}
        <div style={{
          background: 'rgba(30,48,80,0.5)', borderRadius: 8, padding: 14,
          display: 'flex', flexDirection: 'column', gap: 10,
        }}>
          <div style={{ fontSize: 12, color: '#9ec5ee' }}>保存当前布局</div>
          <div style={{ display: 'flex', gap: 8 }}>
            <input
              value={name}
              onChange={e => setName(e.target.value)}
              onKeyDown={e => { if (e.key === 'Enter') handleSave() }}
              placeholder="快照名称，如：第一周安排"
              maxLength={100}
              style={{
                flex: 1, background: '#1a2840', border: '1px solid #2a4060',
                borderRadius: 4, color: '#c8d8e8', fontSize: 12,
                padding: '6px 10px', outline: 'none',
              }}
            />
            <button
              onClick={handleSave}
              disabled={saving || !name.trim()}
              style={{
                background: '#2a5caa', border: 'none', borderRadius: 6,
                color: '#fff', fontSize: 12, padding: '6px 14px', cursor: 'pointer',
                opacity: saving || !name.trim() ? 0.5 : 1, whiteSpace: 'nowrap',
              }}
            >
              {saving ? '保存中...' : '保存快照'}
            </button>
          </div>
          <div style={{ fontSize: 11, color: '#5a7a9e' }}>
            当前 {Object.keys(currentLayout).length} 个节点位置{currentGraphState ? `，含内容快照（${currentGraphState.nodes.length} 节点 / ${currentGraphState.edges.length} 边）` : ' — 无内容快照'}
          </div>
        </div>

        {/* 快照列表 */}
        <div style={{ flex: 1, overflowY: 'auto', display: 'flex', flexDirection: 'column', gap: 8 }}>
          <div style={{ fontSize: 12, color: '#9ec5ee', marginBottom: 2 }}>历史快照</div>
          {loading && (
            <div style={{ opacity: 0.5, fontSize: 12, textAlign: 'center', padding: 12 }}>加载中...</div>
          )}
          {!loading && snapshots.length === 0 && (
            <div style={{ opacity: 0.5, fontSize: 12, textAlign: 'center', padding: 12 }}>暂无保存的快照</div>
          )}
          {snapshots.map(snap => (
            <div key={snap.id} style={{
              background: 'rgba(30,48,80,0.4)', borderRadius: 6, padding: 10,
              border: '1px solid #1e3050',
              display: 'flex', justifyContent: 'space-between', alignItems: 'center',
            }}>
              <div style={{ display: 'flex', flexDirection: 'column', gap: 2 }}>
                <span style={{ fontSize: 13, color: '#c8d8e8', fontWeight: 500 }}>{snap.name}</span>
                <span style={{ fontSize: 11, color: '#5a7a9e' }}>{formatDate(snap.created_at)}</span>
              </div>
              <div style={{ display: 'flex', gap: 6 }}>
                {currentGraphState && (
                  <button
                    onClick={() => diffSnap?.id === snap.id ? (setDiffSnap(null), setDiffResult(null)) : handleDiff(snap)}
                    disabled={diffLoading}
                    style={{
                      background: diffSnap?.id === snap.id ? '#2a5caa' : '#1e3050',
                      border: '1px solid #2a4060', borderRadius: 4,
                      color: '#7ab8ff', fontSize: 11, padding: '4px 10px', cursor: 'pointer',
                    }}
                  >
                    {diffLoading && diffSnap?.id === snap.id ? '对比中…' : '与现在对比'}
                  </button>
                )}
                {onRestore && (
                  <button
                    onClick={() => handleRestore(snap)}
                    style={{
                      background: '#1e3050', border: 'none', borderRadius: 4,
                      color: '#9ec5ee', fontSize: 11, padding: '4px 10px', cursor: 'pointer',
                    }}
                  >
                    恢复
                  </button>
                )}
                <button
                  onClick={() => handleDelete(snap)}
                  style={{
                    background: 'none', border: 'none',
                    color: '#e06060', fontSize: 11, padding: '4px 6px', cursor: 'pointer',
                  }}
                >
                  删除
                </button>
              </div>
            </div>
          ))}
        </div>

        {/* P21 diff 结果面板 */}
        {diffSnap && diffResult && (
          <div style={{
            background: 'rgba(10,20,40,0.9)', border: '1px solid #2a4060', borderRadius: 8,
            padding: 14, display: 'flex', flexDirection: 'column', gap: 10,
          }}>
            <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
              <span style={{ fontSize: 12, color: '#9ec5ee', fontWeight: 600 }}>
                与「{diffSnap.name}」对比结果
              </span>
              <button
                onClick={() => { setDiffSnap(null); setDiffResult(null) }}
                style={{ background: 'none', border: 'none', color: '#5a7a9e', cursor: 'pointer', fontSize: 16 }}
              >✕</button>
            </div>
            {diffResult.addedNodes.length + diffResult.removedNodes.length + diffResult.addedEdges.length + diffResult.removedEdges.length === 0 ? (
              <div style={{ fontSize: 12, color: '#5a9e7a', textAlign: 'center', padding: 8 }}>
                ✓ 与快照「{diffSnap.name}」相比，图谱内容无变化
              </div>
            ) : (
              <div style={{ display: 'flex', flexDirection: 'column', gap: 8 }}>
                {diffResult.addedNodes.length > 0 && (
                  <div>
                    <div style={{ fontSize: 11, color: '#5a9e7a', marginBottom: 4 }}>
                      + 新增节点 ({diffResult.addedNodes.length})
                    </div>
                    {diffResult.addedNodes.map(n => (
                      <div key={n.id} style={{ fontSize: 11, color: '#7adba8', paddingLeft: 10, lineHeight: 1.8 }}>
                        + {n.title || n.id.slice(0, 8)}
                      </div>
                    ))}
                  </div>
                )}
                {diffResult.removedNodes.length > 0 && (
                  <div>
                    <div style={{ fontSize: 11, color: '#e06060', marginBottom: 4 }}>
                      − 删除节点 ({diffResult.removedNodes.length})
                    </div>
                    {diffResult.removedNodes.map(n => (
                      <div key={n.id} style={{ fontSize: 11, color: '#e08080', paddingLeft: 10, lineHeight: 1.8 }}>
                        − {n.title || n.id.slice(0, 8)}
                      </div>
                    ))}
                  </div>
                )}
                {diffResult.addedEdges.length > 0 && (
                  <div>
                    <div style={{ fontSize: 11, color: '#5a9e7a', marginBottom: 4 }}>
                      + 新增关系 ({diffResult.addedEdges.length})
                    </div>
                    {diffResult.addedEdges.map(e => (
                      <div key={e.id} style={{ fontSize: 11, color: '#7adba8', paddingLeft: 10, lineHeight: 1.8 }}>
                        + {e.kind || 'edge'}: {e.src.slice(0, 6)} → {e.dst.slice(0, 6)}
                      </div>
                    ))}
                  </div>
                )}
                {diffResult.removedEdges.length > 0 && (
                  <div>
                    <div style={{ fontSize: 11, color: '#e06060', marginBottom: 4 }}>
                      − 删除关系 ({diffResult.removedEdges.length})
                    </div>
                    {diffResult.removedEdges.map(e => (
                      <div key={e.id} style={{ fontSize: 11, color: '#e08080', paddingLeft: 10, lineHeight: 1.8 }}>
                        − {e.kind || 'edge'}: {e.src.slice(0, 6)} → {e.dst.slice(0, 6)}
                      </div>
                    ))}
                  </div>
                )}
              </div>
            )}
          </div>
        )}
      </div>
    </div>
  )
}
