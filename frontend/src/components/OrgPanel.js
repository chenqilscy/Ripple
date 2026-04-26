import { jsx as _jsx, jsxs as _jsxs, Fragment as _Fragment } from "react/jsx-runtime";
/**
 * P12-C: Organization list, creation form, and member management panel.
 * P13-A: Lake ↔ Org binding tab added.
 */
import { useCallback, useEffect, useState } from 'react';
import { api } from '../api/client';
const ROLE_COLOR = {
    OWNER: '#f5a623',
    ADMIN: '#52c41a',
    MEMBER: '#4a8eff',
};
const INVITE_ROLE_OPTIONS = ['ADMIN', 'MEMBER'];
function OrgMemberList({ org, currentUserId, onBack }) {
    const [tab, setTab] = useState('members');
    const [members, setMembers] = useState([]);
    const [loading, setLoading] = useState(false);
    const [error, setError] = useState(null);
    const [updating, setUpdating] = useState(null);
    const [addUserId, setAddUserId] = useState('');
    const [addEmail, setAddEmail] = useState('');
    const [addRole, setAddRole] = useState('MEMBER');
    const [adding, setAdding] = useState(false);
    const [addingEmail, setAddingEmail] = useState(false);
    // Lakes tab state
    const [lakes, setLakes] = useState([]);
    const [lakesLoading, setLakesLoading] = useState(false);
    const [lakesError, setLakesError] = useState(null);
    const currentMember = members.find(m => m.user_id === currentUserId);
    const isAdmin = currentMember?.role === 'OWNER' || currentMember?.role === 'ADMIN';
    const load = useCallback(async () => {
        setLoading(true);
        setError(null);
        try {
            const res = await api.listOrgMembers(org.id);
            setMembers(res.members);
        }
        catch (e) {
            setError(e instanceof Error ? e.message : 'Failed to load members');
        }
        finally {
            setLoading(false);
        }
    }, [org.id]);
    const loadLakes = useCallback(async () => {
        setLakesLoading(true);
        setLakesError(null);
        try {
            const res = await api.listOrgLakes(org.id);
            setLakes(res.lakes);
        }
        catch (e) {
            setLakesError(e instanceof Error ? e.message : 'Failed to load lakes');
        }
        finally {
            setLakesLoading(false);
        }
    }, [org.id]);
    useEffect(() => { void load(); }, [load]);
    useEffect(() => { if (tab === 'lakes')
        void loadLakes(); }, [tab, loadLakes]);
    const handleRoleChange = useCallback(async (userId, newRole) => {
        setUpdating(userId);
        setError(null);
        try {
            await api.updateOrgMemberRole(org.id, userId, newRole);
            setMembers(prev => prev.map(m => m.user_id === userId ? { ...m, role: newRole } : m));
        }
        catch (e) {
            setError(e instanceof Error ? e.message : 'Failed to update role');
        }
        finally {
            setUpdating(null);
        }
    }, [org.id]);
    const handleRemove = useCallback(async (userId) => {
        setUpdating(userId);
        setError(null);
        try {
            await api.removeOrgMember(org.id, userId);
            setMembers(prev => prev.filter(m => m.user_id !== userId));
        }
        catch (e) {
            setError(e instanceof Error ? e.message : 'Failed to remove member');
        }
        finally {
            setUpdating(null);
        }
    }, [org.id]);
    const handleAdd = useCallback(async () => {
        const uid = addUserId.trim();
        if (!uid)
            return;
        setAdding(true);
        setError(null);
        try {
            await api.addOrgMember(org.id, uid, addRole);
            setAddUserId('');
            await load();
        }
        catch (e) {
            setError(e instanceof Error ? e.message : 'Failed to add member');
        }
        finally {
            setAdding(false);
        }
    }, [org.id, addUserId, addRole, load]);
    const handleAddByEmail = useCallback(async () => {
        const email = addEmail.trim();
        if (!email)
            return;
        setAddingEmail(true);
        setError(null);
        try {
            await api.addOrgMemberByEmail(org.id, email, addRole);
            setAddEmail('');
            await load();
        }
        catch (e) {
            setError(e instanceof Error ? e.message : 'Failed to invite by email');
        }
        finally {
            setAddingEmail(false);
        }
    }, [org.id, addEmail, addRole, load]);
    return (_jsxs("div", { style: { display: 'flex', flexDirection: 'column', gap: 12 }, children: [_jsxs("div", { style: { display: 'flex', alignItems: 'center', gap: 8 }, children: [_jsx("button", { onClick: onBack, style: btnStyle, children: "\u2190 Back" }), _jsx("span", { style: { color: '#c0d8f0', fontWeight: 600, fontSize: 14 }, children: org.name }), _jsx("button", { onClick: () => tab === 'members' ? void load() : void loadLakes(), disabled: loading || lakesLoading, style: { ...btnStyle, marginLeft: 'auto' }, children: (loading || lakesLoading) ? '...' : 'Refresh' })] }), error && _jsx(ErrorMsg, { children: error }), _jsxs("div", { style: { display: 'flex', gap: 6 }, children: [_jsx("button", { onClick: () => setTab('members'), style: { ...btnStyle, ...(tab === 'members' ? { background: 'rgba(74,142,255,0.15)', color: '#4a8eff' } : {}) }, children: "Members" }), _jsx("button", { onClick: () => setTab('lakes'), style: { ...btnStyle, ...(tab === 'lakes' ? { background: 'rgba(74,142,255,0.15)', color: '#4a8eff' } : {}) }, children: "Lakes" })] }), tab === 'members' && (_jsxs(_Fragment, { children: [_jsxs("div", { style: { display: 'flex', flexDirection: 'column', gap: 6 }, children: [members.map(m => (_jsxs("div", { style: memberRowStyle, children: [_jsx("span", { style: { color: '#8ab0d0', fontSize: 12, flex: 1, overflow: 'hidden', textOverflow: 'ellipsis' }, children: m.user_id }), isAdmin && m.role !== 'OWNER' && m.user_id !== currentUserId ? (_jsxs(_Fragment, { children: [_jsx("select", { value: m.role, disabled: updating === m.user_id, onChange: e => void handleRoleChange(m.user_id, e.target.value), style: selectStyle, children: INVITE_ROLE_OPTIONS.map(r => (_jsx("option", { value: r, children: r }, r))) }), _jsx("button", { disabled: updating === m.user_id, onClick: () => void handleRemove(m.user_id), style: { ...btnStyle, color: '#ff6b6b', borderColor: '#5a2222' }, children: "Remove" })] })) : (_jsx("span", { style: { color: ROLE_COLOR[m.role], fontSize: 11, padding: '2px 8px',
                                            background: 'rgba(255,255,255,0.05)', borderRadius: 4 }, children: m.role }))] }, m.user_id))), members.length === 0 && !loading && (_jsx("div", { style: { color: '#4a6a8e', fontSize: 12, textAlign: 'center', padding: 12 }, children: "No members" }))] }), isAdmin && (_jsxs("div", { style: { display: 'flex', flexDirection: 'column', gap: 6, marginTop: 4 }, children: [_jsxs("div", { style: { display: 'flex', gap: 6, flexWrap: 'wrap' }, children: [_jsx("input", { placeholder: "User ID to invite", value: addUserId, onChange: e => setAddUserId(e.target.value), onKeyDown: e => { if (e.key === 'Enter')
                                            void handleAdd(); }, style: inputStyle }), _jsx("select", { value: addRole, onChange: e => setAddRole(e.target.value), style: selectStyle, children: INVITE_ROLE_OPTIONS.map(r => _jsx("option", { value: r, children: r }, r)) }), _jsx("button", { onClick: () => void handleAdd(), disabled: adding || !addUserId.trim(), style: btnStyle, children: adding ? '...' : 'Add' })] }), _jsxs("div", { style: { display: 'flex', gap: 6, flexWrap: 'wrap' }, children: [_jsx("input", { type: "email", placeholder: "Email to invite", value: addEmail, onChange: e => setAddEmail(e.target.value), onKeyDown: e => { if (e.key === 'Enter')
                                            void handleAddByEmail(); }, style: inputStyle }), _jsx("button", { onClick: () => void handleAddByEmail(), disabled: addingEmail || !addEmail.trim(), style: btnStyle, title: "Invite an already-registered user by email", children: addingEmail ? '...' : 'Invite by Email' })] })] }))] })), tab === 'lakes' && (_jsxs(_Fragment, { children: [lakesError && _jsx(ErrorMsg, { children: lakesError }), _jsxs("div", { style: { display: 'flex', flexDirection: 'column', gap: 6 }, children: [lakes.map(l => (_jsxs("div", { style: memberRowStyle, children: [_jsx("span", { style: { color: '#c0d8f0', fontSize: 12, flex: 1, overflow: 'hidden', textOverflow: 'ellipsis' }, children: l.name }), _jsxs("span", { style: { color: '#4a6a8e', fontSize: 10 }, children: [l.id.slice(0, 8), "\u2026"] })] }, l.id))), lakes.length === 0 && !lakesLoading && (_jsx("div", { style: { color: '#4a6a8e', fontSize: 12, textAlign: 'center', padding: 12 }, children: "No lakes linked to this organization" })), lakesLoading && (_jsx("div", { style: { color: '#4a6a8e', fontSize: 12, textAlign: 'center', padding: 12 }, children: "Loading\u2026" }))] })] }))] }));
}
export default function OrgPanel({ currentUserId, onClose }) {
    const [orgs, setOrgs] = useState([]);
    const [loading, setLoading] = useState(false);
    const [error, setError] = useState(null);
    const [creating, setCreating] = useState(false);
    const [newName, setNewName] = useState('');
    const [newSlug, setNewSlug] = useState('');
    const [newDesc, setNewDesc] = useState('');
    const [creating2, setCreating2] = useState(false);
    const [selectedOrg, setSelectedOrg] = useState(null);
    const load = useCallback(async () => {
        setLoading(true);
        setError(null);
        try {
            const res = await api.listOrgs();
            setOrgs(res.organizations);
        }
        catch (e) {
            setError(e instanceof Error ? e.message : 'Failed to load organizations');
        }
        finally {
            setLoading(false);
        }
    }, []);
    useEffect(() => { void load(); }, [load]);
    const handleCreate = useCallback(async () => {
        const name = newName.trim();
        const slug = newSlug.trim();
        if (!name || !slug)
            return;
        setCreating2(true);
        setError(null);
        try {
            const org = await api.createOrg(name, slug, newDesc.trim());
            setOrgs(prev => [org, ...prev]);
            setNewName('');
            setNewSlug('');
            setNewDesc('');
            setCreating(false);
        }
        catch (e) {
            setError(e instanceof Error ? e.message : 'Failed to create organization');
        }
        finally {
            setCreating2(false);
        }
    }, [newName, newSlug, newDesc]);
    return (_jsxs("div", { style: panelStyle, children: [_jsxs("div", { style: { display: 'flex', alignItems: 'center', marginBottom: 14 }, children: [_jsx("span", { style: { color: '#c0d8f0', fontWeight: 700, fontSize: 15, flex: 1 }, children: "Organizations" }), _jsx("button", { onClick: () => void load(), disabled: loading, style: { ...btnStyle, marginRight: 8 }, children: loading ? '...' : 'Refresh' }), _jsx("button", { onClick: onClose, style: { ...btnStyle, color: '#6a8aaa' }, children: "\u2715" })] }), error && _jsx(ErrorMsg, { children: error }), selectedOrg ? (_jsx(OrgMemberList, { org: selectedOrg, currentUserId: currentUserId, onBack: () => setSelectedOrg(null) })) : (_jsxs(_Fragment, { children: [_jsxs("div", { style: { display: 'flex', flexDirection: 'column', gap: 6, marginBottom: 12 }, children: [orgs.map(org => (_jsxs("div", { style: memberRowStyle, children: [_jsxs("div", { style: { flex: 1 }, children: [_jsx("span", { style: { color: '#c0d8f0', fontSize: 13, fontWeight: 500 }, children: org.name }), _jsxs("span", { style: { color: '#4a6a8e', fontSize: 11, marginLeft: 6 }, children: ["/", org.slug] })] }), _jsx("button", { onClick: () => setSelectedOrg(org), style: btnStyle, children: "Members" })] }, org.id))), orgs.length === 0 && !loading && (_jsx("div", { style: { color: '#4a6a8e', fontSize: 12, textAlign: 'center', padding: 16 }, children: "No organizations yet" }))] }), !creating ? (_jsx("button", { onClick: () => setCreating(true), style: { ...btnStyle, width: '100%' }, children: "+ New Organization" })) : (_jsxs("div", { style: { display: 'flex', flexDirection: 'column', gap: 8, border: '1px solid #1e3050',
                            borderRadius: 6, padding: 12 }, children: [_jsx("span", { style: { color: '#8ab0d0', fontSize: 12, fontWeight: 600 }, children: "New Organization" }), _jsx("input", { placeholder: "Name", value: newName, onChange: e => setNewName(e.target.value), style: inputStyle }), _jsx("input", { placeholder: "Slug (e.g. my-org)", value: newSlug, onChange: e => setNewSlug(e.target.value.toLowerCase().replace(/[^a-z0-9-]/g, '')), style: inputStyle }), _jsx("input", { placeholder: "Description (optional)", value: newDesc, onChange: e => setNewDesc(e.target.value), style: inputStyle }), _jsxs("div", { style: { display: 'flex', gap: 8 }, children: [_jsx("button", { onClick: () => void handleCreate(), disabled: creating2 || !newName.trim() || !newSlug.trim(), style: { ...btnStyle, flex: 1, background: 'rgba(74,142,255,0.12)' }, children: creating2 ? 'Creating...' : 'Create' }), _jsx("button", { onClick: () => setCreating(false), style: btnStyle, children: "Cancel" })] })] }))] }))] }));
}
// ---- Shared styles ----
const panelStyle = {
    background: '#0d1526',
    border: '1px solid #1e3050',
    borderRadius: 10,
    padding: 18,
    width: 380,
    maxHeight: '80vh',
    overflowY: 'auto',
    boxShadow: '0 4px 24px rgba(0,0,0,0.5)',
};
const memberRowStyle = {
    display: 'flex',
    alignItems: 'center',
    gap: 8,
    padding: '6px 8px',
    background: 'rgba(255,255,255,0.03)',
    borderRadius: 5,
    border: '1px solid #1a2e4a',
};
const btnStyle = {
    background: 'none',
    border: '1px solid #2a4a7e',
    borderRadius: 4,
    color: '#6a9ab0',
    cursor: 'pointer',
    padding: '3px 10px',
    fontSize: 11,
};
const inputStyle = {
    background: '#081020',
    border: '1px solid #1e3050',
    borderRadius: 4,
    color: '#c0d8f0',
    padding: '5px 8px',
    fontSize: 12,
    width: '100%',
    boxSizing: 'border-box',
    outline: 'none',
};
const selectStyle = {
    background: '#081020',
    border: '1px solid #1e3050',
    borderRadius: 4,
    color: '#c0d8f0',
    padding: '3px 6px',
    fontSize: 11,
    cursor: 'pointer',
};
function ErrorMsg({ children }) {
    return (_jsx("div", { style: {
            color: '#ff6b6b',
            fontSize: 12,
            marginBottom: 8,
            padding: '4px 8px',
            background: 'rgba(255,107,107,0.1)',
            borderRadius: 4,
        }, children: children }));
}
