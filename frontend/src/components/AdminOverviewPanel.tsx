import { useEffect, useState } from 'react'
import { api } from '../api/client'
import type { AdminOverview, AdminOverviewStats, OrgOverview } from '../api/types'

export const EMPTY_ADMIN_OVERVIEW_STATS: AdminOverviewStats = {
  organizations_count: 0,
  users_count: 0,
  graylist_entries_count: 0,
}

export function resolveAdminOverviewStats(overview: AdminOverview | null | undefined): AdminOverviewStats {
  return overview?.stats ?? EMPTY_ADMIN_OVERVIEW_STATS
}

export function resolveAdminOverviewOrganizations(overview: AdminOverview | null | undefined): OrgOverview[] {
  return overview?.organizations ?? []
}

export function adminLatestQuotaAudit(org: Pick<OrgOverview, 'recent_quota_audits'>) {
  return (org.recent_quota_audits ?? [])[0]
}

export default function AdminOverviewPanel() {
  const [overview, setOverview] = useState<AdminOverview | null>(null)
  const [loading, setLoading] = useState(false)
  const [forbidden, setForbidden] = useState(false)
  const [err, setErr] = useState<string | null>(null)

  async function load() {
    setLoading(true)
    setErr(null)
    try {
      const res = await api.getAdminOverview()
      setOverview(res)
      setForbidden(false)
    } catch (e: any) {
      if (e?.status === 403) {
        setForbidden(true)
        setOverview(null)
        setErr('仅平台管理员可查看运营总览')
      } else {
        setErr(e?.message ?? 'load failed')
      }
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => { void load() }, [])

  const stats = resolveAdminOverviewStats(overview)
  const organizations = resolveAdminOverviewOrganizations(overview)

  return (
    <div style={{ padding: 16, maxWidth: 860, minWidth: 360, flex: '2 1 520px' }}>
      <div style={{ display: 'flex', alignItems: 'center', gap: 8, marginBottom: 12 }}>
        <h3 style={{ margin: 0, color: '#cdd6f4', flex: 1 }}>管理员总览</h3>
        <button onClick={() => void load()} disabled={loading} style={btnStyle('#89b4fa')}>
          {loading ? '刷新中…' : '刷新'}
        </button>
      </div>
      <p style={{ margin: '0 0 12px', color: '#6c7086', fontSize: 12, lineHeight: 1.5 }}>
        聚合展示平台级组织、用户与灰度名单规模，以及最近创建的组织 quota 使用概览。
      </p>

      {err && <p style={{ color: forbidden ? '#f9e2af' : '#f38ba8', margin: '0 0 12px' }}>⚠ {err}</p>}

      {forbidden ? (
        <p style={{ color: '#6c7086', margin: 0 }}>当前账号不是平台管理员，只能查看此说明。</p>
      ) : loading && !overview ? (
        <p style={{ color: '#6c7086' }}>加载中…</p>
      ) : overview ? (
        <>
          <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(140px, 1fr))', gap: 10, marginBottom: 14 }}>
            <StatCard label="组织数" value={stats.organizations_count} color="#89b4fa" />
            <StatCard label="用户数" value={stats.users_count} color="#a6e3a1" />
            <StatCard label="灰度邮箱" value={stats.graylist_entries_count} color="#f9e2af" />
          </div>

          {organizations.length === 0 ? (
            <p style={{ color: '#6c7086' }}>暂无组织数据。</p>
          ) : (
            <table style={{ width: '100%', borderCollapse: 'collapse', fontSize: 13 }}>
              <thead>
                <tr style={{ color: '#6c7086', textAlign: 'left' }}>
                  <th style={thStyle}>组织</th>
                  <th style={thStyle}>Members</th>
                  <th style={thStyle}>Lakes</th>
                  <th style={thStyle}>Nodes</th>
                  <th style={thStyle}>Latest audit</th>
                </tr>
              </thead>
              <tbody>
                {organizations.map(org => {
                  const latestAudit = adminLatestQuotaAudit(org)
                  return (
                    <tr key={org.organization.id} style={{ borderBottom: '1px solid #313244' }}>
                      <td style={tdStyle}>
                        <div style={{ color: '#cdd6f4' }}>{org.organization.name}</div>
                        <div style={{ color: '#6c7086', fontSize: 11 }}>/{org.organization.slug}</div>
                      </td>
                      <td style={tdStyle}>{fmtUsage(org.quota.usage?.members_used, org.quota.max_members)}</td>
                      <td style={tdStyle}>{fmtUsage(org.quota.usage?.lakes_used, org.quota.max_lakes)}</td>
                      <td style={tdStyle}>{fmtUsage(org.quota.usage?.nodes_used, org.quota.max_nodes)}</td>
                      <td style={{ ...tdStyle, color: '#6c7086' }}>
                        {latestAudit ? new Date(latestAudit.created_at).toLocaleString() : '—'}
                      </td>
                    </tr>
                  )
                })}
              </tbody>
            </table>
          )}
        </>
      ) : null}
    </div>
  )
}

function StatCard({ label, value, color }: { label: string; value: number; color: string }) {
  return (
    <div style={{ border: '1px solid #313244', borderRadius: 8, padding: '10px 12px', background: '#181825' }}>
      <div style={{ color: '#6c7086', fontSize: 11, marginBottom: 6 }}>{label}</div>
      <div style={{ color, fontSize: 24, fontWeight: 700 }}>{value}</div>
    </div>
  )
}

function fmtUsage(used: number | undefined, limit: number) {
  return `${used ?? 0}/${limit}`
}

function btnStyle(color: string): React.CSSProperties {
  return {
    background: 'transparent', border: `1px solid ${color}`, color,
    borderRadius: 4, padding: '5px 12px', cursor: 'pointer', fontSize: 13,
  }
}

const thStyle: React.CSSProperties = {
  padding: '6px 8px', fontWeight: 500, borderBottom: '1px solid #313244',
}

const tdStyle: React.CSSProperties = {
  padding: '8px 8px', color: '#cdd6f4',
}