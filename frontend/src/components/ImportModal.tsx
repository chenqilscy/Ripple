// ImportModal · 批量导入节点（JSON / CSV）
// 修复：scroll lock + 预览区固定高度分配 + CSS 变量
import React, { useEffect, useState, useRef, useCallback } from 'react'
import { api } from '../api/client'
import { Button } from './ui'

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
          <Button variant="ghost" size="sm" onClick={onClose} aria-label="关闭">✕</Button>
        </div>

        {done ? (
          <div style={{ textAlign: 'center', padding: 'var(--space-xl) 0' }}>
            <div style={{ fontSize: 'var(--font-2xl)', marginBottom: 'var(--space-sm)' }}>✅</div>
            <div style={{ color: 'var(--status-success)', fontSize: 'var(--font-lg)' }}>
              成功导入 {doneCount} 个节点
            </div>
            <Button variant="primary" onClick={onClose} style={{ marginTop: 'var(--space-lg)' }}>
              关闭
            </Button>
          </div>
        ) : (
          <>
            {/* Tab 切换 */}
            <div style={{ display: 'flex', gap: 'var(--space-sm)', alignItems: 'center' }}>
              {(['json', 'csv'] as const).map(t => (
                tab === t
                  ? <Button variant="primary" key={t} onClick={() => { setTab(t); setPreview(null); setParseError('') }}
                      aria-pressed={tab === t}
                      aria-label={`${t.toUpperCase()} 格式`}
                      style={{ padding: 'var(--space-sm) var(--space-lg)' }}
                    >{t.toUpperCase()}</Button>
                  : <Button variant="ghost" size="sm" key={t} onClick={() => { setTab(t); setPreview(null); setParseError('') }}
                      aria-pressed={tab === t}
                      aria-label={`${t.toUpperCase()} 格式`}
                      style={{ padding: 'var(--space-sm) var(--space-lg)' }}
                    >{t.toUpperCase()}</Button>
              ))}
              <Button variant="ghost" size="sm"
                onClick={() => fileRef.current?.click()}
                aria-label="选择文件"
                style={{ marginLeft: 'auto', padding: 'var(--space-sm) var(--space-md)', fontSize: 'var(--font-sm)' }}
              >
                📂 选择文件
              </Button>
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
                <Button variant="ghost" size="sm" onClick={onClose} aria-label="取消导入">取消</Button>
                <Button variant="primary" onClick={handleParse} aria-label="解析预览" disabled={!raw.trim()}>
                  解析预览
                </Button>
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
                  <Button variant="ghost" size="sm"
                    onClick={() => { setPreview(null); setParseError('') }}
                    aria-label="重新编辑"
                  >
                    重新编辑
                  </Button>
                  <Button variant="primary"
                    onClick={handleConfirm}
                    aria-label="确认导入"
                    disabled={loading || preview.length > 100}
                  >
                    {loading ? '导入中…' : `确认导入 ${preview.length} 个节点`}
                  </Button>
                </div>
              </>
            )}
          </>
        )}
      </div>
    </div>
  )
}

