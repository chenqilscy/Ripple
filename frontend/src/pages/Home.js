import { jsx as _jsx, jsxs as _jsxs, Fragment as _Fragment } from "react/jsx-runtime";
import React, { useEffect, useMemo, useRef, useState } from 'react';
import { api } from '../api/client';
const LakeGraph = React.lazy(() => import('../components/LakeGraph'));
import { prompt as modalPrompt, confirm as modalConfirm, alert as modalAlert } from '../components/Modal';
import SpaceSwitcher from '../components/SpaceSwitcher';
import SpaceMembersDrawer from '../components/SpaceMembersDrawer';
import AttachmentBar from '../components/AttachmentBar';
import CollabDemo from '../components/CollabDemo';
import { LakeWS } from '../api/wsClient';
const EDGE_KINDS = ['relates', 'derives', 'opposes', 'refines', 'groups', 'custom'];
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
    // M3-S3：推荐位（基于历史 LIKE 反馈的协同过滤）
    const [recos, setRecos] = useState([]);
    const wsRef = useRef(null);
    useEffect(() => { void refresh(); }, [currentSpaceId]);
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
            if (revisions.length === 0) {
                await modalAlert('暂无历史');
                return;
            }
            const lines = revisions.map(r => `rev ${r.rev_number} | ${new Date(r.created_at).toLocaleString()} | ${r.edit_reason || '(无说明)'}\n  ${r.content.slice(0, 80)}`).join('\n\n');
            const input = await modalPrompt({
                title: `${node.id.slice(0, 8)} 历史`,
                label: `输入 rev 号回滚（取消即放弃）：\n\n${lines}`,
                validate: (v) => {
                    const n = parseInt(v.trim(), 10);
                    return !Number.isFinite(n) || n <= 0 ? '无效 rev 号' : null;
                },
            });
            if (input === null)
                return;
            const target = parseInt(input.trim(), 10);
            if (!(await modalConfirm(`回滚到 rev ${target}？`, { danger: true })))
                return;
            await api.rollbackNode(node.id, target);
            if (active)
                await loadNodes(active.id);
        }
        catch (e) {
            setErr(e.message);
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
            const lake = await api.createLake(newLakeName.trim(), '', false, currentSpaceId || undefined);
            setNewLakeName('');
            setLakes([lake, ...lakes]);
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
    return (_jsxs("div", { style: layout, children: [_jsxs("aside", { style: sidebar, children: [_jsx(SpaceSwitcher, { currentSpaceId: currentSpaceId, onChange: setCurrentSpaceId, onManageMembers: setMembersDrawer }), _jsxs("div", { style: { display: 'flex', justifyContent: 'space-between', alignItems: 'center' }, children: [_jsxs("strong", { style: { letterSpacing: 3 }, children: ["\u9752\u840D \u00B7 \u6211\u7684\u6E56", _jsx("span", { title: wsOnline ? '实时连接已建立' : '实时离线', style: {
                                            display: 'inline-block', width: 8, height: 8, borderRadius: '50%',
                                            marginLeft: 8, background: wsOnline ? '#7fdbb6' : '#777',
                                            boxShadow: wsOnline ? '0 0 6px #7fdbb6' : 'none',
                                        } })] }), _jsx("button", { onClick: onLogout, style: ghostBtn, children: "\u9000\u51FA" })] }), _jsxs("div", { style: { display: 'flex', gap: 6, marginTop: 16 }, children: [_jsx("input", { value: newLakeName, onChange: e => setNewLakeName(e.target.value), placeholder: "\u65B0\u6E56\u540D\u2026", style: inputSmall }), _jsx("button", { onClick: createLake, disabled: busy, style: primaryBtnSmall, children: "+" })] }), _jsx("ul", { style: { listStyle: 'none', padding: 0, margin: '16px 0 0' }, children: lakes.map(l => (_jsxs("li", { onClick: () => setActive(l), style: {
                                ...lakeItem,
                                background: active?.id === l.id ? 'rgba(74,144,226,0.25)' : 'transparent',
                            }, children: [_jsxs("div", { style: { display: 'flex', justifyContent: 'space-between', alignItems: 'center' }, children: [_jsx("div", { children: l.name }), active?.id === l.id && l.role === 'OWNER' && (_jsxs("div", { style: { display: 'flex', gap: 4 }, children: [_jsx("button", { onClick: e => { e.stopPropagation(); void moveLakeUI(l); }, style: { ...miniBtn, padding: '2px 6px', fontSize: 10 }, children: "\u79FB" }), _jsx("button", { onClick: e => { e.stopPropagation(); void manageInvites(); }, style: { ...miniBtn, padding: '2px 6px', fontSize: 10 }, children: "\u9080\u8BF7" })] }))] }), _jsx("div", { style: { fontSize: 10, opacity: 0.5 }, children: l.role })] }, l.id))) })] }), _jsxs("main", { style: main, children: [!active && _jsx("div", { style: { opacity: 0.5 }, children: "\u9009\u62E9\u4E00\u4E2A\u6E56\uFF0C\u6216\u65B0\u5EFA\u4E00\u4E2A" }), active && (_jsxs(_Fragment, { children: [_jsx("h2", { style: { margin: '0 0 8px', fontWeight: 300 }, children: active.name }), _jsx("div", { style: { opacity: 0.5, marginBottom: 24, fontSize: 12 }, children: active.description || '未命名湖区 · ' + active.id.slice(0, 8) }), onlineUsers.length > 0 && (_jsxs("div", { style: { display: 'flex', alignItems: 'center', gap: 6, marginBottom: 16, fontSize: 11, opacity: 0.8 }, children: [_jsxs("span", { children: ["\u5728\u7EBF ", onlineUsers.length, "\uFF1A"] }), onlineUsers.slice(0, 8).map(uid => (_jsx("span", { title: uid, style: presenceDot, children: uid.slice(0, 2).toUpperCase() }, uid))), onlineUsers.length > 8 && _jsxs("span", { children: ["+", onlineUsers.length - 8] })] })), _jsxs("section", { style: card, children: [_jsx("strong", { style: { letterSpacing: 2, fontSize: 13 }, children: "\u9020\u4E91 \u00B7 AI \u53D1\u6563" }), _jsx("textarea", { value: prompt, onChange: e => setPrompt(e.target.value), placeholder: "\u4F8B\u5982\uFF1A\u7ED9\u4E00\u6B3E\u51A5\u60F3 App \u8D77 5 \u4E2A\u540D\u5B57", rows: 3, style: textarea }), _jsxs("div", { style: { display: 'flex', gap: 12, alignItems: 'center' }, children: [_jsx("label", { style: { fontSize: 12, opacity: 0.7 }, children: "\u5019\u9009\u6570" }), _jsx("input", { type: "number", min: 1, max: 10, value: n, onChange: e => setN(Number(e.target.value)), style: { ...inputSmall, width: 60 } }), _jsx("button", { onClick: generate, disabled: busy || !prompt.trim(), style: primaryBtn, children: busy ? '...' : '造云' }), !streaming ? (_jsx("button", { onClick: startWeaveStream, disabled: !prompt.trim(), style: miniBtn, title: "SSE \u6D41\u5F0F\u9884\u89C8\uFF08\u4E0D\u843D\u76D8\uFF09", children: "\u2728 \u6D41\u5F0F\u9884\u89C8" })) : (_jsx("button", { onClick: stopWeaveStream, style: { ...miniBtn, color: '#d24343' }, children: "\u505C\u6B62" }))] }), (streaming || streamText) && (_jsxs("div", { style: {
                                            marginTop: 10, padding: 10, borderRadius: 6,
                                            background: '#0e1218', border: '1px solid #1d2433',
                                            fontSize: 13, lineHeight: 1.6, whiteSpace: 'pre-wrap',
                                            maxHeight: 220, overflow: 'auto',
                                        }, children: [streamText, streaming && _jsx("span", { style: { opacity: 0.5 }, children: "\u258D" }), !streaming && streamText && (_jsxs("div", { style: { marginTop: 8, display: 'flex', gap: 8 }, children: [_jsx("button", { style: miniBtn, onClick: () => { void navigator.clipboard.writeText(streamText); }, children: "\u590D\u5236" }), _jsx("button", { style: miniBtn, onClick: () => setStreamText(''), children: "\u6E05\u7A7A" })] }))] }))] }), tasks.length > 0 && (_jsxs("section", { style: card, children: [_jsx("strong", { style: { letterSpacing: 2, fontSize: 13 }, children: "\u6700\u8FD1\u7684\u4E91" }), tasks.slice(0, 5).map(t => (_jsxs("div", { style: taskRow, children: [_jsx("span", { style: { ...statusPill, background: statusColor(t.status) }, children: t.status }), _jsx("span", { style: { flex: 1, opacity: 0.85, fontSize: 13 }, children: t.prompt }), _jsxs("span", { style: { opacity: 0.5, fontSize: 11 }, children: [t.result_node_ids?.length ?? 0, "/", t.n] })] }, t.id)))] })), _jsxs("section", { style: card, children: [_jsxs("div", { style: { display: 'flex', justifyContent: 'space-between', alignItems: 'center' }, children: [_jsxs("div", { style: { display: 'flex', alignItems: 'center', gap: 12 }, children: [_jsxs("strong", { style: { letterSpacing: 2, fontSize: 13 }, children: ["\u6E56\u4E2D\u8282\u70B9 (", nodes.length, ")"] }), _jsx("div", { style: { display: 'flex', gap: 4 }, children: ['list', 'graph'].map(mode => (_jsx("button", { onClick: () => setViewMode(mode), style: {
                                                                ...miniBtn,
                                                                background: viewMode === mode ? 'rgba(74,144,226,0.35)' : undefined,
                                                                color: viewMode === mode ? '#9ec5ee' : undefined,
                                                                padding: '2px 10px',
                                                            }, children: mode === 'list' ? '列表' : '图谱' }, mode))) })] }), crystalSel.size > 0 && (_jsxs("div", { style: { display: 'flex', gap: 6, alignItems: 'center' }, children: [_jsxs("span", { style: { fontSize: 11, color: '#9ec5ee' }, children: ["\u5DF2\u9009 ", crystalSel.size] }), _jsx("button", { onClick: doCrystallize, style: { ...miniBtn, background: '#4a8eff', color: '#fff' }, children: "\u2744 \u51DD\u7ED3\u6240\u9009" }), _jsx("button", { onClick: () => setCrystalSel(new Set()), style: miniBtn, children: "\u6E05\u7A7A" })] }))] }), viewMode === 'graph' ? (_jsx("div", { style: { marginTop: 12 }, children: _jsx(React.Suspense, { fallback: _jsx("div", { style: { height: 480, display: 'flex', alignItems: 'center', justifyContent: 'center', color: '#4a6a8e', fontSize: 13 }, children: "\u52A0\u8F7D\u56FE\u8C31\u4E2D\u2026" }), children: _jsx(LakeGraph, { nodes: nodes, edges: edges }) }) })) : (_jsxs(_Fragment, { children: [linkSrc && (_jsxs("div", { style: { fontSize: 12, opacity: 0.8, marginTop: 6, color: '#9ec5ee' }, children: ["\u8FDE\u7EBF\u6A21\u5F0F\uFF1A\u5DF2\u9009\u8D77\u70B9 ", linkSrc.slice(0, 8), "\u2026\uFF0C\u70B9\u51FB\u53E6\u4E00\u8282\u70B9\u5B8C\u6210\u3002\u518D\u6B21\u70B9\u540C\u4E00\u8282\u70B9\u53D6\u6D88\u3002"] })), nodes.length === 0 && _jsx("div", { style: { opacity: 0.4, fontSize: 12 }, children: "\u6B64\u5904\u98CE\u5E73\u6D6A\u9759" }), _jsx("div", { style: { display: 'grid', gridTemplateColumns: 'repeat(auto-fill, minmax(220px, 1fr))', gap: 8 }, children: nodes.map(n => {
                                                    const out = outDeg.get(n.id) ?? 0;
                                                    const inc = inDeg.get(n.id) ?? 0;
                                                    const isLinkSrc = linkSrc === n.id;
                                                    const canCrystal = n.state === 'DROP' || n.state === 'FROZEN';
                                                    const isSelected = crystalSel.has(n.id);
                                                    return (_jsxs("div", { style: {
                                                            ...nodeCard,
                                                            opacity: n.state === 'VAPOR' ? 0.4 : 1,
                                                            boxShadow: isLinkSrc
                                                                ? '0 0 0 2px #9ec5ee'
                                                                : isSelected ? '0 0 0 2px #4a8eff' : undefined,
                                                        }, children: [_jsxs("div", { style: { display: 'flex', justifyContent: 'space-between' }, children: [_jsx("span", { style: { ...statePill, background: stateColor(n.state) }, children: n.state }), _jsxs("span", { style: { fontSize: 10, opacity: 0.6 }, children: ["\u2192", out, " \u2190", inc] })] }), _jsx("div", { style: { marginTop: 8, fontSize: 13, lineHeight: 1.5 }, children: n.content }), _jsxs("div", { style: { marginTop: 8, display: 'flex', gap: 6, flexWrap: 'wrap' }, children: [n.state === 'MIST' && (_jsx("button", { onClick: () => condense(n.id), style: miniBtn, children: "\u51DD\u9732 \u2193" })), (n.state === 'DROP' || n.state === 'FROZEN') && (_jsx("button", { onClick: () => evaporate(n.id), style: miniBtn, children: "\u84B8\u53D1 \u2191" })), _jsx("button", { onClick: () => handleNodeClickForLink(n.id), style: miniBtn, title: isLinkSrc ? '取消连线' : '连线（先选起点再选终点）', children: isLinkSrc ? '✕' : '🔗' }), _jsx("button", { onClick: () => editNodeContent(n), style: miniBtn, title: "\u7F16\u8F91\u5185\u5BB9", children: "\u270E" }), _jsx("button", { onClick: () => showHistory(n), style: miniBtn, title: "\u5386\u53F2\u7248\u672C", children: "\u27F2" }), canCrystal && (_jsx("button", { onClick: () => toggleCrystalSel(n.id), style: { ...miniBtn, background: isSelected ? '#4a8eff' : undefined, color: isSelected ? '#fff' : undefined }, title: "\u9009\u5165\u51DD\u7ED3\u96C6\u5408", children: "\u2744" }))] })] }, n.id));
                                                }) })] }))] }), recentPerma && (_jsxs("section", { style: { ...card, borderLeft: '3px solid #4a8eff' }, children: [_jsxs("div", { style: { display: 'flex', justifyContent: 'space-between', alignItems: 'center' }, children: [_jsx("strong", { style: { letterSpacing: 2, fontSize: 13, color: '#9ec5ee' }, children: "\u2744 \u51DD\u7ED3\u7ED3\u679C" }), _jsx("button", { onClick: () => setRecentPerma(null), style: miniBtn, children: "\u5173" })] }), _jsx("div", { style: { marginTop: 8, fontSize: 14, fontWeight: 600 }, children: recentPerma.title }), _jsx("div", { style: { marginTop: 6, fontSize: 13, lineHeight: 1.6, opacity: 0.9 }, children: recentPerma.summary }), _jsxs("div", { style: { marginTop: 8, fontSize: 11, opacity: 0.6 }, children: ["\u6765\u6E90 ", recentPerma.source_node_ids.length, " \u4E2A\u8282\u70B9", recentPerma.llm_provider && ` · ${recentPerma.llm_provider}`, recentPerma.llm_cost_tokens ? ` · ${recentPerma.llm_cost_tokens} tokens` : ''] })] })), _jsxs("section", { style: card, children: [_jsx("div", { style: { marginBottom: 6, fontSize: 12, opacity: 0.7 }, children: "\uD83D\uDCCE \u9644\u4EF6\uFF08M4-B \u672C\u5730 FS\uFF09" }), _jsx(AttachmentBar, {})] }), active && (_jsx("section", { style: card, children: _jsx(CollabDemo, { lakeId: active.id, token: localStorage.getItem('ripple.token') ?? '' }) })), recos.length > 0 && (_jsxs("section", { style: card, children: [_jsxs("div", { style: { display: 'flex', justifyContent: 'space-between', alignItems: 'center' }, children: [_jsx("strong", { style: { letterSpacing: 2, fontSize: 13, color: '#9ec5ee' }, children: "\u2728 \u4F60\u53EF\u80FD\u611F\u5174\u8DA3\u7684\u51DD\u7ED3" }), _jsx("span", { style: { fontSize: 11, opacity: 0.5 }, children: "\u57FA\u4E8E\u5386\u53F2 LIKE \u53CD\u9988" })] }), _jsx("div", { style: { marginTop: 8, display: 'flex', flexDirection: 'column', gap: 6 }, children: recos.map(r => (_jsxs("div", { style: {
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
                                        }) })] }))] })), err && _jsx("div", { style: errBanner, children: err })] }), membersDrawer && (_jsx(SpaceMembersDrawer, { space: membersDrawer, onClose: () => setMembersDrawer(null) }))] }));
}
function statusColor(s) {
    return { queued: '#888', running: '#4a90e2', done: '#52c41a', failed: '#ff4d4f' }[s] ?? '#888';
}
function stateColor(s) {
    return { MIST: '#9ec5ee', DROP: '#52c41a', FROZEN: '#9bb', VAPOR: '#777', ERASED: '#444', GHOST: '#444' }[s] ?? '#888';
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
