import { jsx as _jsx, jsxs as _jsxs } from "react/jsx-runtime";
import { useEffect, useState } from 'react';
import { api } from '../api/client';
import { prompt as modalPrompt } from './Modal';
/**
 * SpaceSwitcher：侧栏组件，列出当前用户的所有 Space + "个人湖"。
 *
 * 行为：
 *   - 切换 → 触发 onChange（父组件应重新拉 lakes 列表）
 *   - "+" → 创建新空间（modal prompt）
 *   - 行内 "成员" → 触发 onManageMembers
 */
export default function SpaceSwitcher(props) {
    const [spaces, setSpaces] = useState([]);
    const [loading, setLoading] = useState(false);
    const [err, setErr] = useState(null);
    async function refresh() {
        setLoading(true);
        setErr(null);
        try {
            const r = await api.listSpaces();
            setSpaces(r.spaces ?? []);
        }
        catch (e) {
            setErr(e.message);
        }
        finally {
            setLoading(false);
        }
    }
    useEffect(() => { void refresh(); }, []);
    async function handleCreate() {
        const name = await modalPrompt({
            title: '创建空间',
            label: '空间是组织多个湖的容器，可以邀请成员协作。',
            placeholder: '空间名称（≤ 64 字）',
            validate: v => !v.trim() ? '名称不能为空' : null,
        });
        if (!name)
            return;
        const desc = await modalPrompt({
            title: '空间描述（可选）',
            placeholder: '简单描述这个空间的用途',
            initial: '',
        });
        try {
            const sp = await api.createSpace(name.trim(), desc?.trim() ?? '');
            await refresh();
            props.onChange(sp.id);
        }
        catch (e) {
            setErr(e.message);
        }
    }
    return (_jsxs("div", { style: { padding: '8px 0', borderBottom: '1px solid #2a2a2a' }, children: [_jsxs("div", { style: { display: 'flex', alignItems: 'center', justifyContent: 'space-between', padding: '0 12px 6px' }, children: [_jsx("span", { style: { fontSize: 12, color: '#888', textTransform: 'uppercase', letterSpacing: 1 }, children: "\u7A7A\u95F4" }), _jsx("button", { onClick: handleCreate, title: "\u521B\u5EFA\u7A7A\u95F4", style: {
                            background: 'transparent', border: '1px solid #3a3a3a', color: '#aaa',
                            borderRadius: 4, width: 22, height: 22, cursor: 'pointer', fontSize: 14,
                        }, children: "+" })] }), loading && _jsx("div", { style: { padding: '0 12px', color: '#666', fontSize: 12 }, children: "\u52A0\u8F7D\u4E2D\u2026" }), err && _jsx("div", { style: { padding: '0 12px', color: '#e66', fontSize: 12 }, children: err }), _jsxs("ul", { style: { listStyle: 'none', margin: 0, padding: 0 }, children: [_jsx(SpaceRow, { name: "\uD83D\uDCCC \u4E2A\u4EBA\u6E56", active: props.currentSpaceId === '', onClick: () => props.onChange('') }), spaces.map(s => (_jsx(SpaceRow, { name: s.name, sub: s.role === 'OWNER' ? '所有者' : s.role === 'EDITOR' ? '编辑' : '查看', active: props.currentSpaceId === s.id, onClick: () => props.onChange(s.id), onMembers: () => props.onManageMembers(s) }, s.id)))] })] }));
}
function SpaceRow(p) {
    return (_jsxs("li", { onClick: p.onClick, style: {
            display: 'flex', alignItems: 'center', justifyContent: 'space-between',
            padding: '6px 12px', cursor: 'pointer',
            background: p.active ? '#1d2433' : 'transparent',
            borderLeft: p.active ? '3px solid #4a8eff' : '3px solid transparent',
            color: p.active ? '#e6e6e6' : '#bbb',
        }, children: [_jsxs("div", { style: { display: 'flex', flexDirection: 'column', gap: 2, minWidth: 0 }, children: [_jsx("span", { style: { fontSize: 13, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }, children: p.name }), p.sub && _jsx("span", { style: { fontSize: 10, color: '#666' }, children: p.sub })] }), p.onMembers && (_jsx("button", { onClick: e => { e.stopPropagation(); p.onMembers(); }, title: "\u7BA1\u7406\u6210\u5458", style: {
                    background: 'transparent', border: 'none', color: '#888', cursor: 'pointer',
                    fontSize: 14, padding: '0 4px',
                }, children: "\uD83D\uDC65" }))] }));
}
