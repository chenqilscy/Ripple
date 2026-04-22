// CollabDemo · P6-D 协作 demo：通过 y-websocket 接入 yjs-bridge :7790。
//
// 用法：在 Home.tsx 渲染 <CollabDemo lakeId={lake.id} token={token} />
// 协议：双向同步一个 Y.Text，每个客户端实时看到对方输入。
// 鉴权（P7-B）：先调 POST /api/v1/ws_token 换取 ws-only 短期 token（5 分钟），
//               再用该 token 建立 yjs-bridge WS 连接，避免主 token 暴露在 URL 日志。

import { useEffect, useRef, useState } from 'react';
import * as Y from 'yjs';
import { WebsocketProvider } from 'y-websocket';

interface Props {
  lakeId: string;
  token: string;            // 主 JWT，用于换取 ws-only token
  apiBase?: string;         // 后端 API 基地址，默认 http://localhost:8080
  bridgeURL?: string;       // 默认 ws://localhost:7790/yjs
}

export default function CollabDemo({
  lakeId,
  token,
  apiBase = 'http://localhost:8080',
  bridgeURL = 'ws://localhost:7790/yjs',
}: Props) {
  const [text, setText] = useState('');
  const [status, setStatus] = useState<'connecting' | 'connected' | 'disconnected'>('connecting');
  const ydocRef = useRef<Y.Doc | null>(null);
  const providerRef = useRef<WebsocketProvider | null>(null);

  useEffect(() => {
    let cancelled = false;

    // P7-B：先换取 ws-only 短期 token，再建立 WebSocket
    const setupCollab = async () => {
      let wsToken = token; // fallback（bridge 禁用鉴权时）
      try {
        const resp = await fetch(`${apiBase}/api/v1/ws_token`, {
          method: 'POST',
          headers: { Authorization: `Bearer ${token}` },
        });
        if (resp.ok) {
          const data = await resp.json() as { token: string };
          wsToken = data.token;
        }
      } catch {
        // 网络错误时继续使用主 token（spike/local dev 用）
      }

      if (cancelled) return;

      const ydoc = new Y.Doc();
      ydocRef.current = ydoc;
      const url = `${bridgeURL}?token=${encodeURIComponent(wsToken)}`;
      const provider = new WebsocketProvider(url, lakeId, ydoc, {
        protocols: ['y-protocol'],
      });
      providerRef.current = provider;
      provider.on('status', (e: { status: string }) => {
        setStatus(e.status === 'connected' ? 'connected' : e.status === 'connecting' ? 'connecting' : 'disconnected');
      });

      const ytext = ydoc.getText('shared');
      const onChange = () => setText(ytext.toString());
      ytext.observe(onChange);
      onChange();
    };

    setStatus('connecting');
    setupCollab();

    return () => {
      cancelled = true;
      providerRef.current?.destroy();
      ydocRef.current?.destroy();
    };
  }, [lakeId, token, apiBase, bridgeURL]);

  const onInput = (next: string) => {
    const ytext = ydocRef.current?.getText('shared');
    if (!ytext) return;
    ydocRef.current?.transact(() => {
      ytext.delete(0, ytext.length);
      ytext.insert(0, next);
    });
  };

  return (
    <div style={{ border: '1px solid #2a2f3b', borderRadius: 6, padding: 8 }}>
      <div style={{ marginBottom: 6, fontSize: 12, opacity: 0.7 }}>
        🤝 协作 demo（Yjs · {status}）
      </div>
      <textarea
        value={text}
        onChange={(e) => onInput(e.target.value)}
        rows={3}
        placeholder="在不同浏览器/标签页打开同一个湖即可看到实时同步…"
        style={{
          width: '100%',
          background: '#0e1116',
          color: '#e6edf3',
          border: '1px solid #2a2f3b',
          borderRadius: 4,
          padding: 6,
          fontFamily: 'inherit',
        }}
      />
    </div>
  );
}
