/**
 * ImportTextModal — P20-A 自由文本一键转图谱（Paste-to-Graph）
 *
 * 流程：粘贴文本 → 设置最大节点数 → LLM 解析 → 展示结果 → 回调刷新视图
 */
import React, { useState, useRef, useCallback, useEffect } from 'react'
import { api } from '../api/client'
import type { ImportTextResult } from '../api/types'
import { Button } from './ui'

// 注入 spin keyframe（全局一次）
let spinStyleInjected = false
function ensureSpinStyle() {
  if (spinStyleInjected) return
  spinStyleInjected = true
  const s = document.createElement('style')
  s.textContent = '@keyframes spin { to { transform: rotate(360deg); } }'
  document.head.appendChild(s)
}

interface Props {
  lakeId: string
  onClose: () => void
  onImported?: (result: ImportTextResult) => void
}

const overlay: React.CSSProperties = {
  position: 'fixed', inset: 0,
  background: 'rgba(0,0,0,0.6)',
  display: 'flex', alignItems: 'center', justifyContent: 'center',
  zIndex: 1100,
}
const modal: React.CSSProperties = {
  background: '#0d1526',
  border: '1px solid #1e3050',
  borderRadius: 12,
  padding: '24px 28px',
  width: 580,
  maxHeight: '85vh',
  overflowY: 'auto',
  display: 'flex',
  flexDirection: 'column',
  gap: 16,
  color: '#cdd6f4',
  fontFamily: 'sans-serif',
}
const textareaStyle: React.CSSProperties = {
  width: '100%',
  minHeight: 200,
  background: '#0a0f1e',
  border: '1px solid #1e3050',
  borderRadius: 8,
  color: '#cdd6f4',
  fontFamily: 'monospace',
  fontSize: 13,
  padding: '10px 12px',
  resize: 'vertical',
  boxSizing: 'border-box',
}
const resultCard: React.CSSProperties = {
  background: '#0a0f1e',
  border: '1px solid #1e3050',
  borderRadius: 8,
  padding: '12px 16px',
  fontSize: 13,
}
const resultItem: React.CSSProperties = {
  padding: '6px 0',
  borderBottom: '1px solid #1e3050',
  display: 'flex',
  alignItems: 'center',
  gap: 8,
}
const badge: React.CSSProperties = {
  background: '#1e3050',
  borderRadius: 4,
  padding: '2px 6px',
  fontSize: 11,
  color: '#89b4fa',
  whiteSpace: 'nowrap',
}

const MAX_NODES_MIN = 5
const MAX_NODES_MAX = 50
const MAX_TEXT_CHARS = 4000

