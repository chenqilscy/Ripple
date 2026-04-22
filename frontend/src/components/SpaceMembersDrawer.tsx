import { useEffect, useState } from 'react'
import { api, type Space, type SpaceMember } from '../api/client'
import { prompt as modalPrompt } from './Modal'

export interface SpaceMembersDrawerProps {
  space: Space
  onClose: () => void
}

/**
 * SpaceMembersDrawer：右侧抽屉，列出空间成员，支持添加/移除（仅 OWNER）。
 *
 * 设计：
 *   - 任何成员可读列表
 *   - 仅 OWNER 看到 "+" 按钮和每行的删除按钮
 *   - 添加成员需要输入对方 user_id（M3-S1 不做用户搜索；M3-S4 加邮箱搜索）
 */
export default function SpaceMembersDrawer(props: SpaceMembersDrawerProps) {
  const [members, setMembers] = useState<SpaceMember[]>([])
  const [loading, setLoading] = useState(false)
  const [err, setErr] = useState<string | null>(null)
  const isOwner = props.space.role === 'OWNER'

  async function refresh() {
    setLoading(true)
    setErr(null)
    try {
      const r = await api.listSpaceMembers(props.space.id)
      setMembers(r.members ?? [])
    } catch (e) {
      setErr((e as Error).message)
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => { void refresh() }, [props.space.id])

  async function handleAdd() {
    const uid = await modalPrompt({
      title: '邀请成员',
      label: '输入用户的 UUID（M3-S4 将支持邮箱邀请）',
      placeholder: '例如：8400ec3a-…',
      validate: v => {
        const s = v.trim()
        // UUID v4 标准 36 字符，含 4 个连字符；放宽允许大小写
        const uuidRe = /^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$/
        return uuidRe.test(s) ? null : 'UUID 格式无效（需 36 位含连字符）'
      },
    })
    if (!uid) return
    const role = await modalPrompt({
      title: '设定权限',
      label: '输入 EDITOR（可写）或 VIEWER（只读）',
      initial: 'EDITOR',
      validate: v => (v === 'EDITOR' || v === 'VIEWER') ? null : '必须是 EDITOR 或 VIEWER',
    })
    if (!role) return
    try {
      await api.addSpaceMember(props.space.id, uid.trim(), role as 'EDITOR' | 'VIEWER')
      await refresh()
    } catch (e) {
      setErr((e as Error).message)
    }
  }

  async function handleRemove(userId: string) {
    if (!confirm('移除该成员？该用户将立即失去访问权限。')) return
    try {
      await api.removeSpaceMember(props.space.id, userId)
      await refresh()
    } catch (e) {
      setErr((e as Error).message)
    }
  }

  return (
    <div
      style={{
        position: 'fixed', top: 0, right: 0, bottom: 0, width: 360,
        background: '#161616', borderLeft: '1px solid #2a2a2a', zIndex: 1000,
        display: 'flex', flexDirection: 'column', boxShadow: '-4px 0 12px rgba(0,0,0,0.4)',
      }}
    >
      <div style={{ padding: '14px 16px', borderBottom: '1px solid #2a2a2a', display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
        <div style={{ minWidth: 0 }}>
          <div style={{ fontSize: 14, color: '#e6e6e6', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
            {props.space.name}
          </div>
          <div style={{ fontSize: 11, color: '#666' }}>成员管理</div>
        </div>
        <button onClick={props.onClose} style={{ background: 'transparent', border: 'none', color: '#999', cursor: 'pointer', fontSize: 18 }}>×</button>
      </div>

      {isOwner && (
        <div style={{ padding: '8px 16px', borderBottom: '1px solid #2a2a2a' }}>
          <button
            onClick={handleAdd}
            style={{
              width: '100%', padding: '8px', background: '#1d2433', border: '1px solid #4a8eff',
              color: '#4a8eff', borderRadius: 4, cursor: 'pointer', fontSize: 13,
            }}
          >+ 邀请成员</button>
        </div>
      )}

      {err && <div style={{ padding: '8px 16px', color: '#e66', fontSize: 12 }}>{err}</div>}
      {loading && <div style={{ padding: '12px 16px', color: '#666', fontSize: 12 }}>加载中…</div>}

      <ul style={{ listStyle: 'none', margin: 0, padding: 0, overflowY: 'auto', flex: 1 }}>
        {members.map(m => (
          <li
            key={m.user_id}
            style={{
              padding: '10px 16px', borderBottom: '1px solid #222',
              display: 'flex', justifyContent: 'space-between', alignItems: 'center',
            }}
          >
            <div style={{ minWidth: 0, flex: 1 }}>
              <div style={{ fontSize: 12, color: '#ccc', fontFamily: 'monospace', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
                {m.user_id}
              </div>
              <div style={{ fontSize: 10, color: '#666', marginTop: 2 }}>{m.role}</div>
            </div>
            {isOwner && m.role !== 'OWNER' && (
              <button
                onClick={() => handleRemove(m.user_id)}
                title="移除成员"
                style={{ background: 'transparent', border: 'none', color: '#e66', cursor: 'pointer', fontSize: 13 }}
              >移除</button>
            )}
          </li>
        ))}
        {!loading && members.length === 0 && (
          <li style={{ padding: '20px 16px', color: '#666', fontSize: 12, textAlign: 'center' }}>
            暂无成员
          </li>
        )}
      </ul>
    </div>
  )
}
