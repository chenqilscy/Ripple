/**
 * P23: 图谱快照版本历史时间线组件。
 * 以竖向时间线可视化展示所有快照，支持点击选中并恢复。
 */
import { useState, useEffect, useCallback } from 'react'
import { api } from '../api/client'
import type { LakeSnapshot } from '../api/types'

interface Props {
  lakeId: string
  onRestore?: (layout: Record<string, { x: number; y: number }>) => void
  onClose: () => void
}

export default function SnapshotTimeline({ lakeId, onRestore, onClose }: Props) {
  const [snapshots, setSnapshots] = useState<LakeSnapshot[]>([])
  const [loading, setLoading] = useState(true)
  const [err, setErr] = useState<string | null>(null)
  const [selected, setSelected] = useState<string | null>(null)
  const [restoring, setRestoring] = useState(false)

  const load = useCallback(async () => {
    setLoading(true)
    setErr(null)
    try {
      const res = await api.listSnapshots(lakeId)
      setSnapshots(res.snapshots)
    } catch (e: unknown) {
      setErr(e instanceof Error ? e.message : '加载快照失败')
    } finally {
      setLoading(false)
    }
  }, [lakeId])

  useEffect(() => { load() }, [load])

  function formatDate(iso: string): string {
    const d = new Date(iso)
    return d.toLocaleString('zh-CN', {
      year: 'numeric', month: '2-digit', day: '2-digit',
      hour: '2-digit', minute: '2-digit', second: '2-digit',
    })
  }

  async function handleRestore() {
    if (!onRestore || !selected) return
    const snap = snapshots.find(s => s.id === selected)
    if (!snap) return
    if (!window.confirm(`确认恢复快照「${snap.name}」？当前布局将被覆盖。`)) return
    setRestoring(true)
    try {
      onRestore(snap.layout)
      onClose()
    } finally {
      setRestoring(false)
    }
  }

  const selectedSnap = snapshots.find(s => s.id === selected) ?? null

  return (
    <div style={{
      position: 'fixed', top: 0, right: 0, bottom: 0,
      width: 320, zIndex: 400,
      background: '#0d1526',
      borderLeft: '1px solid #1e3050',
      display: 'flex', flexDirection: 'column',
      boxShadow: '-4px 0 24px rgba(0,0,0,0.4)',
    }}>
      {/* Header */}
      <div style={{
        padding: '16px 16px 12px', borderBottom: '1px solid #1e3050',
        display: 'flex', justifyContent: 'space-between', alignItems: 'center',
        flexShrink: 0,
      }}>
        <span style={{ fontWeight: 600, fontSize: 14, color: '#c8d8e8' }}>🕐 版本历史时间线</span>
        <button onClick={onClose} style={{
          background: 'none', border: 'none', color: '#9ec5ee',
          fontSize: 18, cursor: 'pointer', lineHeight: 1, padding: '2px 4px',
        }}>✕</button>
      </div>

      {err && (
        <div style={{
          margin: '8px 12px', color: '#ff6b7a', fontSize: 12,
          padding: '6px 10px', background: 'rgba(220,53,69,0.1)', borderRadius: 4, flexShrink: 0,
        }}>
          {err}
        </div>
      )}

      {/* 时间线列表 */}
      <div style={{ flex: 1, overflowY: 'auto', padding: '16px 16px 8px' }}>
        {loading && (
          <div style={{ fontSize: 12, color: '#5a7a9e', textAlign: 'center', marginTop: 24 }}>加载中...</div>
        )}
        {!loading && snapshots.length === 0 && (
          <div style={{ fontSize: 12, color: '#5a7a9e', textAlign: 'center', marginTop: 24 }}>
            暂无快照记录<br />
            <span style={{ opacity: 0.6 }}>在工具栏点击「保存快照」以创建版本记录</span>
          </div>
        )}
        {!loading && snapshots.length > 0 && (
          <div style={{ position: 'relative' }}>
            {/* 竖向连接线 */}
            <div style={{
              position: 'absolute', left: 11, top: 16, bottom: 16,
              width: 2, background: 'linear-gradient(to bottom, #2a5caa, #1e3050)',
              borderRadius: 1,
            }} />
            {snapshots.map((snap, idx) => {
              const isSelected = snap.id === selected
              const isLatest = idx === 0
              return (
                <div
                  key={snap.id}
                  onClick={() => setSelected(isSelected ? null : snap.id)}
                  style={{
                    display: 'flex', gap: 14, marginBottom: 16,
                    cursor: 'pointer',
                  }}
                >
                  {/* 时间线节点圆点 */}
                  <div style={{
                    width: 24, height: 24, borderRadius: '50%', flexShrink: 0,
                    background: isSelected ? '#89b4fa' : isLatest ? '#a6e3a1' : '#2a4060',
                    border: `2px solid ${isSelected ? '#89b4fa' : isLatest ? '#a6e3a1' : '#3a5070'}`,
                    display: 'flex', alignItems: 'center', justifyContent: 'center',
                    fontSize: 10, color: isSelected || isLatest ? '#0d1526' : '#9ec5ee',
                    transition: 'all 0.15s',
                    zIndex: 1,
                    position: 'relative',
                  }}>
                    {isLatest ? '★' : String(snapshots.length - idx)}
                  </div>
                  {/* 快照信息 */}
                  <div style={{
                    flex: 1, background: isSelected ? 'rgba(137,180,250,0.08)' : 'rgba(30,48,80,0.3)',
                    border: `1px solid ${isSelected ? 'rgba(137,180,250,0.3)' : 'rgba(42,64,96,0.5)'}`,
                    borderRadius: 8, padding: '8px 12px',
                    transition: 'all 0.15s',
                  }}>
                    <div style={{ fontSize: 13, fontWeight: 600, color: isSelected ? '#89b4fa' : '#c8d8e8', marginBottom: 4 }}>
                      {snap.name}
                    </div>
                    <div style={{ fontSize: 11, color: '#5a7a9e' }}>{formatDate(snap.created_at)}</div>
                    <div style={{ display: 'flex', gap: 10, marginTop: 4 }}>
                      {snap.graph_state && (
                        <>
                          <span style={{ fontSize: 11, color: '#a6e3a1' }}>节点 {snap.graph_state.nodes.length}</span>
                          <span style={{ fontSize: 11, color: '#89dceb' }}>边 {snap.graph_state.edges.length}</span>
                        </>
                      )}
                      {isLatest && (
                        <span style={{ fontSize: 10, color: '#a6e3a1', background: 'rgba(166,227,161,0.12)', borderRadius: 3, padding: '1px 5px' }}>最新</span>
                      )}
                    </div>
                  </div>
                </div>
              )
            })}
          </div>
        )}
      </div>

      {/* 底部操作区 */}
      {selectedSnap && (
        <div style={{
          padding: '12px 16px', borderTop: '1px solid #1e3050', flexShrink: 0,
          background: '#0d1526',
        }}>
          <div style={{ fontSize: 12, color: '#9ec5ee', marginBottom: 10 }}>
            已选：<strong style={{ color: '#89b4fa' }}>{selectedSnap.name}</strong>
          </div>
          <div style={{ display: 'flex', gap: 8 }}>
            <button
              onClick={handleRestore}
              disabled={!onRestore || restoring}
              style={{
                flex: 1, background: '#2a5caa', border: 'none', borderRadius: 6,
                color: '#fff', fontSize: 12, padding: '8px 0', cursor: 'pointer',
                opacity: !onRestore || restoring ? 0.5 : 1,
              }}
            >
              {restoring ? '恢复中...' : '⟳ 恢复此版本'}
            </button>
            <button
              onClick={() => setSelected(null)}
              style={{
                background: 'rgba(255,255,255,0.06)', border: '1px solid #2a4060',
                borderRadius: 6, color: '#9ec5ee', fontSize: 12, padding: '8px 12px', cursor: 'pointer',
              }}
            >取消</button>
          </div>
        </div>
      )}
    </div>
  )
}
