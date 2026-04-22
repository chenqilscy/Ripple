/**
 * P12-C: Organization list, creation form, and member management panel.
 */
import { useCallback, useEffect, useState } from 'react'
import { api } from '../api/client'
import type { OrgMember, OrgRole, Organization } from '../api/types'

const ROLE_COLOR: Record<OrgRole, string> = {
  OWNER:  '#f5a623',
  ADMIN:  '#52c41a',
  MEMBER: '#4a8eff',
}

const INVITE_ROLE_OPTIONS: OrgRole[] = ['ADMIN', 'MEMBER']

interface MemberListProps {
  org: Organization
  currentUserId: string
  onBack: () => void
}

function OrgMemberList({ org, currentUserId, onBack }: MemberListProps) {
  const [members, setMembers] = useState<OrgMember[]>([])
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [updating, setUpdating] = useState<string | null>(null)
  const [addUserId, setAddUserId] = useState('')
  const [addRole, setAddRole] = useState<OrgRole>('MEMBER')
  const [adding, setAdding] = useState(false)

  const currentMember = members.find(m => m.user_id === currentUserId)
  const isAdmin = currentMember?.role === 'OWNER' || currentMember?.role === 'ADMIN'

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

  useEffect(() => { void load() }, [load])

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

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 12 }}>
      <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
        <button onClick={onBack} style={btnStyle}>← Back</button>
        <span style={{ color: '#c0d8f0', fontWeight: 600, fontSize: 14 }}>
          {org.name} · Members
        </span>
        <button onClick={() => void load()} disabled={loading}
          style={{ ...btnStyle, marginLeft: 'auto' }}>
          {loading ? '...' : 'Refresh'}
        </button>
      </div>

      {error && <ErrorMsg>{error}</ErrorMsg>}

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
        <div style={{ display: 'flex', gap: 6, marginTop: 4, flexWrap: 'wrap' }}>
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
      )}
    </div>
  )
}

interface Props {
  currentUserId: string
  onClose: () => void
}

export default function OrgPanel({ currentUserId, onClose }: Props) {
  const [orgs, setOrgs] = useState<Organization[]>([])
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [creating, setCreating] = useState(false)
  const [newName, setNewName] = useState('')
  const [newSlug, setNewSlug] = useState('')
  const [newDesc, setNewDesc] = useState('')
  const [creating2, setCreating2] = useState(false)
  const [selectedOrg, setSelectedOrg] = useState<Organization | null>(null)

  const load = useCallback(async () => {
    setLoading(true)
    setError(null)
    try {
      const res = await api.listOrgs()
      setOrgs(res.organizations)
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
      const org = await api.createOrg(name, slug, newDesc.trim())
      setOrgs(prev => [org, ...prev])
      setNewName('')
      setNewSlug('')
      setNewDesc('')
      setCreating(false)
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : 'Failed to create organization')
    } finally {
      setCreating2(false)
    }
  }, [newName, newSlug, newDesc])

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
          org={selectedOrg}
          currentUserId={currentUserId}
          onBack={() => setSelectedOrg(null)}
        />
      ) : (
        <>
          {/* Org list */}
          <div style={{ display: 'flex', flexDirection: 'column', gap: 6, marginBottom: 12 }}>
            {orgs.map(org => (
              <div key={org.id} style={memberRowStyle}>
                <div style={{ flex: 1 }}>
                  <span style={{ color: '#c0d8f0', fontSize: 13, fontWeight: 500 }}>{org.name}</span>
                  <span style={{ color: '#4a6a8e', fontSize: 11, marginLeft: 6 }}>/{org.slug}</span>
                </div>
                <button onClick={() => setSelectedOrg(org)} style={btnStyle}>Members</button>
              </div>
            ))}
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
