import { useEffect, useState } from 'react'
import { api } from '../api/client'
import type { APIKeyCreated, APIKeyItem, Organization } from '../api/types'

/** P11-A：API Key 管理面板 */
export default function APIKeyManager() {
  const [keys, setKeys] = useState<APIKeyItem[]>([])
  const [loading, setLoading] = useState(false)
  const [err, setErr] = useState<string | null>(null)
  const [creating, setCreating] = useState(false)
  const [newName, setNewName] = useState('')
  const [orgs, setOrgs] = useState<Organization[]>([])
  const [orgId, setOrgId] = useState('')
  const [newKeyResult, setNewKeyResult] = useState<APIKeyCreated | null>(null)
  const [copied, setCopied] = useState(false)

  async function load() {
    setLoading(true)
    setErr(null)
    try {
      const res = await api.listAPIKeys()
      setKeys(res.keys ?? [])
    } catch (e: any) {
      setErr(e?.message ?? 'load failed')
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => { void load() }, [])
  useEffect(() => {
    api.listOrgs().then(res => setOrgs(res.organizations ?? [])).catch(() => setOrgs([]))
  }, [])

  async function handleCreate() {
    const name = newName.trim()
    if (!name) return
    setCreating(true)
    setErr(null)
    try {
      const created = await api.createAPIKey(name, undefined, orgId || undefined)
      setNewKeyResult(created)
      setNewName('')
      void load()
    } catch (e: any) {
      setErr(e?.message ?? 'create failed')
    } finally {
      setCreating(false)
    }
  }

  async function handleRevoke(id: string) {
    if (!window.confirm('确定撤销此 API Key？撤销后不可恢复。')) return
    try {
      await api.revokeAPIKey(id)
      setKeys(prev => prev.filter(k => k.id !== id))
    } catch (e: any) {
      setErr(e?.message ?? 'revoke failed')
    }
  }

  function handleCopy(text: string) {
    navigator.clipboard.writeText(text).then(() => {
      setCopied(true)
      setTimeout(() => setCopied(false), 2000)
    })
  }

  return (
    <div style={{ padding: '16px', maxWidth: 720 }}>
      <h3 style={{ margin: '0 0 12px', color: '#cdd6f4' }}>API Key 管理</h3>

      {/* 新 key 创建成功弹窗 */}
      {newKeyResult && (
        <div style={{
          background: '#1e3a2f', border: '1px solid #a6e3a1', borderRadius: 8,
          padding: '12px 16px', marginBottom: 16,
        }}>
          <p style={{ margin: '0 0 8px', color: '#a6e3a1', fontWeight: 600 }}>
            ✅ API Key 创建成功 — 请立即复制，关闭后无法再次查看
          </p>
          <code style={{
            display: 'block', background: '#11111b', borderRadius: 4,
            padding: '8px 12px', color: '#f9e2af', wordBreak: 'break-all', marginBottom: 8,
          }}>
            {newKeyResult.raw_key}
          </code>
          <div style={{ display: 'flex', gap: 8 }}>
            <button
              onClick={() => handleCopy(newKeyResult.raw_key)}
              style={btnStyle('#89b4fa')}
            >
              {copied ? '已复制 ✓' : '复制'}
            </button>
            <button onClick={() => setNewKeyResult(null)} style={btnStyle('#6c7086')}>
              关闭
            </button>
          </div>
        </div>
      )}

      {/* 创建表单 */}
      <div style={{ display: 'flex', gap: 8, marginBottom: 16 }}>
        <input
          value={newName}
          onChange={e => setNewName(e.target.value)}
          onKeyDown={e => e.key === 'Enter' && void handleCreate()}
          placeholder="Key 名称（如 my-ci-key）"
          disabled={creating}
          style={inputStyle}
        />
        <select value={orgId} onChange={e => setOrgId(e.target.value)} disabled={creating} style={selectStyle}>
          <option value="">个人 Key</option>
          {orgs.map(org => <option key={org.id} value={org.id}>{org.name}</option>)}
        </select>
        <button onClick={handleCreate} disabled={creating || !newName.trim()} style={btnStyle('#a6e3a1')}>
          {creating ? '创建中…' : '+ 创建'}
        </button>
      </div>

      {err && <p style={{ color: '#f38ba8', margin: '0 0 12px' }}>⚠ {err}</p>}

      {/* Key 列表 */}
      {loading ? (
        <p style={{ color: '#6c7086' }}>加载中…</p>
      ) : keys.length === 0 ? (
        <p style={{ color: '#6c7086' }}>暂无 API Key</p>
      ) : (
        <table style={{ width: '100%', borderCollapse: 'collapse', fontSize: 13 }}>
          <thead>
            <tr style={{ color: '#6c7086', textAlign: 'left' }}>
              <th style={thStyle}>名称</th>
              <th style={thStyle}>前缀</th>
              <th style={thStyle}>组织</th>
              <th style={thStyle}>权限域</th>
              <th style={thStyle}>最后使用</th>
              <th style={thStyle}>创建时间</th>
              <th style={thStyle}></th>
            </tr>
          </thead>
          <tbody>
            {keys.map(k => (
              <tr key={k.id} style={{ borderBottom: '1px solid #313244' }}>
                <td style={tdStyle}>{k.name}</td>
                <td style={{ ...tdStyle, fontFamily: 'monospace', color: '#89dceb' }}>{k.key_prefix}</td>
                <td style={{ ...tdStyle, color: '#6c7086' }}>{k.org_id ? shortID(k.org_id) : '个人'}</td>
                <td style={tdStyle}>{k.scopes.join(', ')}</td>
                <td style={{ ...tdStyle, color: '#6c7086' }}>{k.last_used_at ? fmtDate(k.last_used_at) : '—'}</td>
                <td style={{ ...tdStyle, color: '#6c7086' }}>{fmtDate(k.created_at)}</td>
                <td style={tdStyle}>
                  <button onClick={() => handleRevoke(k.id)} style={btnStyle('#f38ba8', true)}>
                    撤销
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
  return `${s.slice(0, 8)}…`
}

function btnStyle(color: string, small = false): React.CSSProperties {
  return {
    background: 'transparent', border: `1px solid ${color}`, color,
    borderRadius: 4, padding: small ? '2px 8px' : '5px 12px',
    cursor: 'pointer', fontSize: small ? 12 : 13,
  }
}

const inputStyle: React.CSSProperties = {
  flex: 1, background: '#1e1e2e', border: '1px solid #45475a', borderRadius: 4,
  color: '#cdd6f4', padding: '5px 10px', fontSize: 13,
}

const selectStyle: React.CSSProperties = {
  background: '#1e1e2e', border: '1px solid #45475a', borderRadius: 4,
  color: '#cdd6f4', padding: '5px 10px', fontSize: 13, minWidth: 140,
}

const thStyle: React.CSSProperties = {
  padding: '6px 8px', fontWeight: 500, borderBottom: '1px solid #313244',
}

const tdStyle: React.CSSProperties = {
  padding: '8px 8px', color: '#cdd6f4',
}
