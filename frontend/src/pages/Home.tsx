import { useEffect, useMemo, useRef, useState } from 'react'
import { api, type CloudTask, type EdgeItem, type EdgeKind, type Lake, type NodeItem } from '../api/client'
import { LakeWS } from '../api/wsClient'

interface Props { onLogout: () => void }

const EDGE_KINDS: EdgeKind[] = ['relates', 'derives', 'opposes', 'refines', 'groups', 'custom']

export function Home({ onLogout }: Props) {
  const [lakes, setLakes] = useState<Lake[]>([])
  const [active, setActive] = useState<Lake | null>(null)
  const [nodes, setNodes] = useState<NodeItem[]>([])
  const [edges, setEdges] = useState<EdgeItem[]>([])
  const [tasks, setTasks] = useState<CloudTask[]>([])
  const [prompt, setPrompt] = useState('')
  const [n, setN] = useState(5)
  const [newLakeName, setNewLakeName] = useState('')
  const [busy, setBusy] = useState(false)
  const [err, setErr] = useState<string | null>(null)
  const [wsOnline, setWsOnline] = useState(false)
  // 连线状态：null=普通 | source_id=已选起点，等待选终点
  const [linkSrc, setLinkSrc] = useState<string | null>(null)
  const wsRef = useRef<LakeWS | null>(null)

  useEffect(() => { void refresh() }, [])

  // 切换 active 湖时：重建 WS 订阅，并加载节点。
  useEffect(() => {
    if (!active) return
    void loadNodes(active.id)
    void loadEdges(active.id)
    setLinkSrc(null)

    const token = localStorage.getItem('ripple.token') ?? ''
    if (!token) return

    // 关闭旧连接
    wsRef.current?.close()

    const ws = new LakeWS(
      active.id,
      token,
      msg => {
        // node 事件 → 全量刷新节点（MVP 简化，避免增量 merge 复杂度）
        if (msg.type.startsWith('node.')) {
          void loadNodes(active.id)
        }
        if (msg.type.startsWith('edge.')) {
          void loadEdges(active.id)
        }
        // cloud 事件 → 刷新任务列表（如有 task_id）
        if (msg.type.startsWith('cloud.') && msg.payload?.task_id) {
          api.getCloud(msg.payload.task_id)
            .then(t => setTasks(prev => prev.map(x => x.id === t.id ? t : x)))
            .catch(() => { /* ignore */ })
        }
      },
      online => setWsOnline(online),
    )
    ws.connect()
    wsRef.current = ws

    return () => {
      ws.close()
      wsRef.current = null
      setWsOnline(false)
    }
  }, [active])

  async function refresh() {
    try {
      const r = await api.listLakes()
      setLakes(r.lakes)
      if (!active && r.lakes.length > 0) setActive(r.lakes[0])
    } catch (e) { setErr((e as Error).message) }
  }

  async function loadNodes(lakeId: string) {
    try { setNodes((await api.listNodes(lakeId)).nodes) } catch (e) { setErr((e as Error).message) }
  }

  async function loadEdges(lakeId: string) {
    try { setEdges((await api.listEdges(lakeId)).edges) } catch (e) { setErr((e as Error).message) }
  }

  // 进入连线：点第一个节点设为 source；点第二个节点询问 kind 后创建。
  async function handleNodeClickForLink(nodeId: string) {
    if (!linkSrc) {
      setLinkSrc(nodeId)
      return
    }
    if (linkSrc === nodeId) {
      setLinkSrc(null) // 再次点同一个 = 取消
      return
    }
    const kind = window.prompt(
      `选择边的类型（${EDGE_KINDS.join(' / ')}）：`, 'relates',
    )
    if (!kind) { setLinkSrc(null); return }
    if (!EDGE_KINDS.includes(kind as EdgeKind)) {
      setErr(`无效的边类型：${kind}`); setLinkSrc(null); return
    }
    let label: string | undefined
    if (kind === 'custom') {
      label = window.prompt('自定义边的标签：') ?? undefined
      if (!label) { setErr('custom 类型必须填标签'); setLinkSrc(null); return }
    }
    try {
      await api.createEdge(linkSrc, nodeId, kind as EdgeKind, label)
      if (active) await loadEdges(active.id)
    } catch (e) { setErr((e as Error).message) }
    finally { setLinkSrc(null) }
  }

  async function deleteEdge(id: string) {
    if (!confirm('确定删除这条边？')) return
    try {
      await api.deleteEdge(id)
      if (active) await loadEdges(active.id)
    } catch (e) { setErr((e as Error).message) }
  }

  async function createLake() {
    if (!newLakeName.trim()) return
    setBusy(true); setErr(null)
    try {
      const lake = await api.createLake(newLakeName.trim(), '', false)
      setNewLakeName('')
      setLakes([lake, ...lakes])
      setActive(lake)
    } catch (e) { setErr((e as Error).message) }
    finally { setBusy(false) }
  }

  async function generate() {
    if (!active || !prompt.trim()) return
    setBusy(true); setErr(null)
    try {
      const t = await api.generateCloud(active.id, prompt.trim(), n, 'TEXT')
      setTasks([t, ...tasks])
      setPrompt('')
      void poll(t.id)
    } catch (e) { setErr((e as Error).message) }
    finally { setBusy(false) }
  }

  async function poll(taskId: string) {
    for (let i = 0; i < 30; i++) {
      await new Promise(r => setTimeout(r, 1500))
      try {
        const t = await api.getCloud(taskId)
        setTasks(prev => prev.map(x => x.id === taskId ? t : x))
        if (t.status === 'done' || t.status === 'failed') {
          if (active) await loadNodes(active.id)
          return
        }
      } catch { /* ignore */ }
    }
  }

  async function condense(nodeId: string) {
    try {
      await api.condenseNode(nodeId)
      if (active) await loadNodes(active.id)
    } catch (e) { setErr((e as Error).message) }
  }

  async function evaporate(nodeId: string) {
    try {
      await api.evaporateNode(nodeId)
      if (active) await loadNodes(active.id)
    } catch (e) { setErr((e as Error).message) }
  }

  // O(E) 构建节点出入度；避免在 node.map 内每次 O(E) filter。
  const { outDeg, inDeg, nodeContentById } = useMemo(() => {
    const outDeg = new Map<string, number>()
    const inDeg = new Map<string, number>()
    for (const e of edges) {
      outDeg.set(e.src_node_id, (outDeg.get(e.src_node_id) ?? 0) + 1)
      inDeg.set(e.dst_node_id, (inDeg.get(e.dst_node_id) ?? 0) + 1)
    }
    const nodeContentById = new Map<string, string>()
    for (const n of nodes) nodeContentById.set(n.id, n.content)
    return { outDeg, inDeg, nodeContentById }
  }, [edges, nodes])

  return (
    <div style={layout}>
      <aside style={sidebar}>
        <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
          <strong style={{ letterSpacing: 3 }}>
            青萍 · 我的湖
            <span title={wsOnline ? '实时连接已建立' : '实时离线'} style={{
              display: 'inline-block', width: 8, height: 8, borderRadius: '50%',
              marginLeft: 8, background: wsOnline ? '#7fdbb6' : '#777',
              boxShadow: wsOnline ? '0 0 6px #7fdbb6' : 'none',
            }} />
          </strong>
          <button onClick={onLogout} style={ghostBtn}>退出</button>
        </div>
        <div style={{ display: 'flex', gap: 6, marginTop: 16 }}>
          <input
            value={newLakeName} onChange={e => setNewLakeName(e.target.value)}
            placeholder="新湖名…" style={inputSmall}
          />
          <button onClick={createLake} disabled={busy} style={primaryBtnSmall}>+</button>
        </div>
        <ul style={{ listStyle: 'none', padding: 0, margin: '16px 0 0' }}>
          {lakes.map(l => (
            <li key={l.id}
              onClick={() => setActive(l)}
              style={{
                ...lakeItem,
                background: active?.id === l.id ? 'rgba(74,144,226,0.25)' : 'transparent',
              }}
            >
              <div>{l.name}</div>
              <div style={{ fontSize: 10, opacity: 0.5 }}>{l.role}</div>
            </li>
          ))}
        </ul>
      </aside>

      <main style={main}>
        {!active && <div style={{ opacity: 0.5 }}>选择一个湖，或新建一个</div>}
        {active && (
          <>
            <h2 style={{ margin: '0 0 8px', fontWeight: 300 }}>{active.name}</h2>
            <div style={{ opacity: 0.5, marginBottom: 24, fontSize: 12 }}>
              {active.description || '未命名湖区 · ' + active.id.slice(0, 8)}
            </div>

            <section style={card}>
              <strong style={{ letterSpacing: 2, fontSize: 13 }}>造云 · AI 发散</strong>
              <textarea
                value={prompt} onChange={e => setPrompt(e.target.value)}
                placeholder="例如：给一款冥想 App 起 5 个名字" rows={3}
                style={textarea}
              />
              <div style={{ display: 'flex', gap: 12, alignItems: 'center' }}>
                <label style={{ fontSize: 12, opacity: 0.7 }}>候选数</label>
                <input type="number" min={1} max={10} value={n}
                  onChange={e => setN(Number(e.target.value))}
                  style={{ ...inputSmall, width: 60 }} />
                <button onClick={generate} disabled={busy || !prompt.trim()} style={primaryBtn}>
                  {busy ? '...' : '造云'}
                </button>
              </div>
            </section>

            {tasks.length > 0 && (
              <section style={card}>
                <strong style={{ letterSpacing: 2, fontSize: 13 }}>最近的云</strong>
                {tasks.slice(0, 5).map(t => (
                  <div key={t.id} style={taskRow}>
                    <span style={{ ...statusPill, background: statusColor(t.status) }}>
                      {t.status}
                    </span>
                    <span style={{ flex: 1, opacity: 0.85, fontSize: 13 }}>{t.prompt}</span>
                    <span style={{ opacity: 0.5, fontSize: 11 }}>
                      {t.result_node_ids?.length ?? 0}/{t.n}
                    </span>
                  </div>
                ))}
              </section>
            )}

            <section style={card}>
              <strong style={{ letterSpacing: 2, fontSize: 13 }}>湖中节点 ({nodes.length})</strong>
              {linkSrc && (
                <div style={{ fontSize: 12, opacity: 0.8, marginTop: 6, color: '#9ec5ee' }}>
                  连线模式：已选起点 {linkSrc.slice(0, 8)}…，点击另一节点完成。再次点同一节点取消。
                </div>
              )}
              {nodes.length === 0 && <div style={{ opacity: 0.4, fontSize: 12 }}>此处风平浪静</div>}
              <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fill, minmax(220px, 1fr))', gap: 8 }}>
                {nodes.map(n => {
                  const out = outDeg.get(n.id) ?? 0
                  const inc = inDeg.get(n.id) ?? 0
                  const isLinkSrc = linkSrc === n.id
                  return (
                    <div key={n.id} style={{
                      ...nodeCard,
                      opacity: n.state === 'VAPOR' ? 0.4 : 1,
                      boxShadow: isLinkSrc ? '0 0 0 2px #9ec5ee' : undefined,
                    }}>
                      <div style={{ display: 'flex', justifyContent: 'space-between' }}>
                        <span style={{ ...statePill, background: stateColor(n.state) }}>{n.state}</span>
                        <span style={{ fontSize: 10, opacity: 0.6 }}>
                          →{out} ←{inc}
                        </span>
                      </div>
                      <div style={{ marginTop: 8, fontSize: 13, lineHeight: 1.5 }}>{n.content}</div>
                      <div style={{ marginTop: 8, display: 'flex', gap: 6, flexWrap: 'wrap' }}>
                        {n.state === 'MIST' && (
                          <button onClick={() => condense(n.id)} style={miniBtn}>凝露 ↓</button>
                        )}
                        {(n.state === 'DROP' || n.state === 'FROZEN') && (
                          <button onClick={() => evaporate(n.id)} style={miniBtn}>蒸发 ↑</button>
                        )}
                        <button onClick={() => handleNodeClickForLink(n.id)} style={miniBtn}
                          title={isLinkSrc ? '取消连线' : '连线（先选起点再选终点）'}>
                          {isLinkSrc ? '✕' : '🔗'}
                        </button>
                      </div>
                    </div>
                  )
                })}
              </div>
            </section>

            {edges.length > 0 && (
              <section style={card}>
                <strong style={{ letterSpacing: 2, fontSize: 13 }}>边 ({edges.length})</strong>
                <div style={{ display: 'flex', flexDirection: 'column', gap: 4, marginTop: 8 }}>
                  {edges.map(e => {
                    const src = nodeContentById.get(e.src_node_id)
                    const dst = nodeContentById.get(e.dst_node_id)
                    return (
                      <div key={e.id} style={edgeRow}>
                        <span style={{ ...edgeKindPill }}>{e.kind}{e.label ? `: ${e.label}` : ''}</span>
                        <span style={{ flex: 1, fontSize: 12, opacity: 0.85 }}>
                          {(src ?? e.src_node_id.slice(0, 8)).slice(0, 24)}
                          {' → '}
                          {(dst ?? e.dst_node_id.slice(0, 8)).slice(0, 24)}
                        </span>
                        <button onClick={() => deleteEdge(e.id)} style={miniBtn}>删</button>
                      </div>
                    )
                  })}
                </div>
              </section>
            )}
          </>
        )}
        {err && <div style={errBanner}>{err}</div>}
      </main>
    </div>
  )
}

