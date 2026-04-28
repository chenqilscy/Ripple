import { jsx as _jsx, jsxs as _jsxs } from "react/jsx-runtime";
/**
 * Phase 15-B: Organization subscription management panel.
 * Shows current subscription status and available plans for upgrade/downgrade.
 */
import { useCallback, useEffect, useState } from 'react';
import { api } from '../api/client';
const PLAN_COLORS = {
    free: '#6c757d',
    pro: '#4a8eff',
    team: '#f5a623',
};
function planColor(planName) {
    const key = planName.toLowerCase();
    return PLAN_COLORS[key] ?? '#52c41a';
}
function formatCycle(cycle) {
    return cycle === 'monthly' ? '月付' : '年付';
}
function formatDate(iso) {
    return new Date(iso).toLocaleDateString('zh-CN');
}
export default function SubscriptionPanel({ orgId, isOwner }) {
    const [plans, setPlans] = useState([]);
    const [current, setCurrent] = useState(null);
    const [loadingPlans, setLoadingPlans] = useState(false);
    const [loadingSub, setLoadingSub] = useState(false);
    const [error, setError] = useState(null);
    const [selectedCycle, setSelectedCycle] = useState('monthly');
    const [submitting, setSubmitting] = useState(null); // planId being submitted
    const loadData = useCallback(async () => {
        setError(null);
        setLoadingPlans(true);
        setLoadingSub(true);
        try {
            const [plansRes, subRes] = await Promise.allSettled([
                api.listSubscriptionPlans(),
                api.getOrgSubscription(orgId),
            ]);
            if (plansRes.status === 'fulfilled') {
                setPlans(plansRes.value.plans);
            }
            else {
                setError('加载套餐失败: ' + String(plansRes.reason?.message || plansRes.reason));
            }
            if (subRes.status === 'fulfilled') {
                setCurrent(subRes.value.subscription);
            }
            // 404 means no subscription yet — that's OK
        }
        finally {
            setLoadingPlans(false);
            setLoadingSub(false);
        }
    }, [orgId]);
    useEffect(() => { loadData(); }, [loadData]);
    const handleSelect = useCallback(async (plan) => {
        if (!isOwner)
            return;
        if (current?.plan_id === plan.id && current?.billing_cycle === selectedCycle)
            return;
        setSubmitting(plan.id);
        setError(null);
        try {
            const res = await api.createOrgSubscription(orgId, plan.id, selectedCycle, /* stub_confirm */ true);
            setCurrent(res.subscription);
        }
        catch (e) {
            setError('订阅失败: ' + String(e?.message || e));
        }
        finally {
            setSubmitting(null);
        }
    }, [orgId, isOwner, current, selectedCycle]);
    const currentPlan = current ? plans.find(p => p.id === current.plan_id) : null;
    const isLoading = loadingPlans || loadingSub;
    return (_jsxs("div", { style: { padding: '16px 0' }, children: [_jsxs("div", { style: { display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 16 }, children: [_jsx("h3", { style: { margin: 0, color: '#fff' }, children: "\u8BA2\u9605\u5957\u9910" }), _jsx("div", { style: { display: 'flex', gap: 8 }, children: ['monthly', 'yearly'].map(c => (_jsxs("button", { onClick: () => setSelectedCycle(c), style: {
                                padding: '4px 12px',
                                borderRadius: 4,
                                border: 'none',
                                cursor: 'pointer',
                                background: selectedCycle === c ? '#4a8eff' : '#2a2a3a',
                                color: '#fff',
                                fontSize: 13,
                            }, children: [c === 'monthly' ? '月付' : '年付', c === 'yearly' && _jsx("span", { style: { color: '#52c41a', marginLeft: 4, fontSize: 11 }, children: "\u770120%" })] }, c))) })] }), current && (_jsxs("div", { style: {
                    background: '#1a1a2e',
                    border: '1px solid #2a2a4a',
                    borderRadius: 8,
                    padding: '12px 16px',
                    marginBottom: 16,
                }, children: [_jsx("div", { style: { fontSize: 13, color: '#aaa', marginBottom: 4 }, children: "\u5F53\u524D\u5957\u9910" }), _jsxs("div", { style: { display: 'flex', gap: 16, alignItems: 'center', flexWrap: 'wrap' }, children: [_jsx("span", { style: { color: planColor(currentPlan?.name ?? ''), fontWeight: 600, fontSize: 15 }, children: currentPlan?.name ?? current.plan_id }), _jsx("span", { style: { color: '#aaa', fontSize: 13 }, children: formatCycle(current.billing_cycle) }), _jsx("span", { style: {
                                    padding: '2px 8px',
                                    borderRadius: 4,
                                    background: current.status === 'active' ? '#1a3a1a' : '#3a1a1a',
                                    color: current.status === 'active' ? '#52c41a' : '#f5222d',
                                    fontSize: 12,
                                }, children: current.status }), _jsxs("span", { style: { color: '#888', fontSize: 12 }, children: ["\u6709\u6548\u671F\u81F3 ", formatDate(current.current_period_end)] })] })] })), error && (_jsx("div", { style: { color: '#f5222d', background: '#2a1a1a', borderRadius: 6, padding: '8px 12px', marginBottom: 12, fontSize: 13 }, children: error })), isLoading ? (_jsx("div", { style: { color: '#888', textAlign: 'center', padding: 32 }, children: "\u52A0\u8F7D\u4E2D\u2026" })) : (_jsxs("div", { style: { display: 'grid', gridTemplateColumns: 'repeat(auto-fill, minmax(200px, 1fr))', gap: 12 }, children: [plans.map(plan => {
                        const price = selectedCycle === 'monthly' ? plan.price_cny_monthly : plan.price_cny_yearly;
                        const isCurrent = current?.plan_id === plan.id;
                        const isSubmitting = submitting === plan.id;
                        return (_jsxs("div", { style: {
                                background: '#1a1a2e',
                                border: `2px solid ${isCurrent ? planColor(plan.name) : '#2a2a4a'}`,
                                borderRadius: 10,
                                padding: '16px',
                                display: 'flex',
                                flexDirection: 'column',
                                gap: 10,
                                transition: 'border-color 0.2s',
                            }, children: [_jsxs("div", { style: { display: 'flex', justifyContent: 'space-between', alignItems: 'center' }, children: [_jsx("span", { style: { color: planColor(plan.name), fontWeight: 700, fontSize: 16 }, children: plan.name }), isCurrent && (_jsx("span", { style: { fontSize: 11, color: planColor(plan.name), background: '#1a2a1a', padding: '2px 6px', borderRadius: 4 }, children: "\u5F53\u524D" }))] }), _jsx("div", { style: { color: '#888', fontSize: 12, minHeight: 32 }, children: plan.description }), _jsxs("div", { style: { color: '#fff', fontWeight: 600, fontSize: 20 }, children: [price === 0 ? '免费' : `¥${price}`, price > 0 && _jsxs("span", { style: { fontSize: 12, color: '#aaa', fontWeight: 400 }, children: ["/", selectedCycle === 'monthly' ? '月' : '年'] })] }), _jsxs("div", { style: { color: '#888', fontSize: 12, display: 'flex', flexDirection: 'column', gap: 3 }, children: [_jsxs("div", { children: ["\u6210\u5458\u4E0A\u9650\uFF1A", plan.quotas.max_members === -1 ? '不限' : plan.quotas.max_members] }), _jsxs("div", { children: ["\u6E56\u4E0A\u9650\uFF1A", plan.quotas.max_lakes === -1 ? '不限' : plan.quotas.max_lakes] }), _jsxs("div", { children: ["\u8282\u70B9\u4E0A\u9650\uFF1A", plan.quotas.max_nodes === -1 ? '不限' : plan.quotas.max_nodes] }), _jsxs("div", { children: ["\u5B58\u50A8\uFF1A", plan.quotas.max_storage_mb === -1 ? '不限' : `${plan.quotas.max_storage_mb} MB`] })] }), isOwner && (_jsx("button", { disabled: isSubmitting || (isCurrent && current?.billing_cycle === selectedCycle), onClick: () => handleSelect(plan), style: {
                                        marginTop: 4,
                                        padding: '6px 0',
                                        borderRadius: 6,
                                        border: 'none',
                                        cursor: isSubmitting || (isCurrent && current?.billing_cycle === selectedCycle) ? 'not-allowed' : 'pointer',
                                        background: isCurrent && current?.billing_cycle === selectedCycle ? '#2a2a3a' : planColor(plan.name),
                                        color: '#fff',
                                        fontWeight: 600,
                                        fontSize: 13,
                                        opacity: isSubmitting ? 0.7 : 1,
                                    }, children: isSubmitting ? '处理中…' : isCurrent && current?.billing_cycle === selectedCycle ? '当前套餐' : '选择' }))] }, plan.id));
                    }), plans.length === 0 && (_jsx("div", { style: { color: '#888', gridColumn: '1/-1', textAlign: 'center', padding: 32 }, children: "\u6682\u65E0\u53EF\u7528\u5957\u9910" }))] }))] }));
}
