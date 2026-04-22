import { jsxs as _jsxs, jsx as _jsx } from "react/jsx-runtime";
// CollabDemo · P6-D 协作 demo：通过 y-websocket 接入 yjs-bridge :7790。
//
// 用法：在 Home.tsx 渲染 <CollabDemo lakeId={lake.id} nodeId={node.id} token={token} />
// 协议：双向同步一个 Y.Text，每个客户端实时看到对方输入。
// 鉴权（P7-B）：先调 POST /api/v1/ws_token 换取 ws-only 短期 token（5 分钟），
//               再用该 token 建立 yjs-bridge WS 连接，避免主 token 暴露在 URL 日志。
// P8-D/E：若提供 nodeId，则：
//   - WS URL 携带 node=<nodeId>（bridge 按节点维度路由房间）
//   - Y.Text 命名为 "node-content"（绑定节点内容）
//   - Y.Doc 变更时防抖写回 PUT /api/v1/nodes/{id}/doc_state（Yjs 快照）
//   - Y.Text 变更时防抖写回 PUT /api/v1/nodes/{id}/content（Neo4j 文本内容）
import { useEffect, useRef, useState } from 'react';
import * as Y from 'yjs';
import { WebsocketProvider } from 'y-websocket';
// P7-F：后端 API 基地址优先读 Vite 环境变量（与 api/client.ts 保持一致）。
const DEFAULT_API_BASE = import.meta.env.VITE_API_BASE ?? 'http://localhost:8000';
const DEFAULT_BRIDGE_URL = import.meta.env.VITE_YJS_BRIDGE_URL ?? 'ws://localhost:7790/yjs';
/** 防抖：延迟 delay 毫秒后执行 fn（返回取消函数）。 */
function debounce(fn, delay) {
    let timer = null;
    const wrapped = (...args) => {
        if (timer !== null)
            clearTimeout(timer);
        timer = setTimeout(() => fn(...args), delay);
    };
    wrapped.cancel = () => { if (timer !== null) {
        clearTimeout(timer);
        timer = null;
    } };
    return wrapped;
}
export default function CollabDemo({ lakeId, nodeId, token, apiBase = DEFAULT_API_BASE, bridgeURL = DEFAULT_BRIDGE_URL, }) {
    const [text, setText] = useState('');
    const [status, setStatus] = useState('connecting');
    const ydocRef = useRef(null);
    const providerRef = useRef(null);
    useEffect(() => {
        let cancelled = false;
        // P8-E：快照写回（防抖 3s），将 Y.Doc 完整状态保存至 REST API。
        const saveSnapshot = nodeId
            ? debounce((doc, wsToken) => {
                if (cancelled)
                    return;
                const state = Y.encodeStateAsUpdate(doc);
                fetch(`${apiBase}/api/v1/nodes/${nodeId}/doc_state`, {
                    method: 'PUT',
                    headers: { Authorization: `Bearer ${wsToken}`, 'Content-Type': 'application/octet-stream' },
                    body: state,
                }).catch((e) => console.warn('doc_state save error:', e));
            }, 3000)
            : null;
        // P8-E：节点文本写回（防抖 2s），将 Y.Text 内容持久化至 Neo4j。
        const saveContent = nodeId
            ? debounce((content, wsToken) => {
                if (cancelled)
                    return;
                fetch(`${apiBase}/api/v1/nodes/${nodeId}/content`, {
                    method: 'PUT',
                    headers: { Authorization: `Bearer ${wsToken}`, 'Content-Type': 'application/json' },
                    body: JSON.stringify({ content }),
                }).catch((e) => console.warn('content save error:', e));
            }, 2000)
            : null;
        // P7-B：先换取 ws-only 短期 token，再建立 WebSocket
        const setupCollab = async () => {
            let wsToken = token; // fallback（bridge 禁用鉴权时）
            try {
                const resp = await fetch(`${apiBase}/api/v1/ws_token`, {
                    method: 'POST',
                    headers: { Authorization: `Bearer ${token}` },
                });
                if (resp.ok) {
                    const data = await resp.json();
                    wsToken = data.token;
                }
            }
            catch {
                // 网络错误时继续使用主 token（spike/local dev 用）
            }
            if (cancelled)
                return;
            const ydoc = new Y.Doc();
            ydocRef.current = ydoc;
            // P8-D/E：使用 y-websocket params 选项传递认证和路由参数。
            // y-websocket 会将 params 拼入 URL querystring，将 roomName 追加为 /<roomName> 路径段。
            // bridge 只读 querystring（lake/node/token），路径段被忽略。
            const roomName = nodeId ?? lakeId;
            const wsParams = { lake: lakeId, token: wsToken };
            if (nodeId)
                wsParams.node = nodeId;
            const provider = new WebsocketProvider(bridgeURL, roomName, ydoc, {
                protocols: ['y-protocol'],
                params: wsParams,
                connect: false,
            });
            providerRef.current = provider;
            provider.on('status', (e) => {
                setStatus(e.status === 'connected' ? 'connected' : e.status === 'connecting' ? 'connecting' : 'disconnected');
            });
            // P8-E：Y.Text 命名与节点绑定
            const ytextName = nodeId ? 'node-content' : 'shared';
            const ytext = ydoc.getText(ytextName);
            const onChange = () => {
                const content = ytext.toString();
                setText(content);
                if (saveSnapshot)
                    saveSnapshot(ydoc, wsToken);
                if (saveContent)
                    saveContent(content, wsToken);
            };
            ytext.observe(onChange);
            onChange();
            provider.connect();
        };
        setStatus('connecting');
        setupCollab();
        return () => {
            cancelled = true;
            saveSnapshot?.cancel();
            saveContent?.cancel();
            providerRef.current?.destroy();
            ydocRef.current?.destroy();
        };
    }, [lakeId, nodeId, token, apiBase, bridgeURL]);
    const onInput = (next) => {
        const ytextName = nodeId ? 'node-content' : 'shared';
        const ytext = ydocRef.current?.getText(ytextName);
        if (!ytext)
            return;
        ydocRef.current?.transact(() => {
            ytext.delete(0, ytext.length);
            ytext.insert(0, next);
        });
    };
    return (_jsxs("div", { style: { border: '1px solid #2a2f3b', borderRadius: 6, padding: 8 }, children: [_jsxs("div", { style: { marginBottom: 6, fontSize: 12, opacity: 0.7 }, children: ["\uD83E\uDD1D \u534F\u4F5C demo\uFF08Yjs \u00B7 ", status, "\uFF09"] }), _jsx("textarea", { value: text, onChange: (e) => onInput(e.target.value), rows: 3, placeholder: "\u5728\u4E0D\u540C\u6D4F\u89C8\u5668/\u6807\u7B7E\u9875\u6253\u5F00\u540C\u4E00\u4E2A\u6E56\u5373\u53EF\u770B\u5230\u5B9E\u65F6\u540C\u6B65\u2026", style: {
                    width: '100%',
                    background: '#0e1116',
                    color: '#e6edf3',
                    border: '1px solid #2a2f3b',
                    borderRadius: 4,
                    padding: 6,
                    fontFamily: 'inherit',
                } })] }));
}
