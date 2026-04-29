// DiscoveryPanel.tsx — 发现面板：推荐列表 + 路径触发
import React from 'react'
import type { Recommendation, PathResult } from '../../api/types'

interface DiscoveryPanelProps {
  recommendations: Recommendation[]
  loading: boolean
  activePath: PathResult | null
  loadingPath: boolean
  onAccept: (rec: Recommendation) => void
  onReject: (id: string) => void
  onIgnore: (id: string) => void
  onTracePath: (sourceId: string, targetId: string) => void
  onClosePath: () => void
  onClose: () => void
}

const CONFIDENCE_LABEL: Record<string, string> = {
  high: '高置信',
  medium: '中置信',
  low: '低置信',
}

function confidenceLevel(conf: number): 'high' | 'medium' | 'low' {
  if (conf >= 0.7) return 'high'
  if (conf >= 0.4) return 'medium'
  return 'low'
}

const CONFIDENCE_COLOR: Record<string, string> = {
  high: '#52c41a',
  medium: '#faad14',
  low: '#8c8c8c',
}

export default function DiscoveryPanel({
  recommendations, loading, activePath, loadingPath,
  onAccept, onReject, onIgnore, onTracePath, onClosePath, onClose,
}: DiscoveryPanelProps) {
  const pending = recommendations.filter(r => r.status === 'pending')

  return (
    <div style={{
      position: 'absolute', top: 12, right: 70, width: 280,
      background: 'rgba(6,13,26,0.95)', border: '1px solid #2e8b90',
      borderRadius: 8, zIndex: 40, maxHeight: 420, overflowY: 'auto',
      boxShadow: '0 4px 20px rgba(0,0,0,0.5)',
    }}>
      {/* Header */}
      <div style={{
        padding: '10px 14px 8px', borderBottom: '1px solid rgba(46,139,144,0.3)',
        display: 'flex', alignItems: 'center', justifyContent: 'space-between',
      }}>
        <span style={{ color: '#9ec5ee', fontSize: 13, fontWeight: 600 }}>💡 发现关联</span>
        <button onClick={onClose} style={closeBtnStyle}>✕</button>
      </div>

      {/* Content */}
      <div style={{ padding: '8px 0' }}>
        {loading && <div style={emptyStyle}>分析中...</div>}
        {!loading && pending.length === 0 && (
          <div style={emptyStyle}>暂无新发现<br /><span style={{ fontSize: 11, opacity: 0.6 }}>继续积累，关联会逐渐浮现</span></div>
        )}
        {!loading && pending.map(rec => {
          const lvl = confidenceLevel(rec.confidence)
          return (
            <div key={rec.id} style={{
              margin: '0 10px 8px', padding: '8px 10px',
              background: 'rgba(46,139,144,0.1)', borderRadius: 6, border: '1px solid rgba(46,139,144,0.2)',
            }}>
              <div style={{ fontSize: 11, color: CONFIDENCE_COLOR[lvl], marginBottom: 4 }}>
                {CONFIDENCE_LABEL[lvl]} · {Math.round(rec.confidence * 100)}%
              </div>
              <div style={{ fontSize: 12, color: '#c0d8f0', marginBottom: 6, lineHeight: 1.5 }}>
                {rec.reason}
              </div>
              {/* Actions */}
              <div style={{ display: 'flex', gap: 4 }}>
                <button onClick={() => onAccept(rec)} style={{ ...btnStyle, background: 'rgba(82,196,26,0.2)', color: '#52c41a', borderColor: 'rgba(82,196,26,0.4)' }}>
                  建立关联
                </button>
                <button onClick={() => onTracePath(rec.source_node_id, rec.target_node_id)} style={{ ...btnStyle, background: 'rgba(46,139,144,0.2)', color: '#2e8b90' }}>
                  路径
                </button>
                <button onClick={() => onIgnore(rec.id)} style={{ ...btnStyle, color: '#666', fontSize: 10 }}>
                  忽略
                </button>
              </div>
            </div>
          )
        })}
      </div>

      {/* Path Result */}
      {activePath && (
        <div style={{ borderTop: '1px solid rgba(46,139,144,0.3)', padding: '8px 12px' }}>
          <div style={{ display: 'flex', justifyContent: 'space-between', marginBottom: 6 }}>
            <span style={{ color: '#2e8b90', fontSize: 12 }}>路径追溯 · {activePath.total_steps} 步</span>
            <button onClick={onClosePath} style={{ ...closeBtnStyle, fontSize: 10 }}>关闭</button>
          </div>
          <div style={{ fontSize: 11, color: '#9ec5ee', lineHeight: 1.8 }}>
            {activePath.nodes.map((n, i) => (
              <span key={n.id}>
                <span style={{ color: '#fff' }}>{n.title || n.id.slice(0, 8)}</span>
                {i < activePath.nodes.length - 1 && (
                  <span style={{ color: '#2e8b90', margin: '0 4px' }}>→</span>
                )}
              </span>
            ))}
          </div>
        </div>
      )}
      {loadingPath && <div style={{ ...emptyStyle, borderTop: '1px solid rgba(46,139,144,0.3)', padding: '8px 12px' }}>计算路径中...</div>}
    </div>
  )
}

const closeBtnStyle: React.CSSProperties = {
  background: 'transparent', border: 'none', color: '#666', cursor: 'pointer', padding: '0 4px', fontSize: 12,
}

const emptyStyle: React.CSSProperties = {
  padding: '20px 12px', textAlign: 'center' as const, color: '#4a6a8e', fontSize: 12, lineHeight: 1.8,
}

const btnStyle: React.CSSProperties = {
  flex: 1, padding: '3px 0', fontSize: 11, cursor: 'pointer',
  borderRadius: 4, border: '1px solid rgba(46,139,144,0.3)',
  background: 'transparent', color: '#9ec5ee',
}
