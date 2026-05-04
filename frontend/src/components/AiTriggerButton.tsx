/**
 * Phase 15-C: AI Trigger button for a node.
 * Sends a trigger request and polls job status until done/failed (60s timeout).
 * 修复：CSS 变量（Deep Ocean Dark 主题）
 */
import { useCallback, useEffect, useRef, useState } from 'react'
import { Button } from './ui'
import { api } from '../api/client'
import type { AiJob, AiJobStatus, ApiError } from '../api/types'

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

const POLL_INTERVAL_MS = 2000
const POLL_MAX = 30 // 60 seconds total

export default function AiTriggerButton({ lakeId, nodeId, promptTemplateId, inputNodeIds, onDone, onFail }: Props) {
  const [job, setJob] = useState<AiJob | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [triggering, setTriggering] = useState(false)
  const pollCount = useRef(0)
  const [pollCountDisplay, setPollCountDisplay] = useState(0)
  const pollTimer = useRef<ReturnType<typeof setTimeout> | null>(null)

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

  const handleTrigger = useCallback(async () => {
    if (triggering || (job && (job.status === 'pending' || job.status === 'processing'))) return
    setError(null)
    setTriggering(true)
    stopPolling()
    pollCount.current = 0
    setPollCountDisplay(0)
    try {
      const newJob = await api.aiTrigger(lakeId, nodeId, {
        prompt_template_id: promptTemplateId,
        input_node_ids: inputNodeIds,
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
        try {
          const existing = await api.aiStatus(lakeId, nodeId)
          setJob(existing)
          setError('该节点已有 AI 任务，已继续跟踪当前任务状态。')
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
  }, [triggering, job, lakeId, nodeId, promptTemplateId, inputNodeIds, poll, stopPolling, onDone, onFail])

  const isRunning = job?.status === 'pending' || job?.status === 'processing'
  const disabled = triggering || isRunning

  function statusColor(status: AiJobStatus): string {
    if (status === 'pending') return 'var(--status-warning)'
    if (status === 'processing') return 'var(--accent)'
    if (status === 'done') return 'var(--status-success)'
    return 'var(--status-danger)'
  }

  return (
    <div style={{ display: 'inline-flex', flexDirection: 'column', gap: 'var(--space-sm)', alignItems: 'flex-start' }}>
      <Button
        variant="primary"
        size="sm"
        onClick={() => void handleTrigger()}
        disabled={disabled}
        title="触发 AI 处理当前节点"
      >
        <span style={{ fontSize: 'var(--font-lg)' }}>✦</span>
        {isRunning ? 'AI 处理中…' : 'AI 触发'}
      </Button>

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
