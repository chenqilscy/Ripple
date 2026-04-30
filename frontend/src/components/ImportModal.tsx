// ImportModal · 批量导入节点（JSON / CSV）
// 修复：scroll lock + 预览区固定高度分配 + CSS 变量
import React, { useState, useRef, useCallback } from 'react'
import { api } from '../api/client'

interface ImportItem {
  content: string
  type: string
}

interface Props {
  lakeId: string
  lakeName?: string
  onClose: () => void
  onImported?: (count: number) => void
}

function parseCsv(text: string): string[][] {
  const rows: string[][] = []
  const lines = text.split(/\r?\n/)
  for (const line of lines) {
    if (!line.trim()) continue
    const cols: string[] = []
    let cur = ''
    let inQ = false
    for (let i = 0; i < line.length; i++) {
      const c = line[i]
      if (inQ) {
        if (c === '"' && line[i + 1] === '"') { cur += '"'; i++ }
        else if (c === '"') { inQ = false }
        else { cur += c }
      } else {
        if (c === '"') { inQ = true }
        else if (c === ',') { cols.push(cur); cur = '' }
        else { cur += c }
      }
    }
    cols.push(cur)
    rows.push(cols)
  }
  return rows
}

function csvToItems(text: string): ImportItem[] {
  const rows = parseCsv(text)
  if (rows.length === 0) return []
  const header = rows[0].map(h => h.trim().toLowerCase())
  const contentCol = header.findIndex(h => h === 'content' || h === 'text' || h === '内容')
  const typeCol = header.findIndex(h => h === 'type' || h === 'kind' || h === '类型')
  const dataRows = contentCol >= 0 ? rows.slice(1) : rows
  return dataRows.map(row => ({
    content: contentCol >= 0 ? (row[contentCol] ?? '').trim() : row.join(' ').trim(),
    type: typeCol >= 0 ? (row[typeCol] ?? 'TEXT').trim().toUpperCase() : 'TEXT',
  })).filter(it => it.content)
}

