import { useEffect, useState } from 'react'
import { api } from '../api/client'
import type { PlatformAdmin, PlatformAdminRole } from '../api/types'

export default function PlatformAdminManager() {
  const [admins, setAdmins] = useState<PlatformAdmin[]>([])
  const [loading, setLoading] = useState(false)
  const [err, setErr] = useState<string | null>(null)
  const [forbidden, setForbidden] = useState(false)
  const [saving, setSaving] = useState(false)
  const [deletingId, setDeletingId] = useState<string | null>(null)
  const [target, setTarget] = useState('')
  const [role, setRole] = useState<PlatformAdminRole>('ADMIN')
  const [note, setNote] = useState('')

  async function load() {
    setLoading(true)
    setErr(null)
    try {
      const res = await api.listPlatformAdmins()
      setAdmins((res.admins ?? []).slice().sort((left, right) => new Date(right.created_at).getTime() - new Date(left.created_at).getTime()))
      setForbidden(false)
    } catch (e: any) {
      if (e?.status === 403) {
        setForbidden(true)
        setAdmins([])
        setErr('仅平台 OWNER 可管理平台管理员')
      } else {
        setErr(e?.message ?? 'load failed')
      }
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => { void load() }, [])

  async function handleGrant() {
    const value = target.trim()
    if (!value || forbidden || saving || deletingId) return
    if (role === 'OWNER' && !window.confirm(`确定授予 ${value} 平台 OWNER？OWNER 可继续授权或撤销平台管理员。`)) return
    setSaving(true)
    setErr(null)
    try {
      const input = platformAdminGrantInput(value, role, note)
      await api.grantPlatformAdmin(input)
      setTarget('')
      setNote('')
      await load()
    } catch (e: any) {
      setErr(e?.message ?? 'grant failed')
    } finally {
      setSaving(false)
    }
  }

  async function handleRevoke(admin: PlatformAdmin) {
    if (forbidden || saving || deletingId) return
    const message = platformAdminRevokeMessage(admin)
    if (!window.confirm(message)) return
    setDeletingId(admin.user_id)
    setErr(null)
    try {
      await api.revokePlatformAdmin(admin.user_id)
      setAdmins(prev => prev.filter(item => item.user_id !== admin.user_id))
    } catch (e: any) {
      setErr(e?.message ?? 'revoke failed')
    } finally {
      setDeletingId(null)
    }
  }

  return (
    <div style={{ padding: 16, maxWidth: 820, minWidth: 420, flex: '1 1 520px' }}>
      <h3 style={{ margin: '0 0 12px', color: '#cdd6f4' }}>平台管理员 RBAC</h3>
      <p style={{ margin: '0 0 12px', color: '#6c7086', fontSize: 12, lineHeight: 1.5 }}>
        仅 OWNER 可授权或撤销平台管理员。环境变量白名单仍作为 bootstrap OWNER 兜底；API Key 不可调用此面板。
      </p>

      <div style={{ display: 'flex', gap: 8, marginBottom: 16, flexWrap: 'wrap' }}>
        <input
          value={target}
          onChange={e => setTarget(e.target.value)}
          onKeyDown={e => e.key === 'Enter' && void handleGrant()}
          placeholder="用户 ID 或邮箱"
          disabled={saving || !!deletingId || forbidden}
          style={{ ...inputStyle, minWidth: 240, flex: '1 1 240px' }}
        />
        <select value={role} onChange={e => setRole(e.target.value as PlatformAdminRole)} disabled={saving || !!deletingId || forbidden} style={selectStyle}>
          <option value="ADMIN">ADMIN</option>
          <option value="OWNER">OWNER</option>
        </select>
        <input
          value={note}
          onChange={e => setNote(e.target.value)}
          onKeyDown={e => e.key === 'Enter' && void handleGrant()}
          placeholder="授权备注"
          disabled={saving || !!deletingId || forbidden}
          style={{ ...inputStyle, minWidth: 180, flex: '1 1 180px' }}
        />
        <button onClick={() => void handleGrant()} disabled={saving || !!deletingId || forbidden || !target.trim()} style={btnStyle('#a6e3a1')}>
          {saving ? '授权中…' : '+ 授权'}
        </button>
      </div>

      {err && <p style={{ color: forbidden ? '#f9e2af' : '#f38ba8', margin: '0 0 12px' }}>⚠ {err}</p>}

      {forbidden ? (
        <p style={{ color: '#6c7086', margin: 0 }}>当前账号不是平台 OWNER，只能查看此说明。</p>
      ) : loading ? (
        <p style={{ color: '#6c7086' }}>加载中…</p>
      ) : admins.length === 0 ? (
        <p style={{ color: '#6c7086' }}>暂无数据库平台管理员；当前可能仅依赖环境变量 bootstrap OWNER。</p>
      ) : (
        <table style={{ width: '100%', borderCollapse: 'collapse', fontSize: 13 }}>
          <thead>
            <tr style={{ color: '#6c7086', textAlign: 'left' }}>
              <th style={thStyle}>用户</th>
              <th style={thStyle}>角色</th>
              <th style={thStyle}>备注</th>
              <th style={thStyle}>授权人</th>
              <th style={thStyle}>授权时间</th>
              <th style={thStyle}></th>
            </tr>
          </thead>
          <tbody>
            {admins.map(admin => (
              <tr key={admin.user_id} style={{ borderBottom: '1px solid #313244' }}>
                <td style={{ ...tdStyle, color: '#89dceb' }} title={admin.user_id}>{admin.email || shortID(admin.user_id)}</td>
                <td style={{ ...tdStyle, color: admin.role === 'OWNER' ? '#f9e2af' : '#a6e3a1' }}>{admin.role}</td>
                <td style={tdStyle}>{admin.note || '—'}</td>
                <td style={{ ...tdStyle, color: '#6c7086' }}>{shortID(admin.created_by)}</td>
                <td style={{ ...tdStyle, color: '#6c7086' }}>{fmtDate(admin.created_at)}</td>
                <td style={tdStyle}>
                  <button onClick={() => void handleRevoke(admin)} disabled={saving || !!deletingId} style={btnStyle('#f38ba8', true)}>
                    {deletingId === admin.user_id ? '撤销中…' : '撤销'}
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

export function platformAdminGrantInput(value: string, role: PlatformAdminRole, note: string) {
  const normalized = value.trim()
  return normalized.includes('@')
    ? { email: normalized.toLowerCase(), role, note: note.trim() }
    : { user_id: normalized, role, note: note.trim() }
}

export function platformAdminRevokeMessage(admin: Pick<PlatformAdmin, 'email' | 'user_id' | 'role'>) {
  const label = admin.email || admin.user_id
  return admin.role === 'OWNER'
    ? `确定撤销平台 OWNER ${label}？这会移除其平台管理员授权能力。`
    : `确定撤销平台管理员 ${label}？`
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

const selectStyle: React.CSSProperties = {
  background: '#1e1e2e', border: '1px solid #45475a', borderRadius: 4,
  color: '#cdd6f4', padding: '5px 10px', fontSize: 13, minWidth: 120,
}

const thStyle: React.CSSProperties = {
  padding: '6px 8px', fontWeight: 500, borderBottom: '1px solid #313244',
}

const tdStyle: React.CSSProperties = {
  padding: '8px 8px', color: '#cdd6f4',
}
