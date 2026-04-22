import React, { useEffect, useMemo, useRef, useState } from 'react'
import { api, type CloudTask, type EdgeItem, type EdgeKind, type Lake, type NodeItem, type Space, type PermaNode } from '../api/client'

const LakeGraph = React.lazy(() => import('../components/LakeGraph'))
const APIKeyManager = React.lazy(() => import('../components/APIKeyManager'))
const AuditLogViewer = React.lazy(() => import('../components/AuditLogViewer'))
const LakeMemberManager = React.lazy(() => import('../components/LakeMemberManager'))
const SearchModal = React.lazy(() => import('../components/SearchModal'))
const ImportModal = React.lazy(() => import('../components/ImportModal'))
const OrgPanel = React.lazy(() => import('../components/OrgPanel'))
import { prompt as modalPrompt, confirm as modalConfirm, alert as modalAlert } from '../components/Modal'
import SpaceSwitcher from '../components/SpaceSwitcher'
import SpaceMembersDrawer from '../components/SpaceMembersDrawer'
import AttachmentBar from '../components/AttachmentBar'
import CollabDemo from '../components/CollabDemo'
import OfflineBar from '../components/OfflineBar'
import NotificationBell from '../components/NotificationBell'
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
  const [onlineUsers, setOnlineUsers] = useState<string[]>([])
  // 连线状态：null=普通 | source_id=已选起点，等待选终点
  const [linkSrc, setLinkSrc] = useState<string | null>(null)
  // M3-S1：当前选中的空间。'' = 个人湖（无 space_id）
  const [currentSpaceId, setCurrentSpaceId] = useState<string>('')
  // 成员管理抽屉
  const [membersDrawer, setMembersDrawer] = useState<Space | null>(null)
  const [searchOpen, setSearchOpen] = useState(false)
  const [importOpen, setImportOpen] = useState(false)
  const [orgOpen, setOrgOpen] = useState(false)
  const [meId, setMeId] = useState<string>('')
  const [pendingAction, setPendingAction] = useState<string | null>(null)
  const [exportBusy, setExportBusy] = useState(false)

  // P12-C：拉取当前登录用户 ID（用于组织权限判断）
  useEffect(() => {
    api.me().then(u => setMeId(u.id)).catch(() => { /* 静默 */ })
  }, [])

  // P12-E：PWA shortcut 处理 — ?action=search|import（需等 active 湖加载完毕）
  useEffect(() => {
    const action = new URLSearchParams(window.location.search).get('action')
    if (!action) return
    window.history.replaceState({}, '', window.location.pathname)
    setPendingAction(action)
  }, [])

  useEffect(() => {
    if (!pendingAction || !active) return
    if (pendingAction === 'search') setSearchOpen(true)
    if (pendingAction === 'import') setImportOpen(true)
    setPendingAction(null)
  }, [pendingAction, active])
  // M3-S2：凝结多选（DROP/FROZEN 节点 id 集合）
  const [crystalSel, setCrystalSel] = useState<Set<string>>(new Set())
  // 凝结结果（最近一次）
  const [recentPerma, setRecentPerma] = useState<PermaNode | null>(null)
  // M3-T4：SSE 流式预览
  const [streamText, setStreamText] = useState('')
  const [streaming, setStreaming] = useState(false)
  const streamAbortRef = useRef<(() => void) | null>(null)
  // P9-C：节点视图模式（列表 | 图谱）
  const [viewMode, setViewMode] = useState<'list' | 'graph'>('list')
  // P11：主区 Tab（lakes=主流程 | settings=API Key+审计日志）
  const [mainTab, setMainTab] = useState<'lakes' | 'settings'>('lakes')
  // M3-S3：推荐位（基于历史 LIKE 反馈的协同过滤）
  const [recos, setRecos] = useState<{ target_id: string; score: number }[]>([])
  const wsRef = useRef<LakeWS | null>(null)

  useEffect(() => { void refresh() }, [currentSpaceId])

  // P12-D：Cmd+K / Ctrl+K 打开搜索浮层
  useEffect(() => {
    const handler = (e: KeyboardEvent) => {
      if ((e.metaKey || e.ctrlKey) && e.key === 'k') {
        e.preventDefault()
        if (active) setSearchOpen(o => !o)
      }
    }
    window.addEventListener('keydown', handler)
    return () => window.removeEventListener('keydown', handler)
  }, [active])

  // M3-S3：拉取推荐位（异步，失败静默）
  useEffect(() => {
    api.recommend('perma_node', 6)
      .then(r => setRecos(r.recommendations || []))
      .catch(() => setRecos([]))
  }, [])

  // 切换 active 湖时：重建 WS 订阅，并加载节点。
  useEffect(() => {
    if (!active) return
    void loadNodes(active.id)
    void loadEdges(active.id)
    void loadPresence(active.id)
    setLinkSrc(null)
    setViewMode('list')

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
        if (msg.type.startsWith('presence.')) {
          void loadPresence(active.id)
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
      const r = await api.listLakes(currentSpaceId || undefined)
      setLakes(r.lakes)
      // 切换空间后，若当前 active 不在新列表中，自动选第一个/置空
      if (r.lakes.length === 0) {
        setActive(null)
      } else if (!active || !r.lakes.find(l => l.id === active.id)) {
        setActive(r.lakes[0])
      }
    } catch (e) { setErr((e as Error).message) }
  }

  async function loadNodes(lakeId: string) {
    try { setNodes((await api.listNodes(lakeId)).nodes) } catch (e) { setErr((e as Error).message) }
  }

  async function loadEdges(lakeId: string) {
    try { setEdges((await api.listEdges(lakeId)).edges) } catch (e) { setErr((e as Error).message) }
  }

  async function loadPresence(lakeId: string) {
    try { setOnlineUsers((await api.listPresence(lakeId)).users) } catch { /* 非关键：静默 */ }
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
    const kind = await modalPrompt({
      title: '边类型',
      label: `可选：${EDGE_KINDS.join(' / ')}`,
      initial: 'relates',
      validate: (v) => (!EDGE_KINDS.includes(v.trim() as EdgeKind) ? '无效的边类型' : null),
    })
    if (kind === null) { setLinkSrc(null); return }
    let label: string | undefined
    if (kind.trim() === 'custom') {
      const labelIn = await modalPrompt({
        title: '自定义边的标签',
        validate: (v) => (!v.trim() ? '标签不能为空' : null),
      })
      if (labelIn === null) { setLinkSrc(null); return }
      label = labelIn.trim()
    }
    try {
      await api.createEdge(linkSrc, nodeId, kind as EdgeKind, label)
      if (active) await loadEdges(active.id)
    } catch (e) { setErr((e as Error).message) }
    finally { setLinkSrc(null) }
  }

  async function deleteEdge(id: string) {
    if (!(await modalConfirm('确定删除这条边？', { danger: true }))) return
    try {
      await api.deleteEdge(id)
      if (active) await loadEdges(active.id)
    } catch (e) { setErr((e as Error).message) }
  }

  async function editNodeContent(node: NodeItem) {
    const next = await modalPrompt({
      title: '编辑节点内容',
      label: '支持多行；Ctrl+Enter 提交，Esc 取消',
      initial: node.content,
      multiline: true,
      validate: (v) => (!v.trim() ? '内容不能为空' : null),
    })
    if (next === null || next === node.content) return
    const reason = await modalPrompt({
      title: '变更说明（可选）',
      placeholder: '例如：修正措辞 / 补充例子 …',
      initial: '',
    })
    if (reason === null) return
    try {
      await api.updateNodeContent(node.id, next, reason)
      if (active) await loadNodes(active.id)
    } catch (e) { setErr((e as Error).message) }
  }

  async function showHistory(node: NodeItem) {
    try {
      const { revisions } = await api.listNodeRevisions(node.id, 50)
      if (revisions.length === 0) { await modalAlert('暂无历史'); return }
      const lines = revisions.map(r =>
        `rev ${r.rev_number} | ${new Date(r.created_at).toLocaleString()} | ${r.edit_reason || '(无说明)'}\n  ${r.content.slice(0, 80)}`
      ).join('\n\n')
      const input = await modalPrompt({
        title: `${node.id.slice(0, 8)} 历史`,
        label: `输入 rev 号回滚（取消即放弃）：\n\n${lines}`,
        validate: (v) => {
          const n = parseInt(v.trim(), 10)
          return !Number.isFinite(n) || n <= 0 ? '无效 rev 号' : null
        },
      })
      if (input === null) return
      const target = parseInt(input.trim(), 10)
      if (!(await modalConfirm(`回滚到 rev ${target}？`, { danger: true }))) return
      await api.rollbackNode(node.id, target)
      if (active) await loadNodes(active.id)
    } catch (e) { setErr((e as Error).message) }
  }

  async function copyText(text: string): Promise<boolean> {
    try { await navigator.clipboard.writeText(text); return true } catch { return false }
  }

  async function manageInvites() {
    if (!active) return
    try {
      const { invites } = await api.listInvites(active.id, false)
      const aliveLines = invites.length === 0
        ? '(无活跃邀请)'
        : invites.map(i => `${i.id.slice(0, 8)} | ${i.role} | ${i.used_count}/${i.max_uses} | 到期 ${new Date(i.expires_at).toLocaleString()}\n  token: ${i.token}`).join('\n\n')
      const action = await modalPrompt({
        title: `湖 ${active.name} 邀请`,
        label: `${aliveLines}\n\n输入：\nC = 创建新邀请\nR:<id前缀> = 撤销`,
        placeholder: '例如：C 或 R:abcd1234',
      })
      if (action === null || !action.trim()) return
      const cmd = action.trim()
      if (cmd.toUpperCase() === 'C') {
        const roleIn = await modalPrompt({
          title: '邀请角色',
          label: 'NAVIGATOR / PASSENGER / OBSERVER',
          initial: 'PASSENGER',
          validate: (v) => (!['NAVIGATOR', 'PASSENGER', 'OBSERVER'].includes(v.trim().toUpperCase()) ? '无效角色' : null),
        })
        if (roleIn === null) return
        const role = roleIn.trim().toUpperCase()
        const maxUsesIn = await modalPrompt({
          title: '最大使用次数',
          label: '1 - 10000',
          initial: '5',
          validate: (v) => {
            const n = parseInt(v.trim(), 10)
            return !Number.isFinite(n) || n < 1 || n > 10000 ? '无效 max_uses' : null
          },
        })
        if (maxUsesIn === null) return
        const maxUses = parseInt(maxUsesIn.trim(), 10)
        const ttlHoursIn = await modalPrompt({
          title: '有效时长（小时）',
          label: '1 - 8760',
          initial: '168',
          validate: (v) => {
            const n = parseFloat(v.trim())
            return !Number.isFinite(n) || n < 1 || n > 8760 ? '无效 TTL' : null
          },
        })
        if (ttlHoursIn === null) return
        const ttlHours = parseFloat(ttlHoursIn.trim())
        const inv = await api.createInvite(active.id, role as 'NAVIGATOR' | 'PASSENGER' | 'OBSERVER', maxUses, Math.round(ttlHours * 3600))
        const link = `${window.location.origin}/?invite=${encodeURIComponent(inv.token)}`
        const copied = await copyText(link)
        await modalAlert(`邀请已创建\n\nToken: ${inv.token}\n链接: ${link}\n\n${copied ? '（已复制链接到剪贴板）' : '（剪贴板复制失败，请手动复制）'}`)
      } else if (cmd.toUpperCase().startsWith('R:')) {
        const prefix = cmd.slice(2).trim()
        const target = invites.find(i => i.id.startsWith(prefix))
        if (!target) { setErr('未找到匹配邀请'); return }
        if (!(await modalConfirm(`撤销 ${target.id.slice(0, 8)}？`, { danger: true }))) return
        await api.revokeInvite(target.id)
        await modalAlert('已撤销')
      }
    } catch (e) { setErr((e as Error).message) }
  }

  // URL 中带 ?invite=... 时自动尝试接受。
  useEffect(() => {
    const token = new URLSearchParams(window.location.search).get('invite')
    if (!token) return
    ;(async () => {
      try {
        const prev = await api.previewInvite(token)
        if (!prev.alive) { setErr(`邀请已失效（${prev.used_count}/${prev.max_uses}）`); return }
        if (!(await modalConfirm(`加入湖 "${prev.lake_name}" 作为 ${prev.role}？`))) return
        const r = await api.acceptInvite(token)
        window.history.replaceState({}, '', window.location.pathname)
        await refresh()
        // refresh 已把 lakes 刷新并可能自动选中首个；这里不手动设置 active，避免 stale closure。
        void r
      } catch (e) { setErr((e as Error).message) }
    })()
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])

  async function createLake() {
    if (!newLakeName.trim()) return
    setBusy(true); setErr(null)
    try {
      const lake = await api.createLake(newLakeName.trim(), '', false, currentSpaceId || undefined)
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

  // M3-T4：SSE 流式预览，把 AI 回复实时增量渲染到面板。
  function startWeaveStream() {
    if (!active || !prompt.trim() || streaming) return
    streamAbortRef.current?.()
    setStreamText('')
    setStreaming(true)
    setErr(null)
    const stop = api.streamWeave(active.id, prompt.trim(), (ev, data) => {
      if (ev === 'delta') {
        setStreamText(prev => prev + (data?.text ?? ''))
      } else if (ev === 'done') {
        setStreaming(false)
      } else if (ev === 'error') {
        setErr(`流式错误：${data?.message ?? 'unknown'}`)
        setStreaming(false)
      }
    })
    streamAbortRef.current = stop
  }

  function stopWeaveStream() {
    streamAbortRef.current?.()
    streamAbortRef.current = null
    setStreaming(false)
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

  // M3-S2：切换节点凝结选中
  function toggleCrystalSel(nodeId: string) {
    setCrystalSel(prev => {
      const next = new Set(prev)
      if (next.has(nodeId)) next.delete(nodeId)
      else next.add(nodeId)
      return next
    })
  }

  // 执行凝结
  async function doCrystallize() {
    if (!active) return
    const ids = Array.from(crystalSel)
    if (ids.length < 2) { void modalAlert('至少选择 2 个节点'); return }
    if (ids.length > 20) { void modalAlert('最多选择 20 个节点'); return }
    const hint = await modalPrompt({ title: '凝结', label: '标题提示（可留空，由 AI 生成）：', initial: '' })
    if (hint === null) return
    try {
      const p = await api.crystallize(active.id, ids, hint || '')
      setRecentPerma(p)
      setCrystalSel(new Set())
    } catch (e) { setErr((e as Error).message) }
  }

  // P13-D：导出湖内容
  async function exportLakeUI(format: 'json' | 'markdown') {
    if (!active) return
    setExportBusy(true)
    try {
      await api.exportLake(active.id, format)
    } catch (e) { setErr((e as Error).message) }
    finally { setExportBusy(false) }
  }

  // M3 T7：移动湖到其他空间
  async function moveLakeUI(lake: Lake) {
    try {
      const r = await api.listSpaces()
      const opts = ['（个人湖 / 移除归属）', ...(r.spaces ?? []).map(s => `${s.name} [${s.id.slice(0, 8)}]`)]
      const ids = ['', ...(r.spaces ?? []).map(s => s.id)]
      const idx = await modalPrompt({
        title: '移动湖',
        label: `当前：${lake.space_id ? lake.space_id.slice(0, 8) : '个人湖'}\n输入序号选择目标：\n${opts.map((o, i) => `  ${i}. ${o}`).join('\n')}`,
        initial: '0',
        validate: v => {
          const n = parseInt(v.trim(), 10)
          return Number.isInteger(n) && n >= 0 && n < ids.length ? null : '请输入合法序号'
        },
      })
      if (idx === null) return
      const target = ids[parseInt(idx, 10)]
      if (target === (lake.space_id || '')) return
      await api.moveLake(lake.id, target)
      await refresh()
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
      <OfflineBar />
      <aside style={sidebar}>
        <SpaceSwitcher
          currentSpaceId={currentSpaceId}
          onChange={setCurrentSpaceId}
          onManageMembers={setMembersDrawer}
        />
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
          {active && (
            <button
              onClick={() => setSearchOpen(true)}
              title="搜索节点 (Cmd+K)"
              style={ghostBtn}
            >🔍</button>
          )}
          {active && (
            <button
              onClick={() => setImportOpen(true)}
              title="批量导入节点"
              style={ghostBtn}
            >📥</button>
          )}
          <button
            onClick={() => setMainTab(t => t === 'settings' ? 'lakes' : 'settings')}
            style={{ ...ghostBtn, color: mainTab === 'settings' ? '#89b4fa' : undefined }}
          >⚙</button>
          <button
            onClick={() => setOrgOpen(o => !o)}
            title="组织管理"
            style={{ ...ghostBtn, color: orgOpen ? '#89b4fa' : undefined }}
          >🏢</button>
          <NotificationBell />
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
              <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
                <div>{l.name}</div>
                {active?.id === l.id && l.role === 'OWNER' && (
                  <div style={{ display: 'flex', gap: 4 }}>
                    <button onClick={e => { e.stopPropagation(); void moveLakeUI(l) }}
                      style={{ ...miniBtn, padding: '2px 6px', fontSize: 10 }}>移</button>
                    <button onClick={e => { e.stopPropagation(); void manageInvites() }}
                      style={{ ...miniBtn, padding: '2px 6px', fontSize: 10 }}>邀请</button>
                  </div>
                )}
              </div>
              <div style={{ fontSize: 10, opacity: 0.5 }}>{l.role}</div>
            </li>
          ))}
        </ul>
      </aside>

      <main style={main}>
        {mainTab === 'settings' ? (
          <React.Suspense fallback={<div style={{ padding: 16, color: '#6c7086' }}>加载中…</div>}>
            <div style={{ display: 'flex', gap: 24, flexWrap: 'wrap' }}>
              <APIKeyManager />
              <AuditLogViewer />
            </div>
          </React.Suspense>
        ) : (
          <>
            {!active && <div style={{ opacity: 0.5 }}>选择一个湖，或新建一个</div>}
        {active && (
          <>
            <h2 style={{ margin: '0 0 8px', fontWeight: 300 }}>{active.name}</h2>
            <div style={{ opacity: 0.5, marginBottom: 12, fontSize: 12 }}>
              {active.description || '未命名湖区 · ' + active.id.slice(0, 8)}
            </div>
            <div style={{ display: 'flex', gap: 6, marginBottom: 16, flexWrap: 'wrap', alignItems: 'center' }}>
              <span style={{ fontSize: 11, opacity: 0.5 }}>导出：</span>
              <button onClick={() => void exportLakeUI('json')} disabled={exportBusy} style={miniBtn}>
                {exportBusy ? '…' : 'JSON'}
              </button>
              <button onClick={() => void exportLakeUI('markdown')} disabled={exportBusy} style={miniBtn}>
                {exportBusy ? '…' : 'Markdown'}
              </button>
            </div>
            {onlineUsers.length > 0 && (
              <div style={{ display: 'flex', alignItems: 'center', gap: 6, marginBottom: 16, fontSize: 11, opacity: 0.8 }}>
                <span>在线 {onlineUsers.length}：</span>
                {onlineUsers.slice(0, 8).map(uid => (
                  <span key={uid} title={uid} style={presenceDot}>{uid.slice(0, 2).toUpperCase()}</span>
                ))}
                {onlineUsers.length > 8 && <span>+{onlineUsers.length - 8}</span>}
              </div>
            )}

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
                {!streaming ? (
                  <button onClick={startWeaveStream} disabled={!prompt.trim()} style={miniBtn}
                    title="SSE 流式预览（不落盘）">
                    ✨ 流式预览
                  </button>
                ) : (
                  <button onClick={stopWeaveStream} style={{ ...miniBtn, color: '#d24343' }}>
                    停止
                  </button>
                )}
              </div>
              {(streaming || streamText) && (
                <div style={{
                  marginTop: 10, padding: 10, borderRadius: 6,
                  background: '#0e1218', border: '1px solid #1d2433',
                  fontSize: 13, lineHeight: 1.6, whiteSpace: 'pre-wrap',
                  maxHeight: 220, overflow: 'auto',
                }}>
                  {streamText}
                  {streaming && <span style={{ opacity: 0.5 }}>▍</span>}
                  {!streaming && streamText && (
                    <div style={{ marginTop: 8, display: 'flex', gap: 8 }}>
                      <button style={miniBtn} onClick={() => { void navigator.clipboard.writeText(streamText) }}>复制</button>
                      <button style={miniBtn} onClick={() => setStreamText('')}>清空</button>
                    </div>
                  )}
                </div>
              )}
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
              <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
                <div style={{ display: 'flex', alignItems: 'center', gap: 12 }}>
                  <strong style={{ letterSpacing: 2, fontSize: 13 }}>湖中节点 ({nodes.length})</strong>
                  <div style={{ display: 'flex', gap: 4 }}>
                    {(['list', 'graph'] as const).map(mode => (
                      <button
                        key={mode}
                        onClick={() => setViewMode(mode)}
                        style={{
                          ...miniBtn,
                          background: viewMode === mode ? 'rgba(74,144,226,0.35)' : undefined,
                          color: viewMode === mode ? '#9ec5ee' : undefined,
                          padding: '2px 10px',
                        }}
                      >
                        {mode === 'list' ? '列表' : '图谱'}
                      </button>
                    ))}
                  </div>
                </div>
                {crystalSel.size > 0 && (
                  <div style={{ display: 'flex', gap: 6, alignItems: 'center' }}>
                    <span style={{ fontSize: 11, color: '#9ec5ee' }}>已选 {crystalSel.size}</span>
                    <button onClick={doCrystallize} style={{ ...miniBtn, background: '#4a8eff', color: '#fff' }}>
                      ❄ 凝结所选
                    </button>
                    <button onClick={() => setCrystalSel(new Set())} style={miniBtn}>清空</button>
                  </div>
                )}
              </div>

              {viewMode === 'graph' ? (
                <div style={{ marginTop: 12 }}>
                  <React.Suspense fallback={<div style={{ height: 480, display: 'flex', alignItems: 'center', justifyContent: 'center', color: '#4a6a8e', fontSize: 13 }}>加载图谱中…</div>}>
                    <LakeGraph nodes={nodes} edges={edges} />
                  </React.Suspense>
                </div>
              ) : (
                <>
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
                      const canCrystal = n.state === 'DROP' || n.state === 'FROZEN'
                      const isSelected = crystalSel.has(n.id)
                      return (
                        <div key={n.id} style={{
                          ...nodeCard,
                          opacity: n.state === 'VAPOR' ? 0.4 : 1,
                          boxShadow: isLinkSrc
                            ? '0 0 0 2px #9ec5ee'
                            : isSelected ? '0 0 0 2px #4a8eff' : undefined,
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
                        <button onClick={() => editNodeContent(n)} style={miniBtn} title="编辑内容">✎</button>
                        <button onClick={() => showHistory(n)} style={miniBtn} title="历史版本">⟲</button>
                        {canCrystal && (
                          <button
                            onClick={() => toggleCrystalSel(n.id)}
                            style={{ ...miniBtn, background: isSelected ? '#4a8eff' : undefined, color: isSelected ? '#fff' : undefined }}
                            title="选入凝结集合"
                          >❄</button>
                        )}
                      </div>
                    </div>
                  )
                    })}
                  </div>
                </>
              )}
            </section>

            {recentPerma && (
              <section style={{ ...card, borderLeft: '3px solid #4a8eff' }}>
                <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
                  <strong style={{ letterSpacing: 2, fontSize: 13, color: '#9ec5ee' }}>❄ 凝结结果</strong>
                  <button onClick={() => setRecentPerma(null)} style={miniBtn}>关</button>
                </div>
                <div style={{ marginTop: 8, fontSize: 14, fontWeight: 600 }}>{recentPerma.title}</div>
                <div style={{ marginTop: 6, fontSize: 13, lineHeight: 1.6, opacity: 0.9 }}>{recentPerma.summary}</div>
                <div style={{ marginTop: 8, fontSize: 11, opacity: 0.6 }}>
                  来源 {recentPerma.source_node_ids.length} 个节点
                  {recentPerma.llm_provider && ` · ${recentPerma.llm_provider}`}
                  {recentPerma.llm_cost_tokens ? ` · ${recentPerma.llm_cost_tokens} tokens` : ''}
                </div>
              </section>
            )}

            <section style={card}>
              <div style={{ marginBottom: 6, fontSize: 12, opacity: 0.7 }}>📎 附件（M4-B 本地 FS）</div>
              <AttachmentBar />
            </section>

            {active && (
              <section style={card}>
                <CollabDemo lakeId={active.id} token={localStorage.getItem('ripple.token') ?? ''} />
              </section>
            )}

            {active?.role === 'OWNER' && (
              <section style={card}>
                <React.Suspense fallback={<div style={{ color: '#6c7086', fontSize: 12 }}>Loading members...</div>}>
                  <LakeMemberManager
                    lakeId={active.id}
                    currentUserId={active.owner_id}
                    currentRole="OWNER"
                  />
                </React.Suspense>
              </section>
            )}

            {recos.length > 0 && (
              <section style={card}>
                <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
                  <strong style={{ letterSpacing: 2, fontSize: 13, color: '#9ec5ee' }}>✨ 你可能感兴趣的凝结</strong>
                  <span style={{ fontSize: 11, opacity: 0.5 }}>基于历史 LIKE 反馈</span>
                </div>
                <div style={{ marginTop: 8, display: 'flex', flexDirection: 'column', gap: 6 }}>
                  {recos.map(r => (
                    <div key={r.target_id} style={{
                      display: 'flex', alignItems: 'center', gap: 8,
                      padding: '6px 8px', borderRadius: 4, background: '#0e1218',
                      fontSize: 12,
                    }}>
                      <span style={{ flex: 1, fontFamily: 'monospace', opacity: 0.85 }}>
                        {r.target_id.slice(0, 8)}…
                      </span>
                      <span style={{ opacity: 0.6 }}>score {r.score.toFixed(2)}</span>
                      <button style={miniBtn}
                        onClick={() => {
                          void api.sendFeedback('perma_node', r.target_id, 'LIKE')
                            .then(() => setRecos(prev => prev.filter(x => x.target_id !== r.target_id)))
                            .catch(e => modalAlert(`反馈失败：${(e as Error).message}`))
                        }}>👍</button>
                      <button style={miniBtn}
                        onClick={() => {
                          void api.sendFeedback('perma_node', r.target_id, 'DISMISS')
                            .then(() => setRecos(prev => prev.filter(x => x.target_id !== r.target_id)))
                            .catch(() => { /* ignore */ })
                        }}>✕</button>
                    </div>
                  ))}
                </div>
              </section>
            )}

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
          </>
        )}
      </main>
      {membersDrawer && (
        <SpaceMembersDrawer space={membersDrawer} onClose={() => setMembersDrawer(null)} />
      )}
      {searchOpen && active && (
        <React.Suspense fallback={null}>
          <SearchModal
            lakeId={active.id}
            lakeName={active.name}
            onClose={() => setSearchOpen(false)}
          />
        </React.Suspense>
      )}
      {importOpen && active && (
        <React.Suspense fallback={null}>
          <ImportModal
            lakeId={active.id}
            lakeName={active.name}
            onClose={() => setImportOpen(false)}
            onImported={() => api.listNodes(active.id).then(r => setNodes(r.nodes))}
          />
        </React.Suspense>
      )}
      {orgOpen && (
        <div style={{
          position: 'fixed', top: 60, right: 20, zIndex: 300,
        }}>
          <React.Suspense fallback={null}>
            <OrgPanel currentUserId={meId} onClose={() => setOrgOpen(false)} />
          </React.Suspense>
        </div>
      )}
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
const presenceDot: React.CSSProperties = {
  display: 'inline-flex', alignItems: 'center', justifyContent: 'center',
  width: 22, height: 22, borderRadius: '50%',
  background: 'rgba(127,219,182,0.22)',
  border: '1px solid rgba(127,219,182,0.5)',
  color: '#7fdbb6', fontSize: 9, letterSpacing: 0,
}
const errBanner: React.CSSProperties = {
  position: 'fixed', bottom: 16, right: 16,
  padding: 12, background: 'rgba(255,80,80,0.2)',
  border: '1px solid rgba(255,80,80,0.4)',
  borderRadius: 4, color: '#ffb0b0', fontSize: 13,
  maxWidth: 400,
}
