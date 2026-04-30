/**
 * P12-C: Organization list, creation form, and member management panel.
 * P13-A: Lake ↔ Org binding tab added.
 * 修复：scroll lock + Tab min-height + 统一中文标签 + CSS 变量
 */
import { useCallback, useEffect, useState } from 'react'
import { api } from '../api/client'
import AuditLogViewer from './AuditLogViewer'
import SubscriptionPanel from './SubscriptionPanel'
import type { Lake, OrgMember, OrgOverview, OrgQuota, OrgQuotaPatch, OrgRole, Organization } from '../api/types'

const ROLE_COLOR: Record<OrgRole, string> = {
  OWNER:  'var(--status-warning)',
  ADMIN:  'var(--status-success)',
  MEMBER: 'var(--accent)',
}

const INVITE_ROLE_OPTIONS: OrgRole[] = ['ADMIN', 'MEMBER']

export function orgRecentQuotaAudits(overview: OrgOverview | null | undefined) {
  return overview?.recent_quota_audits ?? []
}

export function orgLatestQuotaAudit(org: Pick<OrgOverview, 'recent_quota_audits'>) {
  return (org.recent_quota_audits ?? [])[0]
}

interface MemberListProps {
  org: Organization
  currentUserId: string
  onBack: () => void
}

function OrgMemberList({ org, currentUserId, onBack }: MemberListProps) {
  const [tab, setTab] = useState<'members' | 'lakes' | 'quota' | 'subscription'>('members')
  const [members, setMembers] = useState<OrgMember[]>([])
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [updating, setUpdating] = useState<string | null>(null)
  const [addUserId, setAddUserId] = useState('')
  const [addEmail, setAddEmail] = useState('')
  const [addRole, setAddRole] = useState<OrgRole>('MEMBER')
  const [adding, setAdding] = useState(false)
  const [addingEmail, setAddingEmail] = useState(false)

  // Lakes tab state
  const [lakes, setLakes] = useState<Lake[]>([])
  const [lakesLoading, setLakesLoading] = useState(false)
  const [lakesError, setLakesError] = useState<string | null>(null)

  // Quota tab state
  const [quota, setQuota] = useState<OrgQuota | null>(null)
  const [quotaDraft, setQuotaDraft] = useState<Record<keyof OrgQuotaPatch, string>>({
    max_members: '',
    max_lakes: '',
    max_nodes: '',
    max_attachments: '',
    max_api_keys: '',
    max_storage_mb: '',
  })
  const [quotaLoading, setQuotaLoading] = useState(false)
  const [quotaSaving, setQuotaSaving] = useState(false)
  const [quotaError, setQuotaError] = useState<string | null>(null)
  const [overview, setOverview] = useState<OrgOverview | null>(null)
  const [showQuotaAuditLog, setShowQuotaAuditLog] = useState(false)

  // Scroll lock
  useEffect(() => {
    const prev = document.body.style.overflow
    document.body.style.overflow = 'hidden'
    return () => { document.body.style.overflow = prev }
  }, [])

  const currentMember = members.find(m => m.user_id === currentUserId)
  const isAdmin = currentMember?.role === 'OWNER' || currentMember?.role === 'ADMIN'

  const applyQuotaState = useCallback((nextQuota: OrgQuota) => {
    setQuota(nextQuota)
    setQuotaDraft({
      max_members: String(nextQuota.max_members),
      max_lakes: String(nextQuota.max_lakes),
      max_nodes: String(nextQuota.max_nodes),
      max_attachments: String(nextQuota.max_attachments),
      max_api_keys: String(nextQuota.max_api_keys),
      max_storage_mb: String(nextQuota.max_storage_mb),
    })
  }, [])

  const load = useCallback(async () => {
    setLoading(true)
    setError(null)
    try {
      const res = await api.listOrgMembers(org.id)
      setMembers(res.members)
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : '成员列表加载失败')
    } finally {
      setLoading(false)
    }
  }, [org.id])

  const loadLakes = useCallback(async () => {
    setLakesLoading(true)
    setLakesError(null)
    try {
      const res = await api.listOrgLakes(org.id)
      setLakes(res.lakes)
    } catch (e: unknown) {
      setLakesError(e instanceof Error ? e.message : '湖列表加载失败')
    } finally {
      setLakesLoading(false)
    }
  }, [org.id])

  const loadQuota = useCallback(async () => {
    setQuotaLoading(true)
    setQuotaError(null)
    try {
      const nextOverview = await api.getOrgOverview(org.id)
      setOverview(nextOverview)
      applyQuotaState(nextOverview.quota)
    } catch (e: unknown) {
      setQuotaError(e instanceof Error ? e.message : '配额加载失败')
    } finally {
      setQuotaLoading(false)
    }
  }, [org.id, applyQuotaState])

  useEffect(() => { void load() }, [load])
  useEffect(() => { if (tab === 'lakes') void loadLakes() }, [tab, loadLakes])
  useEffect(() => { if (tab === 'quota') void loadQuota() }, [tab, loadQuota])

  const handleRoleChange = useCallback(async (userId: string, newRole: OrgRole) => {
    setUpdating(userId)
    setError(null)
    try {
      await api.updateOrgMemberRole(org.id, userId, newRole)
      setMembers(prev => prev.map(m => m.user_id === userId ? { ...m, role: newRole } : m))
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : '角色更新失败')
    } finally {
      setUpdating(null)
    }
  }, [org.id])

  const handleRemove = useCallback(async (userId: string) => {
    setUpdating(userId)
    setError(null)
    try {
      await api.removeOrgMember(org.id, userId)
      setMembers(prev => prev.filter(m => m.user_id !== userId))
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : '移除成员失败')
    } finally {
      setUpdating(null)
    }
  }, [org.id])

  const handleAdd = useCallback(async () => {
    const uid = addUserId.trim()
    if (!uid) return
    setAdding(true)
    setError(null)
    try {
      await api.addOrgMember(org.id, uid, addRole)
      setAddUserId('')
      await load()
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : '添加成员失败')
    } finally {
      setAdding(false)
    }
  }, [org.id, addUserId, addRole, load])

  const handleAddByEmail = useCallback(async () => {
    const email = addEmail.trim()
    if (!email) return
    setAddingEmail(true)
    setError(null)
    try {
      await api.addOrgMemberByEmail(org.id, email, addRole)
      setAddEmail('')
      await load()
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : '邮件邀请失败')
    } finally {
      setAddingEmail(false)
    }
  }, [org.id, addEmail, addRole, load])

  const handleSaveQuota = useCallback(async () => {
    const patch: OrgQuotaPatch = {}
    for (const [key, value] of Object.entries(quotaDraft) as [keyof OrgQuotaPatch, string][]) {
      const trimmed = value.trim()
      if (trimmed === '') continue
      const n = Number(trimmed)
      if (!Number.isFinite(n) || !Number.isInteger(n)) {
        setQuotaError(`${key} 必须为整数`)
        return
      }
      patch[key] = n
    }
    if (Object.keys(patch).length === 0) {
      setQuotaError('没有需要更新的字段')
      return
    }
    setQuotaSaving(true)
    setQuotaError(null)
    try {
      await api.updateOrgQuota(org.id, patch)
      const nextOverview = await api.getOrgOverview(org.id)
      setOverview(nextOverview)
      applyQuotaState(nextOverview.quota)
    } catch (e: unknown) {
      setQuotaError(e instanceof Error ? e.message : '配额更新失败')
    } finally {
      setQuotaSaving(false)
    }
  }, [org.id, quotaDraft, applyQuotaState])

  const quotaUsageItems: { label: string; used: number; limit: number }[] = quota?.usage ? [
    { label: '成员', used: quota.usage.members_used, limit: quota.max_members },
    { label: '湖', used: quota.usage.lakes_used, limit: quota.max_lakes },
    { label: '节点', used: quota.usage.nodes_used, limit: quota.max_nodes },
    { label: '附件', used: quota.usage.attachments_used, limit: quota.max_attachments },
    { label: 'API 密钥', used: quota.usage.api_keys_used, limit: quota.max_api_keys },
    { label: '存储 (MB)', used: quota.usage.storage_mb_used, limit: quota.max_storage_mb },
  ] : []
  const recentQuotaAudits = orgRecentQuotaAudits(overview)

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 'var(--space-md)', minHeight: 480 }}>
      <div style={{ display: 'flex', alignItems: 'center', gap: 'var(--space-sm)' }}>
        <button onClick={onBack} style={btnStyle}>← 返回</button>
        <span style={{ color: 'var(--text-primary)', fontWeight: 600, fontSize: 'var(--font-lg)', flex: 1 }}>
          {org.name}
        </span>
        <button onClick={() => tab === 'members' ? void load() : tab === 'lakes' ? void loadLakes() : void loadQuota()} disabled={loading || lakesLoading || quotaLoading}
          style={{ ...btnStyle, marginLeft: 'auto' }}>
          {(loading || lakesLoading || quotaLoading) ? '加载中…' : '刷新'}
        </button>
      </div>

      {error && <ErrorMsg>{error}</ErrorMsg>}

      {/* Tab switcher */}
      <div style={{ display: 'flex', gap: 'var(--space-sm)', flexWrap: 'wrap' }}>
        {(['members', 'lakes', 'quota', 'subscription'] as const).map(t => (
          <button
            key={t}
            onClick={() => setTab(t)}
            style={{ ...btnStyle, ...(tab === t ? { background: 'var(--accent-subtle)', color: 'var(--accent)', borderColor: 'var(--accent)' } : {}) }}
          >
            {t === 'members' ? '成员' : t === 'lakes' ? '湖' : t === 'quota' ? '配额' : '订阅'}
          </button>
        ))}
      </div>

      {tab === 'members' && (
        <>
          <div style={{ display: 'flex', flexDirection: 'column', gap: 'var(--space-sm)' }}>
            {members.map(m => (
              <div key={m.user_id} style={memberRowStyle}>
                <span style={{ color: 'var(--text-secondary)', fontSize: 'var(--font-sm)', flex: 1, overflow: 'hidden', textOverflow: 'ellipsis' }}>
                  {m.user_id}
                </span>
                {isAdmin && m.role !== 'OWNER' && m.user_id !== currentUserId ? (
                  <>
                    <select
                      value={m.role}
                      disabled={updating === m.user_id}
                      onChange={e => void handleRoleChange(m.user_id, e.target.value as OrgRole)}
                      style={selectStyle}
                    >
                      {INVITE_ROLE_OPTIONS.map(r => (
                        <option key={r} value={r}>{r === 'ADMIN' ? '管理员' : '成员'}</option>
                      ))}
                    </select>
                    <button
                      disabled={updating === m.user_id}
                      onClick={() => void handleRemove(m.user_id)}
                      style={{ ...btnStyle, color: 'var(--status-danger)', borderColor: 'var(--border-subtle)' }}
                    >
                      移除
                    </button>
                  </>
                ) : (
                  <span style={{
                    color: ROLE_COLOR[m.role], fontSize: 'var(--font-xs)',
                    padding: '2px var(--space-sm)', background: 'var(--bg-secondary)',
                    borderRadius: 'var(--radius-sm)',
                  }}>
                    {m.role === 'OWNER' ? '所有者' : m.role === 'ADMIN' ? '管理员' : '成员'}
                  </span>
                )}
              </div>
            ))}
            {members.length === 0 && !loading && (
              <div style={{ color: 'var(--text-tertiary)', fontSize: 'var(--font-md)', textAlign: 'center', padding: 'var(--space-lg)' }}>
                暂无成员
              </div>
            )}
          </div>

          {isAdmin && (
            <div style={{ display: 'flex', flexDirection: 'column', gap: 'var(--space-sm)', marginTop: 'var(--space-xs)' }}>
              <div style={{ display: 'flex', gap: 'var(--space-sm)', flexWrap: 'wrap' }}>
                <input
                  placeholder="用户 ID"
                  value={addUserId}
                  onChange={e => setAddUserId(e.target.value)}
                  onKeyDown={e => { if (e.key === 'Enter') void handleAdd() }}
                  style={inputStyle}
                />
                <select value={addRole} onChange={e => setAddRole(e.target.value as OrgRole)} style={selectStyle}>
                  {INVITE_ROLE_OPTIONS.map(r => <option key={r} value={r}>{r === 'ADMIN' ? '管理员' : '成员'}</option>)}
                </select>
                <button onClick={() => void handleAdd()} disabled={adding || !addUserId.trim()} style={btnStyle}>
                  {adding ? '添加中…' : '添加'}
                </button>
              </div>
              <div style={{ display: 'flex', gap: 'var(--space-sm)', flexWrap: 'wrap' }}>
                <input
                  type="email"
                  placeholder="邮箱地址"
                  value={addEmail}
                  onChange={e => setAddEmail(e.target.value)}
                  onKeyDown={e => { if (e.key === 'Enter') void handleAddByEmail() }}
                  style={{ ...inputStyle, flex: 1, minWidth: 160 }}
                />
                <button
                  onClick={() => void handleAddByEmail()}
                  disabled={addingEmail || !addEmail.trim()}
                  style={btnStyle}
                  title="邀请已注册用户"
                >
                  {addingEmail ? '邀请中…' : '邮件邀请'}
                </button>
              </div>
            </div>
          )}
        </>
      )}

      {tab === 'lakes' && (
        <>
          {lakesError && <ErrorMsg>{lakesError}</ErrorMsg>}
          <div style={{ display: 'flex', flexDirection: 'column', gap: 'var(--space-sm)' }}>
            {lakes.map(l => (
              <div key={l.id} style={memberRowStyle}>
                <span style={{ color: 'var(--text-primary)', fontSize: 'var(--font-sm)', flex: 1, overflow: 'hidden', textOverflow: 'ellipsis' }}>
                  {l.name}
                </span>
                <span style={{ color: 'var(--text-tertiary)', fontSize: 'var(--font-xs)' }}>{l.id.slice(0, 8)}…</span>
              </div>
            ))}
            {lakes.length === 0 && !lakesLoading && (
              <div style={{ color: 'var(--text-tertiary)', fontSize: 'var(--font-md)', textAlign: 'center', padding: 'var(--space-lg)' }}>
                暂无关联的湖
              </div>
            )}
            {lakesLoading && (
              <div style={{ color: 'var(--text-tertiary)', fontSize: 'var(--font-md)', textAlign: 'center', padding: 'var(--space-md)' }}>加载中…</div>
            )}
          </div>
        </>
      )}

      {tab === 'quota' && (
        <>
          {quotaError && <ErrorMsg>{quotaError}</ErrorMsg>}
          {quotaLoading && <div style={{ color: 'var(--text-tertiary)', fontSize: 'var(--font-md)', padding: 'var(--space-md)' }}>加载配额中…</div>}
          {quota && (
            <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 'var(--space-sm)' }}>
              {quotaUsageItems.length > 0 && (
                <div style={{ gridColumn: '1 / -1', display: 'grid', gap: 'var(--space-sm)', marginBottom: 'var(--space-xs)' }}>
                  {quotaUsageItems.map(item => {
                    const ratio = item.limit > 0 ? Math.min(1, item.used / item.limit) : 0
                    const barColor = ratio >= 1 ? 'var(--status-danger)' : ratio >= 0.8 ? 'var(--status-warning)' : 'var(--accent)'
                    return (
                      <div key={item.label} style={{
                        display: 'flex', flexDirection: 'column', gap: 'var(--space-xs)',
                        padding: 'var(--space-sm)',
                        border: '1px solid var(--border-subtle)',
                        borderRadius: 'var(--radius-md)',
                        background: 'var(--bg-secondary)',
                      }}>
                        <div style={{ display: 'flex', justifyContent: 'space-between', gap: 'var(--space-sm)', fontSize: 'var(--font-sm)', color: 'var(--text-secondary)' }}>
                          <span>{item.label}</span>
                          <span>{item.used} / {item.limit}</span>
                        </div>
                        <div style={{ height: 6, borderRadius: 999, background: 'var(--bg-tertiary)', overflow: 'hidden' }}>
                          <div style={{ width: `${ratio * 100}%`, height: '100%', background: barColor, borderRadius: 999 }} />
                        </div>
                      </div>
                    )
                  })}
                </div>
              )}
              {overview && (
                <div style={{ gridColumn: '1 / -1', display: 'flex', flexDirection: 'column', gap: 'var(--space-sm)', marginBottom: 'var(--space-xs)' }}>
                  <div style={{ display: 'flex', alignItems: 'center', gap: 'var(--space-sm)' }}>
                    <span style={{ color: 'var(--text-secondary)', fontSize: 'var(--font-sm)', fontWeight: 600, flex: 1 }}>最近配额变更记录</span>
                    <button onClick={() => setShowQuotaAuditLog(prev => !prev)} style={btnStyle}>
                      {showQuotaAuditLog ? '收起详情' : '查看全部'}
                    </button>
                  </div>
                  {recentQuotaAudits.length > 0 ? recentQuotaAudits.slice(0, 3).map(log => (
                    <div key={log.id} style={{
                      display: 'flex', justifyContent: 'space-between', gap: 'var(--space-sm)',
                      padding: 'var(--space-sm)',
                      border: '1px solid var(--border-subtle)',
                      borderRadius: 'var(--radius-md)',
                      background: 'var(--bg-secondary)',
                    }}>
                      <div style={{ display: 'flex', flexDirection: 'column', gap: 2 }}>
                        <span style={{ color: 'var(--text-primary)', fontSize: 'var(--font-sm)' }}>{log.action}</span>
                        <span style={{ color: 'var(--text-tertiary)', fontSize: 'var(--font-xs)' }}>操作人 {log.actor_id.slice(0, 8)}…</span>
                      </div>
                      <span style={{ color: 'var(--text-tertiary)', fontSize: 'var(--font-xs)', textAlign: 'right' }}>{new Date(log.created_at).toLocaleString()}</span>
                    </div>
                  )) : (
                    <div style={{ color: 'var(--text-tertiary)', fontSize: 'var(--font-sm)', padding: 'var(--space-xs) 0 var(--space-sm)' }}>
                      暂无配额变更记录
                    </div>
                  )}
                  {showQuotaAuditLog && (
                    <div style={{ border: '1px solid var(--border-subtle)', borderRadius: 'var(--radius-md)', background: 'var(--bg-secondary)' }}>
                      <AuditLogViewer defaultResourceType="org_quota" defaultResourceId={org.id} />
                    </div>
                  )}
                </div>
              )}
              {quotaFields.map(f => (
                <label key={f.key} style={{ display: 'flex', flexDirection: 'column', gap: 'var(--space-xs)', color: 'var(--text-secondary)', fontSize: 'var(--font-sm)' }}>
                  {f.label}
                  <input
                    type="number"
                    min={f.key === 'max_members' ? 1 : 0}
                    value={quotaDraft[f.key]}
                    disabled={!isAdmin || quotaSaving}
                    onChange={e => setQuotaDraft(prev => ({ ...prev, [f.key]: e.target.value }))}
                    style={inputStyle}
                  />
                </label>
              ))}
              <div style={{ gridColumn: '1 / -1', display: 'flex', alignItems: 'center', gap: 'var(--space-sm)' }}>
                <span style={{ color: 'var(--text-tertiary)', fontSize: 'var(--font-sm)', flex: 1 }}>
                  更新于 {new Date(quota.updated_at).toLocaleString('zh-CN')}
                </span>
                {isAdmin ? (
                  <button onClick={() => void handleSaveQuota()} disabled={quotaSaving} style={btnStyle}>
                    {quotaSaving ? '保存中…' : '保存配额'}
                  </button>
                ) : (
                  <span style={{ color: 'var(--text-tertiary)', fontSize: 'var(--font-sm)' }}>只读</span>
                )}
              </div>
            </div>
          )}
        </>
      )}

      {tab === 'subscription' && (
        <SubscriptionPanel
          orgId={org.id}
          isOwner={currentMember?.role === 'OWNER'}
        />
      )}
    </div>
  )
}

