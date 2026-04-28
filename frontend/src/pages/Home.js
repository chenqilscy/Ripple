import { jsx as _jsx, jsxs as _jsxs, Fragment as _Fragment } from "react/jsx-runtime";
import React, { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { api } from '../api/client';
import { NodeDiffViewer } from '../components/NodeDiffViewer';
import NodeVersionHistory from '../components/NodeVersionHistory';
const LakeGraph = React.lazy(() => import('../components/LakeGraph'));
const AdminOverviewPanel = React.lazy(() => import('../components/AdminOverviewPanel'));
const APIKeyManager = React.lazy(() => import('../components/APIKeyManager'));
const AuditLogViewer = React.lazy(() => import('../components/AuditLogViewer'));
const GraylistManager = React.lazy(() => import('../components/GraylistManager'));
const PlatformAdminManager = React.lazy(() => import('../components/PlatformAdminManager'));
const LakeMemberManager = React.lazy(() => import('../components/LakeMemberManager'));
const SearchModal = React.lazy(() => import('../components/SearchModal'));
const ImportModal = React.lazy(() => import('../components/ImportModal'));
const OrgPanel = React.lazy(() => import('../components/OrgPanel'));
import { prompt as modalPrompt, confirm as modalConfirm, alert as modalAlert } from '../components/Modal';
import SpaceSwitcher from '../components/SpaceSwitcher';
import SpaceMembersDrawer from '../components/SpaceMembersDrawer';
import AttachmentBar from '../components/AttachmentBar';
import CollabDemo from '../components/CollabDemo';
import OfflineBar from '../components/OfflineBar';
import NotificationBell from '../components/NotificationBell';
import { LakeWS } from '../api/wsClient';
const EDGE_KINDS = ['relates', 'derives', 'opposes', 'refines', 'groups', 'custom'];
const LAKE_READY_INITIAL_DELAY_MS = 1000;
const LAKE_READY_ATTEMPTS = 30;
const LAKE_READY_DELAY_MS = 200;
function delay(ms) {
    return new Promise(resolve => setTimeout(resolve, ms));
}
export function Home({ onLogout }) {
    const [lakes, setLakes] = useState([]);
    const [active, setActive] = useState(null);
    const [nodes, setNodes] = useState([]);
    const [edges, setEdges] = useState([]);
    const [tasks, setTasks] = useState([]);
    const [prompt, setPrompt] = useState('');
    const [n, setN] = useState(5);
    const [newLakeName, setNewLakeName] = useState('');
    const [busy, setBusy] = useState(false);
    const [err, setErr] = useState(null);
    const [wsOnline, setWsOnline] = useState(false);
    const [onlineUsers, setOnlineUsers] = useState([]);
    // 连线状态：null=普通 | source_id=已选起点，等待选终点
    const [linkSrc, setLinkSrc] = useState(null);
    // M3-S1：当前选中的空间。'' = 个人湖（无 space_id）
    const [currentSpaceId, setCurrentSpaceId] = useState('');
    // 成员管理抽屉
    const [membersDrawer, setMembersDrawer] = useState(null);
    const [searchOpen, setSearchOpen] = useState(false);
    const [importOpen, setImportOpen] = useState(false);
    const [orgOpen, setOrgOpen] = useState(false);
    const [meId, setMeId] = useState('');
    const [pendingAction, setPendingAction] = useState(null);
    const [exportBusy, setExportBusy] = useState(false);
    const [importBusy, setImportBusy] = useState(false);
    const [importResult, setImportResult] = useState(null);
    const importInputRef = useRef(null);
    // P14-C：批量操作
    const [batchSel, setBatchSel] = useState(new Set());
    const [batchBusy, setBatchBusy] = useState(false);
    // P15-B：版本 diff 视图
    const [diffModal, setDiffModal] = useState(null);
    // P17-A：版本历史时间线
    const [historyModal, setHistoryModal] = useState(null);
    // P17-B：图谱节点搜索高亮
    const [nodeSearch, setNodeSearch] = useState('');
    // P16-B：AI 摘要
    const [aiSummary, setAiSummary] = useState({});
    const [aiSummaryBusy, setAiSummaryBusy] = useState(new Set());
    // P13-C：标签过滤
    const [tagFilter, setTagFilter] = useState('');
    const [tagFilteredIds, setTagFilteredIds] = useState(null);
    const [tagLoading, setTagLoading] = useState(false);
    const tagAbortRef = useRef(null);
    const [lakeTags, setLakeTags] = useState([]);
    const [nodeTags, setNodeTags] = useState({});
    // P18-A：节点关联推荐
    const [relatedPanel, setRelatedPanel] = useState(null);
    const [relatedLoading, setRelatedLoading] = useState(null);
    // P18-C：节点模板库
    const [templateModalOpen, setTemplateModalOpen] = useState(false);
    const [templates, setTemplates] = useState([]);
    const [templatesBusy, setTemplatesBusy] = useState(false);
    const [tplCreateOpen, setTplCreateOpen] = useState(false);
    const [tplForm, setTplForm] = useState({ name: '', content: '', description: '', tags: '' });
    const [tplCreateBusy, setTplCreateBusy] = useState(false);
    const [tplPreviewId, setTplPreviewId] = useState(null);
    // P18-D：图谱快照
    const [snapshotPanelOpen, setSnapshotPanelOpen] = useState(false);
    const [snapshots, setSnapshots] = useState([]);
    const [snapshotBusy, setSnapshotBusy] = useState(false);
    const [graphLayout, setGraphLayout] = useState(undefined);
    // P18-B：节点外链分享
    const [shareModal, setShareModal] = useState(null);
    const [shareLoading, setShareLoading] = useState(null);
    // P12-C：拉取当前登录用户 ID（用于组织权限判断）
    useEffect(() => {
        api.me().then(u => setMeId(u.id)).catch(() => { });
    }, []);
    // P12-E：PWA shortcut 处理 — ?action=search|import（需等 active 湖加载完毕）
    useEffect(() => {
        const action = new URLSearchParams(window.location.search).get('action');
        if (!action)
            return;
        window.history.replaceState({}, '', window.location.pathname);
        setPendingAction(action);
    }, []);
    useEffect(() => {
        if (!pendingAction || !active)
            return;
        if (pendingAction === 'search')
            setSearchOpen(true);
        if (pendingAction === 'import')
            setImportOpen(true);
        setPendingAction(null);
    }, [pendingAction, active]);
    // M3-S2：凝结多选（DROP/FROZEN 节点 id 集合）
    const [crystalSel, setCrystalSel] = useState(new Set());
    // 凝结结果（最近一次）
    const [recentPerma, setRecentPerma] = useState(null);
    // M3-T4：SSE 流式预览
    const [streamText, setStreamText] = useState('');
    const [streaming, setStreaming] = useState(false);
    const streamAbortRef = useRef(null);
    // P9-C：节点视图模式（列表 | 图谱）
    const [viewMode, setViewMode] = useState('list');
    // P11：主区 Tab（lakes=主流程 | settings=API Key+审计日志）
    const [mainTab, setMainTab] = useState('lakes');
    // M3-S3：推荐位（基于历史 LIKE 反馈的协同过滤）
    const [recos, setRecos] = useState([]);
    const wsRef = useRef(null);
    useEffect(() => { void refresh(); }, [currentSpaceId]);
    // P12-D：Cmd+K / Ctrl+K 打开搜索浮层
    useEffect(() => {
        const handler = (e) => {
            if ((e.metaKey || e.ctrlKey) && e.key === 'k') {
                e.preventDefault();
                if (active)
                    setSearchOpen(o => !o);
            }
        };
        window.addEventListener('keydown', handler);
        return () => window.removeEventListener('keydown', handler);
    }, [active]);
    // M3-S3：拉取推荐位（异步，失败静默）
    useEffect(() => {
        api.recommend('perma_node', 6)
            .then(r => setRecos(r.recommendations || []))
            .catch(() => setRecos([]));
    }, []);
    // 切换 active 湖时：重建 WS 订阅，并加载节点。
    useEffect(() => {
        if (!active)
            return;
        void loadNodes(active.id);
        void loadEdges(active.id);
        void loadPresence(active.id);
        setLinkSrc(null);
        setViewMode('list');
        setTagFilter('');
        setTagFilteredIds(null);
        tagAbortRef.current?.abort();
        setImportResult(null);
        setBatchSel(new Set());
        setGraphLayout(undefined);
        // P18-D：加载快照
        void loadSnapshots(active.id);
        // P13-C：加载湖标签列表
        api.getLakeTags(active.id).then(r => setLakeTags(r.tags)).catch(() => setLakeTags([]));
        const token = localStorage.getItem('ripple.token') ?? '';
        if (!token)
            return;
        // 关闭旧连接
        wsRef.current?.close();
        const ws = new LakeWS(active.id, token, msg => {
            // node 事件 → 全量刷新节点（MVP 简化，避免增量 merge 复杂度）
            if (msg.type.startsWith('node.')) {
                void loadNodes(active.id);
            }
            if (msg.type.startsWith('edge.')) {
                void loadEdges(active.id);
            }
            if (msg.type.startsWith('presence.')) {
                void loadPresence(active.id);
            }
            // cloud 事件 → 刷新任务列表（如有 task_id）
            if (msg.type.startsWith('cloud.') && msg.payload?.task_id) {
                api.getCloud(msg.payload.task_id)
                    .then(t => setTasks(prev => prev.map(x => x.id === t.id ? t : x)))
                    .catch(() => { });
            }
            // P14-A：通知实时推送 → 转发给 NotificationBell
            if (msg.type === 'notification.new') {
                window.dispatchEvent(new CustomEvent('ripple:notification', { detail: msg.payload }));
            }
        }, online => setWsOnline(online));
        ws.connect();
        wsRef.current = ws;
        return () => {
            ws.close();
            wsRef.current = null;
            setWsOnline(false);
        };
    }, [active]);
    async function refresh() {
        try {
            const r = await api.listLakes(currentSpaceId || undefined);
            setLakes(r.lakes);
            // 切换空间后，若当前 active 不在新列表中，自动选第一个/置空
            if (r.lakes.length === 0) {
                setActive(null);
            }
            else if (!active || !r.lakes.find(l => l.id === active.id)) {
                setActive(r.lakes[0]);
            }
        }
        catch (e) {
            setErr(e.message);
        }
    }
    async function waitLakeReady(lake) {
        let lastErr = null;
        await delay(LAKE_READY_INITIAL_DELAY_MS);
        for (let i = 0; i < LAKE_READY_ATTEMPTS; i++) {
            try {
                return await api.getLake(lake.id);
            }
            catch (e) {
                lastErr = e;
                await delay(LAKE_READY_DELAY_MS);
            }
        }
        throw lastErr instanceof Error ? lastErr : new Error('lake projection not ready');
    }
    async function loadNodes(lakeId) {
        try {
            setNodes((await api.listNodes(lakeId)).nodes);
        }
        catch (e) {
            setErr(e.message);
        }
    }
    async function loadEdges(lakeId) {
        try {
            setEdges((await api.listEdges(lakeId)).edges);
        }
        catch (e) {
            setErr(e.message);
        }
    }
    async function loadPresence(lakeId) {
        try {
            setOnlineUsers((await api.listPresence(lakeId)).users);
        }
        catch { /* 非关键：静默 */ }
    }
    // 进入连线：点第一个节点设为 source；点第二个节点询问 kind 后创建。
    async function handleNodeClickForLink(nodeId) {
        if (!linkSrc) {
            setLinkSrc(nodeId);
            return;
        }
        if (linkSrc === nodeId) {
            setLinkSrc(null); // 再次点同一个 = 取消
            return;
        }
        const kind = await modalPrompt({
            title: '边类型',
            label: `可选：${EDGE_KINDS.join(' / ')}`,
            initial: 'relates',
            validate: (v) => (!EDGE_KINDS.includes(v.trim()) ? '无效的边类型' : null),
        });
        if (kind === null) {
            setLinkSrc(null);
            return;
        }
        let label;
        if (kind.trim() === 'custom') {
            const labelIn = await modalPrompt({
                title: '自定义边的标签',
                validate: (v) => (!v.trim() ? '标签不能为空' : null),
            });
            if (labelIn === null) {
                setLinkSrc(null);
                return;
            }
            label = labelIn.trim();
        }
        try {
            await api.createEdge(linkSrc, nodeId, kind, label);
            if (active)
                await loadEdges(active.id);
        }
        catch (e) {
            setErr(e.message);
        }
        finally {
            setLinkSrc(null);
        }
    }
    async function deleteEdge(id) {
        if (!(await modalConfirm('确定删除这条边？', { danger: true })))
            return;
        try {
            await api.deleteEdge(id);
            if (active)
                await loadEdges(active.id);
        }
        catch (e) {
            setErr(e.message);
        }
    }
    async function editNodeContent(node) {
        const next = await modalPrompt({
            title: '编辑节点内容',
            label: '支持多行；Ctrl+Enter 提交，Esc 取消',
            initial: node.content,
            multiline: true,
            validate: (v) => (!v.trim() ? '内容不能为空' : null),
        });
        if (next === null || next === node.content)
            return;
        const reason = await modalPrompt({
            title: '变更说明（可选）',
            placeholder: '例如：修正措辞 / 补充例子 …',
            initial: '',
        });
        if (reason === null)
            return;
        try {
            await api.updateNodeContent(node.id, next, reason);
            if (active)
                await loadNodes(active.id);
        }
        catch (e) {
            setErr(e.message);
        }
    }
    async function showHistory(node) {
        try {
            const { revisions } = await api.listNodeRevisions(node.id, 50);
            setHistoryModal({ node, revisions });
        }
        catch (e) {
            setErr(e.message);
        }
    }
    // P17-D：节点导出下载
    function exportNode(node, format) {
        const content = format === 'md'
            ? `# ${node.id}\n\n**状态**: ${node.state}  \n**创建**: ${new Date(node.created_at).toLocaleString('zh-CN')}\n\n${node.content}`
            : JSON.stringify(node, null, 2);
        const blob = new Blob([content], { type: format === 'md' ? 'text/markdown' : 'application/json' });
        const url = URL.createObjectURL(blob);
        const a = document.createElement('a');
        a.href = url;
        a.download = `node-${node.id.slice(0, 8)}.${format}`;
        a.click();
        // Revoke after a tick to ensure download triggers before cleanup
        setTimeout(() => URL.revokeObjectURL(url), 100);
    }
    async function showDiff(node) {
        try {
            const { revisions } = await api.listNodeRevisions(node.id, 50);
            if (revisions.length < 2) {
                await modalAlert('至少需要 2 个版本才能对比');
                return;
            }
            setDiffModal({ nodeId: node.id, revisions });
        }
        catch (e) {
            setErr(e.message);
        }
    }
    // P16-B：AI 节点摘要
    async function requestAiSummary(node) {
        if (aiSummaryBusy.has(node.id))
            return;
        setAiSummaryBusy(prev => new Set([...prev, node.id]));
        try {
            const r = await api.aiSummaryNode(node.id);
            setAiSummary(prev => ({ ...prev, [node.id]: r.summary }));
        }
        catch (e) {
            setErr(e.message);
        }
        finally {
            setAiSummaryBusy(prev => { const next = new Set(prev); next.delete(node.id); return next; });
        }
    }
    async function copyText(text) {
        try {
            await navigator.clipboard.writeText(text);
            return true;
        }
        catch {
            return false;
        }
    }
    async function manageInvites() {
        if (!active)
            return;
        try {
            const { invites } = await api.listInvites(active.id, false);
            const aliveLines = invites.length === 0
                ? '(无活跃邀请)'
                : invites.map(i => `${i.id.slice(0, 8)} | ${i.role} | ${i.used_count}/${i.max_uses} | 到期 ${new Date(i.expires_at).toLocaleString()}\n  token: ${i.token}`).join('\n\n');
            const action = await modalPrompt({
                title: `湖 ${active.name} 邀请`,
                label: `${aliveLines}\n\n输入：\nC = 创建新邀请\nR:<id前缀> = 撤销`,
                placeholder: '例如：C 或 R:abcd1234',
            });
            if (action === null || !action.trim())
                return;
            const cmd = action.trim();
            if (cmd.toUpperCase() === 'C') {
                const roleIn = await modalPrompt({
                    title: '邀请角色',
                    label: 'NAVIGATOR / PASSENGER / OBSERVER',
                    initial: 'PASSENGER',
                    validate: (v) => (!['NAVIGATOR', 'PASSENGER', 'OBSERVER'].includes(v.trim().toUpperCase()) ? '无效角色' : null),
                });
                if (roleIn === null)
                    return;
                const role = roleIn.trim().toUpperCase();
                const maxUsesIn = await modalPrompt({
                    title: '最大使用次数',
                    label: '1 - 10000',
                    initial: '5',
                    validate: (v) => {
                        const n = parseInt(v.trim(), 10);
                        return !Number.isFinite(n) || n < 1 || n > 10000 ? '无效 max_uses' : null;
                    },
                });
                if (maxUsesIn === null)
                    return;
                const maxUses = parseInt(maxUsesIn.trim(), 10);
                const ttlHoursIn = await modalPrompt({
                    title: '有效时长（小时）',
                    label: '1 - 8760',
                    initial: '168',
                    validate: (v) => {
                        const n = parseFloat(v.trim());
                        return !Number.isFinite(n) || n < 1 || n > 8760 ? '无效 TTL' : null;
                    },
                });
                if (ttlHoursIn === null)
                    return;
                const ttlHours = parseFloat(ttlHoursIn.trim());
                const inv = await api.createInvite(active.id, role, maxUses, Math.round(ttlHours * 3600));
                const link = `${window.location.origin}/?invite=${encodeURIComponent(inv.token)}`;
                const copied = await copyText(link);
                await modalAlert(`邀请已创建\n\nToken: ${inv.token}\n链接: ${link}\n\n${copied ? '（已复制链接到剪贴板）' : '（剪贴板复制失败，请手动复制）'}`);
            }
            else if (cmd.toUpperCase().startsWith('R:')) {
                const prefix = cmd.slice(2).trim();
                const target = invites.find(i => i.id.startsWith(prefix));
                if (!target) {
                    setErr('未找到匹配邀请');
                    return;
                }
                if (!(await modalConfirm(`撤销 ${target.id.slice(0, 8)}？`, { danger: true })))
                    return;
                await api.revokeInvite(target.id);
                await modalAlert('已撤销');
            }
        }
        catch (e) {
            setErr(e.message);
        }
    }
    // URL 中带 ?invite=... 时自动尝试接受。
    useEffect(() => {
        const token = new URLSearchParams(window.location.search).get('invite');
        if (!token)
            return;
        (async () => {
            try {
                const prev = await api.previewInvite(token);
                if (!prev.alive) {
                    setErr(`邀请已失效（${prev.used_count}/${prev.max_uses}）`);
                    return;
                }
                if (!(await modalConfirm(`加入湖 "${prev.lake_name}" 作为 ${prev.role}？`)))
                    return;
                const r = await api.acceptInvite(token);
                window.history.replaceState({}, '', window.location.pathname);
                await refresh();
                // refresh 已把 lakes 刷新并可能自动选中首个；这里不手动设置 active，避免 stale closure。
                void r;
            }
            catch (e) {
                setErr(e.message);
            }
        })();
        // eslint-disable-next-line react-hooks/exhaustive-deps
    }, []);
    async function createLake() {
        if (!newLakeName.trim())
            return;
        setBusy(true);
        setErr(null);
        try {
            const created = await api.createLake(newLakeName.trim(), '', false, currentSpaceId || undefined);
            const lake = await waitLakeReady(created);
            setNewLakeName('');
            setLakes(prev => [lake, ...prev.filter(x => x.id !== lake.id)]);
            setActive(lake);
        }
        catch (e) {
            setErr(e.message);
        }
        finally {
            setBusy(false);
        }
    }
    async function generate() {
        if (!active || !prompt.trim())
            return;
        setBusy(true);
        setErr(null);
        try {
            const t = await api.generateCloud(active.id, prompt.trim(), n, 'TEXT');
            setTasks([t, ...tasks]);
            setPrompt('');
            void poll(t.id);
        }
        catch (e) {
            setErr(e.message);
        }
        finally {
            setBusy(false);
        }
    }
    // M3-T4：SSE 流式预览，把 AI 回复实时增量渲染到面板。
    function startWeaveStream() {
        if (!active || !prompt.trim() || streaming)
            return;
        streamAbortRef.current?.();
        setStreamText('');
        setStreaming(true);
        setErr(null);
        const stop = api.streamWeave(active.id, prompt.trim(), (ev, data) => {
            if (ev === 'delta') {
                setStreamText(prev => prev + (data?.text ?? ''));
            }
            else if (ev === 'done') {
                setStreaming(false);
            }
            else if (ev === 'error') {
                setErr(`流式错误：${data?.message ?? 'unknown'}`);
                setStreaming(false);
            }
        });
        streamAbortRef.current = stop;
    }
    function stopWeaveStream() {
        streamAbortRef.current?.();
        streamAbortRef.current = null;
        setStreaming(false);
    }
    async function poll(taskId) {
        for (let i = 0; i < 30; i++) {
            await new Promise(r => setTimeout(r, 1500));
            try {
                const t = await api.getCloud(taskId);
                setTasks(prev => prev.map(x => x.id === taskId ? t : x));
                if (t.status === 'done' || t.status === 'failed') {
                    if (active)
                        await loadNodes(active.id);
                    return;
                }
            }
            catch { /* ignore */ }
        }
    }
    async function condense(nodeId) {
        try {
            await api.condenseNode(nodeId);
            if (active)
                await loadNodes(active.id);
        }
        catch (e) {
            setErr(e.message);
        }
    }
    async function evaporate(nodeId) {
        try {
            await api.evaporateNode(nodeId);
            if (active)
                await loadNodes(active.id);
        }
        catch (e) {
            setErr(e.message);
        }
    }
    // M3-S2：切换节点凝结选中
    function toggleCrystalSel(nodeId) {
        setCrystalSel(prev => {
            const next = new Set(prev);
            if (next.has(nodeId))
                next.delete(nodeId);
            else
                next.add(nodeId);
            return next;
        });
    }
    // 执行凝结
    async function doCrystallize() {
        if (!active)
            return;
        const ids = Array.from(crystalSel);
        if (ids.length < 2) {
            void modalAlert('至少选择 2 个节点');
            return;
        }
        if (ids.length > 20) {
            void modalAlert('最多选择 20 个节点');
            return;
        }
        const hint = await modalPrompt({ title: '凝结', label: '标题提示（可留空，由 AI 生成）：', initial: '' });
        if (hint === null)
            return;
        try {
            const p = await api.crystallize(active.id, ids, hint || '');
            setRecentPerma(p);
            setCrystalSel(new Set());
        }
        catch (e) {
            setErr(e.message);
        }
    }
    // P13-D：导出湖内容
    async function exportLakeUI(format) {
        if (!active)
            return;
        setExportBusy(true);
        try {
            await api.exportLake(active.id, format);
        }
        catch (e) {
            setErr(e.message);
        }
        finally {
            setExportBusy(false);
        }
    }
    // P18-A：加载关联推荐
    async function loadRelated(nodeId) {
        setRelatedLoading(nodeId);
        try {
            const r = await api.getRelatedNodes(nodeId, 5);
            setRelatedPanel({ nodeId, results: r.related });
        }
        catch (e) {
            setErr(e.message);
        }
        finally {
            setRelatedLoading(null);
        }
    }
    // P18-C：加载模板列表
    async function loadTemplates() {
        setTemplatesBusy(true);
        try {
            const r = await api.listTemplates();
            setTemplates(r.templates);
        }
        catch (e) {
            setErr(e.message);
        }
        finally {
            setTemplatesBusy(false);
        }
    }
    // P18-C：从模板创建节点
    async function createFromTemplate(templateId) {
        if (!active)
            return;
        try {
            await api.createNodeFromTemplate(active.id, templateId);
            setTemplateModalOpen(false);
            void loadNodes(active.id);
        }
        catch (e) {
            setErr(e.message);
        }
    }
    // P18-D：加载快照列表
    async function loadSnapshots(lakeId) {
        try {
            const r = await api.listSnapshots(lakeId);
            setSnapshots(r.snapshots);
        }
        catch { /* 静默 */ }
    }
    // P18-D：保存快照
    async function saveSnapshot() {
        if (!active)
            return;
        const name = await modalPrompt({
            title: '保存图谱快照',
            label: '快照名称（便于识别）',
            validate: v => (!v.trim() ? '名称不能为空' : null),
        });
        if (name === null)
            return;
        setSnapshotBusy(true);
        try {
            const layout = {};
            for (const n of nodes)
                layout[n.id] = { x: n.position.x, y: n.position.y };
            await api.createSnapshot(active.id, name.trim(), layout);
            void loadSnapshots(active.id);
        }
        catch (e) {
            setErr(e.message);
        }
        finally {
            setSnapshotBusy(false);
        }
    }
    // P18-D：恢复快照布局
    function restoreSnapshot(snap) {
        setGraphLayout(snap.layout);
        setSnapshotPanelOpen(false);
    }
    // P18-B：加载节点分享
    async function loadShares(nodeId) {
        setShareLoading(nodeId);
        try {
            const r = await api.listNodeShares(nodeId);
            setShareModal({ nodeId, shares: r.shares });
        }
        catch (e) {
            setErr(e.message);
        }
        finally {
            setShareLoading(null);
        }
    }
    // P18-B：创建分享链接
    async function createShare(nodeId) {
        try {
            const ttlIn = await modalPrompt({
                title: '创建分享链接',
                label: '有效小时数（留空 = 永久）',
                placeholder: '例如：24',
            });
            if (ttlIn === null)
                return;
            const ttl = ttlIn.trim() ? parseInt(ttlIn.trim(), 10) : undefined;
            const share = await api.createNodeShare(nodeId, ttl);
            const publicURL = toPublicShareURL(share);
            const ok = await copyText(publicURL);
            await modalAlert(`分享链接已创建\n\n${publicURL}\n\n${ok ? '（已复制到剪贴板）' : '（请手动复制链接）'}`);
            void loadShares(nodeId);
        }
        catch (e) {
            setErr(e.message);
        }
    }
    // P13-E：导入
    async function importLakeUI(file) {
        if (!active)
            return;
        setImportBusy(true);
        setImportResult(null);
        try {
            const r = await api.importLake(active.id, file);
            setImportResult(r);
            void loadNodes(active.id);
        }
        catch (e) {
            setErr(e.message);
        }
        finally {
            setImportBusy(false);
        }
    }
    // P14-C：批量操作
    async function batchOperate(action) {
        if (!active || batchSel.size === 0)
            return;
        setBatchBusy(true);
        try {
            const ids = Array.from(batchSel);
            await api.batchOperateNodes(active.id, action, ids);
            setBatchSel(new Set());
            void loadNodes(active.id);
        }
        catch (e) {
            setErr(e.message);
        }
        finally {
            setBatchBusy(false);
        }
    }
    // M3 T7：移动湖到其他空间
    async function moveLakeUI(lake) {
        try {
            const r = await api.listSpaces();
            const opts = ['（个人湖 / 移除归属）', ...(r.spaces ?? []).map(s => `${s.name} [${s.id.slice(0, 8)}]`)];
            const ids = ['', ...(r.spaces ?? []).map(s => s.id)];
            const idx = await modalPrompt({
                title: '移动湖',
                label: `当前：${lake.space_id ? lake.space_id.slice(0, 8) : '个人湖'}\n输入序号选择目标：\n${opts.map((o, i) => `  ${i}. ${o}`).join('\n')}`,
                initial: '0',
                validate: v => {
                    const n = parseInt(v.trim(), 10);
                    return Number.isInteger(n) && n >= 0 && n < ids.length ? null : '请输入合法序号';
                },
            });
            if (idx === null)
                return;
            const target = ids[parseInt(idx, 10)];
            if (target === (lake.space_id || ''))
                return;
            await api.moveLake(lake.id, target);
            await refresh();
        }
        catch (e) {
            setErr(e.message);
        }
    }
    // O(E) 构建节点出入度；避免在 node.map 内每次 O(E) filter。
    const { outDeg, inDeg, nodeContentById } = useMemo(() => {
        const outDeg = new Map();
        const inDeg = new Map();
        for (const e of edges) {
            outDeg.set(e.src_node_id, (outDeg.get(e.src_node_id) ?? 0) + 1);
            inDeg.set(e.dst_node_id, (inDeg.get(e.dst_node_id) ?? 0) + 1);
        }
        const nodeContentById = new Map();
        for (const n of nodes)
            nodeContentById.set(n.id, n.content);
        return { outDeg, inDeg, nodeContentById };
    }, [edges, nodes]);
    // P13-C：点击标签时服务端查询（带 AbortController 防竞态）
    const applyTagFilter = useCallback((tag) => {
        setTagFilter(tag);
        if (!tag || !active) {
            setTagFilteredIds(null);
            return;
        }
        tagAbortRef.current?.abort();
        const ctrl = new AbortController();
        tagAbortRef.current = ctrl;
        setTagLoading(true);
        api.listNodesByTag(active.id, tag)
            .then(r => {
            if (ctrl.signal.aborted)
                return;
            setTagFilteredIds(new Set(r.node_ids));
        })
            .catch(() => { if (!ctrl.signal.aborted)
            setTagFilteredIds(new Set()); })
            .finally(() => { if (!ctrl.signal.aborted)
            setTagLoading(false); });
    }, [active]);
    // P13-C：按标签过滤节点
    const filteredNodes = useMemo(() => {
        if (!tagFilter || tagFilteredIds === null)
            return nodes;
        return nodes.filter(n => tagFilteredIds.has(n.id));
    }, [nodes, tagFilter, tagFilteredIds]);
    // P13-C：懒加载节点标签（节点列表变化时批量获取）
    useEffect(() => {
        if (!active || nodes.length === 0)
            return;
        const unloaded = nodes.filter(n => nodeTags[n.id] === undefined).map(n => n.id);
        if (unloaded.length === 0)
            return;
        Promise.all(unloaded.map(id => api.getNodeTags(id).then(r => ({ id, tags: r.tags })).catch(() => ({ id, tags: [] }))))
            .then(results => {
            setNodeTags(prev => {
                const next = { ...prev };
                for (const { id, tags } of results)
                    next[id] = tags;
                return next;
            });
        });
    }, [nodes, active]); // eslint-disable-line react-hooks/exhaustive-deps
    return (_jsxs("div", { style: layout, children: [_jsx(OfflineBar, {}), _jsxs("aside", { style: sidebar, children: [_jsx(SpaceSwitcher, { currentSpaceId: currentSpaceId, onChange: setCurrentSpaceId, onManageMembers: setMembersDrawer }), _jsxs("div", { style: { display: 'flex', justifyContent: 'space-between', alignItems: 'center' }, children: [_jsxs("strong", { style: { letterSpacing: 3 }, children: ["\u9752\u840D \u00B7 \u6211\u7684\u6E56", _jsx("span", { title: wsOnline ? '实时连接已建立' : '实时离线', style: {
                                            display: 'inline-block', width: 8, height: 8, borderRadius: '50%',
                                            marginLeft: 8, background: wsOnline ? '#7fdbb6' : '#777',
                                            boxShadow: wsOnline ? '0 0 6px #7fdbb6' : 'none',
                                        } })] }), _jsx("button", { onClick: onLogout, style: ghostBtn, children: "\u9000\u51FA" }), active && (_jsx("button", { onClick: () => setSearchOpen(true), title: "\u641C\u7D22\u8282\u70B9 (Cmd+K)", style: ghostBtn, children: "\uD83D\uDD0D" })), active && (_jsx("button", { onClick: () => setImportOpen(true), title: "\u6279\u91CF\u5BFC\u5165\u8282\u70B9", style: ghostBtn, children: "\uD83D\uDCE5" })), _jsx("button", { onClick: () => setMainTab(t => t === 'settings' ? 'lakes' : 'settings'), style: { ...ghostBtn, color: mainTab === 'settings' ? '#89b4fa' : undefined }, children: "\u2699" }), _jsx("button", { onClick: () => setOrgOpen(o => !o), title: "\u7EC4\u7EC7\u7BA1\u7406", style: { ...ghostBtn, color: orgOpen ? '#89b4fa' : undefined }, children: "\uD83C\uDFE2" }), _jsx(NotificationBell, {})] }), _jsxs("div", { style: { display: 'flex', gap: 6, marginTop: 16 }, children: [_jsx("input", { value: newLakeName, onChange: e => setNewLakeName(e.target.value), placeholder: "\u65B0\u6E56\u540D\u2026", style: inputSmall }), _jsx("button", { onClick: createLake, disabled: busy, style: primaryBtnSmall, children: "+" })] }), _jsx("ul", { style: { listStyle: 'none', padding: 0, margin: '16px 0 0' }, children: lakes.map(l => (_jsxs("li", { onClick: () => setActive(l), style: {
                                ...lakeItem,
                                background: active?.id === l.id ? 'rgba(74,144,226,0.25)' : 'transparent',
                            }, children: [_jsxs("div", { style: { display: 'flex', justifyContent: 'space-between', alignItems: 'center' }, children: [_jsx("div", { children: l.name }), active?.id === l.id && l.role === 'OWNER' && (_jsxs("div", { style: { display: 'flex', gap: 4 }, children: [_jsx("button", { onClick: e => { e.stopPropagation(); void moveLakeUI(l); }, style: { ...miniBtn, padding: '2px 6px', fontSize: 10 }, children: "\u79FB" }), _jsx("button", { onClick: e => { e.stopPropagation(); void manageInvites(); }, style: { ...miniBtn, padding: '2px 6px', fontSize: 10 }, children: "\u9080\u8BF7" })] }))] }), _jsx("div", { style: { fontSize: 10, opacity: 0.5 }, children: l.role })] }, l.id))) })] }), _jsx("main", { style: main, children: mainTab === 'settings' ? (_jsx(React.Suspense, { fallback: _jsx("div", { style: { padding: 16, color: '#6c7086' }, children: "\u52A0\u8F7D\u4E2D\u2026" }), children: _jsxs("div", { style: { display: 'flex', gap: 24, flexWrap: 'wrap' }, children: [_jsx(AdminOverviewPanel, {}), _jsx(PlatformAdminManager, {}), _jsx(APIKeyManager, {}), _jsx(GraylistManager, {}), _jsx(AuditLogViewer, {})] }) })) : (_jsxs(_Fragment, { children: [!active && _jsx("div", { style: { opacity: 0.5 }, children: "\u9009\u62E9\u4E00\u4E2A\u6E56\uFF0C\u6216\u65B0\u5EFA\u4E00\u4E2A" }), active && (_jsxs(_Fragment, { children: [_jsx("h2", { style: { margin: '0 0 8px', fontWeight: 300 }, children: active.name }), _jsx("div", { style: { opacity: 0.5, marginBottom: 12, fontSize: 12 }, children: active.description || '未命名湖区 · ' + active.id.slice(0, 8) }), _jsxs("div", { style: { display: 'flex', gap: 6, marginBottom: 16, flexWrap: 'wrap', alignItems: 'center' }, children: [_jsx("span", { style: { fontSize: 11, opacity: 0.5 }, children: "\u5BFC\u51FA\uFF1A" }), _jsx("button", { onClick: () => void exportLakeUI('json'), disabled: exportBusy, style: miniBtn, children: exportBusy ? '…' : 'JSON' }), _jsx("button", { onClick: () => void exportLakeUI('markdown'), disabled: exportBusy, style: miniBtn, children: exportBusy ? '…' : 'Markdown' }), _jsx("span", { style: { fontSize: 11, opacity: 0.5, marginLeft: 8 }, children: "\u5BFC\u5165\uFF1A" }), _jsx("button", { onClick: () => importInputRef.current?.click(), disabled: importBusy, style: miniBtn, children: importBusy ? '…' : '📂 文件' }), _jsx("input", { ref: importInputRef, type: "file", accept: ".json,.md,.markdown", style: { display: 'none' }, onChange: e => {
                                                const f = e.target.files?.[0];
                                                if (f)
                                                    void importLakeUI(f);
                                                e.target.value = '';
                                            } }), importResult && (_jsxs("span", { style: { fontSize: 11, color: '#a6e3a1' }, children: ["\u2713 \u5BFC\u5165 ", importResult.imported, " \u6761", importResult.skipped > 0 ? `，跳过 ${importResult.skipped}` : ''] })), lakeTags.length > 0 && (_jsxs(_Fragment, { children: [_jsx("span", { style: { fontSize: 11, opacity: 0.5, marginLeft: 8 }, children: "\u6807\u7B7E\u7B5B\u9009\uFF1A" }), tagFilter && (_jsx("button", { onClick: () => applyTagFilter(''), style: { ...miniBtn, color: '#89dceb' }, children: tagLoading ? '…' : `✕ ${tagFilter}` })), lakeTags.filter(t => t !== tagFilter).map(t => (_jsx("button", { onClick: () => applyTagFilter(t), style: miniBtn, children: t }, t)))] }))] }), onlineUsers.length > 0 && (_jsxs("div", { style: { display: 'flex', alignItems: 'center', gap: 6, marginBottom: 16, fontSize: 11, opacity: 0.8 }, children: [_jsxs("span", { children: ["\u5728\u7EBF ", onlineUsers.length, "\uFF1A"] }), onlineUsers.slice(0, 8).map(uid => (_jsx("span", { title: uid, style: presenceDot, children: uid.slice(0, 2).toUpperCase() }, uid))), onlineUsers.length > 8 && _jsxs("span", { children: ["+", onlineUsers.length - 8] })] })), _jsxs("section", { style: card, children: [_jsx("strong", { style: { letterSpacing: 2, fontSize: 13 }, children: "\u9020\u4E91 \u00B7 AI \u53D1\u6563" }), _jsx("textarea", { value: prompt, onChange: e => setPrompt(e.target.value), placeholder: "\u4F8B\u5982\uFF1A\u7ED9\u4E00\u6B3E\u51A5\u60F3 App \u8D77 5 \u4E2A\u540D\u5B57", rows: 3, style: textarea }), _jsxs("div", { style: { display: 'flex', gap: 12, alignItems: 'center' }, children: [_jsx("label", { style: { fontSize: 12, opacity: 0.7 }, children: "\u5019\u9009\u6570" }), _jsx("input", { type: "number", min: 1, max: 10, value: n, onChange: e => setN(Number(e.target.value)), style: { ...inputSmall, width: 60 } }), _jsx("button", { onClick: generate, disabled: busy || !prompt.trim(), style: primaryBtn, children: busy ? '...' : '造云' }), !streaming ? (_jsx("button", { onClick: startWeaveStream, disabled: !prompt.trim(), style: miniBtn, title: "SSE \u6D41\u5F0F\u9884\u89C8\uFF08\u4E0D\u843D\u76D8\uFF09", children: "\u2728 \u6D41\u5F0F\u9884\u89C8" })) : (_jsx("button", { onClick: stopWeaveStream, style: { ...miniBtn, color: '#d24343' }, children: "\u505C\u6B62" }))] }), (streaming || streamText) && (_jsxs("div", { style: {
                                                marginTop: 10, padding: 10, borderRadius: 6,
                                                background: '#0e1218', border: '1px solid #1d2433',
                                                fontSize: 13, lineHeight: 1.6, whiteSpace: 'pre-wrap',
                                                maxHeight: 220, overflow: 'auto',
                                            }, children: [streamText, streaming && _jsx("span", { style: { opacity: 0.5 }, children: "\u258D" }), !streaming && streamText && (_jsxs("div", { style: { marginTop: 8, display: 'flex', gap: 8 }, children: [_jsx("button", { style: miniBtn, onClick: () => { void navigator.clipboard.writeText(streamText); }, children: "\u590D\u5236" }), _jsx("button", { style: miniBtn, onClick: () => setStreamText(''), children: "\u6E05\u7A7A" })] }))] }))] }), tasks.length > 0 && (_jsxs("section", { style: card, children: [_jsx("strong", { style: { letterSpacing: 2, fontSize: 13 }, children: "\u6700\u8FD1\u7684\u4E91" }), tasks.slice(0, 5).map(t => (_jsxs("div", { style: taskRow, children: [_jsx("span", { style: { ...statusPill, background: statusColor(t.status) }, children: t.status }), _jsx("span", { style: { flex: 1, opacity: 0.85, fontSize: 13 }, children: t.prompt }), _jsxs("span", { style: { opacity: 0.5, fontSize: 11 }, children: [t.result_node_ids?.length ?? 0, "/", t.n] })] }, t.id)))] })), _jsxs("section", { style: card, children: [_jsxs("div", { style: { display: 'flex', justifyContent: 'space-between', alignItems: 'center' }, children: [_jsxs("div", { style: { display: 'flex', alignItems: 'center', gap: 12 }, children: [_jsxs("strong", { style: { letterSpacing: 2, fontSize: 13 }, children: ["\u6E56\u4E2D\u8282\u70B9 (", nodes.length, ")"] }), _jsx("div", { style: { display: 'flex', gap: 4 }, children: ['list', 'graph'].map(mode => (_jsx("button", { onClick: () => setViewMode(mode), style: {
                                                                    ...miniBtn,
                                                                    background: viewMode === mode ? 'rgba(74,144,226,0.35)' : undefined,
                                                                    color: viewMode === mode ? '#9ec5ee' : undefined,
                                                                    padding: '2px 10px',
                                                                }, children: mode === 'list' ? '列表' : '图谱' }, mode))) }), _jsx("button", { onClick: () => { void loadTemplates(); setTemplateModalOpen(true); }, style: { ...miniBtn, color: '#cba6f7' }, title: "\u4ECE\u6A21\u677F\u521B\u5EFA\u8282\u70B9", children: "\uD83D\uDCCB \u6A21\u677F" })] }), crystalSel.size > 0 && (_jsxs("div", { style: { display: 'flex', gap: 6, alignItems: 'center' }, children: [_jsxs("span", { style: { fontSize: 11, color: '#9ec5ee' }, children: ["\u5DF2\u9009 ", crystalSel.size] }), _jsx("button", { onClick: doCrystallize, style: { ...miniBtn, background: '#4a8eff', color: '#fff' }, children: "\u2744 \u51DD\u7ED3\u6240\u9009" }), _jsx("button", { onClick: () => setCrystalSel(new Set()), style: miniBtn, children: "\u6E05\u7A7A" })] }))] }), viewMode === 'graph' ? (_jsxs("div", { style: { marginTop: 12 }, children: [_jsx("div", { style: { marginBottom: 8 }, children: _jsx("input", { value: nodeSearch, onChange: e => setNodeSearch(e.target.value), placeholder: "\u641C\u7D22\u8282\u70B9\u9AD8\u4EAE\u2026", style: { width: '100%', padding: '4px 8px', fontSize: 12, background: '#0d1e2e', border: '1px solid #1e3a5a', borderRadius: 4, color: '#c8dff0', boxSizing: 'border-box' } }) }), _jsxs("div", { style: { display: 'flex', gap: 6, marginBottom: 8, alignItems: 'center', flexWrap: 'wrap' }, children: [_jsx("button", { onClick: () => void saveSnapshot(), disabled: snapshotBusy, style: { ...miniBtn, color: '#a6e3a1' }, title: "\u4FDD\u5B58\u5F53\u524D\u56FE\u8C31\u5E03\u5C40\u4E3A\u5FEB\u7167", children: snapshotBusy ? '…' : '📷 保存快照' }), _jsxs("button", { onClick: () => setSnapshotPanelOpen(o => !o), style: { ...miniBtn, color: snapshotPanelOpen ? '#89b4fa' : undefined }, title: "\u67E5\u770B\u56FE\u8C31\u5FEB\u7167", children: ["\uD83D\uDDC2 \u5FEB\u7167 (", snapshots.length, ")"] }), graphLayout && (_jsx("button", { onClick: () => setGraphLayout(undefined), style: { ...miniBtn, color: '#f38ba8' }, children: "\u2715 \u6E05\u9664\u5FEB\u7167\u5E03\u5C40" }))] }), snapshotPanelOpen && (_jsxs("div", { style: {
                                                        marginBottom: 10, padding: 10, background: '#0a1929',
                                                        border: '1px solid #1e3a5a', borderRadius: 6,
                                                    }, children: [snapshots.length === 0 && _jsx("div", { style: { fontSize: 12, opacity: 0.5 }, children: "\u6682\u65E0\u5FEB\u7167" }), snapshots.map(snap => (_jsxs("div", { style: { display: 'flex', alignItems: 'center', gap: 8, marginBottom: 6, fontSize: 12 }, children: [_jsx("span", { style: { flex: 1 }, children: snap.name }), _jsx("span", { style: { opacity: 0.5 }, children: new Date(snap.created_at).toLocaleString('zh-CN') }), _jsx("button", { onClick: () => restoreSnapshot(snap), style: { ...miniBtn, color: '#89b4fa' }, children: "\u6062\u590D" }), _jsx("button", { onClick: async () => {
                                                                        if (!active)
                                                                            return;
                                                                        if (!(await modalConfirm(`删除快照「${snap.name}」？`, { danger: true })))
                                                                            return;
                                                                        await api.deleteSnapshot(active.id, snap.id).catch(e => setErr(e.message));
                                                                        void loadSnapshots(active.id);
                                                                    }, style: { ...miniBtn, color: '#f38ba8' }, children: "\u5220" })] }, snap.id)))] })), _jsx(React.Suspense, { fallback: _jsx("div", { style: { height: 480, display: 'flex', alignItems: 'center', justifyContent: 'center', color: '#4a6a8e', fontSize: 13 }, children: "\u52A0\u8F7D\u56FE\u8C31\u4E2D\u2026" }), children: _jsx(LakeGraph, { nodes: nodes, edges: edges, searchQuery: nodeSearch, snapshotLayout: graphLayout }) })] })) : (_jsxs(_Fragment, { children: [linkSrc && (_jsxs("div", { style: { fontSize: 12, opacity: 0.8, marginTop: 6, color: '#9ec5ee' }, children: ["\u8FDE\u7EBF\u6A21\u5F0F\uFF1A\u5DF2\u9009\u8D77\u70B9 ", linkSrc.slice(0, 8), "\u2026\uFF0C\u70B9\u51FB\u53E6\u4E00\u8282\u70B9\u5B8C\u6210\u3002\u518D\u6B21\u70B9\u540C\u4E00\u8282\u70B9\u53D6\u6D88\u3002"] })), nodes.length === 0 && _jsx("div", { style: { opacity: 0.4, fontSize: 12 }, children: "\u6B64\u5904\u98CE\u5E73\u6D6A\u9759" }), batchSel.size > 0 && (_jsxs("div", { style: { display: 'flex', gap: 8, alignItems: 'center', marginBottom: 8, padding: '6px 10px', background: 'rgba(74,144,226,0.1)', borderRadius: 6 }, children: [_jsxs("span", { style: { fontSize: 12, color: '#9ec5ee' }, children: ["\u5DF2\u9009 ", batchSel.size, " \u4E2A\u8282\u70B9"] }), _jsx("button", { onClick: () => setBatchSel(new Set(filteredNodes.map(n => n.id))), style: { ...miniBtn, opacity: 0.7 }, children: "\u5168\u9009" }), _jsx("button", { onClick: () => void batchOperate('condense'), disabled: batchBusy, style: miniBtn, children: "\u51DD\u9732 \u2193" }), _jsx("button", { onClick: () => void batchOperate('evaporate'), disabled: batchBusy, style: miniBtn, children: "\u84B8\u53D1 \u2191" }), _jsx("button", { onClick: () => { if (window.confirm(`确认彻底删除已选 ${batchSel.size} 个节点？此操作不可恢复。`)) {
                                                                void batchOperate('erase');
                                                            } }, disabled: batchBusy, style: { ...miniBtn, background: 'rgba(220,53,69,0.15)', color: '#ff6b7a' }, children: "\u5220\u9664 \u2715" }), _jsx("button", { onClick: () => setBatchSel(new Set()), style: { ...miniBtn, opacity: 0.6 }, children: "\u53D6\u6D88\u9009\u62E9" })] })), nodes.length > 0 && filteredNodes.length === 0 && (_jsxs("div", { style: { opacity: 0.4, fontSize: 12 }, children: ["\u6CA1\u6709\u5E26\u300C", tagFilter, "\u300D\u6807\u7B7E\u7684\u8282\u70B9"] })), _jsx("div", { style: { display: 'grid', gridTemplateColumns: 'repeat(auto-fill, minmax(220px, 1fr))', gap: 8 }, children: filteredNodes.map(n => {
                                                        const out = outDeg.get(n.id) ?? 0;
                                                        const inc = inDeg.get(n.id) ?? 0;
                                                        const isLinkSrc = linkSrc === n.id;
                                                        const canCrystal = n.state === 'DROP' || n.state === 'FROZEN';
                                                        const isSelected = crystalSel.has(n.id);
                                                        const isBatchSel = batchSel.has(n.id);
                                                        const tags = nodeTags[n.id] ?? [];
                                                        return (_jsxs("div", { style: {
                                                                ...nodeCard,
                                                                opacity: n.state === 'VAPOR' ? 0.4 : 1,
                                                                boxShadow: isLinkSrc
                                                                    ? '0 0 0 2px #9ec5ee'
                                                                    : isSelected ? '0 0 0 2px #4a8eff'
                                                                        : isBatchSel ? '0 0 0 2px #f9e2af' : undefined,
                                                            }, children: [_jsxs("div", { style: { display: 'flex', justifyContent: 'space-between', alignItems: 'center' }, children: [_jsx("span", { style: { ...statePill, background: stateColor(n.state) }, children: n.state }), _jsxs("span", { style: { fontSize: 10, opacity: 0.6, display: 'flex', gap: 6, alignItems: 'center' }, children: ["\u2192", out, " \u2190", inc, _jsx("input", { type: "checkbox", checked: isBatchSel, onChange: e => setBatchSel(prev => {
                                                                                        const next = new Set(prev);
                                                                                        if (e.target.checked)
                                                                                            next.add(n.id);
                                                                                        else
                                                                                            next.delete(n.id);
                                                                                        return next;
                                                                                    }), title: "\u9009\u5165\u6279\u91CF\u64CD\u4F5C", style: { cursor: 'pointer' } })] })] }), _jsx("div", { style: { marginTop: 8, fontSize: 13, lineHeight: 1.5 }, children: n.content }), aiSummary[n.id] && (_jsxs("div", { style: {
                                                                        marginTop: 6, padding: '4px 8px',
                                                                        background: 'rgba(74,144,226,0.08)', borderLeft: '2px solid #4a8eff',
                                                                        borderRadius: 3, fontSize: 11, color: '#9ec5ee', lineHeight: 1.5,
                                                                    }, children: ["\u2726 ", aiSummary[n.id]] })), _jsx(NodeTagEditor, { nodeId: n.id, tags: tags, onChanged: newTags => {
                                                                        setNodeTags(prev => ({ ...prev, [n.id]: newTags }));
                                                                        // 刷新湖标签
                                                                        if (active)
                                                                            api.getLakeTags(active.id).then(r => setLakeTags(r.tags)).catch(() => undefined);
                                                                    } }), _jsxs("div", { style: { marginTop: 8, display: 'flex', gap: 6, flexWrap: 'wrap' }, children: [n.state === 'MIST' && (_jsx("button", { onClick: () => condense(n.id), style: miniBtn, children: "\u51DD\u9732 \u2193" })), (n.state === 'DROP' || n.state === 'FROZEN') && (_jsx("button", { onClick: () => evaporate(n.id), style: miniBtn, children: "\u84B8\u53D1 \u2191" })), _jsx("button", { onClick: () => handleNodeClickForLink(n.id), style: miniBtn, title: isLinkSrc ? '取消连线' : '连线（先选起点再选终点）', children: isLinkSrc ? '✕' : '🔗' }), _jsx("button", { onClick: () => editNodeContent(n), style: miniBtn, title: "\u7F16\u8F91\u5185\u5BB9", children: "\u270E" }), _jsx("button", { onClick: () => showHistory(n), style: miniBtn, title: "\u5386\u53F2\u7248\u672C", children: "\u27F2" }), _jsx("button", { onClick: () => void showDiff(n), style: miniBtn, title: "\u7248\u672C\u5BF9\u6BD4 diff", children: "\u21C4" }), _jsx("button", { onClick: () => {
                                                                                const fmt = window.confirm('确认导出格式？\n确定 = Markdown，取消 = JSON') ? 'md' : 'json';
                                                                                exportNode(n, fmt);
                                                                            }, style: miniBtn, title: "\u5BFC\u51FA\u8282\u70B9", children: "\u2B07" }), _jsx("button", { onClick: () => void requestAiSummary(n), disabled: aiSummaryBusy.has(n.id), style: miniBtn, title: "AI \u6458\u8981", children: aiSummaryBusy.has(n.id) ? '…' : '✦' }), _jsx("button", { onClick: () => void loadRelated(n.id), disabled: relatedLoading === n.id, style: { ...miniBtn, color: '#89dceb' }, title: "\u67E5\u627E\u5173\u8054\u8282\u70B9", children: relatedLoading === n.id ? '…' : '⚡关联' }), _jsx("button", { onClick: () => void loadShares(n.id), disabled: shareLoading === n.id, style: { ...miniBtn, color: '#f9e2af' }, title: "\u5206\u4EAB\u8282\u70B9", children: shareLoading === n.id ? '…' : '🔗分享' }), canCrystal && (_jsx("button", { onClick: () => toggleCrystalSel(n.id), style: { ...miniBtn, background: isSelected ? '#4a8eff' : undefined, color: isSelected ? '#fff' : undefined }, title: "\u9009\u5165\u51DD\u7ED3\u96C6\u5408", children: "\u2744" }))] })] }, n.id));
                                                    }) })] }))] }), recentPerma && (_jsxs("section", { style: { ...card, borderLeft: '3px solid #4a8eff' }, children: [_jsxs("div", { style: { display: 'flex', justifyContent: 'space-between', alignItems: 'center' }, children: [_jsx("strong", { style: { letterSpacing: 2, fontSize: 13, color: '#9ec5ee' }, children: "\u2744 \u51DD\u7ED3\u7ED3\u679C" }), _jsx("button", { onClick: () => setRecentPerma(null), style: miniBtn, children: "\u5173" })] }), _jsx("div", { style: { marginTop: 8, fontSize: 14, fontWeight: 600 }, children: recentPerma.title }), _jsx("div", { style: { marginTop: 6, fontSize: 13, lineHeight: 1.6, opacity: 0.9 }, children: recentPerma.summary }), _jsxs("div", { style: { marginTop: 8, fontSize: 11, opacity: 0.6 }, children: ["\u6765\u6E90 ", recentPerma.source_node_ids.length, " \u4E2A\u8282\u70B9", recentPerma.llm_provider && ` · ${recentPerma.llm_provider}`, recentPerma.llm_cost_tokens ? ` · ${recentPerma.llm_cost_tokens} tokens` : ''] })] })), _jsxs("section", { style: card, children: [_jsx("div", { style: { marginBottom: 6, fontSize: 12, opacity: 0.7 }, children: "\uD83D\uDCCE \u9644\u4EF6\uFF08M4-B \u672C\u5730 FS\uFF09" }), _jsx(AttachmentBar, {})] }), active && (_jsx("section", { style: card, children: _jsx(CollabDemo, { lakeId: active.id, token: localStorage.getItem('ripple.token') ?? '' }) })), active?.role === 'OWNER' && (_jsx("section", { style: card, children: _jsx(React.Suspense, { fallback: _jsx("div", { style: { color: '#6c7086', fontSize: 12 }, children: "Loading members..." }), children: _jsx(LakeMemberManager, { lakeId: active.id, currentUserId: active.owner_id, currentRole: "OWNER" }) }) })), recos.length > 0 && (_jsxs("section", { style: card, children: [_jsxs("div", { style: { display: 'flex', justifyContent: 'space-between', alignItems: 'center' }, children: [_jsx("strong", { style: { letterSpacing: 2, fontSize: 13, color: '#9ec5ee' }, children: "\u2728 \u4F60\u53EF\u80FD\u611F\u5174\u8DA3\u7684\u51DD\u7ED3" }), _jsx("span", { style: { fontSize: 11, opacity: 0.5 }, children: "\u57FA\u4E8E\u5386\u53F2 LIKE \u53CD\u9988" })] }), _jsx("div", { style: { marginTop: 8, display: 'flex', flexDirection: 'column', gap: 6 }, children: recos.map(r => (_jsxs("div", { style: {
                                                    display: 'flex', alignItems: 'center', gap: 8,
                                                    padding: '6px 8px', borderRadius: 4, background: '#0e1218',
                                                    fontSize: 12,
                                                }, children: [_jsxs("span", { style: { flex: 1, fontFamily: 'monospace', opacity: 0.85 }, children: [r.target_id.slice(0, 8), "\u2026"] }), _jsxs("span", { style: { opacity: 0.6 }, children: ["score ", r.score.toFixed(2)] }), _jsx("button", { style: miniBtn, onClick: () => {
                                                            void api.sendFeedback('perma_node', r.target_id, 'LIKE')
                                                                .then(() => setRecos(prev => prev.filter(x => x.target_id !== r.target_id)))
                                                                .catch(e => modalAlert(`反馈失败：${e.message}`));
                                                        }, children: "\uD83D\uDC4D" }), _jsx("button", { style: miniBtn, onClick: () => {
                                                            void api.sendFeedback('perma_node', r.target_id, 'DISMISS')
                                                                .then(() => setRecos(prev => prev.filter(x => x.target_id !== r.target_id)))
                                                                .catch(() => { });
                                                        }, children: "\u2715" })] }, r.target_id))) })] })), edges.length > 0 && (_jsxs("section", { style: card, children: [_jsxs("strong", { style: { letterSpacing: 2, fontSize: 13 }, children: ["\u8FB9 (", edges.length, ")"] }), _jsx("div", { style: { display: 'flex', flexDirection: 'column', gap: 4, marginTop: 8 }, children: edges.map(e => {
                                                const src = nodeContentById.get(e.src_node_id);
                                                const dst = nodeContentById.get(e.dst_node_id);
                                                return (_jsxs("div", { style: edgeRow, children: [_jsxs("span", { style: { ...edgeKindPill }, children: [e.kind, e.label ? `: ${e.label}` : ''] }), _jsxs("span", { style: { flex: 1, fontSize: 12, opacity: 0.85 }, children: [(src ?? e.src_node_id.slice(0, 8)).slice(0, 24), ' → ', (dst ?? e.dst_node_id.slice(0, 8)).slice(0, 24)] }), _jsx("button", { onClick: () => deleteEdge(e.id), style: miniBtn, children: "\u5220" })] }, e.id));
                                            }) })] }))] })), err && _jsx("div", { style: errBanner, children: err })] })) }), membersDrawer && (_jsx(SpaceMembersDrawer, { space: membersDrawer, onClose: () => setMembersDrawer(null) })), searchOpen && active && (_jsx(React.Suspense, { fallback: null, children: _jsx(SearchModal, { lakeId: active.id, lakeName: active.name, onClose: () => setSearchOpen(false) }) })), importOpen && active && (_jsx(React.Suspense, { fallback: null, children: _jsx(ImportModal, { lakeId: active.id, lakeName: active.name, onClose: () => setImportOpen(false), onImported: () => api.listNodes(active.id).then(r => setNodes(r.nodes)) }) })), orgOpen && (_jsx("div", { style: {
                    position: 'fixed', top: 60, right: 20, zIndex: 300,
                }, children: _jsx(React.Suspense, { fallback: null, children: _jsx(OrgPanel, { currentUserId: meId, onClose: () => setOrgOpen(false) }) }) })), diffModal && (_jsx(NodeDiffViewer, { nodeId: diffModal.nodeId, revisions: diffModal.revisions, onClose: () => setDiffModal(null) })), historyModal && (_jsx(NodeVersionHistory, { node: historyModal.node, revisions: historyModal.revisions, onClose: () => setHistoryModal(null), onRolledBack: updatedNode => {
                    setNodes(prev => prev.map(n => n.id === updatedNode.id ? updatedNode : n));
                    setHistoryModal(null);
                } })), relatedPanel && (_jsx("div", { style: modalOverlay, onClick: () => setRelatedPanel(null), children: _jsxs("div", { style: { ...modalBox, minWidth: 360 }, onClick: e => e.stopPropagation(), children: [_jsxs("div", { style: { display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 12 }, children: [_jsx("strong", { style: { color: '#89dceb' }, children: "\u26A1 \u5173\u8054\u8282\u70B9\u63A8\u8350" }), _jsx("button", { onClick: () => setRelatedPanel(null), style: miniBtn, children: "\u2715" })] }), relatedPanel.results.length === 0 ? (_jsx("div", { style: { fontSize: 13, opacity: 0.5 }, children: "\u6682\u65E0\u5173\u8054\u8282\u70B9" })) : relatedPanel.results.map(r => (_jsxs("div", { style: {
                                padding: '8px 10px', marginBottom: 6, background: 'rgba(137,220,235,0.06)',
                                border: '1px solid rgba(137,220,235,0.2)', borderRadius: 6,
                            }, children: [_jsxs("div", { style: { fontSize: 12, opacity: 0.7, marginBottom: 4 }, children: ["id: ", r.node_id.slice(0, 8), "\u2026 \u00B7 score: ", r.score.toFixed(3)] }), _jsxs("div", { style: { fontSize: 13, lineHeight: 1.5 }, children: [r.snippet.slice(0, 120), r.snippet.length > 120 ? '…' : ''] })] }, r.node_id)))] }) })), shareModal && (_jsx("div", { style: modalOverlay, onClick: () => setShareModal(null), children: _jsxs("div", { style: { ...modalBox, minWidth: 420 }, onClick: e => e.stopPropagation(), children: [_jsxs("div", { style: { display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 12 }, children: [_jsx("strong", { style: { color: '#f9e2af' }, children: "\uD83D\uDD17 \u8282\u70B9\u5206\u4EAB" }), _jsx("button", { onClick: () => setShareModal(null), style: miniBtn, children: "\u2715" })] }), _jsx("button", { onClick: () => void createShare(shareModal.nodeId), style: { ...miniBtn, marginBottom: 10, color: '#a6e3a1' }, children: "+ \u521B\u5EFA\u65B0\u5206\u4EAB\u94FE\u63A5" }), shareModal.shares.length === 0 ? (_jsx("div", { style: { fontSize: 13, opacity: 0.5 }, children: "\u6682\u65E0\u5206\u4EAB\u94FE\u63A5" })) : shareModal.shares.map(s => (_jsxs("div", { style: {
                                padding: '8px 10px', marginBottom: 6,
                                background: s.revoked ? 'rgba(255,255,255,0.03)' : 'rgba(249,226,175,0.06)',
                                border: `1px solid ${s.revoked ? '#333' : 'rgba(249,226,175,0.25)'}`,
                                borderRadius: 6, opacity: s.revoked ? 0.5 : 1,
                            }, children: [_jsx("div", { style: { fontSize: 11, wordBreak: 'break-all', marginBottom: 4, color: '#89b4fa' }, children: toPublicShareURL(s) }), _jsxs("div", { style: { fontSize: 11, opacity: 0.6, display: 'flex', gap: 12, alignItems: 'center' }, children: [_jsx("span", { children: s.revoked ? '已撤销' : s.expires_at ? `到期：${new Date(s.expires_at).toLocaleString('zh-CN')}` : '永久有效' }), !s.revoked && (_jsxs(_Fragment, { children: [_jsx("button", { onClick: () => void copyText(toPublicShareURL(s)).then(ok => { if (ok)
                                                        void modalAlert('已复制！'); }), style: miniBtn, children: "\u590D\u5236" }), _jsx("button", { onClick: async () => {
                                                        if (!(await modalConfirm('撤销此分享链接？', { danger: true })))
                                                            return;
                                                        await api.revokeNodeShare(s.id).catch(e => setErr(e.message));
                                                        void loadShares(shareModal.nodeId);
                                                    }, style: { ...miniBtn, color: '#f38ba8' }, children: "\u64A4\u9500" })] }))] })] }, s.id)))] }) })), templateModalOpen && (_jsx("div", { style: modalOverlay, onClick: () => { setTemplateModalOpen(false); setTplCreateOpen(false); setTplPreviewId(null); }, children: _jsxs("div", { style: { ...modalBox, minWidth: 480, maxHeight: '80vh', display: 'flex', flexDirection: 'column' }, onClick: e => e.stopPropagation(), children: [_jsxs("div", { style: { display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 12, flexShrink: 0 }, children: [_jsx("strong", { style: { color: '#cba6f7' }, children: "\uD83D\uDCCB \u8282\u70B9\u6A21\u677F\u5E93" }), _jsxs("div", { style: { display: 'flex', gap: 6 }, children: [_jsx("button", { onClick: () => { setTplCreateOpen(v => !v); setTplPreviewId(null); }, style: { ...miniBtn, color: tplCreateOpen ? '#cba6f7' : '#89b4fa' }, children: tplCreateOpen ? '取消新建' : '+ 新建模板' }), _jsx("button", { onClick: () => { setTemplateModalOpen(false); setTplCreateOpen(false); setTplPreviewId(null); }, style: miniBtn, children: "\u2715" })] })] }), tplCreateOpen && (_jsxs("div", { style: { padding: '12px 14px', background: 'rgba(203,166,247,0.06)', border: '1px solid rgba(203,166,247,0.25)', borderRadius: 8, marginBottom: 12, flexShrink: 0 }, children: [_jsx("div", { style: { fontSize: 12, color: '#cba6f7', marginBottom: 8, fontWeight: 600 }, children: "\u65B0\u5EFA\u81EA\u5B9A\u4E49\u6A21\u677F" }), _jsx("input", { placeholder: "\u6A21\u677F\u540D\u79F0\uFF08\u5FC5\u586B\uFF09", value: tplForm.name, onChange: e => setTplForm(f => ({ ...f, name: e.target.value })), style: { ...tplInput, marginBottom: 6 } }), _jsx("textarea", { placeholder: "\u6A21\u677F\u5185\u5BB9\uFF08\u5FC5\u586B\uFF09", value: tplForm.content, onChange: e => setTplForm(f => ({ ...f, content: e.target.value })), rows: 4, style: { ...tplInput, resize: 'vertical' } }), _jsx("input", { placeholder: "\u63CF\u8FF0\uFF08\u9009\u586B\uFF09", value: tplForm.description, onChange: e => setTplForm(f => ({ ...f, description: e.target.value })), style: { ...tplInput, marginTop: 6 } }), _jsx("input", { placeholder: "\u6807\u7B7E\uFF0C\u9017\u53F7\u5206\u9694\uFF08\u9009\u586B\uFF09", value: tplForm.tags, onChange: e => setTplForm(f => ({ ...f, tags: e.target.value })), style: { ...tplInput, marginTop: 6 } }), tplForm.content.trim() && (_jsxs("div", { style: { marginTop: 8 }, children: [_jsx("div", { style: { fontSize: 11, opacity: 0.7, marginBottom: 4 }, children: "\u5185\u5BB9\u9884\u89C8" }), _jsx("div", { style: {
                                                marginTop: 4, fontSize: 12, lineHeight: 1.6,
                                                background: 'rgba(0,0,0,0.25)', borderRadius: 4, padding: '8px 10px',
                                                whiteSpace: 'pre-wrap', maxHeight: 160, overflowY: 'auto',
                                                color: '#cdd6f4', wordBreak: 'break-word',
                                            }, children: tplForm.content })] })), _jsx("button", { disabled: tplCreateBusy || !tplForm.name.trim() || !tplForm.content.trim(), onClick: async () => {
                                        setTplCreateBusy(true);
                                        try {
                                            const tags = tplForm.tags.split(',').map(s => s.trim()).filter(Boolean);
                                            await api.createTemplate(tplForm.name.trim(), tplForm.content.trim(), tplForm.description.trim() || undefined, tags.length ? tags : undefined);
                                            setTplForm({ name: '', content: '', description: '', tags: '' });
                                            setTplCreateOpen(false);
                                            await api.listTemplates().then(r => setTemplates(r.templates));
                                        }
                                        catch (e) {
                                            void modalAlert(e.message, { title: '创建失败' });
                                        }
                                        finally {
                                            setTplCreateBusy(false);
                                        }
                                    }, style: { ...miniBtn, marginTop: 8, color: '#cba6f7' }, children: tplCreateBusy ? '保存中…' : '保存模板' })] })), _jsx("div", { style: { overflowY: 'auto', flex: 1 }, children: templatesBusy ? (_jsx("div", { style: { fontSize: 13, opacity: 0.5 }, children: "\u52A0\u8F7D\u4E2D\u2026" })) : templates.length === 0 ? (_jsx("div", { style: { fontSize: 13, opacity: 0.5 }, children: "\u6682\u65E0\u6A21\u677F\uFF0C\u70B9\u51FB\u300C+ \u65B0\u5EFA\u6A21\u677F\u300D\u521B\u5EFA" })) : templates.map(t => (_jsxs("div", { style: {
                                    padding: '10px 12px', marginBottom: 8,
                                    background: 'rgba(203,166,247,0.06)',
                                    border: `1px solid ${tplPreviewId === t.id ? 'rgba(203,166,247,0.5)' : 'rgba(203,166,247,0.2)'}`,
                                    borderRadius: 6,
                                }, children: [_jsxs("div", { style: { display: 'flex', justifyContent: 'space-between', alignItems: 'center' }, children: [_jsx("strong", { style: { fontSize: 13, cursor: 'pointer', flex: 1 }, onClick: () => setTplPreviewId(tplPreviewId === t.id ? null : t.id), children: t.name }), _jsxs("div", { style: { display: 'flex', gap: 4, alignItems: 'center' }, children: [t.is_system && _jsx("span", { style: { fontSize: 10, color: '#cba6f7', opacity: 0.7 }, children: "\u7CFB\u7EDF" }), _jsx("button", { onClick: () => setTplPreviewId(tplPreviewId === t.id ? null : t.id), style: { ...miniBtn, color: '#89b4fa', fontSize: 11, padding: '2px 6px' }, children: tplPreviewId === t.id ? '收起' : '预览' }), _jsx("button", { onClick: () => void createFromTemplate(t.id), style: { ...miniBtn, color: '#a6e3a1', fontSize: 11, padding: '2px 8px' }, children: "\u4F7F\u7528" }), !t.is_system && (_jsx("button", { onClick: async () => {
                                                            if (!await modalConfirm(`确认删除「${t.name}」？`, { title: '删除模板' }))
                                                                return;
                                                            try {
                                                                await api.deleteTemplate(t.id);
                                                                setTemplates(prev => prev.filter(x => x.id !== t.id));
                                                            }
                                                            catch (e) {
                                                                void modalAlert(e.message, { title: '删除失败' });
                                                            }
                                                        }, style: { ...miniBtn, color: '#f38ba8', fontSize: 11, padding: '2px 6px' }, children: "\u5220" }))] })] }), t.description && _jsx("div", { style: { fontSize: 12, opacity: 0.7, marginTop: 4 }, children: t.description }), tplPreviewId === t.id ? (_jsx("div", { style: {
                                            marginTop: 8, fontSize: 12, lineHeight: 1.6,
                                            background: 'rgba(0,0,0,0.25)', borderRadius: 4, padding: '8px 10px',
                                            whiteSpace: 'pre-wrap', maxHeight: 200, overflowY: 'auto',
                                            color: '#cdd6f4', wordBreak: 'break-word',
                                        }, children: t.content })) : (_jsxs("div", { style: { fontSize: 11, opacity: 0.5, marginTop: 4, cursor: 'pointer' }, onClick: () => setTplPreviewId(t.id), children: [t.content.slice(0, 120), t.content.length > 120 ? '… 点击展开' : ''] })), t.tags.length > 0 && (_jsx("div", { style: { marginTop: 6, display: 'flex', gap: 4, flexWrap: 'wrap' }, children: t.tags.map(tag => _jsx("span", { style: tagChip, children: tag }, tag)) }))] }, t.id))) })] }) }))] }));
}
// P18 modal styles
const modalOverlay = {
    position: 'fixed', inset: 0, background: 'rgba(0,0,0,0.6)',
    display: 'flex', alignItems: 'center', justifyContent: 'center',
    zIndex: 500,
};
const modalBox = {
    background: '#111827', border: '1px solid rgba(255,255,255,0.15)',
    borderRadius: 10, padding: 24, maxHeight: '80vh', overflowY: 'auto',
    minWidth: 320, maxWidth: 560,
};
// P13-C：节点标签编辑器组件
function NodeTagEditor({ nodeId, tags, onChanged }) {
    const [editing, setEditing] = useState(false);
    const [draft, setDraft] = useState('');
    const [saving, setSaving] = useState(false);
    function openEditor() {
        setDraft(tags.join(', '));
        setEditing(true);
    }
    async function save() {
        const next = draft.split(',').map(t => t.trim()).filter(Boolean);
        setSaving(true);
        try {
            const res = await api.setNodeTags(nodeId, next);
            onChanged(res.tags);
        }
        catch {
            // ignore
        }
        finally {
            setSaving(false);
            setEditing(false);
        }
    }
    if (editing) {
        return (_jsxs("div", { style: { marginTop: 6, display: 'flex', gap: 4, flexWrap: 'wrap', alignItems: 'center' }, children: [_jsx("input", { autoFocus: true, value: draft, onChange: e => setDraft(e.target.value), onKeyDown: e => { if (e.key === 'Enter')
                        void save(); if (e.key === 'Escape')
                        setEditing(false); }, placeholder: "\u9017\u53F7\u5206\u9694\u6807\u7B7E", style: { flex: 1, minWidth: 100, fontSize: 11, padding: '2px 6px', background: '#2a2d3a', border: '1px solid #555', borderRadius: 4, color: '#cdd6f4' } }), _jsx("button", { onClick: () => void save(), disabled: saving, style: tagChipBtn, children: saving ? '…' : '✓' }), _jsx("button", { onClick: () => setEditing(false), style: tagChipBtn, children: "\u2715" })] }));
    }
    return (_jsxs("div", { style: { marginTop: 6, display: 'flex', gap: 4, flexWrap: 'wrap', alignItems: 'center' }, children: [tags.map(t => (_jsx("span", { style: tagChip, children: t }, t))), _jsx("button", { onClick: openEditor, style: tagChipBtn, title: "\u7F16\u8F91\u6807\u7B7E", children: "+\u6807\u7B7E" })] }));
}
const tagChip = {
    display: 'inline-block', fontSize: 10, padding: '1px 7px',
    background: 'rgba(137,220,235,0.15)', color: '#89dceb',
    borderRadius: 10, border: '1px solid rgba(137,220,235,0.3)',
};
const tagChipBtn = {
    fontSize: 10, padding: '1px 6px', cursor: 'pointer',
    background: 'rgba(255,255,255,0.05)', color: '#cdd6f4',
    border: '1px solid #555', borderRadius: 10,
};
const tplInput = {
    width: '100%', background: '#0a1929', border: '1px solid rgba(203,166,247,0.3)',
    borderRadius: 4, color: '#cdd6f4', padding: '6px 10px', fontSize: 12,
    fontFamily: 'inherit', boxSizing: 'border-box',
};
function statusColor(s) {
    return { queued: '#888', running: '#4a90e2', done: '#52c41a', failed: '#ff4d4f' }[s] ?? '#888';
}
function stateColor(s) {
    return { MIST: '#9ec5ee', DROP: '#52c41a', FROZEN: '#9bb', VAPOR: '#777', ERASED: '#444', GHOST: '#444' }[s] ?? '#888';
}
function toPublicShareURL(share) {
    return `${window.location.origin}/share/${encodeURIComponent(share.token)}`;
}
const layout = {
    display: 'flex', height: '100vh', width: '100vw',
    background: '#0a1929', color: '#e0f0ff',
    fontFamily: 'system-ui, -apple-system, sans-serif',
};
const sidebar = {
    width: 260, padding: 20, borderRight: '1px solid rgba(255,255,255,0.1)',
    overflowY: 'auto',
};
const main = {
    flex: 1, padding: 32, overflowY: 'auto',
};
const card = {
    background: 'rgba(255,255,255,0.04)', padding: 16,
    borderRadius: 8, marginTop: 16,
    border: '1px solid rgba(255,255,255,0.08)',
    display: 'flex', flexDirection: 'column', gap: 10,
};
const lakeItem = {
    padding: '8px 12px', borderRadius: 6, marginBottom: 4,
    cursor: 'pointer', fontSize: 14,
};
const inputSmall = {
    padding: '6px 10px', background: 'rgba(255,255,255,0.08)',
    border: '1px solid rgba(255,255,255,0.15)', borderRadius: 4,
    color: '#fff', fontSize: 13, outline: 'none', flex: 1,
};
const textarea = {
    background: 'rgba(255,255,255,0.06)',
    border: '1px solid rgba(255,255,255,0.15)', borderRadius: 4,
    color: '#fff', padding: 8, fontSize: 13,
    fontFamily: 'inherit', resize: 'vertical',
};
const primaryBtn = {
    padding: '8px 20px', background: '#4a90e2',
    border: 'none', borderRadius: 4, color: 'white',
    fontSize: 13, cursor: 'pointer',
};
const primaryBtnSmall = { ...primaryBtn, padding: '6px 12px' };
const ghostBtn = {
    background: 'none', border: '1px solid rgba(255,255,255,0.2)',
    color: 'rgba(255,255,255,0.6)', borderRadius: 4,
    padding: '4px 10px', fontSize: 11, cursor: 'pointer',
};
const taskRow = {
    display: 'flex', alignItems: 'center', gap: 8, padding: '6px 0',
};
const statusPill = {
    fontSize: 10, padding: '2px 8px', borderRadius: 10,
    letterSpacing: 1, color: 'white',
};
const statePill = {
    fontSize: 9, padding: '1px 6px', borderRadius: 6,
    color: '#000', fontWeight: 600, letterSpacing: 1,
};
const nodeCard = {
    background: 'rgba(255,255,255,0.04)', padding: 12,
    borderRadius: 6, border: '1px solid rgba(255,255,255,0.08)',
};
const miniBtn = {
    background: 'rgba(255,255,255,0.08)',
    border: '1px solid rgba(255,255,255,0.15)',
    color: '#cde', padding: '3px 10px', borderRadius: 3,
    fontSize: 11, cursor: 'pointer',
};
const edgeRow = {
    display: 'flex', gap: 8, alignItems: 'center',
    padding: '4px 8px', background: 'rgba(255,255,255,0.03)',
    borderRadius: 4, border: '1px solid rgba(255,255,255,0.06)',
};
const edgeKindPill = {
    fontSize: 10, padding: '2px 8px', borderRadius: 10,
    background: 'rgba(158,197,238,0.18)', color: '#9ec5ee',
    letterSpacing: 1, minWidth: 60, textAlign: 'center',
};
const presenceDot = {
    display: 'inline-flex', alignItems: 'center', justifyContent: 'center',
    width: 22, height: 22, borderRadius: '50%',
    background: 'rgba(127,219,182,0.22)',
    border: '1px solid rgba(127,219,182,0.5)',
    color: '#7fdbb6', fontSize: 9, letterSpacing: 0,
};
const errBanner = {
    position: 'fixed', bottom: 16, right: 16,
    padding: 12, background: 'rgba(255,80,80,0.2)',
    border: '1px solid rgba(255,80,80,0.4)',
    borderRadius: 4, color: '#ffb0b0', fontSize: 13,
    maxWidth: 400,
};
