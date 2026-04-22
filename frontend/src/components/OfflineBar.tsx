import { useEffect, useState } from 'react'

/** 离线状态横幅 — 网络断开时显示在顶部。P12-E */
export default function OfflineBar() {
  const [offline, setOffline] = useState(!navigator.onLine)

  useEffect(() => {
    const goOffline = () => setOffline(true)
    const goOnline = () => setOffline(false)
    window.addEventListener('offline', goOffline)
    window.addEventListener('online', goOnline)
    return () => {
      window.removeEventListener('offline', goOffline)
      window.removeEventListener('online', goOnline)
    }
  }, [])

  if (!offline) return null

  return (
    <div style={{
      position: 'fixed',
      top: 0,
      left: 0,
      right: 0,
      background: '#f38ba8',
      color: '#1e1e2e',
      textAlign: 'center',
      padding: '6px 12px',
      fontSize: 13,
      fontWeight: 600,
      zIndex: 9999,
      letterSpacing: 1,
    }}>
      ⚠ 当前处于离线模式 — 数据来自本地缓存，写操作将在恢复网络后同步
    </div>
  )
}
