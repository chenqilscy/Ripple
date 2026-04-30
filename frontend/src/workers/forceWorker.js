// frontend/src/workers/forceWorker.js
// Web Worker: 运行 d3-force simulation，发送节点位置更新

import { forceSimulation, forceLink, forceManyBody, forceCenter, forceX, forceY } from 'd3-force'

let simulation = null
let workerNodes = null

self.onmessage = function (e) {
  const { type, data } = e.data

  if (type === 'init') {
    const { nodes, edges } = data

    // 构建 Worker 端节点数据（只含 x, y, id）
    workerNodes = nodes.map(n => ({
      id: n.id,
      x: n.x ?? (Math.random() - 0.5) * 800,
      y: n.y ?? (Math.random() - 0.5) * 600,
    }))
    const workerEdges = edges.map(e => ({ source: e.src_node_id, target: e.dst_node_id }))

    simulation = forceSimulation(workerNodes)
      .force('link', forceLink(workerEdges).id(d => d.id).distance(120).strength(0.5))
      .force('charge', forceManyBody().strength(-300))
      .force('center', forceCenter(0, 0))
      .force('x', forceX().strength(0.05))
      .force('y', forceY().strength(0.05))
      .alphaDecay(0.02)
      .velocityDecay(0.4)

    simulation.on('tick', () => {
      // 只发送位置数据
      const positions = workerNodes.map(n => ({ id: n.id, x: n.x, y: n.y }))
      self.postMessage({ type: 'tick', positions })
    })
  }

  if (type === 'drag') {
    const { nodeId, x, y } = data
    const node = workerNodes.find(n => n.id === nodeId)
    if (node) {
      node.fx = x
      node.fy = y
      if (simulation) simulation.alpha(0.3).restart()
    }
  }

  if (type === 'release') {
    const { nodeId } = data
    const node = workerNodes.find(n => n.id === nodeId)
    if (node) {
      node.fx = null
      node.fy = null
      if (simulation) simulation.alpha(0.3).restart()
    }
  }

  if (type === 'stop') {
    if (simulation) simulation.stop()
  }
}
