/**
 * Phase 15-B: Organization subscription management panel.
 * Shows current subscription status and available plans for upgrade/downgrade.
 */
import { useCallback, useEffect, useState } from 'react'
import { api } from '../api/client'
import type { BillingCycle, OrgSubscription, SubscriptionPlan } from '../api/types'

interface Props {
  orgId: string
  /** User's role in the org. Only OWNER can change subscription. */
  isOwner: boolean
}

const PLAN_COLORS: Record<string, string> = {
  free:  '#6c757d',
  pro:   '#4a8eff',
  team:  '#f5a623',
}

function planColor(planName: string): string {
  const key = planName.toLowerCase()
  return PLAN_COLORS[key] ?? '#52c41a'
}

function formatCycle(cycle: BillingCycle) {
  return cycle === 'monthly' ? '月付' : '年付'
}

function formatDate(iso: string) {
  return new Date(iso).toLocaleDateString('zh-CN')
}

export default function SubscriptionPanel({ orgId, isOwner }: Props) {
  const [plans, setPlans] = useState<SubscriptionPlan[]>([])
  const [current, setCurrent] = useState<OrgSubscription | null>(null)
  const [loadingPlans, setLoadingPlans] = useState(false)
  const [loadingSub, setLoadingSub] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [selectedCycle, setSelectedCycle] = useState<BillingCycle>('monthly')
  const [submitting, setSubmitting] = useState<string | null>(null) // planId being submitted

  const loadData = useCallback(async () => {
    setError(null)
    setLoadingPlans(true)
    setLoadingSub(true)
    try {
      const [plansRes, subRes] = await Promise.allSettled([
        api.listSubscriptionPlans(),
        api.getOrgSubscription(orgId),
      ])
      if (plansRes.status === 'fulfilled') {
        setPlans(plansRes.value.plans)
      } else {
        setError('加载套餐失败: ' + String((plansRes.reason as Error)?.message || plansRes.reason))
      }
      if (subRes.status === 'fulfilled') {
        setCurrent(subRes.value.subscription)
      }
      // 404 means no subscription yet — that's OK
    } finally {
      setLoadingPlans(false)
      setLoadingSub(false)
    }
  }, [orgId])

  useEffect(() => { loadData() }, [loadData])

  const handleSelect = useCallback(async (plan: SubscriptionPlan) => {
    if (!isOwner) return
    if (current?.plan_id === plan.id && current?.billing_cycle === selectedCycle) return
    setSubmitting(plan.id)
    setError(null)
    try {
      const res = await api.createOrgSubscription(orgId, plan.id, selectedCycle, /* stub_confirm */ true)
      setCurrent(res.subscription)
    } catch (e) {
      setError('订阅失败: ' + String((e as Error)?.message || e))
    } finally {
      setSubmitting(null)
    }
  }, [orgId, isOwner, current, selectedCycle])

  const currentPlan = current ? plans.find(p => p.id === current.plan_id) : null
  const isLoading = loadingPlans || loadingSub

  return (
    <div style={{ padding: '16px 0' }}>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 16 }}>
        <h3 style={{ margin: 0, color: '#fff' }}>订阅套餐</h3>
        <div style={{ display: 'flex', gap: 8 }}>
          {(['monthly', 'yearly'] as BillingCycle[]).map(c => (
            <button
              key={c}
              onClick={() => setSelectedCycle(c)}
              style={{
                padding: '4px 12px',
                borderRadius: 4,
                border: 'none',
                cursor: 'pointer',
                background: selectedCycle === c ? '#4a8eff' : '#2a2a3a',
                color: '#fff',
                fontSize: 13,
              }}
            >
              {c === 'monthly' ? '月付' : '年付'}
              {c === 'yearly' && <span style={{ color: '#52c41a', marginLeft: 4, fontSize: 11 }}>省20%</span>}
            </button>
          ))}
        </div>
      </div>

      {/* Current subscription summary */}
      {current && (
        <div style={{
          background: '#1a1a2e',
          border: '1px solid #2a2a4a',
          borderRadius: 8,
          padding: '12px 16px',
          marginBottom: 16,
        }}>
          <div style={{ fontSize: 13, color: '#aaa', marginBottom: 4 }}>当前套餐</div>
          <div style={{ display: 'flex', gap: 16, alignItems: 'center', flexWrap: 'wrap' }}>
            <span style={{ color: planColor(currentPlan?.name ?? ''), fontWeight: 600, fontSize: 15 }}>
              {currentPlan?.name ?? current.plan_id}
            </span>
            <span style={{ color: '#aaa', fontSize: 13 }}>{formatCycle(current.billing_cycle)}</span>
            <span style={{
              padding: '2px 8px',
              borderRadius: 4,
              background: current.status === 'active' ? '#1a3a1a' : '#3a1a1a',
              color: current.status === 'active' ? '#52c41a' : '#f5222d',
              fontSize: 12,
            }}>
              {current.status}
            </span>
            <span style={{ color: '#888', fontSize: 12 }}>
              有效期至 {formatDate(current.current_period_end)}
            </span>
          </div>
        </div>
      )}

      {error && (
        <div style={{ color: '#f5222d', background: '#2a1a1a', borderRadius: 6, padding: '8px 12px', marginBottom: 12, fontSize: 13 }}>
          {error}
        </div>
      )}

      {isLoading ? (
        <div style={{ color: '#888', textAlign: 'center', padding: 32 }}>加载中…</div>
      ) : (
        <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fill, minmax(200px, 1fr))', gap: 12 }}>
          {plans.map(plan => {
            const price = selectedCycle === 'monthly' ? plan.price_cny_monthly : plan.price_cny_yearly
            const isCurrent = current?.plan_id === plan.id
            const isSubmitting = submitting === plan.id

            return (
              <div
                key={plan.id}
                style={{
                  background: '#1a1a2e',
                  border: `2px solid ${isCurrent ? planColor(plan.name) : '#2a2a4a'}`,
                  borderRadius: 10,
                  padding: '16px',
                  display: 'flex',
                  flexDirection: 'column',
                  gap: 10,
                  transition: 'border-color 0.2s',
                }}
              >
                <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
                  <span style={{ color: planColor(plan.name), fontWeight: 700, fontSize: 16 }}>
                    {plan.name}
                  </span>
                  {isCurrent && (
                    <span style={{ fontSize: 11, color: planColor(plan.name), background: '#1a2a1a', padding: '2px 6px', borderRadius: 4 }}>
                      当前
                    </span>
                  )}
                </div>
                <div style={{ color: '#888', fontSize: 12, minHeight: 32 }}>{plan.description}</div>
                <div style={{ color: '#fff', fontWeight: 600, fontSize: 20 }}>
                  {price === 0 ? '免费' : `¥${price}`}
                  {price > 0 && <span style={{ fontSize: 12, color: '#aaa', fontWeight: 400 }}>/{selectedCycle === 'monthly' ? '月' : '年'}</span>}
                </div>
                <div style={{ color: '#888', fontSize: 12, display: 'flex', flexDirection: 'column', gap: 3 }}>
                  <div>成员上限：{plan.quotas.max_members === -1 ? '不限' : plan.quotas.max_members}</div>
                  <div>湖上限：{plan.quotas.max_lakes === -1 ? '不限' : plan.quotas.max_lakes}</div>
                  <div>节点上限：{plan.quotas.max_nodes === -1 ? '不限' : plan.quotas.max_nodes}</div>
                  <div>存储：{plan.quotas.max_storage_mb === -1 ? '不限' : `${plan.quotas.max_storage_mb} MB`}</div>
                </div>
                {isOwner && (
                  <button
                    disabled={isSubmitting || (isCurrent && current?.billing_cycle === selectedCycle)}
                    onClick={() => handleSelect(plan)}
                    style={{
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
                    }}
                  >
                    {isSubmitting ? '处理中…' : isCurrent && current?.billing_cycle === selectedCycle ? '当前套餐' : '选择'}
                  </button>
                )}
              </div>
            )
          })}
          {plans.length === 0 && (
            <div style={{ color: '#888', gridColumn: '1/-1', textAlign: 'center', padding: 32 }}>
              暂无可用套餐
            </div>
          )}
        </div>
      )}
    </div>
  )
}
