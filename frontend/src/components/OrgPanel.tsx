/**
 * P12-C: Organization list, creation form, and member management panel.
 * P13-A: Lake ↔ Org binding tab added.
 */
import { useCallback, useEffect, useState } from 'react'
import { api } from '../api/client'
import AuditLogViewer from './AuditLogViewer'
import type { Lake, OrgMember, OrgOverview, OrgQuota, OrgQuotaPatch, OrgRole, Organization } from '../api/types'

const ROLE_COLOR: Record<OrgRole, string> = {
  OWNER:  '#f5a623',
  ADMIN:  '#52c41a',
  MEMBER: '#4a8eff',
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
  const [tab, setTab] = useState<'members' | 'lakes' | 'quota'>('members')
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
      setError(e instanceof Error ? e.message : 'Failed to load members')
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
      setLakesError(e instanceof Error ? e.message : 'Failed to load lakes')
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
      setQuotaError(e instanceof Error ? e.message : 'Failed to load quota')
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
      setError(e instanceof Error ? e.message : 'Failed to update role')
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
      setError(e instanceof Error ? e.message : 'Failed to remove member')
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
      setError(e instanceof Error ? e.message : 'Failed to add member')
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
      setError(e instanceof Error ? e.message : 'Failed to invite by email')
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
        setQuotaError(`${key} must be an integer`)
        return
      }
      patch[key] = n
    }
    if (Object.keys(patch).length === 0) {
      setQuotaError('No quota fields to update')
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
      setQuotaError(e instanceof Error ? e.message : 'Failed to update quota')
    } finally {
      setQuotaSaving(false)
    }
  }, [org.id, quotaDraft, applyQuotaState])

  const quotaUsageItems: { label: string; used: number; limit: number }[] = quota?.usage ? [
    { label: 'Members', used: quota.usage.members_used, limit: quota.max_members },
    { label: 'Lakes', used: quota.usage.lakes_used, limit: quota.max_lakes },
    { label: 'Nodes', used: quota.usage.nodes_used, limit: quota.max_nodes },
    { label: 'Attachments', used: quota.usage.attachments_used, limit: quota.max_attachments },
    { label: 'API Keys', used: quota.usage.api_keys_used, limit: quota.max_api_keys },
    { label: 'Storage (MB)', used: quota.usage.storage_mb_used, limit: quota.max_storage_mb },
  ] : []
  const recentQuotaAudits = orgRecentQuotaAudits(overview)

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 12 }}>
      <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
        <button onClick={onBack} style={btnStyle}>← Back</button>
        <span style={{ color: '#c0d8f0', fontWeight: 600, fontSize: 14 }}>
          {org.name}
        </span>
        <button onClick={() => tab === 'members' ? void load() : tab === 'lakes' ? void loadLakes() : void loadQuota()} disabled={loading || lakesLoading || quotaLoading}
          style={{ ...btnStyle, marginLeft: 'auto' }}>
          {(loading || lakesLoading || quotaLoading) ? '...' : 'Refresh'}
        </button>
      </div>

      {error && <ErrorMsg>{error}</ErrorMsg>}

      {/* Tab switcher */}
      <div style={{ display: 'flex', gap: 6 }}>
        <button
          onClick={() => setTab('members')}
          style={{ ...btnStyle, ...(tab === 'members' ? { background: 'rgba(74,142,255,0.15)', color: '#4a8eff' } : {}) }}
        >
          Members
        </button>
        <button
          onClick={() => setTab('lakes')}
          style={{ ...btnStyle, ...(tab === 'lakes' ? { background: 'rgba(74,142,255,0.15)', color: '#4a8eff' } : {}) }}
        >
          Lakes
        </button>
        <button
          onClick={() => setTab('quota')}
          style={{ ...btnStyle, ...(tab === 'quota' ? { background: 'rgba(74,142,255,0.15)', color: '#4a8eff' } : {}) }}
        >
          Quota
        </button>
      </div>

      {tab === 'members' && (
        <>
          <div style={{ display: 'flex', flexDirection: 'column', gap: 6 }}>
            {members.map(m => (
              <div key={m.user_id} style={memberRowStyle}>
                <span style={{ color: '#8ab0d0', fontSize: 12, flex: 1, overflow: 'hidden', textOverflow: 'ellipsis' }}>
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
                        <option key={r} value={r}>{r}</option>
                      ))}
                    </select>
                    <button
                      disabled={updating === m.user_id}
                      onClick={() => void handleRemove(m.user_id)}
                      style={{ ...btnStyle, color: '#ff6b6b', borderColor: '#5a2222' }}
                    >
                      Remove
                    </button>
                  </>
                ) : (
                  <span style={{ color: ROLE_COLOR[m.role], fontSize: 11, padding: '2px 8px',
                    background: 'rgba(255,255,255,0.05)', borderRadius: 4 }}>
                    {m.role}
                  </span>
                )}
              </div>
            ))}
            {members.length === 0 && !loading && (
              <div style={{ color: '#4a6a8e', fontSize: 12, textAlign: 'center', padding: 12 }}>
                No members
              </div>
            )}
          </div>

          {isAdmin && (
            <div style={{ display: 'flex', flexDirection: 'column', gap: 6, marginTop: 4 }}>
              <div style={{ display: 'flex', gap: 6, flexWrap: 'wrap' }}>
                <input
                  placeholder="User ID to invite"
                  value={addUserId}
                  onChange={e => setAddUserId(e.target.value)}
                  onKeyDown={e => { if (e.key === 'Enter') void handleAdd() }}
                  style={inputStyle}
                />
                <select value={addRole} onChange={e => setAddRole(e.target.value as OrgRole)} style={selectStyle}>
                  {INVITE_ROLE_OPTIONS.map(r => <option key={r} value={r}>{r}</option>)}
                </select>
                <button onClick={() => void handleAdd()} disabled={adding || !addUserId.trim()} style={btnStyle}>
                  {adding ? '...' : 'Add'}
                </button>
              </div>
              <div style={{ display: 'flex', gap: 6, flexWrap: 'wrap' }}>
                <input
                  type="email"
                  placeholder="Email to invite"
                  value={addEmail}
                  onChange={e => setAddEmail(e.target.value)}
                  onKeyDown={e => { if (e.key === 'Enter') void handleAddByEmail() }}
                  style={inputStyle}
                />
                <button
                  onClick={() => void handleAddByEmail()}
                  disabled={addingEmail || !addEmail.trim()}
                  style={btnStyle}
                  title="Invite an already-registered user by email"
                >
                  {addingEmail ? '...' : 'Invite by Email'}
                </button>
              </div>
            </div>
          )}
        </>
      )}

      {tab === 'lakes' && (
        <>
          {lakesError && <ErrorMsg>{lakesError}</ErrorMsg>}
          <div style={{ display: 'flex', flexDirection: 'column', gap: 6 }}>
            {lakes.map(l => (
              <div key={l.id} style={memberRowStyle}>
                <span style={{ color: '#c0d8f0', fontSize: 12, flex: 1, overflow: 'hidden', textOverflow: 'ellipsis' }}>
                  {l.name}
                </span>
                <span style={{ color: '#4a6a8e', fontSize: 10 }}>{l.id.slice(0, 8)}…</span>
              </div>
            ))}
            {lakes.length === 0 && !lakesLoading && (
              <div style={{ color: '#4a6a8e', fontSize: 12, textAlign: 'center', padding: 12 }}>
                No lakes linked to this organization
              </div>
            )}
            {lakesLoading && (
              <div style={{ color: '#4a6a8e', fontSize: 12, textAlign: 'center', padding: 12 }}>Loading…</div>
            )}
          </div>
        </>
      )}

      {tab === 'quota' && (
        <>
          {quotaError && <ErrorMsg>{quotaError}</ErrorMsg>}
          {quotaLoading && <div style={{ color: '#4a6a8e', fontSize: 12, padding: 12 }}>Loading quota…</div>}
          {quota && (
            <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 8 }}>
              {quotaUsageItems.length > 0 && (
                <div style={{ gridColumn: '1 / -1', display: 'grid', gap: 8, marginBottom: 4 }}>
                  {quotaUsageItems.map(item => {
                    const ratio = item.limit > 0 ? Math.min(1, item.used / item.limit) : 0
                    const barColor = ratio >= 1 ? '#ff6b6b' : ratio >= 0.8 ? '#f5a623' : '#4a8eff'
                    return (
                      <div key={item.label} style={{ display: 'flex', flexDirection: 'column', gap: 4, padding: 8, border: '1px solid rgba(255,255,255,0.08)', borderRadius: 8, background: 'rgba(255,255,255,0.03)' }}>
                        <div style={{ display: 'flex', justifyContent: 'space-between', gap: 8, fontSize: 11, color: '#8ab0d0' }}>
                          <span>{item.label}</span>
                          <span>{item.used} / {item.limit}</span>
                        </div>
                        <div style={{ height: 6, borderRadius: 999, background: 'rgba(255,255,255,0.08)', overflow: 'hidden' }}>
                          <div style={{ width: `${ratio * 100}%`, height: '100%', background: barColor }} />
                        </div>
                      </div>
                    )
                  })}
                </div>
              )}
              {overview && (
                <div style={{ gridColumn: '1 / -1', display: 'flex', flexDirection: 'column', gap: 6, marginBottom: 4 }}>
                  <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
                    <span style={{ color: '#8ab0d0', fontSize: 11, fontWeight: 600, flex: 1 }}>Recent quota audits</span>
                    <button onClick={() => setShowQuotaAuditLog(prev => !prev)} style={btnStyle}>
                      {showQuotaAuditLog ? 'Hide full log' : 'View full log'}
                    </button>
                  </div>
                  {recentQuotaAudits.length > 0 ? recentQuotaAudits.slice(0, 3).map(log => (
                    <div key={log.id} style={{ display: 'flex', justifyContent: 'space-between', gap: 8, padding: '8px 10px', border: '1px solid rgba(255,255,255,0.08)', borderRadius: 8, background: 'rgba(255,255,255,0.02)' }}>
                      <div style={{ display: 'flex', flexDirection: 'column', gap: 2 }}>
                        <span style={{ color: '#c0d8f0', fontSize: 11 }}>{log.action}</span>
                        <span style={{ color: '#4a6a8e', fontSize: 10 }}>Actor {log.actor_id}</span>
                      </div>
                      <span style={{ color: '#4a6a8e', fontSize: 10, textAlign: 'right' }}>{new Date(log.created_at).toLocaleString()}</span>
                    </div>
                  )) : (
                    <div style={{ color: '#4a6a8e', fontSize: 11, padding: '2px 0 6px' }}>
                      No quota audits yet
                    </div>
                  )}
                  {showQuotaAuditLog && (
                    <div style={{ border: '1px solid rgba(255,255,255,0.08)', borderRadius: 8, background: 'rgba(255,255,255,0.02)' }}>
                      <AuditLogViewer defaultResourceType="org_quota" defaultResourceId={org.id} />
                    </div>
                  )}
                </div>
              )}
              {quotaFields.map(f => (
                <label key={f.key} style={{ display: 'flex', flexDirection: 'column', gap: 4, color: '#8ab0d0', fontSize: 11 }}>
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
              <div style={{ gridColumn: '1 / -1', display: 'flex', alignItems: 'center', gap: 8 }}>
                <span style={{ color: '#4a6a8e', fontSize: 11, flex: 1 }}>
                  Updated {new Date(quota.updated_at).toLocaleString()}
                </span>
                {isAdmin ? (
                  <button onClick={() => void handleSaveQuota()} disabled={quotaSaving} style={btnStyle}>
                    {quotaSaving ? 'Saving…' : 'Save quota'}
                  </button>
                ) : (
                  <span style={{ color: '#4a6a8e', fontSize: 11 }}>Read-only</span>
                )}
              </div>
            </div>
          )}
        </>
      )}
    </div>
  )
}

