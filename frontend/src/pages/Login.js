import { jsx as _jsx, jsxs as _jsxs } from "react/jsx-runtime";
import { useState } from 'react';
import { api } from '../api/client';
export function Login({ onSuccess }) {
    const [mode, setMode] = useState('login');
    const [email, setEmail] = useState('');
    const [password, setPassword] = useState('');
    const [displayName, setDisplayName] = useState('');
    const [busy, setBusy] = useState(false);
    const [err, setErr] = useState(null);
    async function submit(e) {
        e.preventDefault();
        setErr(null);
        setBusy(true);
        try {
            if (mode === 'register') {
                await api.register(email, password, displayName || email.split('@')[0]);
            }
            await api.login(email, password);
            onSuccess();
        }
        catch (e) {
            const ae = e;
            setErr(ae.message ?? '出错了');
        }
        finally {
            setBusy(false);
        }
    }
    return (_jsx("div", { style: wrap, children: _jsxs("form", { onSubmit: submit, style: card, children: [_jsx("h1", { style: { margin: 0, fontWeight: 300, letterSpacing: 4 }, children: "\u9752\u840D \u00B7 Ripple" }), _jsx("div", { style: { opacity: 0.6, marginBottom: 24, fontSize: 12 }, children: mode === 'login' ? '欢迎回来' : '初次相遇' }), _jsx("input", { type: "email", required: true, placeholder: "\u90AE\u7BB1", value: email, onChange: e => setEmail(e.target.value), style: input, autoFocus: true }), _jsx("input", { type: "password", required: true, placeholder: "\u5BC6\u7801\uFF08\u22658 \u4F4D\uFF09", minLength: 8, value: password, onChange: e => setPassword(e.target.value), style: input }), mode === 'register' && (_jsx("input", { type: "text", placeholder: "\u6635\u79F0\uFF08\u53EF\u9009\uFF09", value: displayName, onChange: e => setDisplayName(e.target.value), style: input })), err && _jsx("div", { style: errStyle, children: err }), _jsx("button", { type: "submit", disabled: busy, style: primaryBtn, children: busy ? '...' : mode === 'login' ? '入湖' : '注册并入湖' }), _jsx("button", { type: "button", disabled: busy, onClick: () => setMode(mode === 'login' ? 'register' : 'login'), style: linkBtn, children: mode === 'login' ? '还没账号？注册' : '已有账号？登录' })] }) }));
}
const wrap = {
    width: '100vw', height: '100vh', display: 'flex',
    alignItems: 'center', justifyContent: 'center',
    background: 'linear-gradient(135deg, #0a1929 0%, #1a3a5a 100%)',
    color: '#e0f0ff', fontFamily: 'system-ui, -apple-system, sans-serif',
};
const card = {
    width: 360, padding: 40, background: 'rgba(255,255,255,0.05)',
    borderRadius: 12, backdropFilter: 'blur(12px)',
    border: '1px solid rgba(255,255,255,0.1)',
    display: 'flex', flexDirection: 'column', gap: 12,
};
const input = {
    padding: '10px 14px', background: 'rgba(255,255,255,0.08)',
    border: '1px solid rgba(255,255,255,0.15)', borderRadius: 6,
    color: '#fff', fontSize: 14, outline: 'none',
};
const primaryBtn = {
    marginTop: 8, padding: '12px', background: '#4a90e2',
    border: 'none', borderRadius: 6, color: 'white',
    fontSize: 14, cursor: 'pointer', letterSpacing: 2,
};
const linkBtn = {
    background: 'none', border: 'none', color: '#9ec5ee',
    cursor: 'pointer', fontSize: 12, padding: 8,
};
const errStyle = {
    padding: 8, background: 'rgba(255,80,80,0.15)',
    borderRadius: 4, color: '#ffb0b0', fontSize: 12,
};
