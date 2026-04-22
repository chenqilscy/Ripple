import { jsx as _jsx, jsxs as _jsxs } from "react/jsx-runtime";
import { useEffect, useState } from 'react';
import { api } from '../api/client';
import { prompt as modalPrompt } from './Modal';
/**
 * SpaceMembersDrawer：右侧抽屉，列出空间成员，支持添加/移除（仅 OWNER）。
 *
 * 设计：
 *   - 任何成员可读列表
 *   - 仅 OWNER 看到 "+" 按钮和每行的删除按钮
 *   - 添加成员需要输入对方 user_id（M3-S1 不做用户搜索；M3-S4 加邮箱搜索）
 */
export default function SpaceMembersDrawer(props) {
    const [members, setMembers] = useState([]);
    const [loading, setLoading] = useState(false);
    const [err, setErr] = useState(null);
    const isOwner = props.space.role === 'OWNER';
    async function refresh() {
        setLoading(true);
        setErr(null);
        try {
            const r = await api.listSpaceMembers(props.space.id);
            setMembers(r.members ?? []);
        }
        catch (e) {
            setErr(e.message);
        }
        finally {
            setLoading(false);
        }
    }
    useEffect(() => { void refresh(); }, [props.space.id]);
    async function handleAdd() {
        const uid = await modalPrompt({
            title: '邀请成员',
            label: '输入用户的 UUID（M3-S4 将支持邮箱邀请）',
            placeholder: '例如：8400ec3a-…',
            validate: v => v.trim().length < 32 ? 'UUID 看起来不对' : null,
        });
        if (!uid)
            return;
        const role = await modalPrompt({
            title: '设定权限',
            label: '输入 EDITOR（可写）或 VIEWER（只读）',
            initial: 'EDITOR',
            validate: v => (v === 'EDITOR' || v === 'VIEWER') ? null : '必须是 EDITOR 或 VIEWER',
        });
        if (!role)
            return;
        try {
            await api.addSpaceMember(props.space.id, uid.trim(), role);
            await refresh();
        }
        catch (e) {
            setErr(e.message);
        }
    }
    async function handleRemove(userId) {
        if (!confirm('移除该成员？该用户将立即失去访问权限。'))
            return;
        try {
            await api.removeSpaceMember(props.space.id, userId);
            await refresh();
        }
        catch (e) {
            setErr(e.message);
        }
    }
    return (_jsxs("div", { style: {
            position: 'fixed', top: 0, right: 0, bottom: 0, width: 360,
            background: '#161616', borderLeft: '1px solid #2a2a2a', zIndex: 1000,
            display: 'flex', flexDirection: 'column', boxShadow: '-4px 0 12px rgba(0,0,0,0.4)',
        }, children: [_jsxs("div", { style: { padding: '14px 16px', borderBottom: '1px solid #2a2a2a', display: 'flex', justifyContent: 'space-between', alignItems: 'center' }, children: [_jsxs("div", { style: { minWidth: 0 }, children: [_jsx("div", { style: { fontSize: 14, color: '#e6e6e6', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }, children: props.space.name }), _jsx("div", { style: { fontSize: 11, color: '#666' }, children: "\u6210\u5458\u7BA1\u7406" })] }), _jsx("button", { onClick: props.onClose, style: { background: 'transparent', border: 'none', color: '#999', cursor: 'pointer', fontSize: 18 }, children: "\u00D7" })] }), isOwner && (_jsx("div", { style: { padding: '8px 16px', borderBottom: '1px solid #2a2a2a' }, children: _jsx("button", { onClick: handleAdd, style: {
                        width: '100%', padding: '8px', background: '#1d2433', border: '1px solid #4a8eff',
                        color: '#4a8eff', borderRadius: 4, cursor: 'pointer', fontSize: 13,
                    }, children: "+ \u9080\u8BF7\u6210\u5458" }) })), err && _jsx("div", { style: { padding: '8px 16px', color: '#e66', fontSize: 12 }, children: err }), loading && _jsx("div", { style: { padding: '12px 16px', color: '#666', fontSize: 12 }, children: "\u52A0\u8F7D\u4E2D\u2026" }), _jsxs("ul", { style: { listStyle: 'none', margin: 0, padding: 0, overflowY: 'auto', flex: 1 }, children: [members.map(m => (_jsxs("li", { style: {
                            padding: '10px 16px', borderBottom: '1px solid #222',
                            display: 'flex', justifyContent: 'space-between', alignItems: 'center',
                        }, children: [_jsxs("div", { style: { minWidth: 0, flex: 1 }, children: [_jsx("div", { style: { fontSize: 12, color: '#ccc', fontFamily: 'monospace', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }, children: m.user_id }), _jsx("div", { style: { fontSize: 10, color: '#666', marginTop: 2 }, children: m.role })] }), isOwner && m.role !== 'OWNER' && (_jsx("button", { onClick: () => handleRemove(m.user_id), title: "\u79FB\u9664\u6210\u5458", style: { background: 'transparent', border: 'none', color: '#e66', cursor: 'pointer', fontSize: 13 }, children: "\u79FB\u9664" }))] }, m.user_id))), !loading && members.length === 0 && (_jsx("li", { style: { padding: '20px 16px', color: '#666', fontSize: 12, textAlign: 'center' }, children: "\u6682\u65E0\u6210\u5458" }))] })] }));
}
