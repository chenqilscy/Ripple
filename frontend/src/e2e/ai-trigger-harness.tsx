import React, { useState } from 'react'
import ReactDOM from 'react-dom/client'
import type { EdgeItem, NodeItem } from '../api/types'
import NodeDetailPanel from '../components/NodeDetailPanel'

const node: NodeItem = {
  id: 'node-e2e',
  lake_id: 'lake-e2e',
  owner_id: 'u-e2e',
  content: '原始节点内容',
  type: 'TEXT',
  state: 'DROP',
  position: { x: 0, y: 0, z: 0 },
  created_at: new Date().toISOString(),
  updated_at: new Date().toISOString(),
}

const edges: EdgeItem[] = []

function Harness() {
  const [successCount, setSuccessCount] = useState(0)

  return (
    <div style={{ minHeight: '100vh', background: '#08111f', color: '#cdd6f4', padding: 32 }}>
      <h1>AI Trigger Harness</h1>
      <p data-testid="success-count">success:{successCount}</p>
      <NodeDetailPanel
        node={node}
        allNodes={[node]}
        edges={edges}
        onClose={() => undefined}
        onAiDone={() => setSuccessCount(prev => prev + 1)}
      />
    </div>
  )
}

ReactDOM.createRoot(document.getElementById('root')!).render(
  <React.StrictMode>
    <Harness />
  </React.StrictMode>,
)