function statusColor(s: string) {
  return ({ queued: '#888', running: '#4a90e2', done: '#52c41a', failed: '#ff4d4f' } as const)[s as 'queued'] ?? '#888'
}
function stateColor(s: string) {
  return ({ MIST: '#9ec5ee', DROP: '#52c41a', FROZEN: '#9bb', VAPOR: '#777', ERASED: '#444', GHOST: '#444' } as const)[s as 'MIST'] ?? '#888'
}

const layout: React.CSSProperties = {
  display: 'flex', height: '100vh', width: '100vw',
  background: '#0a1929', color: '#e0f0ff',
  fontFamily: 'system-ui, -apple-system, sans-serif',
}
const sidebar: React.CSSProperties = {
  width: 260, padding: 20, borderRight: '1px solid rgba(255,255,255,0.1)',
  overflowY: 'auto',
}
const main: React.CSSProperties = {
  flex: 1, padding: 32, overflowY: 'auto',
}
const card: React.CSSProperties = {
  background: 'rgba(255,255,255,0.04)', padding: 16,
  borderRadius: 8, marginTop: 16,
  border: '1px solid rgba(255,255,255,0.08)',
  display: 'flex', flexDirection: 'column', gap: 10,
}
const lakeItem: React.CSSProperties = {
  padding: '8px 12px', borderRadius: 6, marginBottom: 4,
  cursor: 'pointer', fontSize: 14,
}
const inputSmall: React.CSSProperties = {
  padding: '6px 10px', background: 'rgba(255,255,255,0.08)',
  border: '1px solid rgba(255,255,255,0.15)', borderRadius: 4,
  color: '#fff', fontSize: 13, outline: 'none', flex: 1,
}
const textarea: React.CSSProperties = {
  background: 'rgba(255,255,255,0.06)',
  border: '1px solid rgba(255,255,255,0.15)', borderRadius: 4,
  color: '#fff', padding: 8, fontSize: 13,
  fontFamily: 'inherit', resize: 'vertical',
}
const primaryBtn: React.CSSProperties = {
  padding: '8px 20px', background: '#4a90e2',
  border: 'none', borderRadius: 4, color: 'white',
  fontSize: 13, cursor: 'pointer',
}
const primaryBtnSmall: React.CSSProperties = { ...primaryBtn, padding: '6px 12px' }
const ghostBtn: React.CSSProperties = {
  background: 'none', border: '1px solid rgba(255,255,255,0.2)',
  color: 'rgba(255,255,255,0.6)', borderRadius: 4,
  padding: '4px 10px', fontSize: 11, cursor: 'pointer',
}
const taskRow: React.CSSProperties = {
  display: 'flex', alignItems: 'center', gap: 8, padding: '6px 0',
}
const statusPill: React.CSSProperties = {
  fontSize: 10, padding: '2px 8px', borderRadius: 10,
  letterSpacing: 1, color: 'white',
}
const statePill: React.CSSProperties = {
  fontSize: 9, padding: '1px 6px', borderRadius: 6,
  color: '#000', fontWeight: 600, letterSpacing: 1,
}
const nodeCard: React.CSSProperties = {
  background: 'rgba(255,255,255,0.04)', padding: 12,
  borderRadius: 6, border: '1px solid rgba(255,255,255,0.08)',
}
const miniBtn: React.CSSProperties = {
  background: 'rgba(255,255,255,0.08)',
  border: '1px solid rgba(255,255,255,0.15)',
  color: '#cde', padding: '3px 10px', borderRadius: 3,
  fontSize: 11, cursor: 'pointer',
}
const edgeRow: React.CSSProperties = {
  display: 'flex', gap: 8, alignItems: 'center',
  padding: '4px 8px', background: 'rgba(255,255,255,0.03)',
  borderRadius: 4, border: '1px solid rgba(255,255,255,0.06)',
}
const edgeKindPill: React.CSSProperties = {
  fontSize: 10, padding: '2px 8px', borderRadius: 10,
  background: 'rgba(158,197,238,0.18)', color: '#9ec5ee',
  letterSpacing: 1, minWidth: 60, textAlign: 'center',
}
const errBanner: React.CSSProperties = {
  position: 'fixed', bottom: 16, right: 16,
  padding: 12, background: 'rgba(255,80,80,0.2)',
  border: '1px solid rgba(255,80,80,0.4)',
  borderRadius: 4, color: '#ffb0b0', fontSize: 13,
  maxWidth: 400,
}
