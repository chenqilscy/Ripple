/**
 * P18-D: 图谱布局快照面板。
 * 允许用户在湖泊画布中保存当前布局为命名快照，并查看历史快照列表。
 */
import { useState, useEffect, useCallback } from 'react'
import { api } from '../api/client'
import type { LakeSnapshot } from '../api/types'

interface Props {
  lakeId: string
  /** 当前画布布局（nodeId → 坐标），由父组件提供 */
  currentLayout: Record<string, { x: number; y: number }>
  onClose: () => void
  /** 用户点击"恢复快照"时，父组件应用该布局 */
  onRestore?: (layout: Record<string, { x: number; y: number }>) => void
}

export default function SnapshotPanel({ lakeId, currentLayout, onClose, onRestore }: Props) {
  const [snapshots, setSnapshots] = useState<LakeSnapshot[]>([])
  const [loading, setLoading] = useState(true)
  const [saving, setSaving] = useState(false)
  const [name, setName] = useState('')
  const [err, setErr] = useState<string | null>(null)

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
      const snap = await api.createSnapshot(lakeId, trimmed, currentLayout)
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
    } catch (e: unknown) {
      setErr(e instanceof Error ? e.message : '删除失败')
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
          <span style={{ fontWeight: 600, fontSize: 14, color: '#c8d8e8' }}>图谱布局快照</span>
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
            当前布局包含 {Object.keys(currentLayout).length} 个节点位置
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
      </div>
    </div>
  )
}
