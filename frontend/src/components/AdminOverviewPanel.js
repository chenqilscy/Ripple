import { jsx as _jsx, jsxs as _jsxs, Fragment as _Fragment } from "react/jsx-runtime";
import { useEffect, useState } from 'react';
import { api } from '../api/client';
export const EMPTY_ADMIN_OVERVIEW_STATS = {
    organizations_count: 0,
    users_count: 0,
    graylist_entries_count: 0,
};
export function resolveAdminOverviewStats(overview) {
    return overview?.stats ?? EMPTY_ADMIN_OVERVIEW_STATS;
}
export function resolveAdminOverviewOrganizations(overview) {
    return overview?.organizations ?? [];
}
export function adminLatestQuotaAudit(org) {
    return (org.recent_quota_audits ?? [])[0];
}
export default function AdminOverviewPanel() {
    const [overview, setOverview] = useState(null);
    const [loading, setLoading] = useState(false);
    const [forbidden, setForbidden] = useState(false);
    const [err, setErr] = useState(null);
    async function load() {
        setLoading(true);
        setErr(null);
        try {
            const res = await api.getAdminOverview();
            setOverview(res);
            setForbidden(false);
        }
        catch (e) {
            if (e?.status === 403) {
                setForbidden(true);
                setOverview(null);
                setErr('仅平台管理员可查看运营总览');
            }
            else {
                setErr(e?.message ?? 'load failed');
            }
        }
        finally {
            setLoading(false);
        }
    }
    useEffect(() => { void load(); }, []);
    const stats = resolveAdminOverviewStats(overview);
    const organizations = resolveAdminOverviewOrganizations(overview);
    return (_jsxs("div", { style: { padding: 16, maxWidth: 860, minWidth: 360, flex: '2 1 520px' }, children: [_jsxs("div", { style: { display: 'flex', alignItems: 'center', gap: 8, marginBottom: 12 }, children: [_jsx("h3", { style: { margin: 0, color: '#cdd6f4', flex: 1 }, children: "\u7BA1\u7406\u5458\u603B\u89C8" }), _jsx("button", { onClick: () => void load(), disabled: loading, style: btnStyle('#89b4fa'), children: loading ? '刷新中…' : '刷新' })] }), _jsx("p", { style: { margin: '0 0 12px', color: '#6c7086', fontSize: 12, lineHeight: 1.5 }, children: "\u805A\u5408\u5C55\u793A\u5E73\u53F0\u7EA7\u7EC4\u7EC7\u3001\u7528\u6237\u4E0E\u7070\u5EA6\u540D\u5355\u89C4\u6A21\uFF0C\u4EE5\u53CA\u6700\u8FD1\u521B\u5EFA\u7684\u7EC4\u7EC7 quota \u4F7F\u7528\u6982\u89C8\u3002" }), err && _jsxs("p", { style: { color: forbidden ? '#f9e2af' : '#f38ba8', margin: '0 0 12px' }, children: ["\u26A0 ", err] }), forbidden ? (_jsx("p", { style: { color: '#6c7086', margin: 0 }, children: "\u5F53\u524D\u8D26\u53F7\u4E0D\u662F\u5E73\u53F0\u7BA1\u7406\u5458\uFF0C\u53EA\u80FD\u67E5\u770B\u6B64\u8BF4\u660E\u3002" })) : loading && !overview ? (_jsx("p", { style: { color: '#6c7086' }, children: "\u52A0\u8F7D\u4E2D\u2026" })) : overview ? (_jsxs(_Fragment, { children: [_jsxs("div", { style: { display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(140px, 1fr))', gap: 10, marginBottom: 14 }, children: [_jsx(StatCard, { label: "\u7EC4\u7EC7\u6570", value: stats.organizations_count, color: "#89b4fa" }), _jsx(StatCard, { label: "\u7528\u6237\u6570", value: stats.users_count, color: "#a6e3a1" }), _jsx(StatCard, { label: "\u7070\u5EA6\u90AE\u7BB1", value: stats.graylist_entries_count, color: "#f9e2af" })] }), organizations.length === 0 ? (_jsx("p", { style: { color: '#6c7086' }, children: "\u6682\u65E0\u7EC4\u7EC7\u6570\u636E\u3002" })) : (_jsxs("table", { style: { width: '100%', borderCollapse: 'collapse', fontSize: 13 }, children: [_jsx("thead", { children: _jsxs("tr", { style: { color: '#6c7086', textAlign: 'left' }, children: [_jsx("th", { style: thStyle, children: "\u7EC4\u7EC7" }), _jsx("th", { style: thStyle, children: "Members" }), _jsx("th", { style: thStyle, children: "Lakes" }), _jsx("th", { style: thStyle, children: "Nodes" }), _jsx("th", { style: thStyle, children: "Latest audit" })] }) }), _jsx("tbody", { children: organizations.map(org => {
                                    const latestAudit = adminLatestQuotaAudit(org);
                                    return (_jsxs("tr", { style: { borderBottom: '1px solid #313244' }, children: [_jsxs("td", { style: tdStyle, children: [_jsx("div", { style: { color: '#cdd6f4' }, children: org.organization.name }), _jsxs("div", { style: { color: '#6c7086', fontSize: 11 }, children: ["/", org.organization.slug] })] }), _jsx("td", { style: tdStyle, children: fmtUsage(org.quota.usage?.members_used, org.quota.max_members) }), _jsx("td", { style: tdStyle, children: fmtUsage(org.quota.usage?.lakes_used, org.quota.max_lakes) }), _jsx("td", { style: tdStyle, children: fmtUsage(org.quota.usage?.nodes_used, org.quota.max_nodes) }), _jsx("td", { style: { ...tdStyle, color: '#6c7086' }, children: latestAudit ? new Date(latestAudit.created_at).toLocaleString() : '—' })] }, org.organization.id));
                                }) })] }))] })) : null] }));
}
function StatCard({ label, value, color }) {
    return (_jsxs("div", { style: { border: '1px solid #313244', borderRadius: 8, padding: '10px 12px', background: '#181825' }, children: [_jsx("div", { style: { color: '#6c7086', fontSize: 11, marginBottom: 6 }, children: label }), _jsx("div", { style: { color, fontSize: 24, fontWeight: 700 }, children: value })] }));
}
function fmtUsage(used, limit) {
    return `${used ?? 0}/${limit}`;
}
function btnStyle(color) {
    return {
        background: 'transparent', border: `1px solid ${color}`, color,
        borderRadius: 4, padding: '5px 12px', cursor: 'pointer', fontSize: 13,
    };
}
const thStyle = {
    padding: '6px 8px', fontWeight: 500, borderBottom: '1px solid #313244',
};
const tdStyle = {
    padding: '8px 8px', color: '#cdd6f4',
};
