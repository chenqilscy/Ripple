/**
 * P20-D: 节点详情侧边栏
 * 点击图谱节点后在右侧展示节点基本信息和关联边。
 * 修复：scroll lock + 窄屏 bottom sheet + CSS 变量 + 统一样式
 */
import React, { useEffect, useState, type CSSProperties } from 'react'
import { api } from '../api/client'
import type { EdgeItem, NodeItem, PromptTemplate } from '../api/types'
import AiTriggerButton from './AiTriggerButton'
import { Button } from './ui'

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
  const [feedbackSent, setFeedbackSent] = useState<'LIKE' | 'DISLIKE' | null>(null)
  const [feedbackComment, setFeedbackComment] = useState('')
  const [feedbackCommentOpen, setFeedbackCommentOpen] = useState(false)
  const [feedbackBusy, setFeedbackBusy] = useState(false)
  const nodeMap = new Map(allNodes.map(n => [n.id, n]))

  const relatedEdges = edges.filter(
    e => e.src_node_id === node.id || e.dst_node_id === node.id
  )

  // Scroll lock
  useEffect(() => {
    const prev = document.body.style.overflow
    document.body.style.overflow = 'hidden'
    return () => { document.body.style.overflow = prev }
  }, [])

  // Escape to close
  useEffect(() => {
    const handler = (e: KeyboardEvent) => { if (e.key === 'Escape') onClose() }
    window.addEventListener('keydown', handler)
    return () => window.removeEventListener('keydown', handler)
  }, [onClose])

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

  // Panel style: bottom sheet on narrow screens, sidebar on wide
  const isNarrow = typeof window !== 'undefined' && window.innerWidth < 640
  const panelStyle: CSSProperties = isNarrow
    ? {
        position: 'fixed', bottom: 0, left: 0, right: 0,
        height: '65vh', background: 'var(--bg-primary)',
        borderTop: '1px solid var(--border)',
        display: 'flex', flexDirection: 'column',
        zIndex: 400, boxShadow: 'var(--shadow-bottom-sheet)',
        borderRadius: 'var(--radius-xl) var(--radius-xl) 0 0',
        fontFamily: 'var(--font-body)', color: 'var(--text-primary)',
      }
    : {
        position: 'fixed', top: 0, right: 0, bottom: 0,
        width: 300, background: 'var(--bg-primary)',
        borderLeft: '1px solid var(--border)',
        display: 'flex', flexDirection: 'column',
        zIndex: 400, boxShadow: 'var(--shadow-sidebar)',
        fontFamily: 'var(--font-body)', color: 'var(--text-primary)',
      }

  return (
    <div style={panelStyle}>
      {/* 标题栏 */}
      <div style={{
        display: 'flex', alignItems: 'center', justifyContent: 'space-between',
        padding: 'var(--space-lg) var(--space-md)',
        borderBottom: '1px solid var(--border)',
        background: 'var(--bg-surface)',
        flexShrink: 0,
      }}>
        <span style={{ fontWeight: 600, fontSize: 'var(--font-lg)', color: 'var(--accent)' }}>节点详情</span>
        <div style={{ display: 'flex', alignItems: 'center', gap: 'var(--space-sm)' }}>
          {isNarrow && (
            <div style={{ width: 32, height: 3, background: 'var(--border)', borderRadius: 2, marginRight: 'var(--space-sm)' }} />
          )}
          {onlineUsers && onlineUsers.filter(u => u !== meId).length > 0 && (
            <span
              title={`同湖在线协作者：${onlineUsers.filter(u => u !== meId).join(', ')}`}
              style={{ fontSize: 'var(--font-xs)', color: 'var(--status-success)', background: 'var(--accent-subtle)', borderRadius: 'var(--radius-full)', padding: '1px 7px' }}
            >
              ● {onlineUsers.filter(u => u !== meId).length} 人同在
            </span>
          )}
          <Button variant="ghost" size="sm" onClick={onClose} aria-label="关闭节点详情">
            ×
          </Button>
        </div>
      </div>

      {/* 内容区 */}
      <div style={{ flex: 1, overflowY: 'auto', padding: 'var(--space-lg) var(--space-md)' }}>
        {/* 节点内容 */}
        <div style={{ marginBottom: 'var(--space-lg)' }}>
          <Label>内容</Label>
          <div style={{ fontSize: 'var(--font-base)', color: 'var(--text-primary)', lineHeight: 1.5, wordBreak: 'break-word' }}>
            {node.content || <span style={{ color: 'var(--text-tertiary)' }}>（无内容）</span>}
          </div>
        </div>

        {/* 类型 & 状态 */}
        <div style={{ display: 'flex', gap: 'var(--space-md)', marginBottom: 'var(--space-lg)' }}>
          <div style={{ flex: 1 }}>
            <Label>类型</Label>
            <span style={badgeStyle()}>{node.type}</span>
          </div>
          <div style={{ flex: 1 }}>
            <Label>状态</Label>
            <span style={badgeStyle()}>{STATE_LABEL[node.state] ?? node.state}</span>
          </div>
        </div>

        {/* 节点 ID */}
        <div style={{ marginBottom: 'var(--space-lg)' }}>
          <Label>ID</Label>
          <div style={{ display: 'flex', alignItems: 'center', gap: 'var(--space-sm)' }}>
            <code style={{ fontSize: 'var(--font-sm)', color: 'var(--text-secondary)', fontFamily: 'var(--font-mono)' }}>
              {node.id.slice(0, 8)}…
            </code>
            <CopyButton text={node.id} />
          </div>
        </div>

        {/* AI Workflow */}
        <div style={{
          marginBottom: 'var(--space-lg)', padding: 'var(--space-md)',
          border: '1px solid var(--border)', borderRadius: 'var(--radius-lg)',
          background: 'var(--bg-surface)',
        }}>
          <Label>AI Workflow</Label>
          <div style={{ color: 'var(--text-secondary)', fontSize: 'var(--font-md)', lineHeight: 1.5, marginBottom: 'var(--space-sm)' }}>
            选择 Prompt 模板后触发 AI 填充；不选模板时，将直接以当前节点内容作为 Prompt。
          </div>
          <select
            value={promptTemplateId}
            onChange={event => setPromptTemplateId(event.target.value)}
            aria-label="选择 Prompt 模板"
            style={selectStyle()}
          >
            <option value="">不使用模板</option>
            {promptTemplates.map(tpl => (
              <option key={tpl.id} value={tpl.id}>
                {tpl.scope === 'org' ? '组织 · ' : '私有 · '}{tpl.name}
              </option>
            ))}
          </select>
          {promptLoadError && (
            <div style={{ color: 'var(--status-warning)', fontSize: 'var(--font-sm)', margin: 'var(--space-sm) 0' }}>
              模板列表加载失败：{promptLoadError}
            </div>
          )}
          <div style={{ marginTop: 'var(--space-sm)' }}>
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
          {aiMessage && <div style={{ color: 'var(--status-success)', fontSize: 'var(--font-sm)', marginTop: 'var(--space-sm)' }}>{aiMessage}</div>}
        </div>

        {/* 创建时间 */}
        <div style={{ marginBottom: 'var(--space-lg)' }}>
          <Label>创建时间</Label>
          <div style={{ fontSize: 'var(--font-md)', color: 'var(--text-secondary)' }}>
            {new Date(node.created_at).toLocaleString('zh-CN')}
          </div>
        </div>

        {/* 关联边 */}
        <div>
          <div style={{ display: 'flex', alignItems: 'center', gap: 'var(--space-sm)', marginBottom: 'var(--space-sm)' }}>
            <Label>关联边</Label>
            <span style={{ fontSize: 'var(--font-xs)', color: 'var(--text-tertiary)' }}>({relatedEdges.length})</span>
          </div>
          {relatedEdges.length === 0 ? (
            <div style={{ fontSize: 'var(--font-md)', color: 'var(--text-tertiary)' }}>无关联边</div>
          ) : (
            <div style={{ display: 'flex', flexDirection: 'column', gap: 'var(--space-sm)' }}>
              {relatedEdges.map(e => {
                const isSrc = e.src_node_id === node.id
                const otherId = isSrc ? e.dst_node_id : e.src_node_id
                const other = nodeMap.get(otherId)
                const otherLabel = other?.content
                  ? (other.content.length > 16 ? other.content.slice(0, 16) + '…' : other.content)
                  : otherId.slice(0, 8) + '…'
                return (
                  <div key={e.id} style={{
                    background: 'var(--bg-surface)', borderRadius: 'var(--radius-md)',
                    padding: 'var(--space-sm) var(--space-md)', fontSize: 'var(--font-md)',
                    border: '1px solid var(--border)',
                  }}>
                    <span style={{ color: 'var(--accent)', marginRight: 'var(--space-xs)' }}>{isSrc ? '→' : '←'}</span>
                    <span style={{ color: 'var(--text-primary)' }}>{otherLabel}</span>
                    <span style={{ color: 'var(--text-tertiary)', marginLeft: 'var(--space-sm)' }}>
                      [{KIND_LABEL[e.kind] ?? e.kind}]
                    </span>
                    {e.label && (
                      <span style={{ color: 'var(--text-secondary)', marginLeft: 'var(--space-xs)' }}>"{e.label}"</span>
                    )}
                  </div>
                )
              })}
            </div>
          )}
        </div>

        {/* 节点反馈模块 */}
        <div style={{ borderTop: '1px solid var(--border)', paddingTop: 'var(--space-md)', marginTop: 'var(--space-md)' }}>
          <div style={{ fontSize: 'var(--font-md)', color: 'var(--text-secondary)', marginBottom: 'var(--space-sm)' }}>
            这个节点对你有帮助吗？
          </div>
          <div style={{ display: 'flex', gap: 'var(--space-sm)', alignItems: 'center', flexWrap: 'wrap' }}>
            <Button
              variant="ghost"
              size="sm"
              onClick={() => void sendFeedback('LIKE')}
              disabled={feedbackBusy || feedbackSent === 'LIKE'}
              aria-label="有帮助"
              style={{
                background: feedbackSent === 'LIKE' ? 'var(--feedback-like-bg)' : 'var(--bg-secondary)',
                border: `1px solid ${feedbackSent === 'LIKE' ? 'var(--status-success)' : 'var(--border)'}`,
                borderRadius: 'var(--radius-sm)', padding: '4px var(--space-md)',
                cursor: feedbackSent === 'LIKE' ? 'default' : 'pointer',
                color: feedbackSent === 'LIKE' ? 'var(--status-success)' : 'var(--text-primary)',
                fontSize: 'var(--font-md)', transition: 'background 0.2s, border-color 0.2s',
              }}
            >👍{feedbackSent === 'LIKE' ? ' 已反馈' : ''}</Button>
            <Button
              variant="ghost"
              size="sm"
              onClick={() => void sendFeedback('DISLIKE')}
              disabled={feedbackBusy || feedbackSent === 'DISLIKE'}
              aria-label="没帮助"
              style={{
                background: feedbackSent === 'DISLIKE' ? 'var(--feedback-dislike-bg)' : 'var(--bg-secondary)',
                border: `1px solid ${feedbackSent === 'DISLIKE' ? 'var(--status-danger)' : 'var(--border)'}`,
                borderRadius: 'var(--radius-sm)', padding: '4px var(--space-md)',
                cursor: feedbackSent === 'DISLIKE' ? 'default' : 'pointer',
                color: feedbackSent === 'DISLIKE' ? 'var(--status-danger)' : 'var(--text-primary)',
                fontSize: 'var(--font-md)', transition: 'background 0.2s, border-color 0.2s',
              }}
            >👎{feedbackSent === 'DISLIKE' ? ' 已反馈' : ''}</Button>
            <Button
              variant="ghost"
              size="sm"
              onClick={() => setFeedbackCommentOpen(v => !v)}
              aria-expanded={feedbackCommentOpen}
              style={{
                background: feedbackCommentOpen ? 'var(--accent-subtle)' : 'var(--bg-secondary)',
                border: '1px solid var(--border)', borderRadius: 'var(--radius-sm)', padding: '4px var(--space-md)',
                cursor: 'pointer', color: 'var(--text-primary)', fontSize: 'var(--font-md)',
              }}
            >✏ 留言</Button>
          </div>
          {feedbackCommentOpen && (
            <div style={{ marginTop: 'var(--space-sm)', display: 'flex', flexDirection: 'column', gap: 'var(--space-sm)' }}>
              <textarea
                value={feedbackComment}
                onChange={e => setFeedbackComment(e.target.value)}
                placeholder="写下你的想法..."
                rows={3}
                aria-label="反馈留言"
                style={{
                  width: '100%', background: 'var(--bg-input)', border: '1px solid var(--border-input)',
                  borderRadius: 'var(--radius-sm)', color: 'var(--text-primary)', padding: 'var(--space-sm) var(--space-md)',
                  fontSize: 'var(--font-md)', resize: 'vertical', boxSizing: 'border-box',
                }}
              />
              <Button
                variant="primary"
                size="sm"
                onClick={() => void submitComment()}
                disabled={feedbackBusy || !feedbackComment.trim()}
                style={{
                  alignSelf: 'flex-end',
                }}
              >提交</Button>
            </div>
          )}
        </div>
      </div>
    </div>
  )
}

