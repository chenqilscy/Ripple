import { useEffect, useState } from 'react'
import { Login } from './pages/Login'
import { Home } from './pages/Home'
import { getToken, onUnauthorized, api } from './api/client'

export function App() {
  const [authed, setAuthed] = useState(() => !!getToken())

  useEffect(() => {
    onUnauthorized(() => setAuthed(false))
  }, [])

  if (!authed) return <Login onSuccess={() => setAuthed(true)} />
  return <Home onLogout={() => { api.logout(); setAuthed(false) }} />
}
