// NotificationBell — P13-B 通知铃铛 + 下拉列表
import { useCallback, useEffect, useRef, useState } from 'react'
import { api } from '../api/client'
import type { Notification } from '../api/types'

const POLL_MS = 30_000

export default function NotificationBell() {
  const [count, setCount] = useState(0)
  const [open, setOpen] = useState(false)
  const [items, setItems] = useState<Notification[]>([])
  const [loading, setLoading] = useState(false)
  const dropRef = useRef<HTMLDivElement>(null)

  const refreshCount = useCallback(async () => {
    try {
      const { count } = await api.getUnreadNotificationCount()
      setCount(count)
    } catch { /* 静默 */ }
  }, [])

  // 轮询未读数（每 30s）
  useEffect(() => {
    void refreshCount()
    const id = setInterval(() => void refreshCount(), POLL_MS)
    return () => clearInterval(id)
  }, [refreshCount])

  // 点击外部关闭
  useEffect(() => {
    if (!open) return
    function onOutside(e: MouseEvent) {
      if (dropRef.current && !dropRef.current.contains(e.target as Node)) {
        setOpen(false)
      }
    }
    document.addEventListener('mousedown', onOutside)
    return () => document.removeEventListener('mousedown', onOutside)
  }, [open])

  async function toggleOpen() {
    if (open) { setOpen(false); return }
    setOpen(true)
    setLoading(true)
    try {
      const { notifications } = await api.listNotifications(20)
      setItems(notifications)
      // 打开后刷新一次未读数
      void refreshCount()
    } catch { /* 静默 */ }
    finally { setLoading(false) }
  }

  async function markRead(id: number) {
    try {
      await api.markNotificationRead(id)
      setItems(prev => prev.map(n => n.id === id ? { ...n, is_read: true } : n))
      setCount(c => Math.max(0, c - 1))
    } catch { /* 静默 */ }
  }

  async function markAll() {
    try {
      await api.markAllNotificationsRead()
      setItems(prev => prev.map(n => ({ ...n, is_read: true })))
      setCount(0)
    } catch { /* 静默 */ }
  }

  return (
    <div ref={dropRef} style={{ position: 'relative', display: 'inline-block' }}>
      <button
        onClick={toggleOpen}
        title="通知"
        style={{
          background: 'none', border: 'none', cursor: 'pointer',
          color: '#cdd6f4', fontSize: 16, padding: '4px 6px',
          position: 'relative',
        }}
      >
        🔔
        {count > 0 && (
          <span style={{
            position: 'absolute', top: 0, right: 0,
            background: '#f38ba8', color: '#1e1e2e',
            borderRadius: '50%', fontSize: 10, fontWeight: 700,
            width: 16, height: 16, lineHeight: '16px', textAlign: 'center',
            pointerEvents: 'none',
          }}>
            {count > 99 ? '99+' : count}
          </span>
        )}
      </button>

      {open && (
        <div style={{
          position: 'absolute', right: 0, top: '110%', zIndex: 1000,
          width: 320, maxHeight: 400, overflowY: 'auto',
          background: '#1e1e2e', border: '1px solid #313244',
          borderRadius: 8, boxShadow: '0 8px 24px rgba(0,0,0,0.5)',
          padding: '8px 0',
        }}>
          <div style={{
            display: 'flex', justifyContent: 'space-between', alignItems: 'center',
            padding: '4px 12px 8px',
          }}>
            <span style={{ fontSize: 12, fontWeight: 600, color: '#89dceb' }}>通知</span>
            {items.some(n => !n.is_read) && (
              <button onClick={markAll} style={{
                background: 'none', border: 'none', cursor: 'pointer',
                color: '#6c7086', fontSize: 11,
              }}>
                全部已读
              </button>
            )}
          </div>

          {loading && (
            <div style={{ padding: '16px', textAlign: 'center', color: '#6c7086', fontSize: 12 }}>
              加载中…
            </div>
          )}
          {!loading && items.length === 0 && (
            <div style={{ padding: '16px', textAlign: 'center', color: '#6c7086', fontSize: 12 }}>
              暂无通知
            </div>
          )}
          {!loading && items.map(n => (
            <div
              key={n.id}
              onClick={() => { if (!n.is_read) void markRead(n.id) }}
              style={{
                padding: '8px 12px',
                cursor: n.is_read ? 'default' : 'pointer',
                background: n.is_read ? 'transparent' : 'rgba(74,144,226,0.06)',
                borderLeft: n.is_read ? '3px solid transparent' : '3px solid #4a90e2',
                transition: 'background 0.15s',
              }}
            >
              <div style={{ fontSize: 12, color: '#cdd6f4', lineHeight: 1.5 }}>
                {formatNotif(n)}
              </div>
              <div style={{ fontSize: 10, color: '#6c7086', marginTop: 2 }}>
                {new Date(n.created_at).toLocaleString('zh-CN')}
              </div>
            </div>
          ))}
        </div>
      )}
    </div>
  )
}

function formatNotif(n: Notification): string {
  const p = n.payload
  switch (n.type) {
    case 'lake.invite_accepted': return `用户 ${p['invitee_id'] ?? '?'} 接受了邀请加入湖 ${p['lake_id'] ?? '?'}`
    case 'lake.member_removed':  return `你已被移出湖 ${p['lake_id'] ?? '?'}`
    case 'lake.role_updated':    return `你在湖 ${p['lake_id'] ?? '?'} 的角色已更新为 ${p['role'] ?? '?'}`
    default:                     return `[${n.type}] ${JSON.stringify(p)}`
  }
}