export default function ImportModal({ lakeId, lakeName, onClose, onImported }: Props) {
  const [tab, setTab] = useState<'json' | 'csv'>('json')
  const [raw, setRaw] = useState('')
  const [preview, setPreview] = useState<ImportItem[] | null>(null)
  const [parseError, setParseError] = useState('')
  const [loading, setLoading] = useState(false)
  const [done, setDone] = useState(false)
  const [doneCount, setDoneCount] = useState(0)
  const fileRef = useRef<HTMLInputElement>(null)

  // Scroll lock
  React.useEffect(() => {
    const prev = document.body.style.overflow
    document.body.style.overflow = 'hidden'
    return () => { document.body.style.overflow = prev }
  }, [])

  // Escape to close
  React.useEffect(() => {
    const handler = (e: KeyboardEvent) => { if (e.key === 'Escape') onClose() }
    window.addEventListener('keydown', handler)
    return () => window.removeEventListener('keydown', handler)
  }, [onClose])

  const handleParse = useCallback(() => {
    setParseError('')
    try {
      if (tab === 'json') {
        const parsed = JSON.parse(raw)
        if (!Array.isArray(parsed)) throw new Error('JSON 必须是数组')
        const items: ImportItem[] = parsed.map((it: unknown, i) => {
          if (typeof it !== 'object' || it === null) throw new Error(`第 ${i + 1} 项不是对象`)
          const obj = it as Record<string, unknown>
          const content = String(obj.content ?? obj.text ?? '')
          if (!content.trim()) return null
          return { content: content.trim(), type: String(obj.type ?? 'TEXT').toUpperCase() }
        }).filter(Boolean) as ImportItem[]
        if (items.length === 0) throw new Error('解析出 0 个节点')
        setPreview(items)
      } else {
        const items = csvToItems(raw)
        if (items.length === 0) throw new Error('解析出 0 个节点')
        setPreview(items)
      }
    } catch (e) {
      setParseError((e as Error).message)
      setPreview(null)
    }
  }, [tab, raw])

  const handleFileUpload = useCallback((e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0]
    if (!file) return
    const reader = new FileReader()
    reader.onload = () => setRaw(reader.result as string)
    reader.readAsText(file, 'utf-8')
    e.target.value = ''
  }, [])

  const handleConfirm = useCallback(async () => {
    if (!preview || preview.length === 0) return
    setLoading(true)
    try {
      const result = await api.batchImportNodes(lakeId, preview)
      setDone(true)
      setDoneCount(result.created)
      onImported?.(result.created)
    } catch (e) {
      setParseError((e as Error).message)
    } finally {
      setLoading(false)
    }
  }, [lakeId, preview, onImported])

  return (
    <div
      style={{
        position: 'fixed', inset: 0,
        background: 'var(--bg-overlay)',
        display: 'flex', alignItems: 'center', justifyContent: 'center',
        zIndex: 1100,
      }}
      onClick={e => { if (e.target === e.currentTarget) onClose() }}
    >
      <div
        role="dialog"
        aria-modal="true"
        aria-label="批量导入节点"
        style={{
          background: 'var(--bg-primary)',
          border: '1px solid var(--border)',
          borderRadius: 'var(--radius-xl)',
          padding: 'var(--space-xl) var(--space-2xl)',
          width: 560, maxWidth: '92vw',
          maxHeight: '85vh',
          display: 'flex', flexDirection: 'column', gap: 'var(--space-lg)',
          color: 'var(--text-primary)',
          boxShadow: 'var(--shadow-overlay)',
        }}
      >
        <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
          <strong style={{ fontSize: 'var(--font-xl)' }}>
            批量导入节点{lakeName ? ` · ${lakeName}` : ''}
          </strong>
          <button onClick={onClose} aria-label="关闭" style={btnSecondary}>✕</button>
        </div>

        {done ? (
          <div style={{ textAlign: 'center', padding: 'var(--space-xl) 0' }}>
            <div style={{ fontSize: 'var(--font-2xl)', marginBottom: 'var(--space-sm)' }}>✅</div>
            <div style={{ color: 'var(--status-success)', fontSize: 'var(--font-lg)' }}>
              成功导入 {doneCount} 个节点
            </div>
            <button onClick={onClose} style={{ ...btnPrimary, marginTop: 'var(--space-lg)' }}>
              关闭
            </button>
          </div>
        ) : (
          <>
            {/* Tab 切换 */}
            <div style={{ display: 'flex', gap: 'var(--space-sm)', alignItems: 'center' }}>
              {(['json', 'csv'] as const).map(t => (
                <button
                  key={t}
                  onClick={() => { setTab(t); setPreview(null); setParseError('') }}
                  style={tab === t ? { ...btnPrimary, padding: 'var(--space-sm) var(--space-lg)' } : { ...btnSecondary, padding: 'var(--space-sm) var(--space-lg)' }}
                >
                  {t.toUpperCase()}
                </button>
              ))}
              <button
                onClick={() => fileRef.current?.click()}
                aria-label="选择文件"
                style={{ ...btnSecondary, marginLeft: 'auto', padding: 'var(--space-sm) var(--space-md)', fontSize: 'var(--font-sm)' }}
              >
                📂 选择文件
              </button>
              <input ref={fileRef} type="file" accept=".json,.csv,.txt" style={{ display: 'none' }} onChange={handleFileUpload} />
            </div>

            <div style={{ fontSize: 'var(--font-sm)', color: 'var(--text-tertiary)' }}>
              {tab === 'json'
                ? '粘贴 JSON 数组，每项须包含 content 字段，type 字段可选（默认 TEXT）。最多 100 个节点。'
                : '粘贴 CSV 数据，第一行为列标题（content/text, type）。'}
            </div>

            <textarea
              style={{
                width: '100%', minHeight: 180, maxHeight: 240,
                background: 'var(--bg-input)',
                border: '1px solid var(--border-input)',
                borderRadius: 'var(--radius-md)',
                color: 'var(--text-primary)',
                fontFamily: 'var(--font-mono)', fontSize: 'var(--font-md)',
                padding: 'var(--space-md)',
                resize: 'vertical', boxSizing: 'border-box',
              }}
              placeholder={tab === 'json'
                ? '[{"content": "第一个节点", "type": "TEXT"}, ...]'
                : 'content,type\n第一个节点,TEXT\n第二个节点,TEXT'}
              value={raw}
              onChange={e => { setRaw(e.target.value); setPreview(null); setParseError('') }}
              spellCheck={false}
              aria-label="导入内容"
            />

            {parseError && (
              <div style={{ color: 'var(--status-danger)', fontSize: 'var(--font-md)' }}>
                ⚠ {parseError}
              </div>
            )}

            {!preview && (
              <div style={{ display: 'flex', justifyContent: 'flex-end', gap: 'var(--space-sm)' }}>
                <button onClick={onClose} style={btnSecondary}>取消</button>
                <button onClick={handleParse} style={btnPrimary} disabled={!raw.trim()}>
                  解析预览
                </button>
              </div>
            )}

            {preview && (
              <>
                <div style={{ fontSize: 'var(--font-md)', color: 'var(--status-success)' }}>
                  解析成功 {preview.length} 个节点
                  {preview.length > 100 && (
                    <span style={{ color: 'var(--status-danger)' }}> · 超出 100 上限，将拒绝提交</span>
                  )}
                </div>
                {/* 预览区固定高度，内部滚动 */}
                <div style={{
                  display: 'flex', flexDirection: 'column', gap: 'var(--space-sm)',
                  maxHeight: 220, overflowY: 'auto',
                  flex: '0 0 auto',
                }}>
                  {preview.slice(0, 20).map((it, i) => (
                    <div
                      key={i}
                      style={{
                        background: 'var(--bg-input)',
                        border: '1px solid var(--border-input)',
                        borderRadius: 'var(--radius-md)',
                        padding: 'var(--space-sm) var(--space-md)',
                        fontSize: 'var(--font-md)',
                        display: 'flex', gap: 'var(--space-sm)', alignItems: 'center',
                        minHeight: 40,
                      }}
                    >
                      <span style={{
                        background: 'var(--accent-subtle)',
                        borderRadius: 'var(--radius-sm)',
                        padding: '2px 6px',
                        fontSize: 'var(--font-sm)', color: 'var(--accent)',
                        flexShrink: 0,
                      }}>{it.type}</span>
                      <span style={{ flex: 1, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
                        {it.content}
                      </span>
                    </div>
                  ))}
                  {preview.length > 20 && (
                    <div style={{ textAlign: 'center', color: 'var(--text-tertiary)', fontSize: 'var(--font-sm)' }}>
                      … 还有 {preview.length - 20} 个节点未显示
                    </div>
                  )}
                </div>
                <div style={{ display: 'flex', justifyContent: 'flex-end', gap: 'var(--space-sm)' }}>
                  <button
                    onClick={() => { setPreview(null); setParseError('') }}
                    style={btnSecondary}
                  >
                    重新编辑
                  </button>
                  <button
                    onClick={handleConfirm}
                    style={btnPrimary}
                    disabled={loading || preview.length > 100}
                  >
                    {loading ? '导入中…' : `确认导入 ${preview.length} 个节点`}
                  </button>
                </div>
              </>
            )}
          </>
        )}
      </div>
    </div>
  )
}

const btnPrimary: React.CSSProperties = {
  background: 'var(--accent)', color: 'var(--text-inverse)',
  border: 'none', borderRadius: 'var(--radius-md)',
  padding: 'var(--space-sm) var(--space-xl)', cursor: 'pointer', fontWeight: 600, fontSize: 'var(--font-base)',
}
const btnSecondary: React.CSSProperties = {
  background: 'transparent', color: 'var(--text-tertiary)',
  border: '1px solid var(--border)',
  borderRadius: 'var(--radius-md)', padding: 'var(--space-sm) var(--space-lg)',
  cursor: 'pointer', fontSize: 'var(--font-base)',
}