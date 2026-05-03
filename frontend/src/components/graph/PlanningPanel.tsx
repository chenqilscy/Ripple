// PlanningPanel.tsx — 规划面板：知识缺口分析 + 行动建议
import { useState } from 'react'
import type { PlanningSuggestion } from '../../api/types'
import { Button } from '../ui'

interface PlanningPanelProps {
  suggestions: PlanningSuggestion[]
  loading: boolean
  onAccept: (s: PlanningSuggestion) => Promise<{ nodeId?: string; edgeId?: string }>
  onRefresh: () => void
  onClose: () => void
  /** 采纳成功后，通知父组件刷新图谱（新增节点/边） */
  onSuccess?: (nodeId?: string, edgeId?: string) => void
}

interface CardState {
  status: 'idle' | 'loading' | 'success' | 'error'
  errorMsg?: string
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

export default function PlanningPanel({ suggestions, loading, onAccept, onClose, onRefresh, onSuccess }: PlanningPanelProps) {
  const [cardStates, setCardStates] = useState<Map<string, CardState>>(new Map())

  const getCardState = (id: string): CardState =>
    cardStates.get(id) ?? { status: 'idle' }

  const setCardState = (id: string, state: CardState) =>
    setCardStates(prev => new Map(prev).set(id, state))

  const handleAccept = async (s: PlanningSuggestion) => {
    // explore 类型直接跳转到相关节点，不走后端
    if (s.type === 'explore') {
      onSuccess?.(s.related_node_ids[0])
      return
    }
    setCardState(s.id, { status: 'loading' })
    try {
      const result = await onAccept(s)
      setCardState(s.id, { status: 'success' })
      // 成功后 1.5s 移除卡片
      setTimeout(() => {
        setCardStates(prev => {
          const next = new Map(prev)
          next.delete(s.id)
          return next
        })
        onSuccess?.(result?.nodeId, result?.edgeId)
      }, 1500)
    } catch {
      setCardState(s.id, { status: 'error', errorMsg: '采纳失败，请重试' })
      setTimeout(() => setCardState(s.id, { status: 'idle' }), 3000)
    }
  }

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
                  disabled={getCardState(s.id).status === 'loading' || getCardState(s.id).status === 'success'}
                  onClick={() => handleAccept(s)}
                  style={{ width: '100%', marginTop: 6, ...(getCardState(s.id).status === 'success' ? { opacity: 0.6, pointerEvents: 'none' } : {}) }}
                >
                  {getCardState(s.id).status === 'loading' ? '采纳中…' : getCardState(s.id).status === 'success' ? '✓ 已采纳' : getCardState(s.id).status === 'error' ? '采纳失败' : '一键采纳'}
                </Button>
              </div>
            ))}
          </div>
        )
      })}
    </div>
  )
}

