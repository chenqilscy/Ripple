import { describe, expect, it } from 'vitest'
import {
  adminLatestQuotaAudit,
  resolveAdminOverviewOrganizations,
  resolveAdminOverviewStats,
} from '../src/components/AdminOverviewPanel'
import { orgLatestQuotaAudit, orgRecentQuotaAudits } from '../src/components/OrgPanel'
import type { AdminOverview, AuditLogItem, OrgOverview } from '../src/api/types'

const audit: AuditLogItem = {
  id: 'audit-1',
  actor_id: 'u-1',
  action: 'org_quota.update',
  resource_type: 'org_quota',
  resource_id: 'org-1',
  detail: {},
  created_at: '2026-04-28T00:00:00Z',
}

function orgOverview(recentQuotaAudits?: AuditLogItem[]): OrgOverview {
  return {
    organization: {
      id: 'org-1',
      name: 'Alpha',
      slug: 'alpha',
      description: '',
      owner_id: 'u-1',
      created_at: '2026-04-28T00:00:00Z',
      updated_at: '2026-04-28T00:00:00Z',
    },
    quota: {
      org_id: 'org-1',
      max_members: 3,
      max_lakes: 5,
      max_nodes: 10,
      max_attachments: 2,
      max_api_keys: 4,
      max_storage_mb: 8,
      created_at: '2026-04-28T00:00:00Z',
      updated_at: '2026-04-28T00:00:00Z',
    },
    ...(recentQuotaAudits ? { recent_quota_audits: recentQuotaAudits } : {}),
  }
}

describe('运营台缺省字段回归', () => {
  it('管理员总览缺省 organizations 时回退为空列表和零统计', () => {
    expect(resolveAdminOverviewStats(null)).toEqual({
      organizations_count: 0,
      users_count: 0,
      graylist_entries_count: 0,
    })

    const overview: AdminOverview = {
      stats: { organizations_count: 1, users_count: 2, graylist_entries_count: 3 },
    }

    expect(resolveAdminOverviewOrganizations(overview)).toEqual([])
    expect(resolveAdminOverviewStats(overview).users_count).toBe(2)
  })

  it('recent_quota_audits 缺省时 latest audit 不抛错', () => {
    const withoutAudits = orgOverview()
    const withAudits = orgOverview([audit])

    expect(adminLatestQuotaAudit(withoutAudits)).toBeUndefined()
    expect(orgLatestQuotaAudit(withoutAudits)).toBeUndefined()
    expect(orgRecentQuotaAudits(withoutAudits)).toEqual([])

    expect(adminLatestQuotaAudit(withAudits)?.id).toBe('audit-1')
    expect(orgLatestQuotaAudit(withAudits)?.id).toBe('audit-1')
    expect(orgRecentQuotaAudits(withAudits)).toHaveLength(1)
  })
})