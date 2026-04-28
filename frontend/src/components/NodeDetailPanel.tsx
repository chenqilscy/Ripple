/**
 * P20-D: 节点详情侧边栏
 * 点击图谱节点后在右侧展示节点基本信息和关联边。
 */
import type { EdgeItem, NodeItem } from '../api/types'

interface Props {
  node: NodeItem
  allNodes: NodeItem[]
  edges: EdgeItem[]
  onClose: () => void
}

const STATE_LABEL: Record<string, string> = {
  MIST: '雾态', DROP: '水滴', FROZEN: '冻结', VAPOR: '蒸发', ERASED: '删除', GHOST: '幽灵',
}

const KIND_LABEL: Record<string, string> = {
  relates: '关联', derives: '派生', opposes: '对立', refines: '细化', groups: '分组', custom: '自定义',
}

export default function NodeDetailPanel({ node, allNodes, edges, onClose }: Props) {
  const nodeMap = new Map(allNodes.map(n => [n.id, n]))

  const relatedEdges = edges.filter(
    e => e.src_node_id === node.id || e.dst_node_id === node.id
  )

  return (
    <div style={{
      position: 'fixed', top: 0, right: 0, bottom: 0,
      width: 300, background: '#111827',
      borderLeft: '1px solid #1e3a5a',
      display: 'flex', flexDirection: 'column',
      zIndex: 400, boxShadow: '-4px 0 16px rgba(0,0,0,0.4)',
      fontFamily: 'system-ui, sans-serif', color: '#c8d8e8',
    }}>
      {/* 标题栏 */}
      <div style={{
        display: 'flex', alignItems: 'center', justifyContent: 'space-between',
        padding: '12px 14px', borderBottom: '1px solid #1e3a5a',
        background: '#0d1b2a',
      }}>
        <span style={{ fontWeight: 600, fontSize: 14, color: '#9ec5ee' }}>节点详情</span>
        <button
          onClick={onClose}
          style={{
            background: 'none', border: 'none', color: '#6a8aaa', cursor: 'pointer',
            fontSize: 18, lineHeight: 1, padding: '2px 6px', borderRadius: 4,
          }}
          title="关闭"
          aria-label="关闭节点详情"
        >
          ×
        </button>
      </div>

      {/* 内容区 */}
      <div style={{ flex: 1, overflowY: 'auto', padding: '14px' }}>
        {/* 节点内容 */}
        <div style={{ marginBottom: 14 }}>
          <div style={{ fontSize: 11, color: '#4a6a8e', marginBottom: 4, textTransform: 'uppercase', letterSpacing: '0.06em' }}>内容</div>
          <div style={{ fontSize: 13, color: '#c8d8e8', lineHeight: 1.5, wordBreak: 'break-word' }}>
            {node.content || <span style={{ color: '#4a6a8e' }}>（无内容）</span>}
          </div>
        </div>

        {/* 类型 & 状态 */}
        <div style={{ display: 'flex', gap: 10, marginBottom: 14 }}>
          <div style={{ flex: 1 }}>
            <div style={{ fontSize: 11, color: '#4a6a8e', marginBottom: 4, textTransform: 'uppercase', letterSpacing: '0.06em' }}>类型</div>
            <span style={{
              display: 'inline-block', padding: '2px 8px', borderRadius: 10,
              background: '#1e3a5a', color: '#9ec5ee', fontSize: 12,
            }}>
              {node.type}
            </span>
          </div>
          <div style={{ flex: 1 }}>
            <div style={{ fontSize: 11, color: '#4a6a8e', marginBottom: 4, textTransform: 'uppercase', letterSpacing: '0.06em' }}>状态</div>
            <span style={{
              display: 'inline-block', padding: '2px 8px', borderRadius: 10,
              background: '#1e3a5a', color: '#9ec5ee', fontSize: 12,
            }}>
              {STATE_LABEL[node.state] ?? node.state}
            </span>
          </div>
        </div>

        {/* 节点 ID */}
        <div style={{ marginBottom: 14 }}>
          <div style={{ fontSize: 11, color: '#4a6a8e', marginBottom: 4, textTransform: 'uppercase', letterSpacing: '0.06em' }}>ID</div>
          <code style={{ fontSize: 11, color: '#6a8aaa', fontFamily: 'monospace' }}>
            {node.id.slice(0, 8)}…
          </code>
        </div>

        {/* 创建时间 */}
        <div style={{ marginBottom: 18 }}>
          <div style={{ fontSize: 11, color: '#4a6a8e', marginBottom: 4, textTransform: 'uppercase', letterSpacing: '0.06em' }}>创建时间</div>
          <div style={{ fontSize: 12, color: '#6a8aaa' }}>
            {new Date(node.created_at).toLocaleString('zh-CN')}
          </div>
        </div>

        {/* 关联边 */}
        <div>
          <div style={{ fontSize: 11, color: '#4a6a8e', marginBottom: 8, textTransform: 'uppercase', letterSpacing: '0.06em' }}>
            关联边 ({relatedEdges.length})
          </div>
          {relatedEdges.length === 0 ? (
            <div style={{ fontSize: 12, color: '#4a6a8e' }}>无关联边</div>
          ) : (
            <div style={{ display: 'flex', flexDirection: 'column', gap: 6 }}>
              {relatedEdges.map(e => {
                const isSrc = e.src_node_id === node.id
                const otherId = isSrc ? e.dst_node_id : e.src_node_id
                const other = nodeMap.get(otherId)
                const otherLabel = other?.content
                  ? (other.content.length > 16 ? other.content.slice(0, 16) + '…' : other.content)
                  : otherId.slice(0, 8) + '…'
                return (
                  <div key={e.id} style={{
                    background: '#0d1b2a', borderRadius: 6,
                    padding: '6px 10px', fontSize: 12,
                    border: '1px solid #1e3a5a',
                  }}>
                    <span style={{ color: '#4a8eff', marginRight: 4 }}>
                      {isSrc ? '→' : '←'}
                    </span>
                    <span style={{ color: '#9ec5ee' }}>{otherLabel}</span>
                    <span style={{ color: '#4a6a8e', marginLeft: 6 }}>
                      [{KIND_LABEL[e.kind] ?? e.kind}]
                    </span>
                    {e.label && (
                      <span style={{ color: '#6a8aaa', marginLeft: 4 }}>"{e.label}"</span>
                    )}
                  </div>
                )
              })}
            </div>
          )}
        </div>
      </div>
    </div>
  )
}
