import { jsx as _jsx, jsxs as _jsxs } from "react/jsx-runtime";
// NotificationBell — P13-B 通知铃铛 + 下拉列表
import { useCallback, useEffect, useRef, useState } from 'react';
import { api } from '../api/client';
const POLL_MS = 30_000;
const PAGE = 20;
export default function NotificationBell() {
    const [count, setCount] = useState(0);
    const [open, setOpen] = useState(false);
    const [items, setItems] = useState([]);
    const [loading, setLoading] = useState(false);
    const [loadingMore, setLoadingMore] = useState(false);
    const [hasMore, setHasMore] = useState(false);
    const dropRef = useRef(null);
    const refreshCount = useCallback(async () => {
        try {
            const { count } = await api.getUnreadNotificationCount();
            setCount(count);
        }
        catch { /* 静默 */ }
    }, []);
    // P14-A：监听 WS 实时推送通知事件，角标 +1 并追加到列表
    useEffect(() => {
        function onWsNotif(e) {
            const payload = e.detail;
            if (!payload)
                return;
            setCount(c => c + 1);
            setItems(prev => [payload, ...prev]);
        }
        window.addEventListener('ripple:notification', onWsNotif);
        return () => window.removeEventListener('ripple:notification', onWsNotif);
    }, []);
    // 轮询未读数（每 30s，降级保障）
    useEffect(() => {
        void refreshCount();
        const id = setInterval(() => void refreshCount(), POLL_MS);
        return () => clearInterval(id);
    }, [refreshCount]);
    // 点击外部关闭
    useEffect(() => {
        if (!open)
            return;
        function onOutside(e) {
            if (dropRef.current && !dropRef.current.contains(e.target)) {
                setOpen(false);
            }
        }
        document.addEventListener('mousedown', onOutside);
        return () => document.removeEventListener('mousedown', onOutside);
    }, [open]);
    async function toggleOpen() {
        if (open) {
            setOpen(false);
            return;
        }
        setOpen(true);
        setLoading(true);
        try {
            const { notifications } = await api.listNotifications(PAGE);
            setItems(notifications);
            setHasMore(notifications.length === PAGE);
            // 打开后刷新一次未读数
            void refreshCount();
        }
        catch { /* 静默 */ }
        finally {
            setLoading(false);
        }
    }
    async function loadMore() {
        if (items.length === 0)
            return;
        const before = items[items.length - 1].id;
        setLoadingMore(true);
        try {
            const { notifications } = await api.listNotifications(PAGE, before);
            setItems(prev => [...prev, ...notifications]);
            setHasMore(notifications.length === PAGE);
        }
        catch { /* 静默 */ }
        finally {
            setLoadingMore(false);
        }
    }
    async function markRead(id) {
        try {
            await api.markNotificationRead(id);
            setItems(prev => prev.map(n => n.id === id ? { ...n, is_read: true } : n));
            setCount(c => Math.max(0, c - 1));
        }
        catch { /* 静默 */ }
    }
    async function markAll() {
        try {
            await api.markAllNotificationsRead();
            setItems(prev => prev.map(n => ({ ...n, is_read: true })));
            setCount(0);
        }
        catch { /* 静默 */ }
    }
    return (_jsxs("div", { ref: dropRef, style: { position: 'relative', display: 'inline-block' }, children: [_jsxs("button", { onClick: toggleOpen, title: "\u901A\u77E5", style: {
                    background: 'none', border: 'none', cursor: 'pointer',
                    color: '#cdd6f4', fontSize: 16, padding: '4px 6px',
                    position: 'relative',
                }, children: ["\uD83D\uDD14", count > 0 && (_jsx("span", { style: {
                            position: 'absolute', top: 0, right: 0,
                            background: '#f38ba8', color: '#1e1e2e',
                            borderRadius: '50%', fontSize: 10, fontWeight: 700,
                            width: 16, height: 16, lineHeight: '16px', textAlign: 'center',
                            pointerEvents: 'none',
                        }, children: count > 99 ? '99+' : count }))] }), open && (_jsxs("div", { style: {
                    position: 'absolute', right: 0, top: '110%', zIndex: 1000,
                    width: 320, maxHeight: 400, overflowY: 'auto',
                    background: '#1e1e2e', border: '1px solid #313244',
                    borderRadius: 8, boxShadow: '0 8px 24px rgba(0,0,0,0.5)',
                    padding: '8px 0',
                }, children: [_jsxs("div", { style: {
                            display: 'flex', justifyContent: 'space-between', alignItems: 'center',
                            padding: '4px 12px 8px',
                        }, children: [_jsx("span", { style: { fontSize: 12, fontWeight: 600, color: '#89dceb' }, children: "\u901A\u77E5" }), items.some(n => !n.is_read) && (_jsx("button", { onClick: markAll, style: {
                                    background: 'none', border: 'none', cursor: 'pointer',
                                    color: '#6c7086', fontSize: 11,
                                }, children: "\u5168\u90E8\u5DF2\u8BFB" }))] }), loading && (_jsx("div", { style: { padding: '16px', textAlign: 'center', color: '#6c7086', fontSize: 12 }, children: "\u52A0\u8F7D\u4E2D\u2026" })), !loading && items.length === 0 && (_jsx("div", { style: { padding: '16px', textAlign: 'center', color: '#6c7086', fontSize: 12 }, children: "\u6682\u65E0\u901A\u77E5" })), !loading && items.map(n => (_jsxs("div", { onClick: () => { if (!n.is_read)
                            void markRead(n.id); }, style: {
                            padding: '8px 12px',
                            cursor: n.is_read ? 'default' : 'pointer',
                            background: n.is_read ? 'transparent' : 'rgba(74,144,226,0.06)',
                            borderLeft: n.is_read ? '3px solid transparent' : '3px solid #4a90e2',
                            transition: 'background 0.15s',
                        }, children: [_jsx("div", { style: { fontSize: 12, color: '#cdd6f4', lineHeight: 1.5 }, children: formatNotif(n) }), _jsx("div", { style: { fontSize: 10, color: '#6c7086', marginTop: 2 }, children: new Date(n.created_at).toLocaleString('zh-CN') })] }, n.id))), !loading && hasMore && (_jsx("div", { style: { padding: '8px 12px', textAlign: 'center' }, children: _jsx("button", { onClick: () => void loadMore(), disabled: loadingMore, style: { background: 'none', border: 'none', cursor: 'pointer', color: '#89dceb', fontSize: 12 }, children: loadingMore ? '加载中…' : '加载更多' }) }))] }))] }));
}
function formatNotif(n) {
    const p = n.payload;
    switch (n.type) {
        case 'lake.invite_accepted': return `用户 ${p['invitee_id'] ?? '?'} 接受了邀请加入湖 ${p['lake_id'] ?? '?'}`;
        case 'lake.member_removed': return `你已被移出湖 ${p['lake_id'] ?? '?'}`;
        case 'lake.role_updated': return `你在湖 ${p['lake_id'] ?? '?'} 的角色已更新为 ${p['role'] ?? '?'}`;
        default: return `[${n.type}] ${JSON.stringify(p)}`;
    }
}