const quotaFields: { key: keyof OrgQuotaPatch; label: string }[] = [
  { key: 'max_members', label: '成员上限' },
  { key: 'max_lakes', label: '湖上限' },
  { key: 'max_nodes', label: '节点上限' },
  { key: 'max_attachments', label: '附件上限' },
  { key: 'max_api_keys', label: 'API 密钥上限' },
  { key: 'max_storage_mb', label: '存储上限 (MB)' },
]

interface Props {
  currentUserId: string
  onClose: () => void
}

export default function OrgPanel({ currentUserId, onClose }: Props) {
  const [orgs, setOrgs] = useState<OrgOverview[]>([])
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [creating, setCreating] = useState(false)
  const [newName, setNewName] = useState('')
  const [newSlug, setNewSlug] = useState('')
  const [newDesc, setNewDesc] = useState('')
  const [creating2, setCreating2] = useState(false)
  const [selectedOrg, setSelectedOrg] = useState<OrgOverview | null>(null)

  const load = useCallback(async () => {
    setLoading(true)
    setError(null)
    try {
      const res = await api.listOrgOverviews()
      setOrgs(res.organizations ?? [])
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : '组织列表加载失败')
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => { void load() }, [load])

  const handleCreate = useCallback(async () => {
    const name = newName.trim()
    const slug = newSlug.trim()
    if (!name || !slug) return
    setCreating2(true)
    setError(null)
    try {
      await api.createOrg(name, slug, newDesc.trim())
      await load()
      setNewName('')
      setNewSlug('')
      setNewDesc('')
      setCreating(false)
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : '创建组织失败')
    } finally {
      setCreating2(false)
    }
  }, [newName, newSlug, newDesc, load])

  return (
    <div style={panelStyle}>
      {/* Header */}
      <div style={{ display: 'flex', alignItems: 'center', marginBottom: 'var(--space-lg)' }}>
        <span style={{ color: 'var(--text-primary)', fontWeight: 700, fontSize: 'var(--font-xl)', flex: 1 }}>组织管理</span>
        <button onClick={() => void load()} disabled={loading}
          style={{ ...btnStyle, marginRight: 'var(--space-sm)' }}>
          {loading ? '加载中…' : '刷新'}
        </button>
        <button onClick={onClose} style={{ ...btnStyle, color: 'var(--text-tertiary)' }}>✕</button>
      </div>

      {error && <ErrorMsg>{error}</ErrorMsg>}

      {/* Member view */}
      {selectedOrg ? (
        <OrgMemberList
          org={selectedOrg.organization}
          currentUserId={currentUserId}
          onBack={() => setSelectedOrg(null)}
        />
      ) : (
        <>
          {/* Org list */}
          <div style={{ display: 'flex', flexDirection: 'column', gap: 'var(--space-sm)', marginBottom: 'var(--space-md)' }}>
            {orgs.map(org => {
              const latestAudit = orgLatestQuotaAudit(org)
              return (
                <div key={org.organization.id} style={{ ...memberRowStyle, alignItems: 'flex-start' }}>
                  <div style={{ flex: 1, display: 'flex', flexDirection: 'column', gap: 'var(--space-xs)' }}>
                    <div>
                      <span style={{ color: 'var(--text-primary)', fontSize: 'var(--font-base)', fontWeight: 500 }}>{org.organization.name}</span>
                      <span style={{ color: 'var(--text-tertiary)', fontSize: 'var(--font-xs)', marginLeft: 'var(--space-sm)' }}>/{org.organization.slug}</span>
                    </div>
                    {org.quota.usage && (
                      <div style={{ display: 'flex', gap: 'var(--space-sm)', flexWrap: 'wrap', color: 'var(--text-tertiary)', fontSize: 'var(--font-xs)' }}>
                        <span>成员 {org.quota.usage.members_used}/{org.quota.max_members}</span>
                        <span>湖 {org.quota.usage.lakes_used}/{org.quota.max_lakes}</span>
                        <span>节点 {org.quota.usage.nodes_used}/{org.quota.max_nodes}</span>
                      </div>
                    )}
                    {latestAudit && (
                      <div style={{ color: 'var(--text-tertiary)', fontSize: 'var(--font-xs)' }}>
                        最近变更 {new Date(latestAudit.created_at).toLocaleString('zh-CN')}
                      </div>
                    )}
                  </div>
                  <button onClick={() => setSelectedOrg(org)} style={btnStyle}>管理</button>
                </div>
              )
            })}
            {orgs.length === 0 && !loading && (
              <div style={{ color: 'var(--text-tertiary)', fontSize: 'var(--font-md)', textAlign: 'center', padding: 'var(--space-lg)' }}>
                暂无组织
              </div>
            )}
          </div>

          {/* Create form toggle */}
          {!creating ? (
            <button onClick={() => setCreating(true)} style={{ ...btnStyle, width: '100%' }}>
              + 新建组织
            </button>
          ) : (
            <div style={{
              display: 'flex', flexDirection: 'column', gap: 'var(--space-sm)',
              border: '1px solid var(--border)', borderRadius: 'var(--radius-md)',
              padding: 'var(--space-md)',
            }}>
              <span style={{ color: 'var(--text-secondary)', fontSize: 'var(--font-sm)', fontWeight: 600 }}>新建组织</span>
              <input
                placeholder="组织名称"
                value={newName}
                onChange={e => setNewName(e.target.value)}
                style={inputStyle}
              />
              <input
                placeholder="Slug（英文，如 my-org）"
                value={newSlug}
                onChange={e => setNewSlug(e.target.value.toLowerCase().replace(/[^a-z0-9-]/g, ''))}
                style={inputStyle}
              />
              <input
                placeholder="描述（可选）"
                value={newDesc}
                onChange={e => setNewDesc(e.target.value)}
                style={inputStyle}
              />
              <div style={{ display: 'flex', gap: 'var(--space-sm)' }}>
                <button
                  onClick={() => void handleCreate()}
                  disabled={creating2 || !newName.trim() || !newSlug.trim()}
                  style={{ ...btnStyle, flex: 1, background: 'var(--accent-subtle)', color: 'var(--accent)' }}
                >
                  {creating2 ? '创建中…' : '创建'}
                </button>
                <button onClick={() => setCreating(false)} style={btnStyle}>取消</button>
              </div>
            </div>
          )}
        </>
      )}
    </div>
  )
}

