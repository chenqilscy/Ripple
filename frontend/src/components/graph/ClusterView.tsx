// ClusterView.tsx — 聚类视图：颜色区分领域 + 聚焦/重置
import type { Cluster } from '../../api/types'
import { Button } from '../ui'

interface ClusterViewProps {
  clusters: Cluster[]
  focusedClusterId: string | null
  loading: boolean
  onFocus: (id: string | null) => void
  onRefresh: () => void
  onClose: () => void
}

// 预定义聚类颜色（与品牌色协调）
const CLUSTER_COLORS = [
  '#2e8b90', '#4a8eff', '#52c41a', '#faad14',
  '#f5222d', '#722ed1', '#eb2f96', '#13c2c2',
]

export default function ClusterView({ clusters, focusedClusterId, loading, onFocus, onRefresh, onClose }: ClusterViewProps) {
  return (
    <div style={{
      position: 'absolute', bottom: 60, left: 12, width: 220,
      background: 'rgba(6,13,26,0.95)', border: '1px solid #2a4a7e',
      borderRadius: 8, zIndex: 40, maxHeight: 320, overflowY: 'auto' as const,
      boxShadow: '0 4px 20px rgba(0,0,0,0.5)',
    }}>
      <div style={{
        padding: '8px 12px 6px', borderBottom: '1px solid rgba(46,74,126,0.3)',
        display: 'flex', alignItems: 'center', justifyContent: 'space-between',
      }}>
        <span style={{ color: '#9ec5ee', fontSize: 12, fontWeight: 600 }}>🗂 知识领域</span>
        <div style={{ display: 'flex', gap: 4 }}>
          <Button variant="ghost" size="sm" onClick={onRefresh} title="刷新聚类" icon="↻" />
          <Button variant="ghost" size="sm" onClick={() => onFocus(null)} title="显示全部" icon="⊡" />
          <Button variant="ghost" size="sm" onClick={onClose} icon="✕" aria-label="关闭" />
        </div>
      </div>

      {loading && (
        <div style={{ padding: '16px 12px', textAlign: 'center' as const, color: '#4a6a8e', fontSize: 12 }}>
          分析中...
        </div>
      )}

      {!loading && clusters.length === 0 && (
        <div style={{ padding: '16px 12px', textAlign: 'center' as const, color: '#4a6a8e', fontSize: 12 }}>
          节点数量不足<br /><span style={{ fontSize: 11, opacity: 0.6 }}>至少需要 5 个节点才能聚类</span>
        </div>
      )}

      {!loading && clusters.map((cluster, i) => {
        const color = cluster.color || CLUSTER_COLORS[i % CLUSTER_COLORS.length]
        const isFocused = focusedClusterId === cluster.id
        return (
          <div
            key={cluster.id}
            onClick={() => onFocus(isFocused ? null : cluster.id)}
            style={{
              padding: '7px 12px', cursor: 'pointer',
              background: isFocused ? `${color}22` : 'transparent',
              borderBottom: '1px solid rgba(46,74,126,0.2)',
              transition: 'background 0.2s',
            }}
          >
            <div style={{ display: 'flex', alignItems: 'center', gap: 6, marginBottom: 3 }}>
              <div style={{ width: 8, height: 8, borderRadius: '50%', background: color, flexShrink: 0 }} />
              <span style={{
                fontSize: 12, color: isFocused ? '#fff' : '#c0d8f0',
                fontWeight: isFocused ? 600 : 400, flex: 1, minWidth: 0,
                whiteSpace: 'nowrap' as const, overflow: 'hidden', textOverflow: 'ellipsis',
              }}>
                {cluster.label}
              </span>
              <span style={{ fontSize: 10, color: '#4a6a8e' }}>{cluster.node_ids.length}</span>
            </div>
            {cluster.bridge_node_ids.length > 0 && (
              <div style={{ fontSize: 10, color: '#faad14', marginLeft: 14 }}>
                🔗 跨领域节点 {cluster.bridge_node_ids.length} 个
              </div>
            )}
          </div>
        )
      })}
    </div>
  )
}

