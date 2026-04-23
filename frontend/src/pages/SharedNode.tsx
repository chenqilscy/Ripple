import { useEffect, useState } from 'react'
import { api } from '../api/client'
import type { NodeItem } from '../api/types'

/** P18-B 公开分享节点页面 — 无需登录 */
export function SharedNode() {
  const token = window.location.pathname.split('/')[2] ?? ''
  const [node, setNode] = useState<NodeItem | null>(null)
  const [loading, setLoading] = useState(true)
  const [err, setErr] = useState<string | null>(null)

  useEffect(() => {
    if (!token) {
      setErr('无效的分享链接')
      setLoading(false)
      return
    }
    api.getSharedNode(token)
      .then(r => { setNode(r.node) })
      .catch(e => { setErr((e as Error).message ?? '加载失败') })
      .finally(() => setLoading(false))
  }, [token])

  return (
    <div style={{
      minHeight: '100vh', background: '#0a1929', color: '#e0f0ff',
      fontFamily: 'system-ui, -apple-system, sans-serif',
      display: 'flex', flexDirection: 'column', alignItems: 'center',
      padding: '60px 24px',
    }}>
      {/* 品牌头部 */}
      <div style={{ marginBottom: 40, textAlign: 'center' }}>
        <div style={{ fontSize: 22, fontWeight: 600, color: '#9ec5ee', letterSpacing: 3 }}>青萍 Ripple</div>
        <div style={{ fontSize: 12, color: '#6c7086', marginTop: 6 }}>知识节点共享</div>
      </div>

      {loading && (
        <div style={{ color: '#6c7086', fontSize: 14 }}>加载中…</div>
      )}

      {!loading && err && (
        <div style={{
          padding: '20px 28px', maxWidth: 480, width: '100%', textAlign: 'center',
          background: 'rgba(255,80,80,0.08)', border: '1px solid rgba(255,80,80,0.3)',
          borderRadius: 10, color: '#ff9898', fontSize: 14,
        }}>
          <div style={{ fontSize: 24, marginBottom: 10 }}>🔗</div>
          {err === '404 Not Found' || err.includes('404')
            ? '该分享链接不存在或已失效'
            : err}
        </div>
      )}

      {!loading && node && (
        <div style={{ maxWidth: 660, width: '100%' }}>
          {/* 节点卡片 */}
          <div style={{
            background: 'rgba(255,255,255,0.04)',
            border: '1px solid rgba(255,255,255,0.1)',
            borderRadius: 12, padding: '24px 28px',
          }}>
            {/* 元信息行 */}
            <div style={{ display: 'flex', gap: 8, marginBottom: 16, alignItems: 'center', flexWrap: 'wrap' }}>
              <span style={{ background: stateColor(node.state), color: '#001020', fontSize: 10, padding: '2px 10px', borderRadius: 10, letterSpacing: 1, fontWeight: 600 }}>
                {node.state}
              </span>
              <span style={{ fontSize: 11, color: '#6c7086', background: 'rgba(255,255,255,0.06)', padding: '2px 8px', borderRadius: 6 }}>
                {node.type}
              </span>
              <span style={{ flex: 1 }} />
              <span style={{ fontSize: 11, color: '#6c7086' }}>
                {new Date(node.updated_at).toLocaleString('zh-CN', { year: 'numeric', month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit' })}
              </span>
            </div>

            {/* 节点内容 */}
            <div style={{
              fontSize: 15, lineHeight: 1.8, whiteSpace: 'pre-wrap',
              color: '#e0f0ff', wordBreak: 'break-word',
            }}>
              {node.content}
            </div>
          </div>

          {/* 页脚 */}
          <div style={{ textAlign: 'center', marginTop: 28, fontSize: 12, color: '#45475a' }}>
            通过{' '}
            <a href="/" style={{ color: '#9ec5ee', textDecoration: 'none' }}>青萍 Ripple</a>
            {' '}分享 · 节点 ID: <code style={{ fontSize: 10, color: '#6c7086' }}>{node.id.slice(0, 8)}</code>
          </div>
        </div>
      )}
    </div>
  )
}

function stateColor(s: string): string {
  const m: Record<string, string> = {
    MIST: '#7fbfff', DROP: '#89b4fa', FROZEN: '#a5d8ff',
    VAPOR: '#313244', WAVE: '#89dceb', STONE: '#cba6f7',
  }
  return m[s] ?? '#45475a'
}
