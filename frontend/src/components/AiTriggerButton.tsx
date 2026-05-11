/**
 * Phase 15-C: AI Trigger button for a node.
 * Sends a trigger request and polls job status until done/failed (60s timeout).
 * 修复：CSS 变量（Deep Ocean Dark 主题）
 * Phase 15.2: Add template selection dropdown
 */
import { useCallback, useEffect, useRef, useState } from 'react'
import { Button } from './ui'
import { api } from '../api/client'
import type { AiJob, AiJobStatus, ApiError, PromptTemplate } from '../api/types'

interface Props {
  lakeId: string
  nodeId: string
  promptTemplateId?: string
  inputNodeIds?: string[]
  /** Called when job completes successfully */
  onDone?: (job: AiJob) => void
  /** Called when job fails */
  onFail?: (job: AiJob) => void
}

const STATUS_LABEL: Record<AiJobStatus, string> = {
  pending:    '等待中',
  processing: '处理中',
  done:       '完成',
  failed:     '失败',
}

// Generate session-based idempotency key to prevent duplicate AI trigger requests
function generateIdempotencyKey(): string {
  const stored = sessionStorage.getItem('ai_trigger_idem_key')
  if (stored) return stored

  const key = `ai-${Date.now()}-${Math.random().toString(36).substr(2, 9)}`
  sessionStorage.setItem('ai_trigger_idem_key', key)
  return key
}

const POLL_INTERVAL_MS = 2000
const POLL_MAX = 30 // 60 seconds total

