import React, { useState } from 'react'
import ReactDOM from 'react-dom/client'
import SummarizeGraphModal from '../components/SummarizeGraphModal'

function Harness() {
  const [open, setOpen] = useState(true)
  const [successCount, setSuccessCount] = useState(0)

  return (
    <div style={{ minHeight: '100vh', background: '#08111f', color: '#cdd6f4', padding: 32 }}>
      <h1>Summarize Graph Harness</h1>
      <p data-testid="success-count">success:{successCount}</p>
      {!open && <button onClick={() => setOpen(true)}>重新打开</button>}
      {open && (
        <SummarizeGraphModal
          lakeId="lake-e2e"
          nodeIds={['node-a', 'node-b']}
          onClose={() => setOpen(false)}
          onSuccess={() => setSuccessCount(prev => prev + 1)}
        />
      )}
    </div>
  )
}

ReactDOM.createRoot(document.getElementById('root')!).render(
  <React.StrictMode>
    <Harness />
  </React.StrictMode>,
)