// ---- Helper sub-components ----

function Label({ children }: { children: React.ReactNode }) {
  return (
    <div style={{
      fontSize: 'var(--font-sm)', color: 'var(--text-tertiary)',
      marginBottom: 'var(--space-xs)', textTransform: 'uppercase', letterSpacing: '0.06em',
    }}>
      {children}
    </div>
  )
}

function badgeStyle(): CSSProperties {
  return {
    display: 'inline-block', padding: '2px var(--space-sm)', borderRadius: 'var(--radius-full)',
    background: 'var(--bg-secondary)', color: 'var(--accent)', fontSize: 'var(--font-md)',
  }
}

function selectStyle(): CSSProperties {
  return {
    width: '100%', background: 'var(--bg-secondary)',
    border: '1px solid var(--border-input)',
    borderRadius: 'var(--radius-md)', color: 'var(--text-primary)',
    padding: 'var(--space-sm) var(--space-md)', fontSize: 'var(--font-md)',
  }
}

function CopyButton({ text }: { text: string }) {
  const [copied, setCopied] = useState(false)
  return (
    <Button
      variant="ghost"
      size="sm"
      onClick={() => {
        void navigator.clipboard.writeText(text).then(() => {
          setCopied(true)
          setTimeout(() => setCopied(false), 1800)
        })
      }}
      title="复制节点 ID 到剪贴板"
      aria-label="复制节点 ID"
      style={{
        background: 'transparent', border: '1px solid var(--border-subtle)',
        color: copied ? 'var(--status-success)' : 'var(--text-tertiary)',
        borderRadius: 'var(--radius-sm)', padding: '1px var(--space-sm)',
        fontSize: 'var(--font-xs)', cursor: 'pointer', lineHeight: 1.5,
      }}
    >
      {copied ? '已复制' : '复制'}
    </Button>
  )
}