const quotaFields: { key: keyof OrgQuotaPatch; label: string }[] = [
  { key: 'max_members', label: 'Members' },
  { key: 'max_lakes', label: 'Lakes' },
  { key: 'max_nodes', label: 'Nodes' },
  { key: 'max_attachments', label: 'Attachments' },
  { key: 'max_api_keys', label: 'API Keys' },
  { key: 'max_storage_mb', label: 'Storage (MB)' },
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
      setError(e instanceof Error ? e.message : 'Failed to load organizations')
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
      setError(e instanceof Error ? e.message : 'Failed to create organization')
    } finally {
      setCreating2(false)
    }
  }, [newName, newSlug, newDesc, load])

  return (
    <div style={panelStyle}>
      {/* Header */}
      <div style={{ display: 'flex', alignItems: 'center', marginBottom: 14 }}>
        <span style={{ color: '#c0d8f0', fontWeight: 700, fontSize: 15, flex: 1 }}>Organizations</span>
        <button onClick={() => void load()} disabled={loading}
          style={{ ...btnStyle, marginRight: 8 }}>
          {loading ? '...' : 'Refresh'}
        </button>
        <button onClick={onClose} style={{ ...btnStyle, color: '#6a8aaa' }}>✕</button>
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
          <div style={{ display: 'flex', flexDirection: 'column', gap: 6, marginBottom: 12 }}>
            {orgs.map(org => {
              const latestAudit = orgLatestQuotaAudit(org)
              return (
                <div key={org.organization.id} style={{ ...memberRowStyle, alignItems: 'flex-start' }}>
                  <div style={{ flex: 1, display: 'flex', flexDirection: 'column', gap: 4 }}>
                    <div>
                      <span style={{ color: '#c0d8f0', fontSize: 13, fontWeight: 500 }}>{org.organization.name}</span>
                      <span style={{ color: '#4a6a8e', fontSize: 11, marginLeft: 6 }}>/{org.organization.slug}</span>
                    </div>
                    {org.quota.usage && (
                      <div style={{ display: 'flex', gap: 8, flexWrap: 'wrap', color: '#4a6a8e', fontSize: 10 }}>
                        <span>Members {org.quota.usage.members_used}/{org.quota.max_members}</span>
                        <span>Lakes {org.quota.usage.lakes_used}/{org.quota.max_lakes}</span>
                        <span>Nodes {org.quota.usage.nodes_used}/{org.quota.max_nodes}</span>
                      </div>
                    )}
                    {latestAudit && (
                      <div style={{ color: '#4a6a8e', fontSize: 10 }}>
                        Latest audit {new Date(latestAudit.created_at).toLocaleString()}
                      </div>
                    )}
                  </div>
                  <button onClick={() => setSelectedOrg(org)} style={btnStyle}>Members</button>
                </div>
              )
            })}
            {orgs.length === 0 && !loading && (
              <div style={{ color: '#4a6a8e', fontSize: 12, textAlign: 'center', padding: 16 }}>
                No organizations yet
              </div>
            )}
          </div>

          {/* Create form toggle */}
          {!creating ? (
            <button onClick={() => setCreating(true)} style={{ ...btnStyle, width: '100%' }}>
              + New Organization
            </button>
          ) : (
            <div style={{ display: 'flex', flexDirection: 'column', gap: 8, border: '1px solid #1e3050',
              borderRadius: 6, padding: 12 }}>
              <span style={{ color: '#8ab0d0', fontSize: 12, fontWeight: 600 }}>New Organization</span>
              <input
                placeholder="Name"
                value={newName}
                onChange={e => setNewName(e.target.value)}
                style={inputStyle}
              />
              <input
                placeholder="Slug (e.g. my-org)"
                value={newSlug}
                onChange={e => setNewSlug(e.target.value.toLowerCase().replace(/[^a-z0-9-]/g, ''))}
                style={inputStyle}
              />
              <input
                placeholder="Description (optional)"
                value={newDesc}
                onChange={e => setNewDesc(e.target.value)}
                style={inputStyle}
              />
              <div style={{ display: 'flex', gap: 8 }}>
                <button
                  onClick={() => void handleCreate()}
                  disabled={creating2 || !newName.trim() || !newSlug.trim()}
                  style={{ ...btnStyle, flex: 1, background: 'rgba(74,142,255,0.12)' }}
                >
                  {creating2 ? 'Creating...' : 'Create'}
                </button>
                <button onClick={() => setCreating(false)} style={btnStyle}>Cancel</button>
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
  background: '#0d1526',
  border: '1px solid #1e3050',
  borderRadius: 10,
  padding: 18,
  width: 380,
  maxHeight: '80vh',
  overflowY: 'auto',
  boxShadow: '0 4px 24px rgba(0,0,0,0.5)',
}

const memberRowStyle: React.CSSProperties = {
  display: 'flex',
  alignItems: 'center',
  gap: 8,
  padding: '6px 8px',
  background: 'rgba(255,255,255,0.03)',
  borderRadius: 5,
  border: '1px solid #1a2e4a',
}

const btnStyle: React.CSSProperties = {
  background: 'none',
  border: '1px solid #2a4a7e',
  borderRadius: 4,
  color: '#6a9ab0',
  cursor: 'pointer',
  padding: '3px 10px',
  fontSize: 11,
}

const inputStyle: React.CSSProperties = {
  background: '#081020',
  border: '1px solid #1e3050',
  borderRadius: 4,
  color: '#c0d8f0',
  padding: '5px 8px',
  fontSize: 12,
  width: '100%',
  boxSizing: 'border-box',
  outline: 'none',
}

const selectStyle: React.CSSProperties = {
  background: '#081020',
  border: '1px solid #1e3050',
  borderRadius: 4,
  color: '#c0d8f0',
  padding: '3px 6px',
  fontSize: 11,
  cursor: 'pointer',
}

function ErrorMsg({ children }: { children: React.ReactNode }) {
  return (
    <div style={{
      color: '#ff6b6b',
      fontSize: 12,
      marginBottom: 8,
      padding: '4px 8px',
      background: 'rgba(255,107,107,0.1)',
      borderRadius: 4,
    }}>
      {children}
    </div>
  )
}
