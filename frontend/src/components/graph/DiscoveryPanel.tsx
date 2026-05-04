// DiscoveryPanel.tsx — 发现面板：推荐列表 + 路径触发
import React from 'react'
import type { Recommendation, PathResult, HeatNode } from '../../api/types'
import { Button } from '../ui'

interface DiscoveryPanelProps {
  recommendations: Recommendation[]
  loading: boolean
  activePath: PathResult | null
  loadingPath: boolean
  onAccept: (rec: Recommendation) => void
  onIgnore: (id: string) => void
  onTracePath: (sourceId: string, targetId: string) => void
  onClosePath: () => void
  onClose: () => void
  /** 图谱热度趋势 */
  heatNodes?: HeatNode[]
  loadingHeat?: boolean
  onTraceHeat?: (nodeId: string) => void
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
  onAccept, onIgnore, onTracePath, onClosePath, onClose,
  heatNodes, loadingHeat, onTraceHeat,
}: DiscoveryPanelProps) {
  const pending = recommendations.filter(r => r.status === 'pending')
  const [activeTab, setActiveTab] = React.useState<'discover' | 'heat'>('discover')

  return (
    <div style={{
      position: 'absolute', top: 12, right: 70, width: 280,
      background: 'rgba(6,13,26,0.95)', border: '1px solid #2e8b90',
      borderRadius: 8, zIndex: 40, maxHeight: 420, overflowY: 'auto',
      boxShadow: '0 4px 20px rgba(0,0,0,0.5)',
    }}>
      {/* Header */}
      <div style={{
        padding: '10px 14px 8px',
        borderBottom: '1px solid rgba(46,139,144,0.3)',
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'space-between',
      }}>
        <div style={{ display: 'flex', gap: 6 }}>
          <Button
            variant={activeTab === 'discover' ? 'primary' : 'ghost'}
            size="sm"
            onClick={() => setActiveTab('discover')}
          >
            发现
          </Button>
          <Button
            variant={activeTab === 'heat' ? 'primary' : 'ghost'}
            size="sm"
            onClick={() => setActiveTab('heat')}
          >
            热点
          </Button>
        </div>
        <Button variant="ghost" size="sm" onClick={onClose} aria-label="关闭">✕</Button>
      </div>

      {/* Content */}
      {activeTab === 'discover' && (
        <>
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
                    <Button variant="primary" size="sm" onClick={() => onAccept(rec)}>
                      建立关联
                    </Button>
                    <Button variant="ghost" size="sm" onClick={() => onTracePath(rec.source_node_id, rec.target_node_id)}>
                      路径
                    </Button>
                    <Button variant="ghost" size="sm" onClick={() => onIgnore(rec.id)}>
                      忽略
                    </Button>
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
                <Button variant="ghost" size="sm" onClick={onClosePath} style={{ fontSize: 10 }}>关闭</Button>
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
        </>
      )}

      {activeTab === 'heat' && (
        <div style={{ padding: '8px 0' }}>
          {loadingHeat && (
            <div style={{ padding: '20px 0', textAlign: 'center', color: '#6b7280', fontSize: 13 }}>
              加载中...
            </div>
          )}
          {!loadingHeat && (!heatNodes || heatNodes.length === 0) && (
            <div style={{ padding: '20px 14px', textAlign: 'center', color: '#6b7280', fontSize: 13 }}>
              本周暂无热点<br />
              <span style={{ fontSize: 11, opacity: 0.6 }}>继续积累知识网络，热度会逐渐浮现</span>
            </div>
          )}
          {!loadingHeat && heatNodes && heatNodes.map(node => (
            <div
              key={node.node_id}
              onClick={() => onTraceHeat?.(node.node_id)}
              style={{
                margin: '0 10px 8px',
                padding: '8px 10px',
                background: 'rgba(46,139,144,0.1)',
                borderRadius: 6,
                border: '1px solid rgba(46,139,144,0.2)',
                cursor: 'pointer',
              }}
            >
              {/* Rank badge */}
              <div style={{ display: 'flex', alignItems: 'center', gap: 6, marginBottom: 4 }}>
                <span style={{
                  display: 'inline-flex', alignItems: 'center', justifyContent: 'center',
                  width: 18, height: 18, borderRadius: '50%',
                  fontSize: 10, fontWeight: 700,
                  background: node.rank <= 3 ? '#f59e0b' : 'rgba(255,255,255,0.1)',
                  color: node.rank <= 3 ? '#000' : '#9ec5ee',
                }}>
                  {node.rank}
                </span>
                <span style={{ fontSize: 11, color: '#6b7280' }}>
                  编辑 {node.edit_count} · 关联 {node.edge_count}
                </span>
              </div>
              {/* Content */}
              <div style={{ fontSize: 12, color: '#c0d8f0', marginBottom: 6, lineHeight: 1.5 }}>
                {node.content.length > 40 ? node.content.slice(0, 40) + '...' : node.content}
              </div>
              {/* Heat bar */}
              <div style={{ height: 4, borderRadius: 2, background: 'rgba(255,255,255,0.1)', overflow: 'hidden' }}>
                <div style={{
                  height: '100%',
                  width: `${node.heat_score * 100}%`,
                  background: node.heat_score > 0.6 ? '#ef4444' : (node.heat_score > 0.3 ? '#f59e0b' : '#6b7280'),
                  borderRadius: 2,
                  transition: 'width 0.3s ease',
                }} />
              </div>
            </div>
          ))}
        </div>
      )}
    </div>
  )
}

const emptyStyle: React.CSSProperties = {
  padding: '20px 12px', textAlign: 'center' as const, color: '#4a6a8e', fontSize: 12, lineHeight: 1.8,
}
