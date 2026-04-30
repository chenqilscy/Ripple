// PresencePanel.tsx — 协作者头像列表
// 在图谱侧边栏显示当前在线用户头像

interface PresencePanelProps {
  /** 在线用户列表（来自 Home.tsx state） */
  onlineUsers: string[]
  /** 用户 ID → 详细信息（可选） */
  userDetails?: Map<string, { displayName?: string; avatarUrl?: string }>
  /** 当前用户 ID，用于排除自己 */
  currentUserId?: string
}

const PRESENCE_COLORS = ['#ff6b6b', '#4ecdc4', '#ffd93d', '#a8e6cf', '#ff8b94', '#b3cde0']

function userColor(userId: string): string {
  let h = 0
  for (let i = 0; i < userId.length; i++) h = (h * 31 + userId.charCodeAt(i)) >>> 0
  return PRESENCE_COLORS[h % PRESENCE_COLORS.length]
}

export default function PresencePanel({
  onlineUsers,
  userDetails,
  currentUserId,
}: PresencePanelProps) {
  if (onlineUsers.length === 0) return null

  // 排除自己
  const others = currentUserId
    ? onlineUsers.filter(uid => uid !== currentUserId)
    : onlineUsers

  return (
    <div style={{
      position: 'absolute',
      top: 52,
      left: 12,
      zIndex: 25,
      display: 'flex',
      flexDirection: 'column',
      gap: 4,
    }}>
      {/* 在线用户计数 */}
      <div style={{
        fontSize: 10,
        color: '#4a8eff',
        background: 'rgba(0,0,0,0.6)',
        padding: '2px 6px',
        borderRadius: 4,
        display: 'flex',
        alignItems: 'center',
        gap: 4,
      }}>
        <span style={{ color: '#52c41a', fontSize: 8 }}>●</span>
        <span>{others.length} 人在线</span>
      </div>

      {/* 头像列表 */}
      <div style={{ display: 'flex', flexDirection: 'column', gap: 4 }}>
        {others.map(uid => {
          const color = userColor(uid)
          const details = userDetails?.get(uid)
          const initials = (details?.displayName ?? uid.slice(0, 2)).toUpperCase()

          return (
            <div
              key={uid}
              title={details?.displayName ?? uid}
              style={{
                width: 28,
                height: 28,
                borderRadius: '50%',
                background: color,
                border: '2px solid rgba(255,255,255,0.2)',
                display: 'flex',
                alignItems: 'center',
                justifyContent: 'center',
                fontSize: 10,
                fontWeight: 600,
                color: '#fff',
                cursor: 'default',
                boxShadow: `0 2px 8px ${color}40`,
              }}
            >
              {initials.slice(0, 2)}
            </div>
          )
        })}
      </div>
    </div>
  )
}