// PlanningPanel.tsx — 规划面板：知识缺口分析 + 行动建议
import type { PlanningSuggestion } from '../../api/types'
import { Button } from '../ui'

interface PlanningPanelProps {
  suggestions: PlanningSuggestion[]
  loading: boolean
  onAccept: (s: PlanningSuggestion) => void
  onRefresh: () => void
  onClose: () => void
}

const PRIORITY_COLOR: Record<string, string> = {
  high: '#f5222d',
  medium: '#faad14',
  low: '#8c8c8c',
}

const TYPE_LABEL: Record<string, string> = {
  add_node: '添加节点',
  connect: '建立关联',
  explore: '深入探索',
}

export default function PlanningPanel({ suggestions, loading, onAccept, onRefresh, onClose }: PlanningPanelProps) {
  const byPriority = ['high', 'medium', 'low']

  return (
    <div style={{
      position: 'absolute', top: 12, right: 70, width: 300,
      background: 'rgba(6,13,26,0.95)', border: '1px solid #2e8b90',
      borderRadius: 8, zIndex: 40, maxHeight: 480, overflowY: 'auto' as const,
      boxShadow: '0 4px 20px rgba(0,0,0,0.5)',
    }}>
      <div style={{
        padding: '10px 14px 8px', borderBottom: '1px solid rgba(46,139,144,0.3)',
        display: 'flex', alignItems: 'center', justifyContent: 'space-between',
      }}>
        <span style={{ color: '#9ec5ee', fontSize: 13, fontWeight: 600 }}>📋 规划建议</span>
        <div style={{ display: 'flex', gap: 4 }}>
          <Button variant="ghost" size="sm" onClick={onRefresh} title="重新分析" icon="↻" />
          <Button variant="ghost" size="sm" onClick={onClose} icon="✕" aria-label="关闭" />
        </div>
      </div>

      {loading && (
        <div style={{ padding: '20px 12px', textAlign: 'center' as const, color: '#4a6a8e' }}>
          分析知识结构中...
        </div>
      )}

      {!loading && suggestions.length === 0 && (
        <div style={{ padding: '20px 12px', textAlign: 'center' as const, color: '#4a6a8e', lineHeight: 1.8 }}>
          暂无规划建议<br />
          <span style={{ fontSize: 11, opacity: 0.6 }}>继续积累节点和关联，规划建议会更精准</span>
        </div>
      )}

      {!loading && byPriority.map(priority => {
        const items = suggestions.filter(s => s.priority === priority)
        if (items.length === 0) return null
        return (
          <div key={priority} style={{ padding: '0 0 8px' }}>
            <div style={{
              padding: '4px 14px', fontSize: 10,
              color: PRIORITY_COLOR[priority],
              textTransform: 'uppercase' as const, letterSpacing: 1,
            }}>
              {priority === 'high' ? '⚠ 高优先级' : priority === 'medium' ? '中优先级' : '低优先级'}
            </div>
            {items.map(s => (
              <div key={s.id} style={{
                margin: '0 10px 6px', padding: '8px 10px',
                background: 'rgba(0,0,0,0.2)', borderRadius: 6,
                border: `1px solid ${PRIORITY_COLOR[priority]}22`,
              }}>
                <div style={{ display: 'flex', alignItems: 'center', gap: 6, marginBottom: 4 }}>
                  <span style={{
                    fontSize: 10, padding: '1px 5px', borderRadius: 3,
                    background: `${PRIORITY_COLOR[priority]}22`, color: PRIORITY_COLOR[priority],
                  }}>
                    {TYPE_LABEL[s.type]}
                  </span>
                  <span style={{ fontSize: 12, color: '#c0d8f0', fontWeight: 500 }}>
                    {s.title}
                  </span>
                </div>
                <div style={{ fontSize: 11, color: '#4a6a8e', marginBottom: 6, lineHeight: 1.5 }}>
                  {s.description}
                </div>
                <Button
                  variant="secondary"
                  size="sm"
                  onClick={() => onAccept(s)}
                  style={{ width: '100%', marginTop: 6 }}
                >
                  一键采纳
                </Button>
              </div>
            ))}
          </div>
        )
      })}
    </div>
  )
}

