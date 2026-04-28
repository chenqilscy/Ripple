import React, { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { api, type CloudTask, type EdgeItem, type EdgeKind, type Lake, type NodeItem, type Space, type PermaNode } from '../api/client'
import type { NodeRevision, NodeSearchResult, NodeTemplate } from '../api/types'
import { NodeDiffViewer } from '../components/NodeDiffViewer'
import NodeVersionHistory from '../components/NodeVersionHistory'

const LakeGraph = React.lazy(() => import('../components/LakeGraph'))
const AdminOverviewPanel = React.lazy(() => import('../components/AdminOverviewPanel'))
const APIKeyManager = React.lazy(() => import('../components/APIKeyManager'))
const AuditLogViewer = React.lazy(() => import('../components/AuditLogViewer'))
const GraylistManager = React.lazy(() => import('../components/GraylistManager'))
const PlatformAdminManager = React.lazy(() => import('../components/PlatformAdminManager'))
const LakeMemberManager = React.lazy(() => import('../components/LakeMemberManager'))
const SearchModal = React.lazy(() => import('../components/SearchModal'))
const ImportModal = React.lazy(() => import('../components/ImportModal'))
const OrgPanel = React.lazy(() => import('../components/OrgPanel'))
const NodeShareButton = React.lazy(() => import('../components/NodeShareButton'))
const SnapshotPanel = React.lazy(() => import('../components/SnapshotPanel'))
const NodeExplorer = React.lazy(() => import('../components/NodeExplorer'))
import { prompt as modalPrompt, confirm as modalConfirm, alert as modalAlert } from '../components/Modal'
import SpaceSwitcher from '../components/SpaceSwitcher'
import SpaceMembersDrawer from '../components/SpaceMembersDrawer'
import AttachmentBar from '../components/AttachmentBar'
import CollabDemo from '../components/CollabDemo'
import OfflineBar from '../components/OfflineBar'
import NotificationBell from '../components/NotificationBell'

type SettingsTabKey = 'overview' | 'rbac' | 'apiKeys' | 'graylist' | 'audit'

const settingsTabs: { key: SettingsTabKey; label: string }[] = [
  { key: 'overview', label: '总览' },
  { key: 'rbac', label: '平台管理员' },
  { key: 'apiKeys', label: 'API Key' },
  { key: 'graylist', label: '灰度名单' },
  { key: 'audit', label: '审计日志' },
]
import { LakeWS } from '../api/wsClient'

interface Props { onLogout: () => void }

const EDGE_KINDS: EdgeKind[] = ['relates', 'derives', 'opposes', 'refines', 'groups', 'custom']
const LAKE_READY_INITIAL_DELAY_MS = 1000
const LAKE_READY_ATTEMPTS = 30
const LAKE_READY_DELAY_MS = 200

function delay(ms: number) {
  return new Promise<void>(resolve => setTimeout(resolve, ms))
}

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
  // P19-C：meIdRef 与 state 同步，供 WS 闭包读取最新值（WS useEffect 依赖 [active]，不依赖 meId）
  const meIdRef = useRef<string>('')
  const [pendingAction, setPendingAction] = useState<string | null>(null)
  const [exportBusy, setExportBusy] = useState(false)
  const [importBusy, setImportBusy] = useState(false)
  const [importResult, setImportResult] = useState<{ imported: number; skipped: number } | null>(null)
  const importInputRef = useRef<HTMLInputElement>(null)
  // P14-C：批量操作
  const [batchSel, setBatchSel] = useState<Set<string>>(new Set())
  const [batchBusy, setBatchBusy] = useState(false)
  // P15-B：版本 diff 视图
  const [diffModal, setDiffModal] = useState<{ nodeId: string; revisions: NodeRevision[] } | null>(null)
  // P17-A：版本历史时间线
  const [historyModal, setHistoryModal] = useState<{ node: NodeItem; revisions: NodeRevision[] } | null>(null)
  // P17-B：图谱节点搜索高亮
  const [nodeSearch, setNodeSearch] = useState('')
  // P16-B：AI 摘要
  const [aiSummary, setAiSummary] = useState<Record<string, string>>({})
  const [aiSummaryBusy, setAiSummaryBusy] = useState<Set<string>>(new Set())
  // P13-C：标签过滤
  const [tagFilter, setTagFilter] = useState<string>('')
  const [tagFilteredIds, setTagFilteredIds] = useState<Set<string> | null>(null)
  const [tagLoading, setTagLoading] = useState(false)
  const tagAbortRef = useRef<AbortController | null>(null)
  const [lakeTags, setLakeTags] = useState<string[]>([])
  const [nodeTags, setNodeTags] = useState<Record<string, string[]>>({})
  // P18-A：节点关联推荐
  const [relatedPanel, setRelatedPanel] = useState<{ nodeId: string; results: NodeSearchResult[] } | null>(null)
  const [relatedLoading, setRelatedLoading] = useState<string | null>(null)
  // P18-C：节点模板库
  const [templateModalOpen, setTemplateModalOpen] = useState(false)
  const [templates, setTemplates] = useState<NodeTemplate[]>([])
  const [templatesBusy, setTemplatesBusy] = useState(false)
  const [tplCreateOpen, setTplCreateOpen] = useState(false)
  const [tplForm, setTplForm] = useState({ name: '', content: '', description: '', tags: '' })
  const [tplCreateBusy, setTplCreateBusy] = useState(false)
  const [tplPreviewId, setTplPreviewId] = useState<string | null>(null)
  // P18-D：图谱快照
  const [snapshotPanelOpen, setSnapshotPanelOpen] = useState(false)
  const [graphLayout, setGraphLayout] = useState<Record<string, { x: number; y: number }> | undefined>(undefined)
  // P18-B：节点外链分享
  const [shareNode, setShareNode] = useState<NodeItem | null>(null)
  // P19-A：AI 图谱探索
  const [explorerOpen, setExplorerOpen] = useState(false)
  const [exploredNodeIds, setExploredNodeIds] = useState<Set<string>>(new Set())
  // P19-C：协作光标
  const [remoteCursors, setRemoteCursors] = useState<Map<string, { x: number; y: number }>>(new Map())

  // P12-C：拉取当前登录用户 ID（用于组织权限判断）
  useEffect(() => {
    api.me().then(u => { meIdRef.current = u.id; setMeId(u.id) }).catch(() => { /* 静默 */ })
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
  const [settingsTab, setSettingsTab] = useState<SettingsTabKey>('overview')
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
    setTagFilter('')
    setTagFilteredIds(null)
    tagAbortRef.current?.abort()
    setImportResult(null)
    setBatchSel(new Set())
    setGraphLayout(undefined)
    setRemoteCursors(new Map())
    // P13-C：加载湖标签列表
    api.getLakeTags(active.id).then(r => setLakeTags(r.tags)).catch(() => setLakeTags([]))

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
          // P19-C：用户离开时清理其协作光标
          if (msg.type === 'presence.left' && msg.payload?.user_id) {
            setRemoteCursors(prev => {
              const next = new Map(prev)
              next.delete(msg.payload.user_id as string)
              return next
            })
          }
        }
        // cloud 事件 → 刷新任务列表（如有 task_id）
        if (msg.type.startsWith('cloud.') && msg.payload?.task_id) {
          api.getCloud(msg.payload.task_id)
            .then(t => setTasks(prev => prev.map(x => x.id === t.id ? t : x)))
            .catch(() => { /* ignore */ })
        }
        // P14-A：通知实时推送 → 转发给 NotificationBell
        if (msg.type === 'notification.new') {
          window.dispatchEvent(new CustomEvent('ripple:notification', { detail: msg.payload }))
        }
        // P19-C：协作光标位置更新
        if (msg.type === 'cursor.move' && msg.payload?.user_id) {
          const { user_id, x, y } = msg.payload as { user_id: string; x: number; y: number }
          // 过滤自己（后端广播包含发送者自身；用 ref 读最新 meId，避免闭包捕获旧值导致渲染自己光标）
          if (typeof x === 'number' && typeof y === 'number' && user_id !== meIdRef.current) {
            setRemoteCursors(prev => {
              const next = new Map(prev)
              next.set(user_id, { x, y })
              return next
            })
          }
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

  async function waitLakeReady(lake: Lake): Promise<Lake> {
    let lastErr: unknown = null
    await delay(LAKE_READY_INITIAL_DELAY_MS)
    for (let i = 0; i < LAKE_READY_ATTEMPTS; i++) {
      try {
        return await api.getLake(lake.id)
      } catch (e) {
        lastErr = e
        await delay(LAKE_READY_DELAY_MS)
      }
    }
    throw lastErr instanceof Error ? lastErr : new Error('lake projection not ready')
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
      setHistoryModal({ node, revisions })
    } catch (e) { setErr((e as Error).message) }
  }

  // P17-D：节点导出下载
  function exportNode(node: NodeItem, format: 'md' | 'json') {
    const content = format === 'md'
      ? `# ${node.id}\n\n**状态**: ${node.state}  \n**创建**: ${new Date(node.created_at).toLocaleString('zh-CN')}\n\n${node.content}`
      : JSON.stringify(node, null, 2)
    const blob = new Blob([content], { type: format === 'md' ? 'text/markdown' : 'application/json' })
    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url
    a.download = `node-${node.id.slice(0, 8)}.${format}`
    a.click()
    // Revoke after a tick to ensure download triggers before cleanup
    setTimeout(() => URL.revokeObjectURL(url), 100)
  }

  async function showDiff(node: NodeItem) {
    try {
      const { revisions } = await api.listNodeRevisions(node.id, 50)
      if (revisions.length < 2) { await modalAlert('至少需要 2 个版本才能对比'); return }
      setDiffModal({ nodeId: node.id, revisions })
    } catch (e) { setErr((e as Error).message) }
  }

  // P16-B：AI 节点摘要
  async function requestAiSummary(node: NodeItem) {
    if (aiSummaryBusy.has(node.id)) return
    setAiSummaryBusy(prev => new Set([...prev, node.id]))
    try {
      const r = await api.aiSummaryNode(node.id)
      setAiSummary(prev => ({ ...prev, [node.id]: r.summary }))
    } catch (e) {
      setErr((e as Error).message)
    } finally {
      setAiSummaryBusy(prev => { const next = new Set(prev); next.delete(node.id); return next })
    }
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
      const created = await api.createLake(newLakeName.trim(), '', false, currentSpaceId || undefined)
      const lake = await waitLakeReady(created)
      setNewLakeName('')
      setLakes(prev => [lake, ...prev.filter(x => x.id !== lake.id)])
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

  // P18-A：加载关联推荐
  async function loadRelated(nodeId: string) {
    setRelatedLoading(nodeId)
    try {
      const r = await api.getRelatedNodes(nodeId, 5)
      setRelatedPanel({ nodeId, results: r.related })
    } catch (e) { setErr((e as Error).message) }
    finally { setRelatedLoading(null) }
  }

  // P18-C：加载模板列表
  async function loadTemplates() {
    setTemplatesBusy(true)
    try {
      const r = await api.listTemplates()
      setTemplates(r.templates)
    } catch (e) { setErr((e as Error).message) }
    finally { setTemplatesBusy(false) }
  }

  // P18-C：从模板创建节点
  async function createFromTemplate(templateId: string) {
    if (!active) return
    try {
      await api.createNodeFromTemplate(active.id, templateId)
      setTemplateModalOpen(false)
      void loadNodes(active.id)
    } catch (e) { setErr((e as Error).message) }
  }

  // P18-D：保存快照（快捷按钮，直接用当前节点位置）
  async function saveSnapshot() {
    if (!active) return
    const name = await modalPrompt({
      title: '保存图谱快照',
      label: '快照名称（便于识别）',
      validate: v => (!v.trim() ? '名称不能为空' : null),
    })
    if (name === null) return
    try {
      const layout: Record<string, { x: number; y: number }> = {}
      for (const n of nodes) layout[n.id] = { x: n.position.x, y: n.position.y }
      await api.createSnapshot(active.id, name.trim(), layout)
    } catch (e) { setErr((e as Error).message) }
  }

  // P13-E：导入
  async function importLakeUI(file: File) {
    if (!active) return
    setImportBusy(true)
    setImportResult(null)
    try {
      const r = await api.importLake(active.id, file)
      setImportResult(r)
      void loadNodes(active.id)
    } catch (e) { setErr((e as Error).message) }
    finally { setImportBusy(false) }
  }

  // P14-C：批量操作
  async function batchOperate(action: 'evaporate' | 'condense' | 'erase') {
    if (!active || batchSel.size === 0) return
    setBatchBusy(true)
    try {
      const ids = Array.from(batchSel)
      await api.batchOperateNodes(active.id, action, ids)
      setBatchSel(new Set())
      void loadNodes(active.id)
    } catch (e) { setErr((e as Error).message) }
    finally { setBatchBusy(false) }
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

  // P13-C：点击标签时服务端查询（带 AbortController 防竞态）
  const applyTagFilter = useCallback((tag: string) => {
    setTagFilter(tag)
    if (!tag || !active) {
      setTagFilteredIds(null)
      return
    }
    tagAbortRef.current?.abort()
    const ctrl = new AbortController()
    tagAbortRef.current = ctrl
    setTagLoading(true)
    api.listNodesByTag(active.id, tag)
      .then(r => {
        if (ctrl.signal.aborted) return
        setTagFilteredIds(new Set(r.node_ids))
      })
      .catch(() => { if (!ctrl.signal.aborted) setTagFilteredIds(new Set()) })
      .finally(() => { if (!ctrl.signal.aborted) setTagLoading(false) })
  }, [active])

  // P13-C：按标签过滤节点
  const filteredNodes = useMemo(() => {
    if (!tagFilter || tagFilteredIds === null) return nodes
    return nodes.filter(n => tagFilteredIds.has(n.id))
  }, [nodes, tagFilter, tagFilteredIds])

  // P13-C：懒加载节点标签（节点列表变化时批量获取）
  useEffect(() => {
    if (!active || nodes.length === 0) return
    const unloaded = nodes.filter(n => nodeTags[n.id] === undefined).map(n => n.id)
    if (unloaded.length === 0) return
    Promise.all(unloaded.map(id => api.getNodeTags(id).then(r => ({ id, tags: r.tags })).catch(() => ({ id, tags: [] as string[] }))))
      .then(results => {
        setNodeTags(prev => {
          const next = { ...prev }
          for (const { id, tags } of results) next[id] = tags
          return next
        })
      })
  }, [nodes, active]) // eslint-disable-line react-hooks/exhaustive-deps

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
            <div>
              <div style={{ display: 'flex', gap: 8, flexWrap: 'wrap', marginBottom: 16 }}>
                {settingsTabs.map(tab => (
                  <button
                    key={tab.key}
                    onClick={() => setSettingsTab(tab.key)}
                    style={{ ...ghostBtn, color: settingsTab === tab.key ? '#89b4fa' : undefined, borderColor: settingsTab === tab.key ? '#89b4fa' : '#313244' }}
                  >
                    {tab.label}
                  </button>
                ))}
              </div>
              {settingsTab === 'overview' && <AdminOverviewPanel />}
              {settingsTab === 'rbac' && <PlatformAdminManager />}
              {settingsTab === 'apiKeys' && <APIKeyManager />}
              {settingsTab === 'graylist' && <GraylistManager />}
              {settingsTab === 'audit' && <AuditLogViewer />}
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
              {/* P13-E 导入 */}
              <span style={{ fontSize: 11, opacity: 0.5, marginLeft: 8 }}>导入：</span>
              <button
                onClick={() => importInputRef.current?.click()}
                disabled={importBusy}
                style={miniBtn}
              >
                {importBusy ? '…' : '📂 文件'}
              </button>
              <input
                ref={importInputRef}
                type="file"
                accept=".json,.md,.markdown"
                style={{ display: 'none' }}
                onChange={e => {
                  const f = e.target.files?.[0]
                  if (f) void importLakeUI(f)
                  e.target.value = ''
                }}
              />
              {importResult && (
                <span style={{ fontSize: 11, color: '#a6e3a1' }}>
                  ✓ 导入 {importResult.imported} 条{importResult.skipped > 0 ? `，跳过 ${importResult.skipped}` : ''}
                </span>
              )}
              {lakeTags.length > 0 && (
                <>
                  <span style={{ fontSize: 11, opacity: 0.5, marginLeft: 8 }}>标签筛选：</span>
                  {tagFilter && (
                    <button onClick={() => applyTagFilter('')} style={{ ...miniBtn, color: '#89dceb' }}>
                      {tagLoading ? '…' : `✕ ${tagFilter}`}
                    </button>
                  )}
                  {lakeTags.filter(t => t !== tagFilter).map(t => (
                    <button key={t} onClick={() => applyTagFilter(t)} style={miniBtn}>{t}</button>
                  ))}
                </>
              )}
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
                  {/* P18-C：从模板创建节点 */}
                  <button
                    onClick={() => { void loadTemplates(); setTemplateModalOpen(true) }}
                    style={{ ...miniBtn, color: '#cba6f7' }}
                    title="从模板创建节点"
                  >📋 模板</button>
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
                  {/* P17-B：图谱节点搜索 */}
                  <div style={{ marginBottom: 8 }}>
                    <input
                      value={nodeSearch}
                      onChange={e => setNodeSearch(e.target.value)}
                      placeholder="搜索节点高亮…"
                      style={{ width: '100%', padding: '4px 8px', fontSize: 12, background: '#0d1e2e', border: '1px solid #1e3a5a', borderRadius: 4, color: '#c8dff0', boxSizing: 'border-box' }}
                    />
                  </div>
                  {/* P18-D：快照工具栏 */}
                  <div style={{ display: 'flex', gap: 6, marginBottom: 8, alignItems: 'center', flexWrap: 'wrap' }}>
                    <button
                      onClick={() => void saveSnapshot()}
                      style={{ ...miniBtn, color: '#a6e3a1' }}
                      title="保存当前图谱布局为快照"
                    >📷 保存快照</button>
                    <button
                      onClick={() => setSnapshotPanelOpen(true)}
                      style={{ ...miniBtn, color: snapshotPanelOpen ? '#89b4fa' : undefined }}
                      title="查看图谱快照"
                    >🗂 快照</button>
                    {graphLayout && (
                      <button onClick={() => setGraphLayout(undefined)} style={{ ...miniBtn, color: '#f38ba8' }}>
                        ✕ 清除快照布局
                      </button>
                    )}
                    {/* P19-A：AI 探索 */}
                    <button
                      onClick={() => setExplorerOpen(v => !v)}
                      style={{ ...miniBtn, color: explorerOpen ? '#89b4fa' : '#cba6f7' }}
                      title="AI 图谱探索"
                    >🔍 AI探索</button>
                    {exploredNodeIds.size > 0 && (
                      <button
                        onClick={() => setExploredNodeIds(new Set())}
                        style={{ ...miniBtn, color: '#f38ba8' }}
                        title="清除探索高亮"
                      >✕ 清除高亮</button>
                    )}
                  </div>
                  {snapshotPanelOpen && active && (
                    <React.Suspense fallback={null}>
                      <SnapshotPanel
                        lakeId={active.id}
                        currentLayout={Object.fromEntries(nodes.map(n => [n.id, { x: n.position.x, y: n.position.y }]))}
                        onClose={() => setSnapshotPanelOpen(false)}
                        onRestore={layout => { setGraphLayout(layout); setSnapshotPanelOpen(false) }}
                      />
                    </React.Suspense>
                  )}
                  {/* P19-A：AI 图谱探索面板 */}
                  {explorerOpen && active && (
                    <React.Suspense fallback={null}>
                      <NodeExplorer
                        lakeId={active.id}
                        onHighlight={ids => setExploredNodeIds(ids)}
                        onClose={() => { setExplorerOpen(false); setExploredNodeIds(new Set()) }}
                      />
                    </React.Suspense>
                  )}
                  <React.Suspense fallback={<div style={{ height: 480, display: 'flex', alignItems: 'center', justifyContent: 'center', color: '#4a6a8e', fontSize: 13 }}>加载图谱中…</div>}>
                    <LakeGraph
                      nodes={nodes}
                      edges={edges}
                      searchQuery={nodeSearch}
                      snapshotLayout={graphLayout}
                      remoteCursors={remoteCursors}
                      onSendCursor={(x, y) => wsRef.current?.send({ type: 'cursor.move', payload: { x, y } })}
                    />
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
                  {/* P14-C：批量操作工具栏 */}
                  {batchSel.size > 0 && (
                    <div style={{ display: 'flex', gap: 8, alignItems: 'center', marginBottom: 8, padding: '6px 10px', background: 'rgba(74,144,226,0.1)', borderRadius: 6 }}>
                      <span style={{ fontSize: 12, color: '#9ec5ee' }}>已选 {batchSel.size} 个节点</span>
                      <button onClick={() => setBatchSel(new Set(filteredNodes.map(n => n.id)))} style={{ ...miniBtn, opacity: 0.7 }}>全选</button>
                      <button onClick={() => void batchOperate('condense')} disabled={batchBusy} style={miniBtn}>凝露 ↓</button>
                      <button onClick={() => void batchOperate('evaporate')} disabled={batchBusy} style={miniBtn}>蒸发 ↑</button>
                      <button onClick={() => { if (window.confirm(`确认彻底删除已选 ${batchSel.size} 个节点？此操作不可恢复。`)) { void batchOperate('erase') } }} disabled={batchBusy} style={{ ...miniBtn, background: 'rgba(220,53,69,0.15)', color: '#ff6b7a' }}>删除 ✕</button>
                      <button onClick={() => setBatchSel(new Set())} style={{ ...miniBtn, opacity: 0.6 }}>取消选择</button>
                    </div>
                  )}
                  {nodes.length > 0 && filteredNodes.length === 0 && (
                    <div style={{ opacity: 0.4, fontSize: 12 }}>没有带「{tagFilter}」标签的节点</div>
                  )}
                  <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fill, minmax(220px, 1fr))', gap: 8 }}>
                    {filteredNodes.map(n => {
                      const out = outDeg.get(n.id) ?? 0
                      const inc = inDeg.get(n.id) ?? 0
                      const isLinkSrc = linkSrc === n.id
                      const canCrystal = n.state === 'DROP' || n.state === 'FROZEN'
                      const isSelected = crystalSel.has(n.id)
                      const isBatchSel = batchSel.has(n.id)
                      const tags = nodeTags[n.id] ?? []
                      return (
                        <div key={n.id} style={{
                          ...nodeCard,
                          opacity: n.state === 'VAPOR' ? 0.4 : 1,
                          boxShadow: isLinkSrc
                            ? '0 0 0 2px #9ec5ee'
                            : isSelected ? '0 0 0 2px #4a8eff'
                            : isBatchSel ? '0 0 0 2px #f9e2af' : undefined,
                        }}>
                          <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
                            <span style={{ ...statePill, background: stateColor(n.state) }}>{n.state}</span>
                        <span style={{ fontSize: 10, opacity: 0.6, display: 'flex', gap: 6, alignItems: 'center' }}>
                          →{out} ←{inc}
                          {/* P14-C 批量选择 checkbox */}
                          <input
                            type="checkbox"
                            checked={isBatchSel}
                            onChange={e => setBatchSel(prev => {
                              const next = new Set(prev)
                              if (e.target.checked) next.add(n.id); else next.delete(n.id)
                              return next
                            })}
                            title="选入批量操作"
                            style={{ cursor: 'pointer' }}
                          />
                        </span>
                      </div>
                      <div style={{ marginTop: 8, fontSize: 13, lineHeight: 1.5 }}>{n.content}</div>
                      {/* P16-B AI 摘要展示 */}
                      {aiSummary[n.id] && (
                        <div style={{
                          marginTop: 6, padding: '4px 8px',
                          background: 'rgba(74,144,226,0.08)', borderLeft: '2px solid #4a8eff',
                          borderRadius: 3, fontSize: 11, color: '#9ec5ee', lineHeight: 1.5,
                        }}>
                          ✦ {aiSummary[n.id]}
                        </div>
                      )}
                      {/* P13-C 标签 */}
                      <NodeTagEditor
                        nodeId={n.id}
                        tags={tags}
                        onChanged={newTags => {
                          setNodeTags(prev => ({ ...prev, [n.id]: newTags }))
                          // 刷新湖标签
                          if (active) api.getLakeTags(active.id).then(r => setLakeTags(r.tags)).catch(() => undefined)
                        }}
                      />
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
                        <button onClick={() => void showDiff(n)} style={miniBtn} title="版本对比 diff">⇄</button>
                        {/* P17-D 节点导出 */}
                        <button
                          onClick={() => {
                            const fmt = window.confirm('确认导出格式？\n确定 = Markdown，取消 = JSON') ? 'md' : 'json'
                            exportNode(n, fmt)
                          }}
                          style={miniBtn}
                          title="导出节点"
                        >⬇</button>
                        {/* P16-B AI 摘要 */}
                        <button
                          onClick={() => void requestAiSummary(n)}
                          disabled={aiSummaryBusy.has(n.id)}
                          style={miniBtn}
                          title="AI 摘要"
                        >{aiSummaryBusy.has(n.id) ? '…' : '✦'}</button>
                        {/* P18-A 关联推荐 */}
                        <button
                          onClick={() => void loadRelated(n.id)}
                          disabled={relatedLoading === n.id}
                          style={{ ...miniBtn, color: '#89dceb' }}
                          title="查找关联节点"
                        >{relatedLoading === n.id ? '…' : '⚡关联'}</button>
                        {/* P18-B 节点分享 */}
                        <button
                          onClick={() => setShareNode(n)}
                          disabled={false}
                          style={{ ...miniBtn, color: '#f9e2af' }}
                          title="分享节点"
                        >🔗分享</button>
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
      {/* P15-B：版本 diff 对比视图 */}
      {diffModal && (
        <NodeDiffViewer
          nodeId={diffModal.nodeId}
          revisions={diffModal.revisions}
          onClose={() => setDiffModal(null)}
        />
      )}
      {/* P17-A：版本历史时间线 */}
      {historyModal && (
        <NodeVersionHistory
          node={historyModal.node}
          revisions={historyModal.revisions}
          onClose={() => setHistoryModal(null)}
          onRolledBack={updatedNode => {
            setNodes(prev => prev.map(n => n.id === updatedNode.id ? updatedNode : n))
            setHistoryModal(null)
          }}
        />
      )}
      {/* P18-A：关联推荐面板 */}
      {relatedPanel && (
        <div style={modalOverlay} onClick={() => setRelatedPanel(null)}>
          <div style={{ ...modalBox, minWidth: 360 }} onClick={e => e.stopPropagation()}>
            <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 12 }}>
              <strong style={{ color: '#89dceb' }}>⚡ 关联节点推荐</strong>
              <button onClick={() => setRelatedPanel(null)} style={miniBtn}>✕</button>
            </div>
            {relatedPanel.results.length === 0 ? (
              <div style={{ fontSize: 13, opacity: 0.5 }}>暂无关联节点</div>
            ) : relatedPanel.results.map(r => (
              <div key={r.node_id} style={{
                padding: '8px 10px', marginBottom: 6, background: 'rgba(137,220,235,0.06)',
                border: '1px solid rgba(137,220,235,0.2)', borderRadius: 6,
              }}>
                <div style={{ fontSize: 12, opacity: 0.7, marginBottom: 4 }}>
                  id: {r.node_id.slice(0, 8)}… · score: {r.score.toFixed(3)}
                </div>
                <div style={{ fontSize: 13, lineHeight: 1.5 }}>{r.snippet.slice(0, 120)}{r.snippet.length > 120 ? '…' : ''}</div>
              </div>
            ))}
          </div>
        </div>
      )}
      {/* P18-B：节点分享管理（NodeShareButton 组件） */}
      {shareNode && (
        <React.Suspense fallback={null}>
          <NodeShareButton node={shareNode} onClose={() => setShareNode(null)} />
        </React.Suspense>
      )}
      {/* P18-C：节点模板选择器 */}
      {templateModalOpen && (
        <div style={modalOverlay} onClick={() => { setTemplateModalOpen(false); setTplCreateOpen(false); setTplPreviewId(null) }}>
          <div style={{ ...modalBox, minWidth: 480, maxHeight: '80vh', display: 'flex', flexDirection: 'column' }} onClick={e => e.stopPropagation()}>
            {/* 标题栏 */}
            <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 12, flexShrink: 0 }}>
              <strong style={{ color: '#cba6f7' }}>📋 节点模板库</strong>
              <div style={{ display: 'flex', gap: 6 }}>
                <button
                  onClick={() => { setTplCreateOpen(v => !v); setTplPreviewId(null) }}
                  style={{ ...miniBtn, color: tplCreateOpen ? '#cba6f7' : '#89b4fa' }}
                >
                  {tplCreateOpen ? '取消新建' : '+ 新建模板'}
                </button>
                <button onClick={() => { setTemplateModalOpen(false); setTplCreateOpen(false); setTplPreviewId(null) }} style={miniBtn}>✕</button>
              </div>
            </div>

            {/* 新建表单 */}
            {tplCreateOpen && (
              <div style={{ padding: '12px 14px', background: 'rgba(203,166,247,0.06)', border: '1px solid rgba(203,166,247,0.25)', borderRadius: 8, marginBottom: 12, flexShrink: 0 }}>
                <div style={{ fontSize: 12, color: '#cba6f7', marginBottom: 8, fontWeight: 600 }}>新建自定义模板</div>
                <input
                  placeholder="模板名称（必填）"
                  value={tplForm.name}
                  onChange={e => setTplForm(f => ({ ...f, name: e.target.value }))}
                  style={{ ...tplInput, marginBottom: 6 }}
                />
                <textarea
                  placeholder="模板内容（必填）"
                  value={tplForm.content}
                  onChange={e => setTplForm(f => ({ ...f, content: e.target.value }))}
                  rows={4}
                  style={{ ...tplInput, resize: 'vertical' }}
                />
                <input
                  placeholder="描述（选填）"
                  value={tplForm.description}
                  onChange={e => setTplForm(f => ({ ...f, description: e.target.value }))}
                  style={{ ...tplInput, marginTop: 6 }}
                />
                <input
                  placeholder="标签，逗号分隔（选填）"
                  value={tplForm.tags}
                  onChange={e => setTplForm(f => ({ ...f, tags: e.target.value }))}
                  style={{ ...tplInput, marginTop: 6 }}
                />
                {tplForm.content.trim() && (
                  <div style={{ marginTop: 8 }}>
                    <div style={{ fontSize: 11, opacity: 0.7, marginBottom: 4 }}>内容预览</div>
                    <div style={{
                      marginTop: 4, fontSize: 12, lineHeight: 1.6,
                      background: 'rgba(0,0,0,0.25)', borderRadius: 4, padding: '8px 10px',
                      whiteSpace: 'pre-wrap', maxHeight: 160, overflowY: 'auto',
                      color: '#cdd6f4', wordBreak: 'break-word',
                    }}>
                      {tplForm.content}
                    </div>
                  </div>
                )}
                <button
                  disabled={tplCreateBusy || !tplForm.name.trim() || !tplForm.content.trim()}
                  onClick={async () => {
                    setTplCreateBusy(true)
                    try {
                      const tags = tplForm.tags.split(',').map(s => s.trim()).filter(Boolean)
                      await api.createTemplate(tplForm.name.trim(), tplForm.content.trim(), tplForm.description.trim() || undefined, tags.length ? tags : undefined)
                      setTplForm({ name: '', content: '', description: '', tags: '' })
                      setTplCreateOpen(false)
                      await api.listTemplates().then(r => setTemplates(r.templates))
                    } catch (e) { void modalAlert((e as Error).message, { title: '创建失败' }) }
                    finally { setTplCreateBusy(false) }
                  }}
                  style={{ ...miniBtn, marginTop: 8, color: '#cba6f7' }}
                >
                  {tplCreateBusy ? '保存中…' : '保存模板'}
                </button>
              </div>
            )}

            {/* 模板列表 */}
            <div style={{ overflowY: 'auto', flex: 1 }}>
              {templatesBusy ? (
                <div style={{ fontSize: 13, opacity: 0.5 }}>加载中…</div>
              ) : templates.length === 0 ? (
                <div style={{ fontSize: 13, opacity: 0.5 }}>暂无模板，点击「+ 新建模板」创建</div>
              ) : templates.map(t => (
                <div key={t.id} style={{
                  padding: '10px 12px', marginBottom: 8,
                  background: 'rgba(203,166,247,0.06)',
                  border: `1px solid ${tplPreviewId === t.id ? 'rgba(203,166,247,0.5)' : 'rgba(203,166,247,0.2)'}`,
                  borderRadius: 6,
                }}>
                  <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
                    <strong style={{ fontSize: 13, cursor: 'pointer', flex: 1 }} onClick={() => setTplPreviewId(tplPreviewId === t.id ? null : t.id)}>{t.name}</strong>
                    <div style={{ display: 'flex', gap: 4, alignItems: 'center' }}>
                      {t.is_system && <span style={{ fontSize: 10, color: '#cba6f7', opacity: 0.7 }}>系统</span>}
                      <button
                        onClick={() => setTplPreviewId(tplPreviewId === t.id ? null : t.id)}
                        style={{ ...miniBtn, color: '#89b4fa', fontSize: 11, padding: '2px 6px' }}
                      >{tplPreviewId === t.id ? '收起' : '预览'}</button>
                      <button
                        onClick={() => void createFromTemplate(t.id)}
                        style={{ ...miniBtn, color: '#a6e3a1', fontSize: 11, padding: '2px 8px' }}
                      >使用</button>
                      {!t.is_system && (
                        <button
                          onClick={async () => {
                            if (!await modalConfirm(`确认删除「${t.name}」？`, { title: '删除模板' })) return
                            try {
                              await api.deleteTemplate(t.id)
                              setTemplates(prev => prev.filter(x => x.id !== t.id))
                            } catch (e) { void modalAlert((e as Error).message, { title: '删除失败' }) }
                          }}
                          style={{ ...miniBtn, color: '#f38ba8', fontSize: 11, padding: '2px 6px' }}
                        >删</button>
                      )}
                    </div>
                  </div>
                  {t.description && <div style={{ fontSize: 12, opacity: 0.7, marginTop: 4 }}>{t.description}</div>}

                  {/* 内容预览 —— 点击模板名展开/折叠 */}
                  {tplPreviewId === t.id ? (
                    <div style={{
                      marginTop: 8, fontSize: 12, lineHeight: 1.6,
                      background: 'rgba(0,0,0,0.25)', borderRadius: 4, padding: '8px 10px',
                      whiteSpace: 'pre-wrap', maxHeight: 200, overflowY: 'auto',
                      color: '#cdd6f4', wordBreak: 'break-word',
                    }}>{t.content}</div>
                  ) : (
                    <div style={{ fontSize: 11, opacity: 0.5, marginTop: 4, cursor: 'pointer' }} onClick={() => setTplPreviewId(t.id)}>
                      {t.content.slice(0, 120)}{t.content.length > 120 ? '… 点击展开' : ''}
                    </div>
                  )}

                  {t.tags.length > 0 && (
                    <div style={{ marginTop: 6, display: 'flex', gap: 4, flexWrap: 'wrap' }}>
                      {t.tags.map(tag => <span key={tag} style={tagChip}>{tag}</span>)}
                    </div>
                  )}
                </div>
              ))}
            </div>
          </div>
        </div>
      )}
    </div>
  )
}

// P18 modal styles
const modalOverlay: React.CSSProperties = {
  position: 'fixed', inset: 0, background: 'rgba(0,0,0,0.6)',
  display: 'flex', alignItems: 'center', justifyContent: 'center',
  zIndex: 500,
}
const modalBox: React.CSSProperties = {
  background: '#111827', border: '1px solid rgba(255,255,255,0.15)',
  borderRadius: 10, padding: 24, maxHeight: '80vh', overflowY: 'auto',
  minWidth: 320, maxWidth: 560,
}

// P13-C：节点标签编辑器组件
function NodeTagEditor({ nodeId, tags, onChanged }: {
  nodeId: string
  tags: string[]
  onChanged: (tags: string[]) => void
}) {
  const [editing, setEditing] = useState(false)
  const [draft, setDraft] = useState('')
  const [saving, setSaving] = useState(false)

  function openEditor() {
    setDraft(tags.join(', '))
    setEditing(true)
  }

  async function save() {
    const next = draft.split(',').map(t => t.trim()).filter(Boolean)
    setSaving(true)
    try {
      const res = await api.setNodeTags(nodeId, next)
      onChanged(res.tags)
    } catch {
      // ignore
    } finally {
      setSaving(false)
      setEditing(false)
    }
  }

  if (editing) {
    return (
      <div style={{ marginTop: 6, display: 'flex', gap: 4, flexWrap: 'wrap', alignItems: 'center' }}>
        <input
          autoFocus
          value={draft}
          onChange={e => setDraft(e.target.value)}
          onKeyDown={e => { if (e.key === 'Enter') void save(); if (e.key === 'Escape') setEditing(false) }}
          placeholder="逗号分隔标签"
          style={{ flex: 1, minWidth: 100, fontSize: 11, padding: '2px 6px', background: '#2a2d3a', border: '1px solid #555', borderRadius: 4, color: '#cdd6f4' }}
        />
        <button onClick={() => void save()} disabled={saving} style={tagChipBtn}>{saving ? '…' : '✓'}</button>
        <button onClick={() => setEditing(false)} style={tagChipBtn}>✕</button>
      </div>
    )
  }

  return (
    <div style={{ marginTop: 6, display: 'flex', gap: 4, flexWrap: 'wrap', alignItems: 'center' }}>
      {tags.map(t => (
        <span key={t} style={tagChip}>{t}</span>
      ))}
      <button onClick={openEditor} style={tagChipBtn} title="编辑标签">+标签</button>
    </div>
  )
}

const tagChip: React.CSSProperties = {
  display: 'inline-block', fontSize: 10, padding: '1px 7px',
  background: 'rgba(137,220,235,0.15)', color: '#89dceb',
  borderRadius: 10, border: '1px solid rgba(137,220,235,0.3)',
}

const tagChipBtn: React.CSSProperties = {
  fontSize: 10, padding: '1px 6px', cursor: 'pointer',
  background: 'rgba(255,255,255,0.05)', color: '#cdd6f4',
  border: '1px solid #555', borderRadius: 10,
}

const tplInput: React.CSSProperties = {
  width: '100%', background: '#0a1929', border: '1px solid rgba(203,166,247,0.3)',
  borderRadius: 4, color: '#cdd6f4', padding: '6px 10px', fontSize: 12,
  fontFamily: 'inherit', boxSizing: 'border-box',
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
