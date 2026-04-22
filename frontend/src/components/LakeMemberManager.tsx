/**
 * P11-C: Lake member role management panel.
 * Only shown when the current user is OWNER of the lake.
 * Allows changing roles of other members (not to OWNER).
 */
import { useCallback, useEffect, useState } from 'react'
import { api } from '../api/client'
import type { LakeMember, LakeRole } from '../api/types'

interface Props {
  lakeId: string
  currentUserId: string
  currentRole: LakeRole
}

const ROLE_OPTIONS: LakeRole[] = ['NAVIGATOR', 'PASSENGER', 'OBSERVER']

const ROLE_COLOR: Record<LakeRole, string> = {
  OWNER:     '#f5a623',
  NAVIGATOR: '#52c41a',
  PASSENGER: '#4a8eff',
  OBSERVER:  '#888899',
}

export default function LakeMemberManager({ lakeId, currentUserId, currentRole }: Props) {
  const [members, setMembers] = useState<LakeMember[]>([])
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [updating, setUpdating] = useState<string | null>(null)
  const [removing, setRemoving] = useState<string | null>(null)

  const load = useCallback(async () => {
    setLoading(true)
    setError(null)
    try {
      const res = await api.listLakeMembers(lakeId)
      setMembers(res.members)
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : 'Failed to load members')
    } finally {
      setLoading(false)
    }
  }, [lakeId])

  useEffect(() => { void load() }, [load])

  const handleRemove = useCallback(async (userId: string) => {
    if (!window.confirm('确认移除该成员？')) return
    setRemoving(userId)
    setError(null)
    try {
      await api.removeLakeMember(lakeId, userId)
      setMembers(prev => prev.filter(m => m.user_id !== userId))
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : '移除失败')
    } finally {
      setRemoving(null)
    }
  }, [lakeId])

  const handleRoleChange = useCallback(async (userId: string, newRole: LakeRole) => {
    setUpdating(userId)
    setError(null)
    try {
      await api.updateMemberRole(lakeId, userId, newRole)
      setMembers(prev => prev.map(m => m.user_id === userId ? { ...m, role: newRole } : m))
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : 'Failed to update role')
    } finally {
      setUpdating(null)
    }
  }, [lakeId])

  const isOwner = currentRole === 'OWNER'

  return (
    <div
      style={{
        background: '#0d1526',
        border: '1px solid #1e3050',
        borderRadius: 8,
        padding: 16,
        minWidth: 340,
        maxWidth: 480,
      }}
    >
      <div
        style={{
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'space-between',
          marginBottom: 12,
        }}
      >
        <span style={{ color: '#c0d8f0', fontWeight: 600, fontSize: 14 }}>
          湖成员管理
        </span>
        <button
          onClick={() => void load()}
          disabled={loading}
          style={{
            background: 'none',
            border: '1px solid #2a4a7e',
            borderRadius: 4,
            color: '#6a9ab0',
            cursor: 'pointer',
            padding: '2px 8px',
            fontSize: 11,
          }}
        >
          {loading ? '…' : '刷新'}
        </button>
      </div>

      {error && (
        <div
          style={{
            color: '#ff6b6b',
            fontSize: 12,
            marginBottom: 8,
            padding: '4px 8px',
            background: 'rgba(255,107,107,0.1)',
            borderRadius: 4,
          }}
        >
          {error}
        </div>
      )}

      {members.length === 0 && !loading && (
        <div style={{ color: '#4a6a8e', fontSize: 12, textAlign: 'center', padding: 16 }}>
          暂无成员
        </div>
      )}

      <table style={{ width: '100%', borderCollapse: 'collapse', fontSize: 12 }}>
        <thead>
          <tr style={{ borderBottom: '1px solid #1e3050' }}>
            <th style={{ textAlign: 'left', padding: '4px 6px', color: '#4a6a8e', fontWeight: 500 }}>用户 ID</th>
            <th style={{ textAlign: 'left', padding: '4px 6px', color: '#4a6a8e', fontWeight: 500 }}>角色</th>
            {isOwner && (
              <th style={{ textAlign: 'left', padding: '4px 6px', color: '#4a6a8e', fontWeight: 500 }}>变更</th>
            )}
            {isOwner && (
              <th style={{ padding: '4px 6px' }} />
            )}
          </tr>
        </thead>
        <tbody>
          {members.map(m => (
            <tr key={m.user_id} style={{ borderBottom: '1px solid rgba(30,48,80,0.5)' }}>
              <td
                style={{
                  padding: '6px 6px',
                  color: m.user_id === currentUserId ? '#89dceb' : '#8ab0c8',
                  fontFamily: 'monospace',
                  fontSize: 11,
                  maxWidth: 160,
                  overflow: 'hidden',
                  textOverflow: 'ellipsis',
                  whiteSpace: 'nowrap',
                }}
                title={m.user_id}
              >
                {m.user_id.slice(0, 8)}&hellip;
                {m.user_id === currentUserId && (
                  <span style={{ color: '#89dceb', marginLeft: 4, fontSize: 10 }}>（你）</span>
                )}
              </td>
              <td style={{ padding: '6px 6px' }}>
                <span
                  style={{
                    color: ROLE_COLOR[m.role] ?? '#888',
                    fontWeight: 500,
                    fontSize: 11,
                  }}
                >
                  {m.role}
                </span>
              </td>
              {isOwner && (
                <td style={{ padding: '6px 6px' }}>
                  {m.role === 'OWNER' || m.user_id === currentUserId ? (
                    <span style={{ color: '#334466', fontSize: 11 }}>—</span>
                  ) : (
                    <select
                      value={m.role}
                      disabled={updating === m.user_id}
                      onChange={e => void handleRoleChange(m.user_id, e.target.value as LakeRole)}
                      style={{
                        background: '#0a1020',
                        border: '1px solid #2a4a7e',
                        borderRadius: 3,
                        color: '#c0d8f0',
                        fontSize: 11,
                        padding: '2px 4px',
                        cursor: 'pointer',
                        opacity: updating === m.user_id ? 0.5 : 1,
                      }}
                    >
                      {ROLE_OPTIONS.map(r => (
                        <option key={r} value={r}>{r}</option>
                      ))}
                    </select>
                  )}
                </td>
              )}
              {isOwner && (
                <td style={{ padding: '6px 6px' }}>
                  {m.role !== 'OWNER' && m.user_id !== currentUserId && (
                    <button
                      disabled={removing === m.user_id}
                      onClick={() => void handleRemove(m.user_id)}
                      style={{
                        background: 'rgba(220,53,69,0.12)',
                        border: '1px solid rgba(220,53,69,0.3)',
                        borderRadius: 3, color: '#ff6b7a',
                        cursor: 'pointer', padding: '1px 6px', fontSize: 11,
                      }}
                    >
                      {removing === m.user_id ? '…' : '移除'}
                    </button>
                  )}
                </td>
              )}
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  )
}
