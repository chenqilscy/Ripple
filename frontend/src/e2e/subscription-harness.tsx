import React from 'react'
import ReactDOM from 'react-dom/client'
import SubscriptionPanel from '../components/SubscriptionPanel'

function Harness() {
  return (
    <div style={{ minHeight: '100vh', background: 'linear-gradient(180deg, #0f1020 0%, #14162b 100%)', color: '#fff', padding: '32px 20px' }}>
      <div style={{ maxWidth: 1120, margin: '0 auto' }}>
        <div style={{ marginBottom: 20 }}>
          <h1 style={{ margin: '0 0 8px', fontSize: 28, fontWeight: 600 }}>Subscription Harness</h1>
          <p style={{ margin: 0, color: '#8b93a7', fontSize: 14 }}>仅用于 Playwright 回归验证 SubscriptionPanel 与 AI 用量账单视图。</p>
        </div>
        <SubscriptionPanel orgId="org-e2e" isOwner />
      </div>
    </div>
  )
}

ReactDOM.createRoot(document.getElementById('root')!).render(
  <React.StrictMode>
    <Harness />
  </React.StrictMode>,
)