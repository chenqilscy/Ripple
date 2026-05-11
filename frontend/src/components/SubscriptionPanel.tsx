/**
 * Phase 15-B: Organization subscription management panel.
 * Shows current subscription status and available plans for upgrade/downgrade.
 * 修复：scroll lock + CSS 变量（Deep Ocean Dark 主题）
 */
import { useCallback, useEffect, useState } from 'react'
import { Button } from './ui'
import { api } from '../api/client'
import type { BillingCycle, OrgLLMUsage, OrgSubscription, OrgUsage, SubscriptionPlan, UsageAlert } from '../api/types'

interface Props {
  orgId: string
  /** User's role in the org. Only OWNER can change subscription. */
  isOwner: boolean
}

function planColor(planName: string): string {
  const key = planName.toLowerCase()
  if (key === 'free') return 'var(--text-tertiary)'
  if (key === 'pro') return 'var(--accent)'
  if (key === 'team') return 'var(--status-warning)'
  return 'var(--status-success)'
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

// UsageAlertBanner - Warning banner when usage exceeds threshold
function UsageAlertBanner({
  usedPercent,
  threshold,
  onConfigClick
}: {
  usedPercent: number
  threshold: number
  onConfigClick: () => void
}) {
  if (usedPercent < threshold) {
    return null
  }

  return (
    <div style={{
      background: 'var(--status-danger-subtle)',
      border: '1px solid var(--status-danger)',
      borderRadius: 'var(--radius-md)',
      padding: 'var(--space-md)',
      marginBottom: 'var(--space-md)',
      display: 'flex',
      justifyContent: 'space-between',
      alignItems: 'center',
    }}>
      <div>
        <strong style={{ color: 'var(--status-danger)' }}>⚠ AI用量告警</strong>
        <p style={{ margin: 'var(--space-xs) 0 0', color: 'var(--text-secondary)', fontSize: 'var(--font-sm)' }}>
          您的AI调用量已达到配额的 {usedPercent}%，建议升级套餐
        </p>
      </div>
      <Button variant="primary" size="sm" onClick={onConfigClick}>
        设置告警
      </Button>
    </div>
  )
}

// UsageAlertConfigModal - Settings dialog for alert configuration
function UsageAlertConfigModal({
  orgId,
  currentSettings,
  onSave,
  onClose
}: {
  orgId: string
  currentSettings: { threshold_percent: number; enabled: boolean }
  onSave: (settings: { threshold_percent: number; enabled: boolean }) => void
  onClose: () => void
}) {
  const [threshold, setThreshold] = useState(currentSettings.threshold_percent)
  const [enabled, setEnabled] = useState(currentSettings.enabled)
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState<string | null>(null)

  async function handleSave() {
    setSaving(true)
    setError(null)
    try {
      const updated = await api.updateUsageAlert(orgId, { threshold_percent: threshold, enabled })
      onSave({ threshold_percent: updated.threshold_percent, enabled: updated.enabled })
      onClose()
    } catch (e: unknown) {
      const err = e as { message?: string }
      setError(err?.message || '保存失败')
    } finally {
      setSaving(false)
    }
  }

  return (
    <div style={{
      position: 'fixed',
      inset: 0,
      background: 'rgba(0,0,0,0.6)',
      display: 'flex',
      alignItems: 'center',
      justifyContent: 'center',
      zIndex: 1000,
    }} onClick={onClose}>
      <div style={{
        background: 'var(--bg-surface)',
        borderRadius: 'var(--radius-lg)',
        padding: 'var(--space-xl)',
        width: 400,
        maxWidth: '90vw',
      }} onClick={e => e.stopPropagation()}>
        <h3 style={{ margin: '0 0 var(--space-lg) 0', color: 'var(--text-primary)' }}>
          AI用量告警设置
        </h3>

        <label style={{ display: 'flex', alignItems: 'center', gap: 'var(--space-sm)', marginBottom: 'var(--space-md)', cursor: 'pointer' }}>
          <input
            type="checkbox"
            checked={enabled}
            onChange={e => setEnabled(e.target.checked)}
          />
          启用用量告警
        </label>

        {enabled && (
          <div style={{ marginBottom: 'var(--space-lg)' }}>
            <label style={{ display: 'block', marginBottom: 'var(--space-sm)', color: 'var(--text-secondary)' }}>
              告警阈值
            </label>
            <select
              value={threshold}
              onChange={e => setThreshold(Number(e.target.value))}
              style={{
                width: '100%',
                padding: 'var(--space-sm)',
                background: 'var(--bg-input)',
                border: '1px solid var(--border-input)',
                borderRadius: 'var(--radius-md)',
                color: 'var(--text-primary)',
              }}
            >
              <option value={50}>50%</option>
              <option value={80}>80%</option>
              <option value={90}>90%</option>
              <option value={100}>100%</option>
            </select>
            <p style={{ margin: 'var(--space-xs) 0 0', fontSize: 'var(--font-sm)', color: 'var(--text-tertiary)' }}>
              当用量达到此百分比时发送通知
            </p>
          </div>
        )}

        {error && (
          <p style={{ color: 'var(--status-danger)', marginBottom: 'var(--space-md)', fontSize: 'var(--font-sm)' }}>
            {error}
          </p>
        )}

        <div style={{ display: 'flex', gap: 'var(--space-sm)', justifyContent: 'flex-end' }}>
          <Button variant="secondary" onClick={onClose}>
            取消
          </Button>
          <Button variant="primary" onClick={handleSave} disabled={saving}>
            {saving ? '保存中…' : '保存'}
          </Button>
        </div>
      </div>
    </div>
  )
}

/** 用量进度条：max=-1 表示不限 */
function UsageBar({ label, used, max }: { label: string; used: number; max: number }) {
  const unlimited = max === -1
  const pct = unlimited ? 0 : Math.min(100, max === 0 ? 100 : Math.round((used / max) * 100))
  const danger = !unlimited && pct >= 90
  return (
    <div style={{ background: 'var(--bg-surface)', border: '1px solid var(--border)', borderRadius: 'var(--radius-lg)', padding: 'var(--space-md) var(--space-lg)', marginBottom: 'var(--space-sm)' }}>
      <div style={{ display: 'flex', justifyContent: 'space-between', fontSize: 'var(--font-md)', color: 'var(--text-secondary)', marginBottom: 'var(--space-xs)' }}>
        <span>{label}</span>
        <span style={{ color: danger ? 'var(--status-danger)' : 'var(--text-primary)' }}>
          {used} / {unlimited ? '不限' : max}
          {!unlimited && <span style={{ color: 'var(--text-tertiary)', marginLeft: 'var(--space-xs)' }}>({pct}%)</span>}
        </span>
      </div>
      {!unlimited && (
        <div style={{ height: 4, background: 'var(--bg-tertiary)', borderRadius: 'var(--radius-sm)' }}>
          <div style={{ height: '100%', width: `${pct}%`, background: danger ? 'var(--status-danger)' : 'var(--accent)', borderRadius: 'var(--radius-sm)', transition: 'width 0.3s' }} />
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
  const [submitting, setSubmitting] = useState<string | null>(null)
  const [alertSettings, setAlertSettings] = useState<{ threshold_percent: number; enabled: boolean } | null>(null)
  const [showAlertConfig, setShowAlertConfig] = useState(false)

  // Scroll lock
  useEffect(() => {
    const prev = document.body.style.overflow
    document.body.style.overflow = 'hidden'
    return () => { document.body.style.overflow = prev }
  }, [])

  const loadData = useCallback(async () => {
    setError(null)
    setLoadingPlans(true)
    setLoadingSub(true)
    try {
      const loadErrors: string[] = []
      const [plansRes, subRes, usageRes, llmUsageRes] = await Promise.allSettled([
        api.listSubscriptionPlans(),
        api.getOrgSubscription(orgId),
        api.getOrgUsage(orgId),
        api.getOrgLLMUsage(orgId),
      ])
      if (plansRes.status === 'fulfilled') {
        setPlans(plansRes.value.plans)
      } else {
        loadErrors.push('加载套餐失败: ' + String((plansRes.reason as Error)?.message || plansRes.reason))
        setPlans([])
      }
      if (subRes.status === 'fulfilled') {
        setCurrent(subRes.value.subscription)
      } else {
        setCurrent(null)
        loadErrors.push('加载当前订阅失败: ' + String((subRes.reason as Error)?.message || subRes.reason))
      }
      if (usageRes.status === 'fulfilled') {
        setUsage(usageRes.value.usage)
      } else {
        setUsage(null)
        loadErrors.push('加载组织用量失败: ' + String((usageRes.reason as Error)?.message || usageRes.reason))
      }
      if (llmUsageRes.status === 'fulfilled') {
        setLlmUsage(llmUsageRes.value)
      } else {
        setLlmUsage(null)
        loadErrors.push('加载 AI 用量失败: ' + String((llmUsageRes.reason as Error)?.message || llmUsageRes.reason))
      }
      setError(loadErrors.length > 0 ? loadErrors.join('；') : null)
    } finally {
      setLoadingPlans(false)
      setLoadingSub(false)
    }
  }, [orgId])

  useEffect(() => { loadData() }, [loadData])

  // Load alert settings for org owner
  useEffect(() => {
    if (orgId && isOwner) {
      api.getUsageAlert(orgId)
        .then((settings: UsageAlert) => setAlertSettings({
          threshold_percent: settings.threshold_percent,
          enabled: settings.enabled,
        }))
        .catch(() => setAlertSettings({ threshold_percent: 80, enabled: false }))
    }
  }, [orgId, isOwner])

  const handleSelect = useCallback(async (plan: SubscriptionPlan) => {
    if (!isOwner) return
    if (current?.plan_id === plan.id && current?.billing_cycle === selectedCycle) return
    setSubmitting(plan.id)
    setError(null)
    try {
      const res = await api.createOrgSubscription(orgId, plan.id, selectedCycle, true)
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
  // Calculate AI usage percentage based on LLM usage data
  // TODO: Use actual quota from Space or plan when available
  const aiUsedPercent = llmUsage && llmUsage.total_calls > 0 ? Math.min(100, llmUsage.total_calls) : 0

  return (
    <div style={{ padding: 'var(--space-lg) 0' }}>
      {/* Usage alert banner */}
      {alertSettings?.enabled && alertSettings && (
        <UsageAlertBanner
          usedPercent={aiUsedPercent}
          threshold={alertSettings.threshold_percent}
          onConfigClick={() => setShowAlertConfig(true)}
        />
      )}

      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 'var(--space-lg)' }}>
        <h3 style={{ margin: 0, color: 'var(--text-primary)', fontSize: 'var(--font-xl)', fontWeight: 600 }}>订阅套餐</h3>
        <div style={{ display: 'flex', gap: 'var(--space-sm)' }}>
          {(['monthly', 'yearly'] as BillingCycle[]).map(c => (
            <Button
              key={c}
              variant={selectedCycle === c ? 'primary' : 'secondary'}
              size="sm"
              onClick={() => setSelectedCycle(c)}
            >
              {c === 'monthly' ? '月付' : '年付'}
            </Button>
          ))}
        </div>
      </div>

      {/* Current subscription summary */}
      {current && (
        <div style={{
          background: 'var(--bg-surface)',
          border: '1px solid var(--border)',
          borderRadius: 'var(--radius-lg)',
          padding: 'var(--space-md) var(--space-lg)',
          marginBottom: 'var(--space-lg)',
        }}>
          <div style={{ fontSize: 'var(--font-md)', color: 'var(--text-tertiary)', marginBottom: 'var(--space-xs)' }}>当前套餐</div>
          <div style={{ display: 'flex', gap: 'var(--space-lg)', alignItems: 'center', flexWrap: 'wrap' }}>
            <span style={{ color: planColor(currentPlan?.name ?? ''), fontWeight: 600, fontSize: 'var(--font-lg)' }}>
              {currentPlan?.name ?? current.plan_id}
            </span>
            <span style={{ color: 'var(--text-secondary)', fontSize: 'var(--font-md)' }}>{formatCycle(current.billing_cycle)}</span>
            <span style={{
              padding: '2px var(--space-sm)',
              borderRadius: 'var(--radius-sm)',
              background: current.status === 'active' ? 'var(--status-success-subtle)' : 'var(--status-danger-subtle)',
              color: current.status === 'active' ? 'var(--status-success)' : 'var(--status-danger)',
              fontSize: 'var(--font-sm)',
            }}>
              {current.status}
            </span>
            <span style={{ color: 'var(--text-tertiary)', fontSize: 'var(--font-sm)' }}>
              有效期至 {formatDate(current.current_period_end)}
            </span>
          </div>
        </div>
      )}

      {/* Real usage progress bars */}
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
        <div style={{ background: 'var(--bg-surface)', border: '1px solid var(--border)', borderRadius: 'var(--radius-lg)', padding: 'var(--space-md) var(--space-lg)', marginBottom: 'var(--space-lg)' }}>
          <div style={{ fontSize: 'var(--font-md)', color: 'var(--text-tertiary)', marginBottom: 'var(--space-sm)' }}>当前用量</div>
          <div style={{ display: 'flex', gap: 'var(--space-lg)', fontSize: 'var(--font-md)', color: 'var(--text-secondary)' }}>
            <span>成员 <b style={{ color: 'var(--text-primary)' }}>{usage.members}</b></span>
            <span>湖 <b style={{ color: 'var(--text-primary)' }}>{usage.lakes}</b></span>
            <span>节点 <b style={{ color: 'var(--text-primary)' }}>{usage.nodes}</b></span>
          </div>
        </div>
      )}

      {llmUsage !== null && (
        <div style={{ background: 'var(--bg-surface)', border: '1px solid var(--border)', borderRadius: 'var(--radius-lg)', padding: 'var(--space-md) var(--space-lg)', marginBottom: 'var(--space-lg)' }}>
          <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', gap: 'var(--space-md)', flexWrap: 'wrap', marginBottom: 'var(--space-md)' }}>
            <div>
              <div style={{ fontSize: 'var(--font-md)', color: 'var(--text-tertiary)', marginBottom: 'var(--space-xs)' }}>AI 用量账单</div>
              <div style={{ display: 'flex', gap: 'var(--space-lg)', flexWrap: 'wrap', fontSize: 'var(--font-md)', color: 'var(--text-secondary)' }}>
                <span>周期 <b style={{ color: 'var(--text-primary)' }}>近 {llmUsage.period_days} 天</b></span>
                <span>调用 <b style={{ color: 'var(--text-primary)' }}>{llmUsage.total_calls}</b></span>
                <span>估算费用 <b style={{ color: 'var(--text-primary)' }}>{formatMoney(llmUsage.total_estimated_cost_cny)}</b></span>
              </div>
            </div>
            <div style={{ fontSize: 'var(--font-sm)', color: 'var(--text-tertiary)' }}>按 provider / 按日聚合</div>
          </div>

          {llmUsage.by_provider.length > 0 ? (
            <div style={{ display: 'grid', gap: 'var(--space-sm)' }}>
              <div style={{ display: 'grid', gridTemplateColumns: 'minmax(84px,1fr) minmax(72px,96px) minmax(96px,120px) minmax(96px,120px)', gap: 'var(--space-sm)', fontSize: 'var(--font-sm)', color: 'var(--text-tertiary)' }}>
                <span>Provider</span>
                <span>调用次数</span>
                <span>平均耗时</span>
                <span>估算费用</span>
              </div>
              {llmUsage.by_provider.map(item => (
                <div
                  key={item.provider}
                  style={{ display: 'grid', gridTemplateColumns: 'minmax(84px,1fr) minmax(72px,96px) minmax(96px,120px) minmax(96px,120px)', gap: 'var(--space-sm)', fontSize: 'var(--font-md)', color: 'var(--text-primary)', padding: 'var(--space-md)', borderRadius: 'var(--radius-md)', background: 'var(--bg-secondary)', border: '1px solid var(--border)' }}
                >
                  <span style={{ fontWeight: 600 }}>{item.provider}</span>
                  <span>{item.calls}</span>
                  <span>{item.avg_duration_ms} ms</span>
                  <span>{formatMoney(item.estimated_cost_cny)}</span>
                </div>
              ))}
            </div>
          ) : (
            <div style={{ color: 'var(--text-tertiary)', fontSize: 'var(--font-md)' }}>最近 {llmUsage.period_days} 天暂无 AI 调用记录</div>
          )}

          {llmTrend.length > 0 && (
            <div style={{ marginTop: 'var(--space-lg)' }}>
              <div style={{ fontSize: 'var(--font-sm)', color: 'var(--text-tertiary)', marginBottom: 'var(--space-sm)' }}>近 7 天趋势</div>
              <div style={{ display: 'grid', gridTemplateColumns: `repeat(${llmTrend.length}, minmax(0, 1fr))`, gap: 'var(--space-sm)', alignItems: 'end', minHeight: 110 }}>
                {llmTrend.map(item => {
                  const heightPct = maxTrendCalls === 0 ? 0 : Math.max(8, Math.round((item.calls / maxTrendCalls) * 100))
                  return (
                    <div key={item.date} style={{ display: 'flex', flexDirection: 'column', gap: 'var(--space-sm)', alignItems: 'stretch' }}>
                      <div title={`${item.date} · ${item.calls} 次 · ${formatMoney(item.estimated_cost_cny)}`} style={{ minHeight: 78, display: 'flex', alignItems: 'end' }}>
                        <div style={{ width: '100%', height: `${heightPct}%`, background: 'linear-gradient(180deg, var(--accent-subtle) 0%, var(--accent) 100%)', borderRadius: 'var(--radius-md)', minHeight: item.calls > 0 ? 8 : 0 }} />
                      </div>
                      <div style={{ display: 'grid', gap: 2, textAlign: 'center' }}>
                        <span style={{ fontSize: 'var(--font-sm)', color: 'var(--text-primary)' }}>{item.calls}</span>
                        <span style={{ fontSize: 'var(--font-xs)', color: 'var(--text-tertiary)' }}>{item.date.slice(5)}</span>
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
        <div style={{ color: 'var(--status-danger)', background: 'var(--status-danger-subtle)', borderRadius: 'var(--radius-md)', padding: 'var(--space-sm) var(--space-md)', marginBottom: 'var(--space-md)', fontSize: 'var(--font-md)' }}>
          {error}
        </div>
      )}

      {isLoading ? (
        <div style={{ color: 'var(--text-tertiary)', textAlign: 'center', padding: 'var(--space-xl)' }}>加载中…</div>
      ) : (
        <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fill, minmax(200px, 1fr))', gap: 'var(--space-md)' }}>
          {plans.map(plan => {
            const price = selectedCycle === 'monthly' ? plan.price_cny_monthly : plan.price_cny_yearly
            const hasExactPrice = typeof price === 'number'
            const isFreePlan = plan.price_cny_monthly === 0 && (!hasExactPrice || price === 0)
            const isCurrent = current?.plan_id === plan.id
            const isSubmitting = submitting === plan.id
            const planColorVal = planColor(plan.name)

            return (
              <div
                data-testid={`plan-card-${plan.id}`}
                key={plan.id}
                style={{
                  background: 'var(--bg-surface)',
                  border: `2px solid ${isCurrent ? planColorVal : 'var(--border)'}`,
                  borderRadius: 'var(--radius-xl)',
                  padding: 'var(--space-lg)',
                  display: 'flex',
                  flexDirection: 'column',
                  gap: 'var(--space-sm)',
                  transition: 'border-color 0.2s',
                }}
              >
                <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
                  <span style={{ color: planColorVal, fontWeight: 700, fontSize: 'var(--font-lg)' }}>
                    {plan.name}
                  </span>
                  {isCurrent && (
                    <span style={{ fontSize: 'var(--font-xs)', color: planColorVal, background: 'var(--status-success-subtle)', padding: '2px var(--space-sm)', borderRadius: 'var(--radius-sm)' }}>
                      当前
                    </span>
                  )}
                </div>
                <div style={{ color: 'var(--text-tertiary)', fontSize: 'var(--font-sm)', minHeight: 32 }}>{plan.description}</div>
                <div style={{ color: 'var(--text-primary)', fontWeight: 600, fontSize: 'var(--font-xl)' }}>
                  {isFreePlan ? '免费' : hasExactPrice ? `¥${price}` : '价格待配置'}
                  {hasExactPrice && !isFreePlan && <span style={{ fontSize: 'var(--font-sm)', color: 'var(--text-secondary)', fontWeight: 400 }}>/{selectedCycle === 'monthly' ? '月' : '年'}</span>}
                </div>
                {!hasExactPrice && selectedCycle === 'yearly' && !isFreePlan && (
                  <div style={{ color: 'var(--text-tertiary)', fontSize: 'var(--font-sm)' }}>年付金额以后端套餐配置为准</div>
                )}
                <div style={{ color: 'var(--text-tertiary)', fontSize: 'var(--font-sm)', display: 'flex', flexDirection: 'column', gap: 3 }}>
                  <div>成员上限：{plan.quotas.max_members === -1 ? '不限' : plan.quotas.max_members}</div>
                  <div>湖上限：{plan.quotas.max_lakes === -1 ? '不限' : plan.quotas.max_lakes}</div>
                  <div>节点上限：{plan.quotas.max_nodes === -1 ? '不限' : plan.quotas.max_nodes}</div>
                  <div>存储：{plan.quotas.max_storage_mb === -1 ? '不限' : `${plan.quotas.max_storage_mb} MB`}</div>
                </div>
                {isOwner && (
                  <Button
                    variant={isCurrent && current?.billing_cycle === selectedCycle ? 'secondary' : 'primary'}
                    size="md"
                    disabled={isSubmitting || (isCurrent && current?.billing_cycle === selectedCycle)}
                    onClick={() => handleSelect(plan)}
                  >
                    {isSubmitting ? '处理中…' : isCurrent && current?.billing_cycle === selectedCycle ? '当前套餐' : '选择'}
                  </Button>
                )}
              </div>
            )
          })}
          {plans.length === 0 && (
            <div style={{ color: 'var(--text-tertiary)', gridColumn: '1/-1', textAlign: 'center', padding: 'var(--space-xl)' }}>
              暂无可用套餐
            </div>
          )}
        </div>
      )}

      {/* Usage alert config modal */}
      {showAlertConfig && alertSettings && (
        <UsageAlertConfigModal
          orgId={orgId}
          currentSettings={alertSettings}
          onSave={setAlertSettings}
          onClose={() => setShowAlertConfig(false)}
        />
      )}
    </div>
  )
}
