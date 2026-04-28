/**
 * P18-B: 节点外链分享按钮与管理面板。
 * 显示当前节点的分享链接列表，支持创建新链接和撤销已有链接。
 */
import { useState, useEffect, useCallback } from 'react'
import { api } from '../api/client'
import type { NodeItem, NodeShare } from '../api/types'

interface Props {
  node: NodeItem
  onClose: () => void
}

export default function NodeShareButton({ node, onClose }: Props) {
  const [shares, setShares] = useState<NodeShare[]>([])
  const [loading, setLoading] = useState(true)
  const [creating, setCreating] = useState(false)
  const [ttlHours, setTtlHours] = useState<number>(0)
  const [err, setErr] = useState<string | null>(null)
  const [copied, setCopied] = useState<string | null>(null)

  const loadShares = useCallback(async () => {
    try {
      const res = await api.listNodeShares(node.id)
      setShares(res.shares.filter(s => !s.revoked))
    } catch {
      // 非致命
    } finally {
      setLoading(false)
    }
  }, [node.id])

  useEffect(() => {
    loadShares()
  }, [loadShares])

  async function handleCreate() {
    setCreating(true)
    setErr(null)
    try {
      const share = await api.createNodeShare(node.id, ttlHours || undefined)
      setShares(prev => [share, ...prev])
    } catch (e: unknown) {
      setErr(e instanceof Error ? e.message : '创建分享失败')
    } finally {
      setCreating(false)
    }
  }

  async function handleRevoke(id: string) {
    if (!window.confirm('确认撤销此分享链接？')) return
    try {
      await api.revokeNodeShare(id)
      setShares(prev => prev.filter(s => s.id !== id))
    } catch (e: unknown) {
      setErr(e instanceof Error ? e.message : '撤销失败')
    }
  }

  async function handleCopy(url: string, id: string) {
    try {
      await navigator.clipboard.writeText(url)
      setCopied(id)
      setTimeout(() => setCopied(null), 2000)
    } catch {
      setErr('复制失败，请手动复制')
    }
  }

  function formatExpiry(expiresAt: string | null): string {
    if (!expiresAt) return '永不过期'
    const d = new Date(expiresAt)
    return d.toLocaleString('zh-CN', { month: 'short', day: 'numeric', hour: '2-digit', minute: '2-digit' })
  }

  function getShareURL(share: NodeShare): string {
    return share.url ?? ''
  }

  return (
    <div style={{
      position: 'fixed', inset: 0, zIndex: 2000,
      background: 'rgba(0,0,0,0.7)',
      display: 'flex', alignItems: 'center', justifyContent: 'center',
      padding: 24,
    }} onClick={e => { if (e.target === e.currentTarget) onClose() }}>
      <div style={{
        background: '#0d1526', border: '1px solid #1e3050', borderRadius: 10,
        padding: 24, width: '100%', maxWidth: 500, maxHeight: '80vh',
        display: 'flex', flexDirection: 'column', gap: 16,
      }}>
        {/* Header */}
        <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
          <span style={{ fontWeight: 600, fontSize: 14, color: '#c8d8e8' }}>
            分享节点 — {node.id.slice(0, 8)}
          </span>
          <button onClick={onClose} style={{
            background: 'none', border: 'none', color: '#9ec5ee',
            fontSize: 20, cursor: 'pointer', lineHeight: 1,
          }}>✕</button>
        </div>

        {err && (
          <div style={{ color: '#ff6b7a', fontSize: 12, padding: '6px 10px', background: 'rgba(220,53,69,0.1)', borderRadius: 4 }}>
            {err}
          </div>
        )}

        {/* 创建新分享 */}
        <div style={{
          background: 'rgba(30,48,80,0.5)', borderRadius: 8, padding: 14,
          display: 'flex', flexDirection: 'column', gap: 10,
        }}>
          <div style={{ fontSize: 12, color: '#9ec5ee', marginBottom: 2 }}>创建新分享链接</div>
          <div style={{ display: 'flex', gap: 8, alignItems: 'center' }}>
            <label style={{ fontSize: 12, color: '#7a9ec8', whiteSpace: 'nowrap' }}>有效期</label>
            <select
              value={ttlHours}
              onChange={e => setTtlHours(Number(e.target.value))}
              style={{
                background: '#1a2840', border: '1px solid #2a4060', borderRadius: 4,
                color: '#c8d8e8', fontSize: 12, padding: '4px 8px', flex: 1,
              }}
            >
              <option value={0}>永不过期</option>
              <option value={1}>1 小时</option>
              <option value={24}>24 小时</option>
              <option value={168}>7 天</option>
              <option value={720}>30 天</option>
            </select>
            <button
              onClick={handleCreate}
              disabled={creating}
              style={{
                background: '#2a5caa', border: 'none', borderRadius: 6,
                color: '#fff', fontSize: 12, padding: '6px 14px', cursor: 'pointer',
                opacity: creating ? 0.5 : 1,
              }}
            >
              {creating ? '创建中...' : '生成链接'}
            </button>
          </div>
        </div>

        {/* 分享列表 */}
        <div style={{ flex: 1, overflowY: 'auto', display: 'flex', flexDirection: 'column', gap: 8 }}>
          {loading && (
            <div style={{ opacity: 0.5, fontSize: 12, textAlign: 'center', padding: 12 }}>加载中...</div>
          )}
          {!loading && shares.length === 0 && (
            <div style={{ opacity: 0.5, fontSize: 12, textAlign: 'center', padding: 12 }}>暂无有效分享链接</div>
          )}
          {shares.map(share => {
            const url = getShareURL(share)
            return (
              <div key={share.id} style={{
                background: 'rgba(30,48,80,0.4)', borderRadius: 6, padding: 10,
                display: 'flex', flexDirection: 'column', gap: 6,
                border: '1px solid #1e3050',
              }}>
                <div style={{ display: 'flex', alignItems: 'center', gap: 6 }}>
                  <input
                    readOnly
                    value={url}
                    style={{
                      flex: 1, background: '#111d30', border: '1px solid #2a4060',
                      borderRadius: 4, color: '#9ec5ee', fontSize: 11,
                      padding: '4px 8px', overflow: 'hidden', textOverflow: 'ellipsis',
                    }}
                    onClick={e => (e.target as HTMLInputElement).select()}
                  />
                  <button
                    onClick={() => handleCopy(url, share.id)}
                    style={{
                      background: copied === share.id ? '#1a6b3c' : '#1e3050',
                      border: 'none', borderRadius: 4, color: '#9ec5ee',
                      fontSize: 11, padding: '4px 8px', cursor: 'pointer', whiteSpace: 'nowrap',
                    }}
                  >
                    {copied === share.id ? '✓ 已复制' : '复制'}
                  </button>
                </div>
                <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
                  <span style={{ fontSize: 11, color: '#5a7a9e' }}>
                    过期时间：{formatExpiry(share.expires_at)}
                  </span>
                  <button
                    onClick={() => handleRevoke(share.id)}
                    style={{
                      background: 'none', border: 'none', color: '#e06060',
                      fontSize: 11, cursor: 'pointer', padding: '0 4px',
                    }}
                  >
                    撤销
                  </button>
                </div>
              </div>
            )
          })}
        </div>
      </div>
    </div>
  )
}
