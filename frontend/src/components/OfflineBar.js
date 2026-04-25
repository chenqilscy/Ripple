import { jsx as _jsx } from "react/jsx-runtime";
import { useEffect, useState } from 'react';
/** 离线状态横幅 — 网络断开时显示在顶部。P12-E */
export default function OfflineBar() {
    const [offline, setOffline] = useState(!navigator.onLine);
    useEffect(() => {
        const goOffline = () => setOffline(true);
        const goOnline = () => setOffline(false);
        window.addEventListener('offline', goOffline);
        window.addEventListener('online', goOnline);
        return () => {
            window.removeEventListener('offline', goOffline);
            window.removeEventListener('online', goOnline);
        };
    }, []);
    if (!offline)
        return null;
    return (_jsx("div", { style: {
            position: 'fixed',
            top: 0,
            left: 0,
            right: 0,
            background: '#f38ba8',
            color: '#1e1e2e',
            textAlign: 'center',
            padding: '6px 12px',
            fontSize: 13,
            fontWeight: 600,
            zIndex: 9999,
            letterSpacing: 1,
        }, children: "\u26A0 \u5F53\u524D\u5904\u4E8E\u79BB\u7EBF\u6A21\u5F0F \u2014 \u6570\u636E\u6765\u81EA\u672C\u5730\u7F13\u5B58\uFF0C\u5199\u64CD\u4F5C\u5C06\u5728\u6062\u590D\u7F51\u7EDC\u540E\u540C\u6B65" }));
}
