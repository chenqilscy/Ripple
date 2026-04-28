/**
 * Phase 15-B: Organization subscription management panel.
 * Shows current subscription status and available plans for upgrade/downgrade.
 */
import { useCallback, useEffect, useState } from 'react'
import { api } from '../api/client'
import type { BillingCycle, OrgLLMUsage, OrgSubscription, OrgUsage, SubscriptionPlan } from '../api/types'

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

function formatMoney(amount: number) {
  return `¥${amount.toFixed(amount >= 1 ? 2 : 3)}`
}

/** 用量进度条：max=-1 表示不限 */
function UsageBar({ label, used, max }: { label: string; used: number; max: number }) {
  const unlimited = max === -1
  const pct = unlimited ? 0 : Math.min(100, max === 0 ? 100 : Math.round((used / max) * 100))
  const danger = !unlimited && pct >= 90
  const barColor = danger ? '#f5222d' : '#4a8eff'
  return (
    <div style={{ background: '#1a1a2e', border: '1px solid #2a2a4a', borderRadius: 8, padding: '10px 16px', marginBottom: 8 }}>
      <div style={{ display: 'flex', justifyContent: 'space-between', fontSize: 13, color: '#aaa', marginBottom: 5 }}>
        <span>{label}</span>
        <span style={{ color: danger ? '#f5222d' : '#ccc' }}>
          {used} / {unlimited ? '不限' : max}
          {!unlimited && <span style={{ color: '#888', marginLeft: 4 }}>({pct}%)</span>}
        </span>
      </div>
      {!unlimited && (
        <div style={{ height: 4, background: '#2a2a4a', borderRadius: 2 }}>
          <div style={{ height: '100%', width: `${pct}%`, background: barColor, borderRadius: 2, transition: 'width 0.3s' }} />
        </div>
      )}
    </div>
  )
}

