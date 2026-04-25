import { jsx as _jsx, jsxs as _jsxs } from "react/jsx-runtime";
/**
 * P11-C: Lake member role management panel.
 * Only shown when the current user is OWNER of the lake.
 * Allows changing roles of other members (not to OWNER).
 */
import { useCallback, useEffect, useState } from 'react';
import { api } from '../api/client';
const ROLE_OPTIONS = ['NAVIGATOR', 'PASSENGER', 'OBSERVER'];
const ROLE_COLOR = {
    OWNER: '#f5a623',
    NAVIGATOR: '#52c41a',
    PASSENGER: '#4a8eff',
    OBSERVER: '#888899',
};
export default function LakeMemberManager({ lakeId, currentUserId, currentRole }) {
    const [members, setMembers] = useState([]);
    const [loading, setLoading] = useState(false);
    const [error, setError] = useState(null);
    const [updating, setUpdating] = useState(null);
    const [removing, setRemoving] = useState(null);
    const load = useCallback(async () => {
        setLoading(true);
        setError(null);
        try {
            const res = await api.listLakeMembers(lakeId);
            setMembers(res.members);
        }
        catch (e) {
            setError(e instanceof Error ? e.message : 'Failed to load members');
        }
        finally {
            setLoading(false);
        }
    }, [lakeId]);
    useEffect(() => { void load(); }, [load]);
    const handleRemove = useCallback(async (userId) => {
        if (!window.confirm('确认移除该成员？'))
            return;
        setRemoving(userId);
        setError(null);
        try {
            await api.removeLakeMember(lakeId, userId);
            setMembers(prev => prev.filter(m => m.user_id !== userId));
        }
        catch (e) {
            setError(e instanceof Error ? e.message : '移除失败');
        }
        finally {
            setRemoving(null);
        }
    }, [lakeId]);
    const handleRoleChange = useCallback(async (userId, newRole) => {
        setUpdating(userId);
        setError(null);
        try {
            await api.updateMemberRole(lakeId, userId, newRole);
            setMembers(prev => prev.map(m => m.user_id === userId ? { ...m, role: newRole } : m));
        }
        catch (e) {
            setError(e instanceof Error ? e.message : 'Failed to update role');
        }
        finally {
            setUpdating(null);
        }
    }, [lakeId]);
    const isOwner = currentRole === 'OWNER';
    return (_jsxs("div", { style: {
            background: '#0d1526',
            border: '1px solid #1e3050',
            borderRadius: 8,
            padding: 16,
            minWidth: 340,
            maxWidth: 480,
        }, children: [_jsxs("div", { style: {
                    display: 'flex',
                    alignItems: 'center',
                    justifyContent: 'space-between',
                    marginBottom: 12,
                }, children: [_jsx("span", { style: { color: '#c0d8f0', fontWeight: 600, fontSize: 14 }, children: "\u6E56\u6210\u5458\u7BA1\u7406" }), _jsx("button", { onClick: () => void load(), disabled: loading, style: {
                            background: 'none',
                            border: '1px solid #2a4a7e',
                            borderRadius: 4,
                            color: '#6a9ab0',
                            cursor: 'pointer',
                            padding: '2px 8px',
                            fontSize: 11,
                        }, children: loading ? '…' : '刷新' })] }), error && (_jsx("div", { style: {
                    color: '#ff6b6b',
                    fontSize: 12,
                    marginBottom: 8,
                    padding: '4px 8px',
                    background: 'rgba(255,107,107,0.1)',
                    borderRadius: 4,
                }, children: error })), members.length === 0 && !loading && (_jsx("div", { style: { color: '#4a6a8e', fontSize: 12, textAlign: 'center', padding: 16 }, children: "\u6682\u65E0\u6210\u5458" })), _jsxs("table", { style: { width: '100%', borderCollapse: 'collapse', fontSize: 12 }, children: [_jsx("thead", { children: _jsxs("tr", { style: { borderBottom: '1px solid #1e3050' }, children: [_jsx("th", { style: { textAlign: 'left', padding: '4px 6px', color: '#4a6a8e', fontWeight: 500 }, children: "\u7528\u6237 ID" }), _jsx("th", { style: { textAlign: 'left', padding: '4px 6px', color: '#4a6a8e', fontWeight: 500 }, children: "\u89D2\u8272" }), isOwner && (_jsx("th", { style: { textAlign: 'left', padding: '4px 6px', color: '#4a6a8e', fontWeight: 500 }, children: "\u53D8\u66F4" })), isOwner && (_jsx("th", { style: { padding: '4px 6px' } }))] }) }), _jsx("tbody", { children: members.map(m => (_jsxs("tr", { style: { borderBottom: '1px solid rgba(30,48,80,0.5)' }, children: [_jsxs("td", { style: {
                                        padding: '6px 6px',
                                        color: m.user_id === currentUserId ? '#89dceb' : '#8ab0c8',
                                        fontFamily: 'monospace',
                                        fontSize: 11,
                                        maxWidth: 160,
                                        overflow: 'hidden',
                                        textOverflow: 'ellipsis',
                                        whiteSpace: 'nowrap',
                                    }, title: m.user_id, children: [m.user_id.slice(0, 8), "\u2026", m.user_id === currentUserId && (_jsx("span", { style: { color: '#89dceb', marginLeft: 4, fontSize: 10 }, children: "\uFF08\u4F60\uFF09" }))] }), _jsx("td", { style: { padding: '6px 6px' }, children: _jsx("span", { style: {
                                            color: ROLE_COLOR[m.role] ?? '#888',
                                            fontWeight: 500,
                                            fontSize: 11,
                                        }, children: m.role }) }), isOwner && (_jsx("td", { style: { padding: '6px 6px' }, children: m.role === 'OWNER' || m.user_id === currentUserId ? (_jsx("span", { style: { color: '#334466', fontSize: 11 }, children: "\u2014" })) : (_jsx("select", { value: m.role, disabled: updating === m.user_id, onChange: e => void handleRoleChange(m.user_id, e.target.value), style: {
                                            background: '#0a1020',
                                            border: '1px solid #2a4a7e',
                                            borderRadius: 3,
                                            color: '#c0d8f0',
                                            fontSize: 11,
                                            padding: '2px 4px',
                                            cursor: 'pointer',
                                            opacity: updating === m.user_id ? 0.5 : 1,
                                        }, children: ROLE_OPTIONS.map(r => (_jsx("option", { value: r, children: r }, r))) })) })), isOwner && (_jsx("td", { style: { padding: '6px 6px' }, children: m.role !== 'OWNER' && m.user_id !== currentUserId && (_jsx("button", { disabled: removing === m.user_id, onClick: () => void handleRemove(m.user_id), style: {
                                            background: 'rgba(220,53,69,0.12)',
                                            border: '1px solid rgba(220,53,69,0.3)',
                                            borderRadius: 3, color: '#ff6b7a',
                                            cursor: 'pointer', padding: '1px 6px', fontSize: 11,
                                        }, children: removing === m.user_id ? '…' : '移除' })) }))] }, m.user_id))) })] })] }));
}
