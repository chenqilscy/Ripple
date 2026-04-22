import { jsx as _jsx } from "react/jsx-runtime";
import { useEffect, useState } from 'react';
import { Login } from './pages/Login';
import { Home } from './pages/Home';
import { getToken, onUnauthorized, api } from './api/client';
export function App() {
    const [authed, setAuthed] = useState(() => !!getToken());
    useEffect(() => {
        onUnauthorized(() => setAuthed(false));
    }, []);
    if (!authed)
        return _jsx(Login, { onSuccess: () => setAuthed(true) });
    return _jsx(Home, { onLogout: () => { api.logout(); setAuthed(false); } });
}
