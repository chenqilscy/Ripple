/**
 * SpaceSwitcher：侧栏组件，列出当前用户的所有 Space + "个人湖"。
 * 修复：scroll lock + sticky header + CSS 变量
 *
 * 行为：
 *   - 切换 → 触发 onChange（父组件应重新拉 lakes 列表）
 *   - "+" → 创建新空间（modal prompt）
 *   - 行内 "成员" → 触发 onManageMembers
 */
import { useEffect, useState } from 'react'
import { api, type Space } from '../api/client'
import { prompt as modalPrompt, confirm as modalConfirm } from './Modal'

export interface SpaceSwitcherProps {
  currentSpaceId: string
  onChange: (spaceId: string) => void
  onManageMembers: (space: Space) => void
}

export default function SpaceSwitcher(props: SpaceSwitcherProps) {
  const [spaces, setSpaces] = useState<Space[]>([])
  const [loading, setLoading] = useState(false)
  const [err, setErr] = useState<string | null>(null)

  // Scroll lock
  useEffect(() => {
    const prev = document.body.style.overflow
    document.body.style.overflow = 'hidden'
    return () => { document.body.style.overflow = prev }
  }, [])

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
    <div style={{
      borderBottom: '1px solid var(--border)',
      display: 'flex',
      flexDirection: 'column',
    }}>
      {/* Sticky header */}
      <div style={{
        position: 'sticky', top: 0,
        background: 'var(--bg-primary)',
        display: 'flex', alignItems: 'center', justifyContent: 'space-between',
        padding: 'var(--space-sm) var(--space-md) var(--space-xs)',
        zIndex: 1,
        borderBottom: '1px solid var(--border-subtle)',
      }}>
        <span style={{ fontSize: 'var(--font-xs)', color: 'var(--text-tertiary)', textTransform: 'uppercase', letterSpacing: '0.06em' }}>空间</span>
        <button
          onClick={handleCreate}
          title="创建空间"
          aria-label="创建空间"
          style={{
            background: 'transparent', border: '1px solid var(--border)',
            color: 'var(--text-tertiary)',
            borderRadius: 'var(--radius-sm)', width: 22, height: 22,
            cursor: 'pointer', fontSize: 'var(--font-lg)', lineHeight: 1,
            display: 'flex', alignItems: 'center', justifyContent: 'center',
          }}
        >+</button>
      </div>

      {/* List — independent scroll */}
      <div style={{ overflowY: 'auto', flex: 1, minHeight: 0 }}>
        {loading && (
          <div style={{ padding: 'var(--space-sm) var(--space-md)', color: 'var(--text-tertiary)', fontSize: 'var(--font-sm)' }}>
            加载中…
          </div>
        )}
        {err && (
          <div style={{ padding: 'var(--space-sm) var(--space-md)', color: 'var(--status-danger)', fontSize: 'var(--font-sm)' }}>
            {err}
          </div>
        )}
        <ul style={{ listStyle: 'none', margin: 0, padding: 'var(--space-xs) 0' }}>
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
  const barColor = ratio > 0.9 ? 'var(--status-danger)' : ratio > 0.7 ? 'var(--status-warning)' : 'var(--accent)'
  return (
    <li
      onClick={p.onClick}
      style={{
        display: 'flex', alignItems: 'center', justifyContent: 'space-between',
        padding: 'var(--space-sm) var(--space-md)', cursor: 'pointer',
        background: p.active ? 'var(--accent-subtle)' : 'transparent',
        borderLeft: p.active ? '3px solid var(--accent)' : '3px solid transparent',
        color: p.active ? 'var(--text-primary)' : 'var(--text-secondary)',
        transition: 'background 0.15s, color 0.15s',
      }}
    >
      <div style={{ display: 'flex', flexDirection: 'column', gap: 2, minWidth: 0, flex: 1 }}>
        <span style={{ fontSize: 'var(--font-base)', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
          {p.name}
        </span>
        {p.sub && (
          <span style={{ fontSize: 'var(--font-xs)', color: 'var(--text-tertiary)' }}>{p.sub}</span>
        )}
        {showQuota && (
          <div
            title={`${p.quotaUsed || 0} / ${p.quotaTotal} tokens`}
            style={{ marginTop: 2, height: 3, background: 'var(--bg-tertiary)', borderRadius: 2, overflow: 'hidden' }}
          >
            <div style={{ width: `${ratio * 100}%`, height: '100%', background: barColor }} />
          </div>
        )}
      </div>
      <div style={{ display: 'flex', alignItems: 'center', gap: 2, flexShrink: 0 }}>
        {p.onMembers && (
          <button
            onClick={e => { e.stopPropagation(); p.onMembers!() }}
            title="管理成员"
            aria-label="管理成员"
            style={{
              background: 'transparent', border: 'none', color: 'var(--text-tertiary)',
              cursor: 'pointer', fontSize: 'var(--font-lg)', padding: '0 var(--space-xs)',
            }}
          >
            👥
          </button>
        )}
        {p.isOwner && p.onDelete && (
          <button
            onClick={e => { e.stopPropagation(); p.onDelete!() }}
            title="删除空间"
            aria-label="删除空间"
            style={{
              background: 'transparent', border: 'none', color: 'var(--text-tertiary)',
              cursor: 'pointer', fontSize: 'var(--font-sm)', padding: '0 var(--space-xs)',
            }}
          >
            🗑
          </button>
        )}
      </div>
    </li>
  )
}