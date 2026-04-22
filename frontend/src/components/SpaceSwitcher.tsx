import { useEffect, useState } from 'react'
import { api, type Space } from '../api/client'
import { prompt as modalPrompt, confirm as modalConfirm } from './Modal'

export interface SpaceSwitcherProps {
  // 当前选中的 spaceId（''=个人湖）
  currentSpaceId: string
  onChange: (spaceId: string) => void
  onManageMembers: (space: Space) => void
}

/**
 * SpaceSwitcher：侧栏组件，列出当前用户的所有 Space + "个人湖"。
 *
 * 行为：
 *   - 切换 → 触发 onChange（父组件应重新拉 lakes 列表）
 *   - "+" → 创建新空间（modal prompt）
 *   - 行内 "成员" → 触发 onManageMembers
 */
export default function SpaceSwitcher(props: SpaceSwitcherProps) {
  const [spaces, setSpaces] = useState<Space[]>([])
  const [loading, setLoading] = useState(false)
  const [err, setErr] = useState<string | null>(null)

  async function refresh() {
    setLoading(true)
    setErr(null)
    try {
      const r = await api.listSpaces()
      setSpaces(r.spaces ?? [])
    } catch (e) {
      setErr((e as Error).message)
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => { void refresh() }, [])

  async function handleCreate() {
    const name = await modalPrompt({
      title: '创建空间',
      label: '空间是组织多个湖的容器，可以邀请成员协作。',
      placeholder: '空间名称（≤ 64 字）',
      validate: v => !v.trim() ? '名称不能为空' : null,
    })
    if (!name) return
    const desc = await modalPrompt({
      title: '空间描述（可选）',
      placeholder: '简单描述这个空间的用途',
      initial: '',
    })
    try {
      const sp = await api.createSpace(name.trim(), desc?.trim() ?? '')
      await refresh()
      props.onChange(sp.id)
    } catch (e) {
      setErr((e as Error).message)
    }
  }

  return (
    <div style={{ padding: '8px 0', borderBottom: '1px solid #2a2a2a' }}>
      <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', padding: '0 12px 6px' }}>
        <span style={{ fontSize: 12, color: '#888', textTransform: 'uppercase', letterSpacing: 1 }}>空间</span>
        <button
          onClick={handleCreate}
          title="创建空间"
          style={{
            background: 'transparent', border: '1px solid #3a3a3a', color: '#aaa',
            borderRadius: 4, width: 22, height: 22, cursor: 'pointer', fontSize: 14,
          }}
        >+</button>
      </div>
      {loading && <div style={{ padding: '0 12px', color: '#666', fontSize: 12 }}>加载中…</div>}
      {err && <div style={{ padding: '0 12px', color: '#e66', fontSize: 12 }}>{err}</div>}
      <ul style={{ listStyle: 'none', margin: 0, padding: 0 }}>
        <SpaceRow
          name="📌 个人湖"
          active={props.currentSpaceId === ''}
          onClick={() => props.onChange('')}
        />
        {spaces.map(s => (
          <SpaceRow
            key={s.id}
            name={s.name}
            sub={s.role === 'OWNER' ? '所有者' : s.role === 'EDITOR' ? '编辑' : '查看'}
            quotaUsed={s.llm_used_current_month}
            quotaTotal={s.llm_quota_monthly}
            active={props.currentSpaceId === s.id}
            isOwner={s.role === 'OWNER'}
            onClick={() => props.onChange(s.id)}
            onMembers={() => props.onManageMembers(s)}
            onDelete={async () => {
              const ok = await modalConfirm(
                `确定删除空间「${s.name}」？此操作不可撤销。\n空间下的湖不会被删除（会变成个人湖）。`,
                { title: '删除空间', danger: true },
              )
              if (!ok) return
              try {
                await api.deleteSpace(s.id)
                if (props.currentSpaceId === s.id) props.onChange('')
                await refresh()
              } catch (e) {
                setErr((e as Error).message)
              }
            }}
          />
        ))}
      </ul>
    </div>
  )
}

function SpaceRow(p: {
  name: string
  sub?: string
  quotaUsed?: number
  quotaTotal?: number
  active: boolean
  isOwner?: boolean
  onClick: () => void
  onMembers?: () => void
  onDelete?: () => void
}) {
  const showQuota = p.quotaTotal !== undefined && p.quotaTotal > 0
  const ratio = showQuota ? Math.min(1, (p.quotaUsed || 0) / p.quotaTotal!) : 0
  const barColor = ratio > 0.9 ? '#d24343' : ratio > 0.7 ? '#e0a23a' : '#4a8eff'
  return (
    <li
      onClick={p.onClick}
      style={{
        display: 'flex', alignItems: 'center', justifyContent: 'space-between',
        padding: '6px 12px', cursor: 'pointer',
        background: p.active ? '#1d2433' : 'transparent',
        borderLeft: p.active ? '3px solid #4a8eff' : '3px solid transparent',
        color: p.active ? '#e6e6e6' : '#bbb',
      }}
    >
      <div style={{ display: 'flex', flexDirection: 'column', gap: 2, minWidth: 0, flex: 1 }}>
        <span style={{ fontSize: 13, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>{p.name}</span>
        {p.sub && <span style={{ fontSize: 10, color: '#666' }}>{p.sub}</span>}
        {showQuota && (
          <div title={`${p.quotaUsed || 0} / ${p.quotaTotal} tokens`}
            style={{ marginTop: 2, height: 3, background: '#2a2a2a', borderRadius: 2, overflow: 'hidden' }}>
            <div style={{ width: `${ratio * 100}%`, height: '100%', background: barColor }} />
          </div>
        )}
      </div>
      <div style={{ display: 'flex', alignItems: 'center', gap: 2 }}>
        {p.onMembers && (
          <button
            onClick={e => { e.stopPropagation(); p.onMembers!() }}
            title="管理成员"
            style={{
              background: 'transparent', border: 'none', color: '#888', cursor: 'pointer',
              fontSize: 14, padding: '0 4px',
            }}
          >👥</button>
        )}
        {p.isOwner && p.onDelete && (
          <button
            onClick={e => { e.stopPropagation(); p.onDelete!() }}
            title="删除空间"
            style={{
              background: 'transparent', border: 'none', color: '#888', cursor: 'pointer',
              fontSize: 12, padding: '0 4px',
            }}
          >🗑</button>
        )}
      </div>
    </li>
  )
}
