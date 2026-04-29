/**
 * P20-D: 节点详情侧边栏
 * 点击图谱节点后在右侧展示节点基本信息和关联边。
 */
import { useEffect, useState, type CSSProperties } from 'react'
import { api } from '../api/client'
import type { EdgeItem, NodeItem, PromptTemplate } from '../api/types'
import AiTriggerButton from './AiTriggerButton'

interface Props {
  node: NodeItem
  allNodes: NodeItem[]
  edges: EdgeItem[]
  onClose: () => void
  onAiDone?: (nodeId: string) => void | Promise<void>
  onlineUsers?: string[]
  meId?: string
}

const STATE_LABEL: Record<string, string> = {
  MIST: '雾态', DROP: '水滴', FROZEN: '冻结', VAPOR: '蒸发', ERASED: '删除', GHOST: '幽灵',
}

const KIND_LABEL: Record<string, string> = {
  relates: '关联', derives: '派生', opposes: '对立', refines: '细化', groups: '分组', summarizes: '摘要', custom: '自定义',
}

export default function NodeDetailPanel({ node, allNodes, edges, onClose, onAiDone, onlineUsers, meId }: Props) {
  const [promptTemplates, setPromptTemplates] = useState<PromptTemplate[]>([])
  const [promptTemplateId, setPromptTemplateId] = useState('')
  const [promptLoadError, setPromptLoadError] = useState('')
  const [aiMessage, setAiMessage] = useState('')
  /** 3-P2-01: 反馈状态 */
  const [feedbackSent, setFeedbackSent] = useState<'LIKE' | 'DISLIKE' | null>(null)
  const [feedbackComment, setFeedbackComment] = useState('')
  const [feedbackCommentOpen, setFeedbackCommentOpen] = useState(false)
  const [feedbackBusy, setFeedbackBusy] = useState(false)
  const nodeMap = new Map(allNodes.map(n => [n.id, n]))

  const relatedEdges = edges.filter(
    e => e.src_node_id === node.id || e.dst_node_id === node.id
  )

  useEffect(() => {
    let cancelled = false
    setPromptLoadError('')
    api.listPromptTemplates()
      .then(res => {
        if (!cancelled) setPromptTemplates(res.items ?? [])
      })
      .catch(e => {
        if (!cancelled) setPromptLoadError(String((e as Error)?.message || e))
      })
    return () => { cancelled = true }
  }, [])

  useEffect(() => {
    setPromptTemplateId('')
    setAiMessage('')
    setFeedbackSent(null)
    setFeedbackComment('')
    setFeedbackCommentOpen(false)
  }, [node.id])

  async function sendFeedback(type: 'LIKE' | 'DISLIKE') {
    if (feedbackBusy) return
    setFeedbackBusy(true)
    try {
      await api.sendFeedback('node', node.id, type)
      setFeedbackSent(type)
    } catch { /* 静默 */ } finally {
      setFeedbackBusy(false)
    }
  }

  async function submitComment() {
    const text = feedbackComment.trim()
    if (!text || feedbackBusy) return
    setFeedbackBusy(true)
    try {
      await api.sendFeedback('node', node.id, 'COMMENT', text)
      setFeedbackComment('')
      setFeedbackCommentOpen(false)
    } catch { /* 静默 */ } finally {
      setFeedbackBusy(false)
    }
  }

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
        <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
          {/* P2-02: 在线协作者提示 */}
          {onlineUsers && onlineUsers.filter(u => u !== meId).length > 0 && (
            <span title={`同湖在线协作者：${onlineUsers.filter(u => u !== meId).join(', ')}`} style={{ fontSize: 11, color: '#7fdbb6', background: 'rgba(127,219,182,0.12)', borderRadius: 10, padding: '1px 7px' }}>
              ● {onlineUsers.filter(u => u !== meId).length} 人同在
            </span>
          )}
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
          <div style={{ display: 'flex', alignItems: 'center', gap: 6 }}>
            <code style={{ fontSize: 11, color: '#6a8aaa', fontFamily: 'monospace' }}>
              {node.id.slice(0, 8)}…
            </code>
            <button
              onClick={() => {
                void navigator.clipboard.writeText(node.id)
                  .then(() => {
                    const el = document.createElement('div')
                    el.textContent = '已复制'
                    Object.assign(el.style, {
                      position: 'fixed', bottom: '24px', right: '24px', zIndex: '9999',
                      background: '#1e3a5a', color: '#9ec5ee', padding: '6px 14px',
                      borderRadius: '6px', fontSize: '12px', pointerEvents: 'none',
                    })
                    document.body.appendChild(el)
                    setTimeout(() => document.body.removeChild(el), 1800)
                  })
              }}
              title="复制完整节点 ID 到剪贴板"
              style={{
                background: 'transparent', border: '1px solid #2a3a4a',
                color: '#4a6a8e', borderRadius: 4, padding: '1px 6px',
                fontSize: 10, cursor: 'pointer', lineHeight: 1.5,
              }}
            >复制</button>
          </div>
        </div>

        {/* AI Workflow */}
        <div style={{ marginBottom: 18, padding: 10, border: '1px solid #1e3a5a', borderRadius: 8, background: '#0d1b2a' }}>
          <div style={{ fontSize: 11, color: '#4a6a8e', marginBottom: 6, textTransform: 'uppercase', letterSpacing: '0.06em' }}>AI Workflow</div>
          <div style={{ color: '#8fb7dc', fontSize: 12, lineHeight: 1.5, marginBottom: 8 }}>
            选择 Prompt 模板后触发 AI 填充；不选模板时，将直接以当前节点内容作为 Prompt。
          </div>
          <select
            value={promptTemplateId}
            onChange={event => setPromptTemplateId(event.target.value)}
            style={aiSelectStyle}
            aria-label="选择 Prompt 模板"
          >
            <option value="">不使用模板</option>
            {promptTemplates.map(tpl => (
              <option key={tpl.id} value={tpl.id}>
                {tpl.scope === 'org' ? '组织 · ' : '私有 · '}{tpl.name}
              </option>
            ))}
          </select>
          {promptLoadError && (
            <div style={{ color: '#f9e2af', fontSize: 11, margin: '6px 0' }}>模板列表加载失败：{promptLoadError}</div>
          )}
          <div style={{ marginTop: 8 }}>
            <AiTriggerButton
              lakeId={node.lake_id}
              nodeId={node.id}
              promptTemplateId={promptTemplateId || undefined}
              onDone={job => {
                setAiMessage(`AI 已完成：${job.job_id.slice(0, 8)}…`)
                void onAiDone?.(node.id)
              }}
              onFail={job => setAiMessage(`AI 失败：${job.error || job.status}`)}
            />
          </div>
          {aiMessage && <div style={{ color: '#94e2d5', fontSize: 11, marginTop: 8 }}>{aiMessage}</div>}
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

        {/* 3-P2-01: 节点反馈模块 */}
        <div style={{ borderTop: '1px solid #1e3a5a', paddingTop: 10, marginTop: 4 }}>
          <div style={{ fontSize: 11, color: '#6a8aaa', marginBottom: 6 }}>这个节点对你有帮助吗？</div>
          <div style={{ display: 'flex', gap: 8, alignItems: 'center' }}>
            <button
              onClick={() => void sendFeedback('LIKE')}
              disabled={feedbackBusy || feedbackSent === 'LIKE'}
              style={{
                background: feedbackSent === 'LIKE' ? '#1a4a2e' : '#0f1f2e',
                border: `1px solid ${feedbackSent === 'LIKE' ? '#3ddc84' : '#2d5278'}`,
                borderRadius: 5, padding: '4px 12px', cursor: feedbackSent === 'LIKE' ? 'default' : 'pointer',
                color: feedbackSent === 'LIKE' ? '#3ddc84' : '#9ec5ee', fontSize: 14,
              }}
              title="有帮助"
            >👍{feedbackSent === 'LIKE' ? ' 已反馈' : ''}</button>
            <button
              onClick={() => void sendFeedback('DISLIKE')}
              disabled={feedbackBusy || feedbackSent === 'DISLIKE'}
              style={{
                background: feedbackSent === 'DISLIKE' ? '#2d1a1a' : '#0f1f2e',
                border: `1px solid ${feedbackSent === 'DISLIKE' ? '#f38ba8' : '#2d5278'}`,
                borderRadius: 5, padding: '4px 12px', cursor: feedbackSent === 'DISLIKE' ? 'default' : 'pointer',
                color: feedbackSent === 'DISLIKE' ? '#f38ba8' : '#9ec5ee', fontSize: 14,
              }}
              title="没帮助"
            >👎{feedbackSent === 'DISLIKE' ? ' 已反馈' : ''}</button>
            <button
              onClick={() => setFeedbackCommentOpen(v => !v)}
              style={{
                background: feedbackCommentOpen ? '#1a2d4a' : '#0f1f2e',
                border: '1px solid #2d5278', borderRadius: 5, padding: '4px 10px',
                cursor: 'pointer', color: '#9ec5ee', fontSize: 12,
              }}
              title="留下文字反馈"
            >✏ 留言</button>
          </div>
          {feedbackCommentOpen && (
            <div style={{ marginTop: 8, display: 'flex', flexDirection: 'column', gap: 6 }}>
              <textarea
                value={feedbackComment}
                onChange={e => setFeedbackComment(e.target.value)}
                placeholder="写下你的想法..."
                rows={3}
                style={{
                  width: '100%', background: '#0a1526', border: '1px solid #2d5278',
                  borderRadius: 5, color: '#c8d8e8', padding: '6px 8px', fontSize: 12,
                  resize: 'vertical', boxSizing: 'border-box',
                }}
              />
              <button
                onClick={() => void submitComment()}
                disabled={feedbackBusy || !feedbackComment.trim()}
                style={{
                  alignSelf: 'flex-end', background: '#1e4d9e', border: '1px solid #4a8eff',
                  borderRadius: 5, padding: '4px 14px', color: '#9ec5ee',
                  fontSize: 12, cursor: feedbackComment.trim() ? 'pointer' : 'default',
                }}
              >提交</button>
            </div>
          )}
        </div>

      </div>
    </div>
  )
}

const aiSelectStyle: CSSProperties = {
  width: '100%',
  background: '#111827',
  border: '1px solid #2d5278',
  borderRadius: 6,
  color: '#c8d8e8',
  padding: '7px 8px',
  fontSize: 12,
}