// ---- Shared styles ----

const panelStyle: React.CSSProperties = {
  background: 'var(--bg-primary)',
  border: '1px solid var(--border)',
  borderRadius: 'var(--radius-xl)',
  padding: 'var(--space-xl)',
  width: 380,
  maxHeight: '80vh',
  overflowY: 'auto',
  boxShadow: 'var(--shadow-card)',
  color: 'var(--text-primary)',
}

const memberRowStyle: React.CSSProperties = {
  display: 'flex',
  alignItems: 'center',
  gap: 'var(--space-sm)',
  padding: 'var(--space-sm)',
  background: 'var(--bg-secondary)',
  borderRadius: 'var(--radius-md)',
  border: '1px solid var(--border-subtle)',
}

const btnStyle: React.CSSProperties = {
  background: 'none',
  border: '1px solid var(--border)',
  borderRadius: 'var(--radius-md)',
  color: 'var(--text-secondary)',
  cursor: 'pointer',
  padding: 'var(--space-xs) var(--space-md)',
  fontSize: 'var(--font-sm)',
}

const inputStyle: React.CSSProperties = {
  background: 'var(--bg-input)',
  border: '1px solid var(--border-input)',
  borderRadius: 'var(--radius-md)',
  color: 'var(--text-primary)',
  padding: 'var(--space-xs) var(--space-sm)',
  fontSize: 'var(--font-sm)',
  width: '100%',
  boxSizing: 'border-box',
  outline: 'none',
}

const selectStyle: React.CSSProperties = {
  background: 'var(--bg-secondary)',
  border: '1px solid var(--border-input)',
  borderRadius: 'var(--radius-md)',
  color: 'var(--text-primary)',
  padding: 'var(--space-xs) var(--space-sm)',
  fontSize: 'var(--font-sm)',
  cursor: 'pointer',
}

function ErrorMsg({ children }: { children: React.ReactNode }) {
  return (
    <div style={{
      color: 'var(--status-danger)',
      fontSize: 'var(--font-sm)',
      marginBottom: 'var(--space-sm)',
      padding: 'var(--space-xs) var(--space-sm)',
      background: 'var(--status-danger-subtle)',
      borderRadius: 'var(--radius-md)',
    }}>
      {children}
    </div>
  )
}