export default function ImportTextModal({ lakeId, onClose, onImported }: Props) {
  useEffect(() => { ensureSpinStyle() }, [])
  const [text, setText] = useState('')
  const [maxNodes, setMaxNodes] = useState(20)
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [result, setResult] = useState<ImportTextResult | null>(null)
  const textareaRef = useRef<HTMLTextAreaElement>(null)

  const runeCount = Array.from(text).length
  const overLimit = runeCount > MAX_TEXT_CHARS

  const handleSubmit = useCallback(async () => {
    const trimmed = text.trim()
    if (!trimmed) { setError('请输入文本内容'); return }
    setLoading(true)
    setError(null)
    try {
      const res = await api.importText(lakeId, trimmed, maxNodes)
      setResult(res)
      onImported?.(res)
    } catch (e: unknown) {
      setError((e as { message?: string })?.message ?? '导入失败，请稍后重试')
    } finally {
      setLoading(false)
    }
  }, [text, maxNodes, lakeId, onImported])

  // ESC 关闭
  const handleKeyDown = useCallback((e: React.KeyboardEvent) => {
    if (e.key === 'Escape') onClose()
  }, [onClose])

  return (
    <div style={overlay} onClick={onClose} onKeyDown={handleKeyDown}>
      <div style={modal} onClick={e => e.stopPropagation()}>
        <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
          <h3 style={{ margin: 0, fontSize: 16, color: '#cdd6f4' }}>✨ 文本转图谱</h3>
          <Button variant="ghost" size="sm" onClick={onClose} style={{ padding: '4px 10px' }}>✕</Button>
        </div>

        {!result ? (
          <>
            <p style={{ margin: 0, fontSize: 13, color: '#6c7086' }}>
              将任意文本（笔记、文章、会议记录等）粘贴到下方，AI 将自动提取关键概念和关系，生成知识图谱节点。
            </p>

            <div>
              <label style={{ fontSize: 13, color: '#a6adc8', display: 'block', marginBottom: 6 }}>
                输入文本
                <span style={{
                  marginLeft: 8,
                  color: overLimit ? '#f38ba8' : '#6c7086',
                  fontSize: 12,
                }}>
                  {runeCount} / {MAX_TEXT_CHARS} 字符{overLimit ? '（将截断至 4000 字）' : ''}
                </span>
              </label>
              <textarea
                ref={textareaRef}
                style={{ ...textareaStyle, borderColor: overLimit ? '#f38ba8' : '#1e3050', opacity: loading ? 0.6 : 1 }}
                value={text}
                onChange={e => setText(e.target.value)}
                placeholder="粘贴文本内容（支持中英文，最多 4000 字）..."
                disabled={loading}
                autoFocus
              />
            </div>

            <div>
              <label style={{ fontSize: 13, color: '#a6adc8', display: 'flex', alignItems: 'center', gap: 12 }}>
                最大节点数：<strong style={{ color: '#89b4fa' }}>{maxNodes}</strong>
                <input
                  type="range"
                  min={MAX_NODES_MIN}
                  max={MAX_NODES_MAX}
                  step={5}
                  value={maxNodes}
                  onChange={e => setMaxNodes(Number(e.target.value))}
                  style={{ flex: 1 }}
                />
                <span style={{ fontSize: 11, color: '#6c7086' }}>{MAX_NODES_MIN}–{MAX_NODES_MAX}</span>
              </label>
            </div>

            {error && (
              <p style={{ margin: 0, color: '#f38ba8', fontSize: 13 }}>⚠ {error}</p>
            )}

            <div style={{ display: 'flex', gap: 10, justifyContent: 'flex-end' }}>
              <Button variant="ghost" size="sm" onClick={onClose} disabled={loading}>取消</Button>
            <Button
                variant="primary"
                onClick={handleSubmit}
                disabled={loading || !text.trim()}
              >
                {loading ? (
                  <span style={{ display: 'inline-flex', alignItems: 'center', gap: 6 }}>
                    <span style={{ display: 'inline-block', width: 14, height: 14, border: '2px solid #0d1526', borderTopColor: 'transparent', borderRadius: '50%', animation: 'spin 0.8s linear infinite' }} />
                    解析中…
                  </span>
                ) : '🚀 生成图谱'}
              </Button>
            </div>
          </>
        ) : (
          <>
            <div style={{ display: 'flex', alignItems: 'center', gap: 12 }}>
              <span style={{ fontSize: 20 }}>{result.imported > 0 ? '✅' : '⚠️'}</span>
              <span style={{ fontSize: 14, color: result.imported > 0 ? '#a6e3a1' : '#f9e2af' }}>
                {result.imported > 0
                  ? <>成功导入 <strong>{result.imported}</strong> 个节点、<strong>{result.edges.length}</strong> 条边</>
                  : 'LLM 未提取到有效节点，请尝试换一段更明确的文本'}
              </span>
            </div>

            {result.nodes.length > 0 && (
              <div>
                <p style={{ margin: '0 0 8px', fontSize: 12, color: '#6c7086' }}>新建节点：</p>
                <div style={resultCard}>
                  {result.nodes.slice(0, 10).map((n, i) => (
                    <div key={n.id} style={{ ...resultItem, borderBottom: i < Math.min(result.nodes.length, 10) - 1 ? '1px solid #1e3050' : 'none' }}>
                      <span style={badge}>节点</span>
                      <span style={{ fontSize: 13, flex: 1 }}>{n.content}</span>
                    </div>
                  ))}
                  {result.nodes.length > 10 && (
                    <p style={{ margin: '6px 0 0', fontSize: 12, color: '#6c7086' }}>…还有 {result.nodes.length - 10} 个节点</p>
                  )}
                </div>
              </div>
            )}

            {result.edges.length > 0 && (
              <div>
                <p style={{ margin: '0 0 8px', fontSize: 12, color: '#6c7086' }}>建立关系：</p>
                <div style={resultCard}>
                  {result.edges.slice(0, 5).map((e, i) => (
                    <div key={`${e.source_id}-${e.target_id}`} style={{ ...resultItem, borderBottom: i < Math.min(result.edges.length, 5) - 1 ? '1px solid #1e3050' : 'none' }}>
                      <span style={{ ...badge, color: '#cba6f7' }}>边</span>
                      <span style={{ fontSize: 12, color: '#6c7086', flex: 1 }}>
                        {e.source_id.slice(0, 6)}… → {e.target_id.slice(0, 6)}…
                        <span style={{ marginLeft: 8, color: '#89dceb' }}>[{e.kind}]</span>
                      </span>
                    </div>
                  ))}
                  {result.edges.length > 5 && (
                    <p style={{ margin: '6px 0 0', fontSize: 12, color: '#6c7086' }}>…还有 {result.edges.length - 5} 条边</p>
                  )}
                </div>
              </div>
            )}

            <div style={{ display: 'flex', gap: 10, justifyContent: 'flex-end' }}>
              <Button variant="ghost" size="sm" onClick={() => setResult(null)}>继续导入</Button>
              <Button variant="primary" onClick={onClose}>完成</Button>
            </div>
          </>
        )}
      </div>
    </div>
  )
}
