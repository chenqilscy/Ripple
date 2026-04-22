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

// 简单 CSV 解析：支持带引号字段、\n 分隔行
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
  // 检测 header 行
  const header = rows[0].map(h => h.trim().toLowerCase())
  const contentCol = header.findIndex(h => h === 'content' || h === 'text' || h === '内容')
  const typeCol = header.findIndex(h => h === 'type' || h === 'kind' || h === '类型')
  const dataRows = contentCol >= 0 ? rows.slice(1) : rows
  return dataRows.map(row => ({
    content: contentCol >= 0 ? (row[contentCol] ?? '').trim() : row.join(' ').trim(),
    type: typeCol >= 0 ? (row[typeCol] ?? 'TEXT').trim().toUpperCase() : 'TEXT',
  })).filter(it => it.content)
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
  width: 560,
  maxHeight: '80vh',
  overflowY: 'auto',
  display: 'flex',
  flexDirection: 'column',
  gap: 16,
  color: '#cdd6f4',
  fontFamily: 'sans-serif',
}
const textarea: React.CSSProperties = {
  width: '100%',
  minHeight: 180,
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
const btnPrimary: React.CSSProperties = {
  background: '#89b4fa',
  color: '#0d1526',
  border: 'none',
  borderRadius: 8,
  padding: '8px 20px',
  cursor: 'pointer',
  fontWeight: 700,
}
const btnSecondary: React.CSSProperties = {
  background: 'transparent',
  color: '#6c7086',
  border: '1px solid #313244',
  borderRadius: 8,
  padding: '8px 20px',
  cursor: 'pointer',
}
const previewRow: React.CSSProperties = {
  background: '#0a0f1e',
  border: '1px solid #1e3050',
  borderRadius: 6,
  padding: '8px 12px',
  fontSize: 13,
  display: 'flex',
  gap: 8,
  alignItems: 'flex-start',
}
const badge: React.CSSProperties = {
  background: '#1e3050',
  borderRadius: 4,
  padding: '2px 6px',
  fontSize: 11,
  color: '#89b4fa',
  flexShrink: 0,
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

  // Escape to close
  React.useEffect(() => {
    const handler = (e: KeyboardEvent) => { if (e.key === 'Escape') onClose() }
    window.addEventListener('keydown', handler)
    return () => window.removeEventListener('keydown', handler)
  }, [onClose])

  return (
    <div style={overlay} onClick={e => { if (e.target === e.currentTarget) onClose() }}>
      <div style={modal}>
        <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
          <strong style={{ fontSize: 16 }}>
            批量导入节点{lakeName ? ` · ${lakeName}` : ''}
          </strong>
          <button onClick={onClose} style={{ ...btnSecondary, padding: '4px 10px' }}>✕</button>
        </div>

        {done ? (
          <div style={{ textAlign: 'center', padding: '24px 0' }}>
            <div style={{ fontSize: 32, marginBottom: 8 }}>✅</div>
            <div style={{ color: '#a6e3a1', fontSize: 15 }}>成功导入 {doneCount} 个节点</div>
            <button onClick={onClose} style={{ ...btnPrimary, marginTop: 16 }}>关闭</button>
          </div>
        ) : (
          <>
            {/* Tab 切换 */}
            <div style={{ display: 'flex', gap: 8 }}>
              {(['json', 'csv'] as const).map(t => (
                <button
                  key={t}
                  onClick={() => { setTab(t); setPreview(null); setParseError('') }}
                  style={tab === t ? { ...btnPrimary, padding: '6px 16px' } : { ...btnSecondary, padding: '6px 16px' }}
                >
                  {t.toUpperCase()}
                </button>
              ))}
              <button
                onClick={() => fileRef.current?.click()}
                style={{ ...btnSecondary, marginLeft: 'auto', padding: '6px 14px', fontSize: 12 }}
                title="选择文件（.json/.csv）"
              >
                📂 选择文件
              </button>
              <input ref={fileRef} type="file" accept=".json,.csv,.txt" style={{ display: 'none' }} onChange={handleFileUpload} />
            </div>

            <div style={{ fontSize: 12, color: '#6c7086' }}>
              {tab === 'json'
                ? '粘贴 JSON 数组，每项须包含 content 字段，type 字段可选（默认 TEXT）。最多 100 个节点。'
                : '粘贴 CSV 数据，第一行为列标题（content/text, type）。'}
            </div>

            <textarea
              style={textarea}
              placeholder={tab === 'json'
                ? '[{"content": "第一个节点", "type": "TEXT"}, ...]'
                : 'content,type\n第一个节点,TEXT\n第二个节点,TEXT'}
              value={raw}
              onChange={e => { setRaw(e.target.value); setPreview(null); setParseError('') }}
              spellCheck={false}
            />

            {parseError && (
              <div style={{ color: '#f38ba8', fontSize: 13 }}>⚠ {parseError}</div>
            )}

            {!preview && (
              <div style={{ display: 'flex', justifyContent: 'flex-end', gap: 8 }}>
                <button onClick={onClose} style={btnSecondary}>取消</button>
                <button onClick={handleParse} style={btnPrimary} disabled={!raw.trim()}>
                  解析预览
                </button>
              </div>
            )}

            {preview && (
              <>
                <div style={{ fontSize: 13, color: '#a6e3a1' }}>
                  解析成功 {preview.length} 个节点（共 {preview.length} 项，空内容已跳过）
                  {preview.length > 100 && (
                    <span style={{ color: '#f38ba8' }}> · 超出 100 上限，将拒绝提交</span>
                  )}
                </div>
                <div style={{ display: 'flex', flexDirection: 'column', gap: 6, maxHeight: 240, overflowY: 'auto' }}>
                  {preview.slice(0, 20).map((it, i) => (
                    <div key={i} style={previewRow}>
                      <span style={badge}>{it.type}</span>
                      <span style={{ flex: 1, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
                        {it.content}
                      </span>
                    </div>
                  ))}
                  {preview.length > 20 && (
                    <div style={{ textAlign: 'center', color: '#6c7086', fontSize: 12 }}>
                      ... 还有 {preview.length - 20} 个节点未显示
                    </div>
                  )}
                </div>
                <div style={{ display: 'flex', justifyContent: 'flex-end', gap: 8 }}>
                  <button onClick={() => { setPreview(null); setParseError('') }} style={btnSecondary}>
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
