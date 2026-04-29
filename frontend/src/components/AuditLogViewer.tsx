/**
 * P11-B：审计日志浏览器
 * 修复：scroll lock + CSS 变量 + table min-height
 */
import { useEffect, useState } from 'react'
import { api, type AuditLogItem } from '../api/client'

interface Props {
  /** 预填资源类型（如 "node"）*/
  defaultResourceType?: string
  /** 预填资源 ID */
  defaultResourceId?: string
}

export default function AuditLogViewer({ defaultResourceType = '', defaultResourceId = '' }: Props) {
  const [resourceType, setResourceType] = useState(defaultResourceType)
  const [resourceId, setResourceId] = useState(defaultResourceId)
  const [limit, setLimit] = useState(50)
  const [logs, setLogs] = useState<AuditLogItem[]>([])
  const [total, setTotal] = useState(0)
  const [loading, setLoading] = useState(false)
  const [err, setErr] = useState<string | null>(null)
  const [queried, setQueried] = useState(false)

  async function handleQuery() {
    const rt = resourceType.trim()
    const rid = resourceId.trim()
    if (!rt || !rid) return
    setLoading(true)
    setErr(null)
    try {
      const res = await api.listAuditLogs(rt, rid, limit)
      setLogs(res.logs ?? [])
      setTotal(res.total ?? 0)
      setQueried(true)
    } catch (e: any) {
      setErr(e?.message ?? '查询失败')
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    const rt = defaultResourceType.trim()
    const rid = defaultResourceId.trim()
    setResourceType(defaultResourceType)
    setResourceId(defaultResourceId)
    if (!rt || !rid) return
    void (async () => {
      setLoading(true)
      setErr(null)
      try {
        const res = await api.listAuditLogs(rt, rid, limit)
        setLogs(res.logs ?? [])
        setTotal(res.total ?? 0)
        setQueried(true)
      } catch (e: any) {
        setErr(e?.message ?? '查询失败')
      } finally {
        setLoading(false)
      }
    })()
  }, [defaultResourceType, defaultResourceId, limit])

  return (
    <div style={{ padding: 'var(--space-lg)', maxWidth: 900 }}>
      <h3 style={{ margin: '0 0 var(--space-md)', color: 'var(--text-primary)', fontSize: 'var(--font-xl)', fontWeight: 600 }}>审计日志</h3>

      {/* 查询条件 */}
      <div style={{ display: 'flex', gap: 'var(--space-sm)', marginBottom: 'var(--space-md)', flexWrap: 'wrap' }}>
        <select
          value={resourceType}
          onChange={e => setResourceType(e.target.value)}
          style={selectStyle}
        >
          <option value="">— 资源类型 —</option>
          <option value="node">node</option>
          <option value="edge">edge</option>
          <option value="lake">lake</option>
          <option value="organization">organization</option>
          <option value="org_quota">org_quota</option>
          <option value="api_key">api_key</option>
        </select>
        <input
          value={resourceId}
          onChange={e => setResourceId(e.target.value)}
          placeholder="资源 ID"
          style={{ ...inputStyle, width: 240 }}
          onKeyDown={e => e.key === 'Enter' && void handleQuery()}
        />
        <select
          value={limit}
          onChange={e => setLimit(Number(e.target.value))}
          style={{ ...selectStyle, width: 90 }}
        >
          <option value={20}>20 条</option>
          <option value={50}>50 条</option>
          <option value={100}>100 条</option>
          <option value={200}>200 条</option>
        </select>
        <button
          onClick={() => void handleQuery()}
          disabled={loading || !resourceType.trim() || !resourceId.trim()}
          style={btnStyle}
        >
          {loading ? '查询中…' : '查询'}
        </button>
      </div>

      {err && <p style={{ color: 'var(--status-danger)', margin: '0 0 var(--space-md)' }}>⚠ {err}</p>}

      {/* 结果 */}
      {queried && !loading && (
        <p style={{ color: 'var(--text-tertiary)', marginBottom: 'var(--space-sm)', fontSize: 'var(--font-sm)' }}>
          共 {total} 条记录（最多显示 {limit} 条）
        </p>
      )}

      {loading ? (
        <p style={{ color: 'var(--text-tertiary)' }}>查询中…</p>
      ) : queried && logs.length === 0 ? (
        <p style={{ color: 'var(--text-tertiary)' }}>无记录</p>
      ) : logs.length > 0 ? (
        <div style={{ overflowY: 'auto', maxHeight: '60vh', minHeight: 120 }}>
          <table style={{ width: '100%', borderCollapse: 'collapse', fontSize: 'var(--font-sm)' }}>
            <thead>
              <tr style={{ color: 'var(--text-tertiary)', textAlign: 'left' }}>
                <th style={thStyle}>时间</th>
                <th style={thStyle}>操作</th>
                <th style={thStyle}>操作人</th>
                <th style={thStyle}>详情</th>
              </tr>
            </thead>
            <tbody>
              {logs.map(l => (
                <tr key={l.id} style={{ borderBottom: '1px solid var(--border)' }}>
                  <td style={{ ...tdStyle, color: 'var(--text-tertiary)', whiteSpace: 'nowrap' }}>
                    {fmtDate(l.created_at)}
                  </td>
                  <td style={{ ...tdStyle, color: 'var(--accent)' }}>{l.action}</td>
                  <td style={{ ...tdStyle, fontFamily: 'var(--font-mono)', fontSize: 'var(--font-xs)', color: 'var(--text-secondary)' }}>
                    {l.actor_id.slice(0, 8)}…
                  </td>
                  <td style={{ ...tdStyle, color: 'var(--text-secondary)' }}>
                    {Object.keys(l.detail).length > 0
                      ? <code style={{ fontSize: 'var(--font-xs)' }}>{JSON.stringify(l.detail)}</code>
                      : <span style={{ color: 'var(--text-tertiary)' }}>—</span>}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      ) : null}
    </div>
  )
}

function fmtDate(s: string) {
  return new Date(s).toLocaleString('zh-CN', {
    month: '2-digit', day: '2-digit',
    hour: '2-digit', minute: '2-digit', second: '2-digit',
  })
}

const inputStyle: React.CSSProperties = {
  background: 'var(--bg-input)', border: '1px solid var(--border-input)', borderRadius: 'var(--radius-md)',
  color: 'var(--text-primary)', padding: 'var(--space-sm) var(--space-md)', fontSize: 'var(--font-md)',
}

const selectStyle: React.CSSProperties = {
  ...inputStyle, cursor: 'pointer',
}

const btnStyle: React.CSSProperties = {
  background: 'var(--accent)', border: 'none', color: 'var(--text-inverse)',
  borderRadius: 'var(--radius-md)', padding: 'var(--space-sm) var(--space-lg)', cursor: 'pointer', fontSize: 'var(--font-md)',
}

const thStyle: React.CSSProperties = {
  padding: 'var(--space-sm) var(--space-md)', fontWeight: 500, borderBottom: '1px solid var(--border)',
  color: 'var(--text-tertiary)',
}

const tdStyle: React.CSSProperties = {
  padding: 'var(--space-sm) var(--space-md)', color: 'var(--text-primary)', verticalAlign: 'top',
}
