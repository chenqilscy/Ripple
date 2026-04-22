// CollabDemo · P6-D 协作 demo：通过 y-websocket 接入 yjs-bridge :7790。
//
// 用法：在 Home.tsx 渲染 <CollabDemo lakeId={lake.id} token={token} />
// 协议：双向同步一个 Y.Text，每个客户端实时看到对方输入。
// 鉴权：URL 参数 token=<jwt>（与 yjs-bridge 的 P6-B 对齐）。

import { useEffect, useRef, useState } from 'react';
import * as Y from 'yjs';
import { WebsocketProvider } from 'y-websocket';

interface Props {
  lakeId: string;
  token: string;
  bridgeURL?: string; // 默认 ws://localhost:7790/yjs
}

export default function CollabDemo({ lakeId, token, bridgeURL = 'ws://localhost:7790/yjs' }: Props) {
  const [text, setText] = useState('');
  const [status, setStatus] = useState<'connecting' | 'connected' | 'disconnected'>('connecting');
  const ydocRef = useRef<Y.Doc | null>(null);
  const providerRef = useRef<WebsocketProvider | null>(null);

  useEffect(() => {
    const ydoc = new Y.Doc();
    ydocRef.current = ydoc;
    // y-websocket Provider 用 (serverUrl, roomName) 形式；我们把 lakeID 作为 room、附 token 在 URL 上
    const url = `${bridgeURL}?token=${encodeURIComponent(token)}`;
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

    return () => {
      ytext.unobserve(onChange);
      provider.destroy();
      ydoc.destroy();
    };
  }, [lakeId, token, bridgeURL]);

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
