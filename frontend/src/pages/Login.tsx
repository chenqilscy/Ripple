import { useState } from 'react'
import { api, type ApiError } from '../api/client'

interface Props {
  onSuccess: () => void
}

export function Login({ onSuccess }: Props) {
  const [mode, setMode] = useState<'login' | 'register'>('login')
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [displayName, setDisplayName] = useState('')
  const [busy, setBusy] = useState(false)
  const [err, setErr] = useState<string | null>(null)

  async function submit(e: React.FormEvent) {
    e.preventDefault()
    setErr(null); setBusy(true)
    try {
      if (mode === 'register') {
        await api.register(email, password, displayName || email.split('@')[0])
      }
      await api.login(email, password)
      onSuccess()
    } catch (e) {
      const ae = e as ApiError
      setErr(ae.message ?? '出错了')
    } finally {
      setBusy(false)
    }
  }

  return (
    <div style={wrap}>
      <form onSubmit={submit} style={card}>
        <h1 style={{ margin: 0, fontWeight: 300, letterSpacing: 4 }}>青萍 · Ripple</h1>
        <div style={{ opacity: 0.6, marginBottom: 24, fontSize: 12 }}>
          {mode === 'login' ? '欢迎回来' : '初次相遇'}
        </div>

        <input
          type="email" required placeholder="邮箱" value={email}
          onChange={e => setEmail(e.target.value)} style={input} autoFocus
        />
        <input
          type="password" required placeholder="密码（≥8 位）" minLength={8}
          value={password} onChange={e => setPassword(e.target.value)} style={input}
        />
        {mode === 'register' && (
          <input
            type="text" placeholder="昵称（可选）" value={displayName}
            onChange={e => setDisplayName(e.target.value)} style={input}
          />
        )}

        {err && <div style={errStyle}>{err}</div>}

        <button type="submit" disabled={busy} style={primaryBtn}>
          {busy ? '...' : mode === 'login' ? '入湖' : '注册并入湖'}
        </button>

        <button
          type="button" disabled={busy}
          onClick={() => setMode(mode === 'login' ? 'register' : 'login')}
          style={linkBtn}
        >
          {mode === 'login' ? '还没账号？注册' : '已有账号？登录'}
        </button>
      </form>
    </div>
  )
}

const wrap: React.CSSProperties = {
  width: '100vw', height: '100vh', display: 'flex',
  alignItems: 'center', justifyContent: 'center',
  background: 'linear-gradient(135deg, #0a1929 0%, #1a3a5a 100%)',
  color: '#e0f0ff', fontFamily: 'system-ui, -apple-system, sans-serif',
}
const card: React.CSSProperties = {
  width: 360, padding: 40, background: 'rgba(255,255,255,0.05)',
  borderRadius: 12, backdropFilter: 'blur(12px)',
  border: '1px solid rgba(255,255,255,0.1)',
  display: 'flex', flexDirection: 'column', gap: 12,
}
const input: React.CSSProperties = {
  padding: '10px 14px', background: 'rgba(255,255,255,0.08)',
  border: '1px solid rgba(255,255,255,0.15)', borderRadius: 6,
  color: '#fff', fontSize: 14, outline: 'none',
}
const primaryBtn: React.CSSProperties = {
  marginTop: 8, padding: '12px', background: '#4a90e2',
  border: 'none', borderRadius: 6, color: 'white',
  fontSize: 14, cursor: 'pointer', letterSpacing: 2,
}
const linkBtn: React.CSSProperties = {
  background: 'none', border: 'none', color: '#9ec5ee',
  cursor: 'pointer', fontSize: 12, padding: 8,
}
const errStyle: React.CSSProperties = {
  padding: 8, background: 'rgba(255,80,80,0.15)',
  borderRadius: 4, color: '#ffb0b0', fontSize: 12,
}
