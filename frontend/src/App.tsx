import { useEffect, useState } from 'react'
import { Login } from './pages/Login'
import { Home } from './pages/Home'
import { SharedNode } from './pages/SharedNode'
import { ModalHost } from './components/Modal'
import { getToken, onUnauthorized, api } from './api/client'

export function App() {
  const isSharePage = window.location.pathname.startsWith('/share/')
  const [authed, setAuthed] = useState(() => !!getToken())

  useEffect(() => {
    if (!isSharePage) {
      onUnauthorized(() => setAuthed(false))
    }
  }, [isSharePage])

  if (isSharePage) {
    return (
      <>
        <SharedNode />
        <ModalHost />
      </>
    )
  }

  return (
    <>
      {!authed
        ? <Login onSuccess={() => setAuthed(true)} />
        : <Home onLogout={() => { api.logout(); setAuthed(false) }} />}
      <ModalHost />
    </>
  )
}
