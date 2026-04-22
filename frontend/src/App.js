import { jsx as _jsx, Fragment as _Fragment, jsxs as _jsxs } from "react/jsx-runtime";
import { useEffect, useState } from 'react';
import { Login } from './pages/Login';
import { Home } from './pages/Home';
import { ModalHost } from './components/Modal';
import { getToken, onUnauthorized, api } from './api/client';
export function App() {
    const [authed, setAuthed] = useState(() => !!getToken());
    useEffect(() => {
        onUnauthorized(() => setAuthed(false));
    }, []);
    return (_jsxs(_Fragment, { children: [!authed
                ? _jsx(Login, { onSuccess: () => setAuthed(true) })
                : _jsx(Home, { onLogout: () => { api.logout(); setAuthed(false); } }), _jsx(ModalHost, {})] }));
}
