// PathTracePanel.tsx — 路径追溯面板：展示两节点间的关联链路
import React from 'react'
import type { PathResult } from '../../api/types'

interface PathTracePanelProps {
  path: PathResult | null
  loading: boolean
  onTracePath: (sourceId: string, targetId: string) => void
  onClose: () => void
}

export default function PathTracePanel({ path, loading, onTracePath, onClose }: PathTracePanelProps) {
  if (!path && !loading) return null

  return (
    <div style={{
      position: 'absolute', top: 12, right: 70, width: 300,
      background: 'rgba(6,13,26,0.96)', border: '1px solid #2e8b90',
      borderRadius: 8, zIndex: 40, boxShadow: '0 4px 20px rgba(0,0,0,0.5)',
    }}>
      <div style={{
        padding: '10px 14px 8px', borderBottom: '1px solid rgba(46,139,144,0.3)',
        display: 'flex', alignItems: 'center', justifyContent: 'space-between',
      }}>
        <span style={{ color: '#9ec5ee', fontSize: 13, fontWeight: 600 }}>🔍 路径追溯</span>
        <button onClick={onClose} style={closeBtnStyle}>✕</button>
      </div>

      {loading && (
        <div style={{ padding: '20px 12px', textAlign: 'center', color: '#4a6a8e' }}>
          计算路径中...
        </div>
      )}

      {path && (
        <div style={{ padding: '8px 12px' }}>
          <div style={{ fontSize: 11, color: '#4a6a8e', marginBottom: 8 }}>
            {path.total_steps} 步关联链路
          </div>
          {path.nodes.map((n, i) => (
            <div key={n.id} style={{ display: 'flex', alignItems: 'center', marginBottom: 8 }}>
              {/* 步骤圆点 */}
              <div style={{
                width: 20, height: 20, borderRadius: '50%',
                background: i === 0 || i === path.nodes.length - 1 ? '#2e8b90' : 'rgba(46,139,144,0.3)',
                color: '#fff', fontSize: 10, display: 'flex', alignItems: 'center', justifyContent: 'center',
                marginRight: 8, flexShrink: 0,
              }}>
                {i + 1}
              </div>
              <div style={{ flex: 1, minWidth: 0 }}>
                <div style={{ fontSize: 12, color: '#c0d8f0', whiteSpace: 'nowrap' as const, overflow: 'hidden', textOverflow: 'ellipsis' }}>
                  {n.title || n.id.slice(0, 12)}
                </div>
                {n.reason && (
                  <div style={{ fontSize: 10, color: '#4a6a8e', marginTop: 2 }}>
                    {n.reason}
                  </div>
                )}
              </div>
            </div>
          ))}
        </div>
      )}
    </div>
  )
}

const closeBtnStyle: React.CSSProperties = {
  background: 'transparent', border: 'none', color: '#666', cursor: 'pointer', padding: '0 4px', fontSize: 12,
}