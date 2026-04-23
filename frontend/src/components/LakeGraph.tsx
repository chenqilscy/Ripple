/**
 * M5-A: Three.js full animation
 * - Spring edges: D3 live simulation driven each frame, edges follow nodes
 * - Particle flow: one particle per edge flows src -> dst
 * - Weave animation: new nodes spring-scale in on first appearance
 *
 * deps: @react-three/fiber@8 + @react-three/drei@9 + three@0.160 + d3-force@3
 */
import React, { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { Canvas, useFrame } from '@react-three/fiber'
import { OrbitControls, Html } from '@react-three/drei'
import * as THREE from 'three'
import {
  forceSimulation,
  forceLink,
  forceManyBody,
  forceCollide,
  forceCenter,
  type Simulation,
  type SimulationNodeDatum,
  type SimulationLinkDatum,
} from 'd3-force'
import type { EdgeItem, NodeItem, NodeState } from '../api/types'

const MAX_NODES = 200

const STATE_COLOR: Record<NodeState, string> = {
  MIST:   '#9ec5ee',
  DROP:   '#52c41a',
  FROZEN: '#4a8eff',
  VAPOR:  '#555566',
  ERASED: '#333344',
  GHOST:  '#333344',
}

interface SimNode extends SimulationNodeDatum {
  id: string
  item: NodeItem
}

interface SimLink extends SimulationLinkDatum<SimNode> {
  edgeId: string
}

// ---------------------------------------------------------------------------
// Particle flow -- one particle per edge
// ---------------------------------------------------------------------------
interface ParticleProps {
  simLinks: SimLink[]
  speed?: number
}

function EdgeParticles({ simLinks, speed = 0.4 }: ParticleProps) {
  const progressRef = useRef<Float32Array>(
    Float32Array.from({ length: simLinks.length }, () => Math.random()),
  )
  const meshRef = useRef<THREE.InstancedMesh>(null)

  // Resize progress buffer when edge count changes
  useEffect(() => {
    const cur = progressRef.current
    const next = new Float32Array(simLinks.length)
    for (let i = 0; i < next.length; i++) {
      next[i] = i < cur.length ? cur[i] : Math.random()
    }
    progressRef.current = next
  }, [simLinks.length])

  const tmpMat = useMemo(() => new THREE.Matrix4(), [])
  const tmpVec = useMemo(() => new THREE.Vector3(), [])

  useFrame((_state, delta) => {
    if (!meshRef.current) return
    const prog = progressRef.current
    for (let i = 0; i < simLinks.length; i++) {
      prog[i] = (prog[i] + delta * speed) % 1.0
      const lk = simLinks[i]
      const src = lk.source as SimNode
      const dst = lk.target as SimNode
      const t = prog[i]
      tmpVec.set(
        (src.x ?? 0) * (1 - t) + (dst.x ?? 0) * t,
        (src.y ?? 0) * (1 - t) + (dst.y ?? 0) * t,
        2,
      )
      tmpMat.setPosition(tmpVec)
      meshRef.current.setMatrixAt(i, tmpMat)
    }
    meshRef.current.instanceMatrix.needsUpdate = true
  })

  if (simLinks.length === 0) return null

  return (
    <instancedMesh ref={meshRef} args={[undefined, undefined, simLinks.length]}>
      <sphereGeometry args={[2.5, 6, 6]} />
      <meshBasicMaterial color="#89dceb" transparent opacity={0.9} />
    </instancedMesh>
  )
}

// ---------------------------------------------------------------------------
// Animated node with weave (spring scale-in) animation
// ---------------------------------------------------------------------------
interface AnimNodeProps {
  node: NodeItem
  position: [number, number, number]
  selected: boolean
  onClick: () => void
  isNew: boolean
  onDragStart: (nodeId: string) => void
  onDragEnd: (nodeId: string, x: number, y: number) => void
  simNode: SimNode
  highlighted?: boolean
}

const STATE_LABEL: Record<NodeState, string> = {
  MIST:   '雾态',
  DROP:   '水滴',
  FROZEN: '冻结',
  VAPOR:  '蒸发',
  ERASED: '已消除',
  GHOST:  '幽灵',
}

/** easeOutBack: slight overshoot then settle at 1.0 (spring weave effect) */
function easeOutBack(x: number): number {
  const c1 = 1.70158
  const c3 = c1 + 1
  return 1 + c3 * Math.pow(x - 1, 3) + c1 * Math.pow(x - 1, 2)
}

function AnimatedNode({ node, position, selected, onClick, isNew, onDragStart, onDragEnd, simNode, highlighted }: AnimNodeProps) {
  const meshRef = useRef<THREE.Mesh>(null)
  const scaleRef = useRef(isNew ? 0 : 1)
  const color = STATE_COLOR[node.state] ?? '#888888'
  const isDraggingRef = useRef(false)
  const dragOffsetRef = useRef({ x: 0, y: 0 })
  const [hovered, setHovered] = useState(false)

  useFrame((_state, delta) => {
    if (!meshRef.current) return
    if (scaleRef.current < 1.0) {
      scaleRef.current = Math.min(1.0, scaleRef.current + delta * 3.5)
      const s = easeOutBack(scaleRef.current)
      meshRef.current.scale.setScalar(Math.max(0, s))
    }
    // Follow live simulation position during non-drag
    if (!isDraggingRef.current) {
      meshRef.current.position.set(simNode.x ?? 0, simNode.y ?? 0, 0)
    }
  })

  return (
    <mesh
      ref={meshRef}
      position={position}
      scale={isNew ? 0 : 1}
      onClick={e => { if (!isDraggingRef.current) { e.stopPropagation(); onClick() } }}
      onPointerDown={e => {
        e.stopPropagation()
        isDraggingRef.current = true
        dragOffsetRef.current = { x: e.point.x - (simNode.x ?? 0), y: e.point.y - (simNode.y ?? 0) }
        onDragStart(node.id)
      }}
      onPointerMove={e => {
        if (!isDraggingRef.current || !meshRef.current) return
        const nx = e.point.x - dragOffsetRef.current.x
        const ny = e.point.y - dragOffsetRef.current.y
        simNode.x = nx; simNode.y = ny
        simNode.fx = nx; simNode.fy = ny
        meshRef.current.position.set(nx, ny, 0)
      }}
      onPointerUp={e => {
        if (!isDraggingRef.current) return
        isDraggingRef.current = false
        const nx = e.point.x - dragOffsetRef.current.x
        const ny = e.point.y - dragOffsetRef.current.y
        onDragEnd(node.id, nx, ny)
      }}
      onPointerEnter={e => { e.stopPropagation(); if (!isDraggingRef.current) setHovered(true) }}
      onPointerLeave={() => setHovered(false)}
    >
      <sphereGeometry args={selected ? [7, 14, 14] : [5, 12, 12]} />
      <meshStandardMaterial
        color={color}
        emissive={selected ? color : (highlighted ? '#ffd700' : (hovered ? color : '#000000'))}
        emissiveIntensity={selected ? 0.6 : (highlighted ? 0.8 : (hovered ? 0.25 : 0))}
        roughness={0.4}
        metalness={selected ? 0.3 : 0.1}
      />
      {/* P17-B: search highlight ring */}
      {highlighted && (
        <mesh scale={1.6}>
          <torusGeometry args={[5, 0.8, 8, 24]} />
          <meshBasicMaterial color="#ffd700" transparent opacity={0.55} />
        </mesh>
      )}
      {/* P16-A: 悬停详情 tooltip */}
      {hovered && !selected && (
        <Html
          position={[0, 14, 0]}
          style={{
            pointerEvents: 'none',
            background: 'rgba(4,10,22,0.95)',
            border: '1px solid rgba(74,144,226,0.5)',
            borderRadius: 6,
            padding: '6px 10px',
            fontSize: 11,
            color: '#c0d8f0',
            minWidth: 140,
            maxWidth: 240,
            lineHeight: '1.6',
          }}
        >
          <div style={{ fontWeight: 600, marginBottom: 3, color: '#9ec5ee' }}>
            {STATE_LABEL[node.state] ?? node.state}
          </div>
          <div style={{ wordBreak: 'break-all', opacity: 0.9 }}>
            {node.content.slice(0, 80)}{node.content.length > 80 ? '…' : ''}
          </div>
          <div style={{ marginTop: 4, opacity: 0.55, fontSize: 10 }}>
            {new Date(node.created_at).toLocaleDateString('zh-CN')}
          </div>
        </Html>
      )}
      {selected && (
        <Html
          position={[0, 12, 0]}
          style={{
            pointerEvents: 'none',
            background: 'rgba(6,13,26,0.9)',
            border: '1px solid rgba(74,144,226,0.4)',
            borderRadius: 4,
            padding: '4px 8px',
            fontSize: 11,
            color: '#c0d8f0',
            whiteSpace: 'nowrap',
            maxWidth: 200,
            overflow: 'hidden',
            textOverflow: 'ellipsis',
          }}
        >
          {node.content.slice(0, 60)}{node.content.length > 60 ? '\u2026' : ''}
        </Html>
      )}
    </mesh>
  )
}

// ---------------------------------------------------------------------------
// Spring edges -- LineSegments driven by live D3 positions each frame
// ---------------------------------------------------------------------------
interface SpringEdgesProps {
  simLinks: SimLink[]
}

function SpringEdges({ simLinks }: SpringEdgesProps) {
  const geoRef = useRef<THREE.BufferGeometry>(null)

  useFrame(() => {
    if (!geoRef.current || simLinks.length === 0) return
    const attr = geoRef.current.attributes['position'] as THREE.BufferAttribute | undefined
    if (!attr) return
    const arr = attr.array as Float32Array
    for (let i = 0; i < simLinks.length; i++) {
      const lk = simLinks[i]
      const src = lk.source as SimNode
      const dst = lk.target as SimNode
      const base = i * 6
      arr[base]     = src.x ?? 0
      arr[base + 1] = src.y ?? 0
      arr[base + 2] = 0
      arr[base + 3] = dst.x ?? 0
      arr[base + 4] = dst.y ?? 0
      arr[base + 5] = 0
    }
    attr.needsUpdate = true
  })

  const initPositions = useMemo(() => {
    const pts = new Float32Array(simLinks.length * 6)
    for (let i = 0; i < simLinks.length; i++) {
      const lk = simLinks[i]
      const src = lk.source as SimNode
      const dst = lk.target as SimNode
      const base = i * 6
      pts[base]     = src.x ?? 0
      pts[base + 1] = src.y ?? 0
      pts[base + 2] = 0
      pts[base + 3] = dst.x ?? 0
      pts[base + 4] = dst.y ?? 0
      pts[base + 5] = 0
    }
    return pts
  }, [simLinks])

  if (simLinks.length === 0) return null

  return (
    <lineSegments>
      <bufferGeometry ref={geoRef}>
        <bufferAttribute
          attach="attributes-position"
          array={initPositions}
          count={simLinks.length * 2}
          itemSize={3}
        />
      </bufferGeometry>
      <lineBasicMaterial color="#2a4a7e" transparent opacity={0.6} />
    </lineSegments>
  )
}

// ---------------------------------------------------------------------------
// SimTicker -- advances the D3 live simulation one tick per frame
// ---------------------------------------------------------------------------
interface SimTickerProps {
  sim: Simulation<SimNode, SimLink>
}

function SimTicker({ sim }: SimTickerProps) {
  useFrame(() => {
    if (sim.alpha() > 0.001) sim.tick()
  })
  return null
}

// ---------------------------------------------------------------------------
// GraphScene
// ---------------------------------------------------------------------------
interface SceneProps {
  displayNodes: NodeItem[]
  displayEdges: EdgeItem[]
  onNodeSelect?: (node: NodeItem | null) => void
  newNodeIds?: Set<string>
  resetToken?: number
  searchQuery?: string
  snapshotLayout?: Record<string, { x: number; y: number }>
}

function GraphScene({ displayNodes, displayEdges, onNodeSelect, newNodeIds, resetToken, searchQuery, snapshotLayout }: SceneProps) {
  const [selectedId, setSelectedId] = useState<string | null>(null)
  const selectedIdRef = useRef(selectedId)
  useEffect(() => { selectedIdRef.current = selectedId }, [selectedId])

  const { sim, simLinks, positions, simNodes } = useMemo(() => {
    const simNodes: SimNode[] = displayNodes.map(n => {
      const forced = snapshotLayout?.[n.id]
      return { id: n.id, item: n, x: forced?.x ?? 0, y: forced?.y ?? 0, fx: forced?.x, fy: forced?.y }
    })
    const idSet = new Set(simNodes.map(n => n.id))

    const simLinks: SimLink[] = displayEdges
      .filter(e => idSet.has(e.src_node_id) && idSet.has(e.dst_node_id))
      .map(e => ({
        edgeId: e.id,
        source: e.src_node_id,
        target: e.dst_node_id,
      }))

    const sim = forceSimulation<SimNode>(simNodes)
      .force(
        'link',
        forceLink<SimNode, SimLink>(simLinks).id(n => n.id).distance(80).strength(0.6),
      )
      .force('charge', forceManyBody<SimNode>().strength(-120))
      .force('collide', forceCollide<SimNode>(22))
      .force('center', forceCenter(0, 0))
      .alphaDecay(0.01)
      .stop()

    // Pre-warm 100 ticks for stable initial layout
    for (let i = 0; i < 100; i++) sim.tick()

    const positions = new Map<string, THREE.Vector3>()
    for (const n of simNodes) {
      positions.set(n.id, new THREE.Vector3(n.x ?? 0, n.y ?? 0, 0))
    }

    // Reset alpha so SimTicker continues spring animation
    sim.alpha(0.3).restart().stop()

    return { sim, simLinks, positions, simNodes }
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [displayNodes, displayEdges, resetToken, snapshotLayout])

  const simNodeMap = useMemo(() => {
    const m = new Map<string, SimNode>()
    for (const n of simNodes) m.set(n.id, n)
    return m
  }, [simNodes])

  const handleNodeClick = useCallback(
    (node: NodeItem) => {
      const nextId = selectedIdRef.current === node.id ? null : node.id
      setSelectedId(nextId)
      onNodeSelect?.(nextId ? node : null)
    },
    [onNodeSelect],
  )

  const handleDragStart = useCallback((nodeId: string) => {
    const sn = simNodeMap.get(nodeId)
    if (sn) { sn.fx = sn.x; sn.fy = sn.y }
    sim.alphaTarget(0.1)
  }, [sim, simNodeMap])

  const handleDragEnd = useCallback((nodeId: string, x: number, y: number) => {
    const sn = simNodeMap.get(nodeId)
    if (sn) { sn.x = x; sn.y = y; sn.fx = x; sn.fy = y }
  }, [simNodeMap])

  const highlightedIds = useMemo(() => {
    if (!searchQuery) return new Set<string>()
    const q = searchQuery.toLowerCase()
    return new Set(displayNodes.filter(n => n.content.toLowerCase().includes(q)).map(n => n.id))
  }, [searchQuery, displayNodes])

  return (
    <>
      <ambientLight intensity={0.5} />
      <pointLight position={[0, 200, 200]} intensity={1.2} />
      <pointLight position={[0, -200, -100]} intensity={0.4} color="#4a8eff" />

      <SimTicker sim={sim} />
      <SpringEdges simLinks={simLinks} />
      <EdgeParticles simLinks={simLinks} />

      {displayNodes.map(node => {
        const pos3 = positions.get(node.id) ?? new THREE.Vector3()
        const sn = simNodeMap.get(node.id)!
        return (
          <AnimatedNode
            key={node.id}
            node={node}
            position={[pos3.x, pos3.y, 0]}
            selected={selectedId === node.id}
            onClick={() => handleNodeClick(node)}
            isNew={newNodeIds?.has(node.id) ?? false}
            onDragStart={handleDragStart}
            onDragEnd={handleDragEnd}
            simNode={sn}
            highlighted={highlightedIds.has(node.id)}
          />
        )
      })}

      <OrbitControls makeDefault />
    </>
  )
}

// ---------------------------------------------------------------------------
// LakeGraph -- public export
// ---------------------------------------------------------------------------
export interface LakeGraphProps {
  nodes: NodeItem[]
  edges: EdgeItem[]
  onNodeSelect?: (node: NodeItem | null) => void
  searchQuery?: string
  snapshotLayout?: Record<string, { x: number; y: number }>
}

export default function LakeGraph({ nodes, edges, onNodeSelect, searchQuery, snapshotLayout }: LakeGraphProps) {
  const displayNodes = useMemo(
    () => nodes.filter(n => n.state !== 'ERASED' && n.state !== 'GHOST').slice(0, MAX_NODES),
    [nodes],
  )
  const tooMany =
    nodes.filter(n => n.state !== 'ERASED' && n.state !== 'GHOST').length > MAX_NODES

  // P15-C: reset layout token — incrementing re-randomizes force simulation
  const [resetToken, setResetToken] = useState(0)

  // Track newly added node IDs for weave animation
  const prevNodeIdsRef = useRef(new Set<string>())
  const [newNodeIds, setNewNodeIds] = useState<Set<string>>(new Set())
  useEffect(() => {
    const prev = prevNodeIdsRef.current
    const currentIds = new Set(displayNodes.map(n => n.id))
    const incoming = new Set([...currentIds].filter(id => !prev.has(id)))
    prevNodeIdsRef.current = currentIds
    if (incoming.size > 0) {
      setNewNodeIds(incoming)
      const t = setTimeout(() => setNewNodeIds(new Set()), 800)
      return () => clearTimeout(t)
    }
  }, [displayNodes])

  if (displayNodes.length === 0) {
    return (
      <div
        style={{
          width: '100%',
          height: 480,
          borderRadius: 8,
          background: '#060d1a',
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'center',
          color: '#4a6a8e',
          fontSize: 13,
        }}
      >
        此湖暂无节点
      </div>
    )
  }

  return (
    <div
      style={{
        position: 'relative',
        width: '100%',
        height: 480,
        borderRadius: 8,
        overflow: 'hidden',
        background: '#060d1a',
      }}
    >
      {tooMany && (
        <div
          style={{
            position: 'absolute',
            top: 8,
            right: 12,
            zIndex: 10,
            fontSize: 11,
            color: '#7a9ab0',
            background: 'rgba(0,0,0,0.6)',
            padding: '3px 8px',
            borderRadius: 4,
          }}
        >
          仅展示前 {MAX_NODES} 个节点
        </div>
      )}
      {/* P15-C: 重置布局按钞 */}
      <button
        onClick={() => setResetToken(t => t + 1)}
        title="重置布局"
        style={{
          position: 'absolute', bottom: 12, right: 12, zIndex: 10,
          background: 'rgba(0,0,0,0.6)', border: '1px solid #2a4a7e',
          color: '#9ec5ee', borderRadius: 4, padding: '3px 10px',
          fontSize: 12, cursor: 'pointer',
        }}
      >
        重排布局 ↻
      </button>
      <Canvas camera={{ position: [0, 0, 600], fov: 50 }} gl={{ antialias: true }}>
        <React.Suspense fallback={null}>
          <GraphScene
            displayNodes={displayNodes}
            displayEdges={edges}
            onNodeSelect={onNodeSelect}
            newNodeIds={newNodeIds}
            resetToken={resetToken}
            searchQuery={searchQuery}
            snapshotLayout={snapshotLayout}
          />
        </React.Suspense>
      </Canvas>
    </div>
  )
}
