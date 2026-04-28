import { useEffect, useState } from 'react'
import { api, type AuditLogItem } from '../api/client'

interface Props {
  /** 预填资源类型（如 "node"）*/
  defaultResourceType?: string
  /** 预填资源 ID */
  defaultResourceId?: string
}

/** P11-B：审计日志浏览器 */
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
      setErr(e?.message ?? 'query failed')
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
        setErr(e?.message ?? 'query failed')
      } finally {
        setLoading(false)
      }
    })()
  }, [defaultResourceType, defaultResourceId, limit])

  return (
    <div style={{ padding: '16px', maxWidth: 900 }}>
      <h3 style={{ margin: '0 0 12px', color: '#cdd6f4' }}>审计日志</h3>

      {/* 查询条件 */}
      <div style={{ display: 'flex', gap: 8, marginBottom: 12, flexWrap: 'wrap' }}>
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
          onClick={handleQuery}
          disabled={loading || !resourceType.trim() || !resourceId.trim()}
          style={btnStyle}
        >
          {loading ? '查询中…' : '查询'}
        </button>
      </div>

      {err && <p style={{ color: '#f38ba8', margin: '0 0 12px' }}>⚠ {err}</p>}

      {/* 结果 */}
      {queried && !loading && (
        <p style={{ color: '#6c7086', marginBottom: 8, fontSize: 12 }}>
          共 {total} 条记录（最多显示 {limit} 条）
        </p>
      )}

      {loading ? (
        <p style={{ color: '#6c7086' }}>查询中…</p>
      ) : queried && logs.length === 0 ? (
        <p style={{ color: '#6c7086' }}>无记录</p>
      ) : logs.length > 0 ? (
        <table style={{ width: '100%', borderCollapse: 'collapse', fontSize: 12 }}>
          <thead>
            <tr style={{ color: '#6c7086', textAlign: 'left' }}>
              <th style={thStyle}>时间</th>
              <th style={thStyle}>操作</th>
              <th style={thStyle}>操作人</th>
              <th style={thStyle}>详情</th>
            </tr>
          </thead>
          <tbody>
            {logs.map(l => (
              <tr key={l.id} style={{ borderBottom: '1px solid #313244' }}>
                <td style={{ ...tdStyle, color: '#6c7086', whiteSpace: 'nowrap' }}>
                  {fmtDate(l.created_at)}
                </td>
                <td style={{ ...tdStyle, color: '#89dceb' }}>{l.action}</td>
                <td style={{ ...tdStyle, fontFamily: 'monospace', fontSize: 11, color: '#bac2de' }}>
                  {l.actor_id.slice(0, 8)}…
                </td>
                <td style={{ ...tdStyle, color: '#a6adc8' }}>
                  {Object.keys(l.detail).length > 0
                    ? <code style={{ fontSize: 11 }}>{JSON.stringify(l.detail)}</code>
                    : <span style={{ color: '#45475a' }}>—</span>}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
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
  background: '#1e1e2e', border: '1px solid #45475a', borderRadius: 4,
  color: '#cdd6f4', padding: '5px 10px', fontSize: 13,
}

const selectStyle: React.CSSProperties = {
  ...inputStyle, cursor: 'pointer',
}

const btnStyle: React.CSSProperties = {
  background: 'transparent', border: '1px solid #89b4fa', color: '#89b4fa',
  borderRadius: 4, padding: '5px 14px', cursor: 'pointer', fontSize: 13,
}

const thStyle: React.CSSProperties = {
  padding: '6px 8px', fontWeight: 500, borderBottom: '1px solid #313244',
}

const tdStyle: React.CSSProperties = {
  padding: '7px 8px', color: '#cdd6f4', verticalAlign: 'top',
}