export default function AiTriggerButton({ lakeId, nodeId, promptTemplateId, inputNodeIds, onDone, onFail }: Props) {
  const [job, setJob] = useState<AiJob | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [triggering, setTriggering] = useState(false)
  const pollCount = useRef(0)
  const [pollCountDisplay, setPollCountDisplay] = useState(0)
  const pollTimer = useRef<ReturnType<typeof setTimeout> | null>(null)

  // Phase 15.2: Template selection dropdown state
  const [showDropdown, setShowDropdown] = useState(false)
  const [templates, setTemplates] = useState<PromptTemplate[]>([])
  const [selectedTemplate, setSelectedTemplate] = useState<PromptTemplate | null>(null)
  const [loadingTemplates, setLoadingTemplates] = useState(false)
  const dropdownRef = useRef<HTMLDivElement>(null)

  // Load templates when dropdown opens
  useEffect(() => {
    if (showDropdown && templates.length === 0) {
      setLoadingTemplates(true)
      api.listPromptTemplates()
        .then(res => setTemplates(res.items))
        .catch(() => setTemplates([]))
        .finally(() => setLoadingTemplates(false))
    }
  }, [showDropdown, templates.length])

  // Click outside to close dropdown
  useEffect(() => {
    function handleClickOutside(e: MouseEvent) {
      if (dropdownRef.current && !dropdownRef.current.contains(e.target as Node)) {
        setShowDropdown(false)
      }
    }
    document.addEventListener('mousedown', handleClickOutside)
    return () => document.removeEventListener('mousedown', handleClickOutside)
  }, [])

  // Group templates by scope
  const privateTemplates = templates.filter(t => t.scope === 'private')
  const orgTemplates = templates.filter(t => t.scope === 'org')

  const stopPolling = useCallback(() => {
    if (pollTimer.current !== null) {
      clearTimeout(pollTimer.current)
      pollTimer.current = null
    }
  }, [])

  const poll = useCallback(async (currentJob: AiJob) => {
    if (pollCount.current >= POLL_MAX) {
      stopPolling()
      const timeoutJob: AiJob = { ...currentJob, status: 'failed', progress_pct: 0, error: '轮询超时（60s）' }
      setJob(timeoutJob)
      onFail?.(timeoutJob)
      return
    }
    try {
      const updated = await api.aiStatus(lakeId, nodeId)
      setJob(updated)
      if (updated.status === 'done') {
        stopPolling()
        onDone?.(updated)
        return
      }
      if (updated.status === 'failed') {
        stopPolling()
        onFail?.(updated)
        return
      }
      pollCount.current += 1
      setPollCountDisplay(pollCount.current)
      pollTimer.current = setTimeout(() => poll(updated), POLL_INTERVAL_MS)
    } catch {
      // transient network error — keep polling
      pollCount.current += 1
      setPollCountDisplay(pollCount.current)
      pollTimer.current = setTimeout(() => poll(currentJob), POLL_INTERVAL_MS)
    }
  }, [lakeId, nodeId, onDone, onFail, stopPolling])

  useEffect(() => () => stopPolling(), [stopPolling])

  useEffect(() => {
    stopPolling()
    setJob(null)
    setError(null)
    pollCount.current = 0
    setPollCountDisplay(0)
  }, [lakeId, nodeId, stopPolling])

  const handleTrigger = useCallback(async (templateIdOverride?: string) => {
    if (triggering || (job && (job.status === 'pending' || job.status === 'processing'))) return
    setError(null)
    setTriggering(true)
    stopPolling()
    pollCount.current = 0
    setPollCountDisplay(0)
    try {
      const idempotencyKey = generateIdempotencyKey()
      // Use override templateId (from dropdown), prop templateId, or dropdown-selected template
      const templateId = templateIdOverride || promptTemplateId || selectedTemplate?.id
      const newJob = await api.aiTrigger(lakeId, nodeId, {
        prompt_template_id: templateId,
        input_node_ids: inputNodeIds,
        idempotency_key: idempotencyKey,
      })
      setJob(newJob)
      if (newJob.status !== 'done' && newJob.status !== 'failed') {
        pollTimer.current = setTimeout(() => poll(newJob), POLL_INTERVAL_MS)
      } else if (newJob.status === 'done') {
        onDone?.(newJob)
      } else {
        onFail?.(newJob)
      }
    } catch (e) {
      const err = e as ApiError
      if (err.status === 409) {
        // Duplicate request - continue tracking existing job
        const existingNodeId = (err as { data?: { existing_ai_node_id?: string } }).data?.existing_ai_node_id
        setError(existingNodeId ? '该请求已处理，继续跟踪当前任务。' : '该节点已有 AI 任务，继续跟踪当前任务状态。')
        try {
          const existing = await api.aiStatus(lakeId, nodeId)
          setJob(existing)
          if (existing.status !== 'done' && existing.status !== 'failed') {
            pollTimer.current = setTimeout(() => poll(existing), POLL_INTERVAL_MS)
          } else if (existing.status === 'done') {
            onDone?.(existing)
          } else {
            onFail?.(existing)
          }
          return
        } catch (statusErr) {
          setError(String((statusErr as Error)?.message || statusErr))
          return
        }
      }
      setError(String((e as Error)?.message || e))
    } finally {
      setTriggering(false)
    }
  }, [triggering, job, lakeId, nodeId, promptTemplateId, selectedTemplate, inputNodeIds, poll, stopPolling, onDone, onFail])

  const isRunning = job?.status === 'pending' || job?.status === 'processing'
  const disabled = triggering || isRunning

  function statusColor(status: AiJobStatus): string {
    if (status === 'pending') return 'var(--status-warning)'
    if (status === 'processing') return 'var(--accent)'
    if (status === 'done') return 'var(--status-success)'
    return 'var(--status-danger)'
  }

  return (
    <div style={{ display: 'inline-flex', flexDirection: 'column', gap: 'var(--space-sm)', alignItems: 'flex-start', position: 'relative' }}>
      <Button
        variant="primary"
        size="sm"
        onClick={() => setShowDropdown(!showDropdown)}
        disabled={disabled}
        title="选择Prompt模板并触发AI处理"
      >
        <span style={{ fontSize: 'var(--font-lg)' }}>✦</span>
        {isRunning ? 'AI 处理中…' : (selectedTemplate ? selectedTemplate.name : 'AI 触发')}
      </Button>

      {/* Phase 15.2: Template selection dropdown */}
      {showDropdown && (
        <div
          ref={dropdownRef}
          style={{
            position: 'absolute',
            top: '100%',
            left: 0,
            zIndex: 1000,
            background: 'var(--bg-surface)',
            border: '1px solid var(--border)',
            borderRadius: 'var(--radius-lg)',
            boxShadow: 'var(--shadow-lg)',
            minWidth: 280,
            maxHeight: 320,
            overflow: 'auto',
            marginTop: 'var(--space-xs)',
          }}
        >
          {loadingTemplates ? (
            <div style={{ padding: 'var(--space-md)', color: 'var(--text-tertiary)' }}>
              加载中…
            </div>
          ) : (
            <>
              {privateTemplates.length > 0 && (
                <div>
                  <div style={{
                    padding: 'var(--space-sm) var(--space-md)',
                    fontSize: 'var(--font-sm)',
                    color: 'var(--text-tertiary)',
                    fontWeight: 600,
                    borderBottom: '1px solid var(--border)',
                  }}>
                    📋 私有模板
                  </div>
                  {privateTemplates.map(t => (
                    <div
                      key={t.id}
                      style={{
                        padding: 'var(--space-sm) var(--space-md)',
                        cursor: 'pointer',
                        borderBottom: '1px solid var(--border)',
                      }}
                      onClick={() => {
                        setSelectedTemplate(t)
                        setShowDropdown(false)
                        // Immediately trigger with selected template
                        void handleTrigger(t.id)
                      }}
                      onMouseEnter={e => e.currentTarget.style.background = 'var(--bg-hover)'}
                      onMouseLeave={e => e.currentTarget.style.background = 'transparent'}
                    >
                      <div style={{ fontWeight: 600, color: 'var(--text-primary)' }}>
                        {t.name}
                      </div>
                      <div style={{ fontSize: 'var(--font-sm)', color: 'var(--text-tertiary)' }}>
                        {t.description || '无描述'}
                      </div>
                    </div>
                  ))}
                </div>
              )}
              {orgTemplates.length > 0 && (
                <div>
                  <div style={{
                    padding: 'var(--space-sm) var(--space-md)',
                    fontSize: 'var(--font-sm)',
                    color: 'var(--text-tertiary)',
                    fontWeight: 600,
                    borderBottom: '1px solid var(--border)',
                  }}>
                    📂 组织共享
                  </div>
                  {orgTemplates.map(t => (
                    <div
                      key={t.id}
                      style={{
                        padding: 'var(--space-sm) var(--space-md)',
                        cursor: 'pointer',
                        borderBottom: '1px solid var(--border)',
                      }}
                      onClick={() => {
                        setSelectedTemplate(t)
                        setShowDropdown(false)
                        // Immediately trigger with selected template
                        void handleTrigger(t.id)
                      }}
                      onMouseEnter={e => e.currentTarget.style.background = 'var(--bg-hover)'}
                      onMouseLeave={e => e.currentTarget.style.background = 'transparent'}
                    >
                      <div style={{ fontWeight: 600, color: 'var(--text-primary)' }}>
                        {t.name}
                      </div>
                      <div style={{ fontSize: 'var(--font-sm)', color: 'var(--text-tertiary)' }}>
                        {t.description || '无描述'}
                      </div>
                    </div>
                  ))}
                </div>
              )}
              {templates.length === 0 && (
                <div style={{ padding: 'var(--space-md)', color: 'var(--text-tertiary)' }}>
                  暂无可用模板，请在设置中创建
                </div>
              )}
            </>
          )}
        </div>
      )}

      {/* Show selected template info below button */}
      {selectedTemplate && !showDropdown && (
        <div style={{
          fontSize: 'var(--font-sm)',
          color: 'var(--accent)',
          display: 'flex',
          alignItems: 'center',
          gap: 'var(--space-xs)',
        }}>
          <span>✓</span>
          <span>已选择: {selectedTemplate.name}</span>
        </div>
      )}

      {job && (
        <div style={{
          display: 'flex',
          alignItems: 'center',
          gap: 'var(--space-sm)',
          fontSize: 'var(--font-sm)',
          color: 'var(--text-secondary)',
        }}>
          <span style={{
            width: 8,
            height: 8,
            borderRadius: '50%',
            background: statusColor(job.status),
            display: 'inline-block',
            flexShrink: 0,
          }} />
          <span style={{ color: statusColor(job.status) }}>{STATUS_LABEL[job.status]}</span>
          {isRunning && job.progress_pct > 0 && (
            <span style={{ color: 'var(--text-tertiary)' }}>{job.progress_pct}%</span>
          )}
          {job.status === 'failed' && job.error && (
            <span style={{ color: 'var(--status-danger)', maxWidth: 200, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
              — {job.error}
            </span>
          )}
          {isRunning && (
            <span style={{ color: 'var(--text-tertiary)', fontSize: 'var(--font-xs)' }}>
              轮询 {pollCountDisplay}/{POLL_MAX}
            </span>
          )}
          {!isRunning && (
            <Button
              variant="ghost"
              size="sm"
              onClick={() => void handleTrigger()}
            >
              重试
            </Button>
          )}
        </div>
      )}

      {error && (
        <div style={{ color: 'var(--status-danger)', fontSize: 'var(--font-sm)' }}>{error}</div>
      )}
    </div>
  )
}
