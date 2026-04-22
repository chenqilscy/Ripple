import { useEffect, useState } from 'react'
import { Login } from './pages/Login'
import { Home } from './pages/Home'
import { ModalHost } from './components/Modal'
import { getToken, onUnauthorized, api } from './api/client'

export function App() {
  const [authed, setAuthed] = useState(() => !!getToken())

  useEffect(() => {
    onUnauthorized(() => setAuthed(false))
  }, [])

  return (
    <>
      {!authed
        ? <Login onSuccess={() => setAuthed(true)} />
        : <Home onLogout={() => { api.logout(); setAuthed(false) }} />}
      <ModalHost />
    </>
  )
}
