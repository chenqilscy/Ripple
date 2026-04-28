import { useEffect, useState } from 'react'
import { api } from '../api/client'
import type { GraylistEntry } from '../api/types'

export default function GraylistManager() {
  const [entries, setEntries] = useState<GraylistEntry[]>([])
  const [loading, setLoading] = useState(false)
  const [err, setErr] = useState<string | null>(null)
  const [forbidden, setForbidden] = useState(false)
  const [saving, setSaving] = useState(false)
  const [deletingId, setDeletingId] = useState<string | null>(null)
  const [email, setEmail] = useState('')
  const [note, setNote] = useState('')

  async function load() {
    setLoading(true)
    setErr(null)
    try {
      const res = await api.listGraylist()
      setEntries((res.entries ?? []).slice().sort((left, right) => new Date(right.created_at).getTime() - new Date(left.created_at).getTime()))
      setForbidden(false)
    } catch (e: any) {
      if (e?.status === 403) {
        setForbidden(true)
        setEntries([])
        setErr('仅平台管理员可管理灰度名单')
      } else {
        setErr(e?.message ?? 'load failed')
      }
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => { void load() }, [])

  async function handleSave() {
    const normalized = email.trim().toLowerCase()
    if (!normalized || forbidden) return
    setSaving(true)
    setErr(null)
    try {
      await api.upsertGraylist(normalized, note.trim())
      setEmail('')
      setNote('')
      await load()
    } catch (e: any) {
      setErr(e?.message ?? 'save failed')
    } finally {
      setSaving(false)
    }
  }

  async function handleDelete(entry: GraylistEntry) {
    if (forbidden) return
    if (!window.confirm(`确定移除灰度邮箱 ${entry.email}？`)) return
    setDeletingId(entry.id)
    setErr(null)
    try {
      await api.deleteGraylist(entry.id)
      setEntries(prev => prev.filter(item => item.id !== entry.id))
    } catch (e: any) {
      setErr(e?.message ?? 'delete failed')
    } finally {
      setDeletingId(null)
    }
  }

  return (
    <div style={{ padding: 16, maxWidth: 760, minWidth: 360, flex: '1 1 420px' }}>
      <h3 style={{ margin: '0 0 12px', color: '#cdd6f4' }}>灰度名单</h3>
      <p style={{ margin: '0 0 12px', color: '#6c7086', fontSize: 12, lineHeight: 1.5 }}>
        仅平台管理员可编辑。只有当后端开启注册灰度开关时，这里的邮箱才允许注册。
      </p>

      <div style={{ display: 'flex', gap: 8, marginBottom: 16, flexWrap: 'wrap' }}>
        <input
          value={email}
          onChange={e => setEmail(e.target.value)}
          onKeyDown={e => e.key === 'Enter' && void handleSave()}
          placeholder="允许注册的邮箱"
          disabled={saving || forbidden}
          style={{ ...inputStyle, minWidth: 220, flex: '1 1 220px' }}
        />
        <input
          value={note}
          onChange={e => setNote(e.target.value)}
          onKeyDown={e => e.key === 'Enter' && void handleSave()}
          placeholder="备注（可选）"
          disabled={saving || forbidden}
          style={{ ...inputStyle, minWidth: 180, flex: '1 1 180px' }}
        />
        <button
          onClick={() => void handleSave()}
          disabled={saving || forbidden || !email.trim()}
          style={btnStyle('#f9e2af')}
        >
          {saving ? '保存中…' : '+ 添加 / 更新'}
        </button>
      </div>

      {err && <p style={{ color: forbidden ? '#f9e2af' : '#f38ba8', margin: '0 0 12px' }}>⚠ {err}</p>}

      {forbidden ? (
        <p style={{ color: '#6c7086', margin: 0 }}>当前账号不是平台管理员，只能查看此说明。</p>
      ) : loading ? (
        <p style={{ color: '#6c7086' }}>加载中…</p>
      ) : entries.length === 0 ? (
        <p style={{ color: '#6c7086' }}>暂无灰度邮箱，开启灰度开关后将拒绝所有未列入邮箱的注册。</p>
      ) : (
        <table style={{ width: '100%', borderCollapse: 'collapse', fontSize: 13 }}>
          <thead>
            <tr style={{ color: '#6c7086', textAlign: 'left' }}>
              <th style={thStyle}>邮箱</th>
              <th style={thStyle}>备注</th>
              <th style={thStyle}>创建人</th>
              <th style={thStyle}>创建时间</th>
              <th style={thStyle}></th>
            </tr>
          </thead>
          <tbody>
            {entries.map(entry => (
              <tr key={entry.id} style={{ borderBottom: '1px solid #313244' }}>
                <td style={{ ...tdStyle, color: '#89dceb' }}>{entry.email}</td>
                <td style={tdStyle}>{entry.note || '—'}</td>
                <td style={{ ...tdStyle, color: '#6c7086' }}>{shortID(entry.created_by)}</td>
                <td style={{ ...tdStyle, color: '#6c7086' }}>{fmtDate(entry.created_at)}</td>
                <td style={tdStyle}>
                  <button
                    onClick={() => void handleDelete(entry)}
                    disabled={deletingId === entry.id}
                    style={btnStyle('#f38ba8', true)}
                  >
                    {deletingId === entry.id ? '移除中…' : '移除'}
                  </button>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      )}
    </div>
  )
}

function fmtDate(s: string) {
  return new Date(s).toLocaleString('zh-CN', { month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit' })
}

function shortID(s: string) {
  return s ? `${s.slice(0, 8)}…` : '—'
}

function btnStyle(color: string, small = false): React.CSSProperties {
  return {
    background: 'transparent', border: `1px solid ${color}`, color,
    borderRadius: 4, padding: small ? '2px 8px' : '5px 12px',
    cursor: 'pointer', fontSize: small ? 12 : 13,
  }
}

const inputStyle: React.CSSProperties = {
  background: '#1e1e2e', border: '1px solid #45475a', borderRadius: 4,
  color: '#cdd6f4', padding: '5px 10px', fontSize: 13,
}

const thStyle: React.CSSProperties = {
  padding: '6px 8px', fontWeight: 500, borderBottom: '1px solid #313244',
}

const tdStyle: React.CSSProperties = {
  padding: '8px 8px', color: '#cdd6f4',
}