export default function SubscriptionPanel({ orgId, isOwner }: Props) {
  const [plans, setPlans] = useState<SubscriptionPlan[]>([])
  const [current, setCurrent] = useState<OrgSubscription | null>(null)
  const [usage, setUsage] = useState<OrgUsage | null>(null)
  const [llmUsage, setLlmUsage] = useState<OrgLLMUsage | null>(null)
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
      const [plansRes, subRes, usageRes, llmUsageRes] = await Promise.allSettled([
        api.listSubscriptionPlans(),
        api.getOrgSubscription(orgId),
        api.getOrgUsage(orgId),
        api.getOrgLLMUsage(orgId),
      ])
      if (plansRes.status === 'fulfilled') {
        setPlans(plansRes.value.plans)
      } else {
        setError('加载套餐失败: ' + String((plansRes.reason as Error)?.message || plansRes.reason))
      }
      if (subRes.status === 'fulfilled') {
        setCurrent(subRes.value.subscription)
      }
      if (usageRes.status === 'fulfilled') {
        setUsage(usageRes.value.usage)
      }
      if (llmUsageRes.status === 'fulfilled') {
        setLlmUsage(llmUsageRes.value)
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
  const llmTrend = llmUsage?.by_day.slice(-7) ?? []
  const maxTrendCalls = llmTrend.reduce((max, item) => Math.max(max, item.calls), 0)

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

      {/* Real usage progress bars (Phase 16) */}
      {usage !== null && currentPlan && (
        <UsageBar label="成员" used={usage.members} max={currentPlan.quotas.max_members} />
      )}
      {usage !== null && currentPlan && (
        <UsageBar label="湖" used={usage.lakes} max={currentPlan.quotas.max_lakes} />
      )}
      {usage !== null && currentPlan && (
        <UsageBar label="节点" used={usage.nodes} max={currentPlan.quotas.max_nodes} />
      )}
      {usage !== null && !currentPlan && (
        <div style={{ background: '#1a1a2e', border: '1px solid #2a2a4a', borderRadius: 8, padding: '12px 16px', marginBottom: 16 }}>
          <div style={{ fontSize: 13, color: '#aaa', marginBottom: 8 }}>当前用量</div>
          <div style={{ display: 'flex', gap: 24, fontSize: 13, color: '#ccc' }}>
            <span>成员 <b style={{ color: '#fff' }}>{usage.members}</b></span>
            <span>湖 <b style={{ color: '#fff' }}>{usage.lakes}</b></span>
            <span>节点 <b style={{ color: '#fff' }}>{usage.nodes}</b></span>
          </div>
        </div>
      )}

      {llmUsage !== null && (
        <div style={{ background: '#1a1a2e', border: '1px solid #2a2a4a', borderRadius: 8, padding: '12px 16px', marginBottom: 16 }}>
          <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', gap: 12, flexWrap: 'wrap', marginBottom: 12 }}>
            <div>
              <div style={{ fontSize: 13, color: '#aaa', marginBottom: 4 }}>AI 用量账单</div>
              <div style={{ display: 'flex', gap: 16, flexWrap: 'wrap', fontSize: 13, color: '#ccc' }}>
                <span>周期 <b style={{ color: '#fff' }}>近 {llmUsage.period_days} 天</b></span>
                <span>调用 <b style={{ color: '#fff' }}>{llmUsage.total_calls}</b></span>
                <span>估算费用 <b style={{ color: '#fff' }}>{formatMoney(llmUsage.total_estimated_cost_cny)}</b></span>
              </div>
            </div>
            <div style={{ fontSize: 12, color: '#888' }}>按 provider / 按日聚合</div>
          </div>

          {llmUsage.by_provider.length > 0 ? (
            <div style={{ display: 'grid', gap: 10 }}>
              <div style={{ display: 'grid', gridTemplateColumns: 'minmax(84px,1fr) minmax(72px,96px) minmax(96px,120px) minmax(96px,120px)', gap: 8, fontSize: 12, color: '#7f8ea3' }}>
                <span>Provider</span>
                <span>调用次数</span>
                <span>平均耗时</span>
                <span>估算费用</span>
              </div>
              {llmUsage.by_provider.map(item => (
                <div
                  key={item.provider}
                  style={{ display: 'grid', gridTemplateColumns: 'minmax(84px,1fr) minmax(72px,96px) minmax(96px,120px) minmax(96px,120px)', gap: 8, fontSize: 13, color: '#d7def0', padding: '10px 12px', borderRadius: 8, background: '#141424', border: '1px solid #23233a' }}
                >
                  <span style={{ fontWeight: 600 }}>{item.provider}</span>
                  <span>{item.calls}</span>
                  <span>{item.avg_duration_ms} ms</span>
                  <span>{formatMoney(item.estimated_cost_cny)}</span>
                </div>
              ))}
            </div>
          ) : (
            <div style={{ color: '#888', fontSize: 13 }}>最近 {llmUsage.period_days} 天暂无 AI 调用记录</div>
          )}

          {llmTrend.length > 0 && (
            <div style={{ marginTop: 14 }}>
              <div style={{ fontSize: 12, color: '#7f8ea3', marginBottom: 8 }}>近 7 天趋势</div>
              <div style={{ display: 'grid', gridTemplateColumns: `repeat(${llmTrend.length}, minmax(0, 1fr))`, gap: 8, alignItems: 'end', minHeight: 110 }}>
                {llmTrend.map(item => {
                  const heightPct = maxTrendCalls === 0 ? 0 : Math.max(8, Math.round((item.calls / maxTrendCalls) * 100))
                  return (
                    <div key={item.date} style={{ display: 'flex', flexDirection: 'column', gap: 6, alignItems: 'stretch' }}>
                      <div title={`${item.date} · ${item.calls} 次 · ${formatMoney(item.estimated_cost_cny)}`} style={{ minHeight: 78, display: 'flex', alignItems: 'end' }}>
                        <div style={{ width: '100%', height: `${heightPct}%`, background: 'linear-gradient(180deg, #71a7ff 0%, #4a8eff 100%)', borderRadius: 6, minHeight: item.calls > 0 ? 8 : 0 }} />
                      </div>
                      <div style={{ display: 'grid', gap: 2, textAlign: 'center' }}>
                        <span style={{ fontSize: 11, color: '#d7def0' }}>{item.calls}</span>
                        <span style={{ fontSize: 10, color: '#7f8ea3' }}>{item.date.slice(5)}</span>
                      </div>
                    </div>
                  )
                })}
              </div>
            </div>
          )}
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
