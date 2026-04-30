/**
 * M5-A: Three.js full animation
 * - Spring edges: D3 live simulation driven each frame, edges follow nodes
 * - Particle flow: one particle per edge flows src -> dst
 * - Weave animation: new nodes spring-scale in on first appearance
 *
 * deps: @react-three/fiber@8 + @react-three/drei@9 + three@0.160 + d3-force@3
 */
import React, { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { Canvas, useFrame, useThree } from '@react-three/fiber'
import { OrbitControls, Html } from '@react-three/drei'
import * as THREE from 'three'
import {
  forceSimulation,
  forceLink,
  forceManyBody,
  forceCollide,
  forceCenter,
  forceX,
  forceY,
  type Simulation,
  type SimulationNodeDatum,
  type SimulationLinkDatum,
} from 'd3-force'
import type { EdgeItem, NodeItem, NodeState, Recommendation, PathResult, Cluster, PlanningSuggestion } from '../api/types'
import DiscoveryPanel from './graph/DiscoveryPanel'
import PathTracePanel from './graph/PathTracePanel'
import ClusterView from './graph/ClusterView'
import PlanningPanel from './graph/PlanningPanel'

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
  // Attached by AnimatedNode for global drag handler
  _mesh?: THREE.Mesh | null
}

interface SimLink extends SimulationLinkDatum<SimNode> {
  edgeId: string
  strength?: number
  kind?: string
}

function finiteNumber(v: unknown): v is number {
  return typeof v === 'number' && Number.isFinite(v)
}

function initialNodePosition(node: NodeItem, index: number, forced?: { x: number; y: number }): { x: number; y: number; fx?: number; fy?: number } {
  if (forced && finiteNumber(forced.x) && finiteNumber(forced.y)) {
    return { x: forced.x, y: forced.y, fx: forced.x, fy: forced.y }
  }
  const persisted = node.position
  if (
    persisted && finiteNumber(persisted.x) && finiteNumber(persisted.y) &&
    (Math.abs(persisted.x) > 0.001 || Math.abs(persisted.y) > 0.001) &&
    Math.abs(persisted.x) <= 1000 && Math.abs(persisted.y) <= 1000
  ) {
    return { x: persisted.x, y: persisted.y }
  }
  const goldenAngle = Math.PI * (3 - Math.sqrt(5))
  const radius = Math.min(220, 35 + Math.sqrt(index) * 36)
  return { x: Math.cos(index * goldenAngle) * radius, y: Math.sin(index * goldenAngle) * radius }
}

// ---------------------------------------------------------------------------
// Crystallize burst effect -- particles fly from source nodes toward centroid
// ---------------------------------------------------------------------------
interface CrystallizeEffectProps {
  sourcePositions: THREE.Vector3[]
  duration?: number // seconds, default 0.9
}

function CrystallizeEffect({ sourcePositions, duration = 0.9 }: CrystallizeEffectProps) {
  const PARTICLES_PER_SRC = 8
  const count = sourcePositions.length * PARTICLES_PER_SRC
  const meshRef = useRef<THREE.InstancedMesh>(null)
  const progressRef = useRef<Float32Array>(Float32Array.from({ length: count }, (_, i) => (i % PARTICLES_PER_SRC) / PARTICLES_PER_SRC))
  const centroid = useMemo(() => {
    if (sourcePositions.length === 0) return new THREE.Vector3()
    const c = new THREE.Vector3()
    for (const p of sourcePositions) c.add(p)
    c.divideScalar(sourcePositions.length)
    return c
  }, [sourcePositions])
  const tmpMat = useMemo(() => new THREE.Matrix4(), [])
  const tmpVec = useMemo(() => new THREE.Vector3(), [])
  const { invalidate } = useThree()

  useFrame((_s, delta) => {
    if (!meshRef.current) return
    const speed = 1 / duration
    let anyActive = false
    for (let i = 0; i < count; i++) {
      const t = progressRef.current[i]
      if (t >= 1) {
        meshRef.current.setMatrixAt(i, new THREE.Matrix4().makeScale(0, 0, 0))
        continue
      }
      progressRef.current[i] = Math.min(1, t + delta * speed)
      const srcIdx = Math.floor(i / PARTICLES_PER_SRC)
      const src = sourcePositions[srcIdx]
      // random offset per particle (stable via index)
      const angle = (i % PARTICLES_PER_SRC) * ((Math.PI * 2) / PARTICLES_PER_SRC)
      const off = 12
      const sx = src.x + Math.cos(angle) * off
      const sy = src.y + Math.sin(angle) * off
      const px = sx + (centroid.x - sx) * progressRef.current[i]
      const py = sy + (centroid.y - sy) * progressRef.current[i]
      const scale = (1 - progressRef.current[i]) * 2.5
      tmpVec.set(px, py, 2)
      tmpMat.makeTranslation(px, py, 2)
      tmpMat.scale(tmpVec.set(scale, scale, scale))
      meshRef.current.setMatrixAt(i, tmpMat)
      anyActive = true
    }
    meshRef.current.instanceMatrix.needsUpdate = true
    if (anyActive) invalidate()
  })

  if (count === 0) return null
  return (
    <instancedMesh key={`cryst-${count}`} ref={meshRef} args={[undefined, undefined, count]}>
      <sphereGeometry args={[1, 6, 6]} />
      <meshBasicMaterial color="#a0d8ef" transparent opacity={0.8} />
    </instancedMesh>
  )
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
  const edgeKey = useMemo(() => simLinks.map(l => l.edgeId).join('|'), [simLinks])

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

  const { invalidate } = useThree()
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
    if (simLinks.length > 0) invalidate()
  })

  if (simLinks.length === 0) return null

  return (
    <instancedMesh key={edgeKey} ref={meshRef} args={[undefined, undefined, simLinks.length]}>
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
  multiSelected: boolean
  onClick: (isMulti: boolean) => void
  isNew: boolean
  onDragStart: (nodeId: string, initialX: number, initialY: number, evt: any) => void
  simNode: SimNode
  highlighted?: boolean
  isDragging: boolean
  recCount?: number
  dimmed?: boolean
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

function AnimatedNode({ node, position, selected, multiSelected, onClick, isNew, onDragStart, simNode, highlighted, isDragging, recCount = 0, dimmed = false }: AnimNodeProps) {
  const meshRef = useRef<THREE.Mesh>(null)
  const scaleRef = useRef(isNew ? 0 : 1)
  const color = STATE_COLOR[node.state] ?? '#888888'
  const [hovered, setHovered] = useState(false)

  // P0-04 fix: 将 mesh ref 附加到 simNode 上，供 GraphScene 全局拖动事件使用
  useEffect(() => {
    simNode._mesh = meshRef.current
    return () => { if (simNode._mesh === meshRef.current) simNode._mesh = null }
  })

  useEffect(() => {
    if (!meshRef.current) return
    if (isNew) {
      scaleRef.current = 0
      meshRef.current.scale.setScalar(0)
      return
    }
    if (scaleRef.current >= 1) meshRef.current.scale.setScalar(1)
  }, [isNew])

  useFrame((_state, delta) => {
    if (!meshRef.current) return
    if (scaleRef.current < 1.0) {
      scaleRef.current = Math.min(1.0, scaleRef.current + delta * 3.5)
      const s = easeOutBack(scaleRef.current)
      meshRef.current.scale.setScalar(Math.max(0, s))
    }
    // 拖动中的位置由 GraphScene 全局事件处理器更新
    if (!isDragging) {
      meshRef.current.position.set(simNode.x ?? 0, simNode.y ?? 0, 0)
    }
  })

  return (
    <mesh
      ref={meshRef}
      position={position}
      scale={isNew ? 0 : 1}
      onClick={e => { e.stopPropagation(); onClick(e.nativeEvent.ctrlKey || e.nativeEvent.metaKey) }}
      onPointerDown={e => {
        e.stopPropagation()
        onDragStart(node.id, simNode.x ?? 0, simNode.y ?? 0, e)
      }}
      onPointerEnter={e => { e.stopPropagation(); setHovered(true) }}
      onPointerLeave={() => setHovered(false)}
    >
      <sphereGeometry args={(selected || multiSelected) ? [7, 14, 14] : [5, 12, 12]} />
      <meshStandardMaterial
        color={multiSelected ? '#4ecdc4' : color}
        emissive={multiSelected ? '#4ecdc4' : (selected ? color : (highlighted ? '#ffd700' : (hovered ? color : '#000000')))}
        emissiveIntensity={multiSelected ? 0.7 : (selected ? 0.6 : (highlighted ? 0.8 : (hovered ? 0.25 : 0)))}
        transparent={dimmed}
        opacity={dimmed ? 0.3 : 1}
        roughness={0.4}
        metalness={(selected || multiSelected) ? 0.3 : 0.1}
      />
      {/* P20-B: 多选 ring */}
      {multiSelected && (
        <mesh scale={1.5}>
          <torusGeometry args={[5, 0.9, 8, 24]} />
          <meshBasicMaterial color="#4ecdc4" transparent opacity={0.7} />
        </mesh>
      )}
      {/* P17-B: search highlight ring */}
      {highlighted && (
        <mesh scale={1.6}>
          <torusGeometry args={[5, 0.8, 8, 24]} />
          <meshBasicMaterial color="#ffd700" transparent opacity={0.55} />
        </mesh>
      )}
      {/* 图谱价值增强：推荐徽章 */}
      {recCount > 0 && !selected && (
        <mesh position={[6, 6, 0]}>
          <sphereGeometry args={[2.5, 6, 6]} />
          <meshBasicMaterial color="#faad14" />
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
  onEdgeHover?: (info: { x: number; y: number; strength: number; kind: string } | null) => void
}

function SpringEdges({ simLinks, onEdgeHover }: SpringEdgesProps) {
  const geoRef = useRef<THREE.BufferGeometry>(null)
  const edgeKey = useMemo(() => simLinks.map(l => l.edgeId).join('|'), [simLinks])
  const { camera, gl } = useThree()

  // Edge hover detection via canvas mousemove -> world coords -> nearest edge midpoint
  useEffect(() => {
    if (!onEdgeHover) return
    const canvas = gl.domElement
    const handleMove = (ev: MouseEvent) => {
      const rect = canvas.getBoundingClientRect()
      const ndcX = ((ev.clientX - rect.left) / rect.width) * 2 - 1
      const ndcY = -((ev.clientY - rect.top) / rect.height) * 2 + 1
      // Unproject to world Z=0 plane
      const near = new THREE.Vector3(ndcX, ndcY, -1).unproject(camera)
      const far = new THREE.Vector3(ndcX, ndcY, 1).unproject(camera)
      const t = -near.z / (far.z - near.z)
      const worldX = near.x + t * (far.x - near.x)
      const worldY = near.y + t * (far.y - near.y)

      let bestDist = Infinity
      let bestLink: SimLink | null = null
      for (const lk of simLinks) {
        const src = lk.source as SimNode
        const dst = lk.target as SimNode
        const mx = ((src.x ?? 0) + (dst.x ?? 0)) / 2
        const my = ((src.y ?? 0) + (dst.y ?? 0)) / 2
        // Point-to-segment distance
        const ax = src.x ?? 0, ay = src.y ?? 0
        const bx = dst.x ?? 0, by = dst.y ?? 0
        const dx = bx - ax, dy = by - ay
        const lenSq = dx * dx + dy * dy
        let segDist: number
        if (lenSq < 0.0001) {
          segDist = Math.hypot(worldX - mx, worldY - my)
        } else {
          const tt = Math.max(0, Math.min(1, ((worldX - ax) * dx + (worldY - ay) * dy) / lenSq))
          segDist = Math.hypot(worldX - (ax + tt * dx), worldY - (ay + tt * dy))
        }
        if (segDist < bestDist) { bestDist = segDist; bestLink = lk }
      }
      // Project world threshold to screen: ~12px in world units
      const camZ = (camera as THREE.PerspectiveCamera).position?.z ?? 600
      const fov = ((camera as THREE.PerspectiveCamera).fov ?? 50) * (Math.PI / 180)
      const worldPerPx = (2 * Math.tan(fov / 2) * camZ) / rect.height
      const threshold = 12 * worldPerPx
      if (bestLink && bestDist < threshold) {
        const src = bestLink.source as SimNode
        const dst = bestLink.target as SimNode
        const mid3 = new THREE.Vector3(
          ((src.x ?? 0) + (dst.x ?? 0)) / 2,
          ((src.y ?? 0) + (dst.y ?? 0)) / 2,
          0,
        ).project(camera)
        const sx = (mid3.x * 0.5 + 0.5) * rect.width
        const sy = (-mid3.y * 0.5 + 0.5) * rect.height
        onEdgeHover({ x: sx, y: sy, strength: bestLink.strength ?? 0, kind: bestLink.kind ?? 'relates' })
      } else {
        onEdgeHover(null)
      }
    }
    const handleLeave = () => onEdgeHover(null)
    canvas.addEventListener('mousemove', handleMove)
    canvas.addEventListener('mouseleave', handleLeave)
    return () => {
      canvas.removeEventListener('mousemove', handleMove)
      canvas.removeEventListener('mouseleave', handleLeave)
    }
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [simLinks, camera, gl, onEdgeHover])

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
    <lineSegments key={edgeKey}>
      <bufferGeometry ref={geoRef}>
        <bufferAttribute
          attach="attributes-position"
          array={initPositions}
          count={simLinks.length * 2}
          itemSize={3}
        />
      </bufferGeometry>
      <lineBasicMaterial color="#2e8b90" transparent opacity={0.75} />
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
  const { invalidate } = useThree()
  useFrame(() => {
    if (sim.alpha() > 0.001) {
      sim.tick()
      invalidate()
    }
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
  onMultiSelectChange?: (ids: Set<string>) => void
  newNodeIds?: Set<string>
  resetToken?: number
  searchQuery?: string
  snapshotLayout?: Record<string, { x: number; y: number }>
  onEdgeHover?: (info: { x: number; y: number; strength: number; kind: string } | null) => void
  /** 3-P1-02: 凝结动画 — 当前正在凝结的节点 IDs */
  crystallizeIds?: Set<string>
  /** 图谱价值增强：推荐徽章 */
  recCountByNode?: Map<string, number>
  /** 图谱价值增强：聚类高亮 */
  clusters?: Cluster[]
  focusedClusterId?: string | null
}

function GraphScene({ displayNodes, displayEdges, onNodeSelect, onMultiSelectChange, newNodeIds, resetToken, searchQuery, snapshotLayout, onEdgeHover, crystallizeIds, recCountByNode, clusters, focusedClusterId }: SceneProps) {
  const [selectedId, setSelectedId] = useState<string | null>(null)
  const selectedIdRef = useRef(selectedId)
  useEffect(() => { selectedIdRef.current = selectedId }, [selectedId])
  const [multiSelectedIds, setMultiSelectedIds] = useState<Set<string>>(new Set())

  const { sim, simLinks, positions, simNodes } = useMemo(() => {
    const simNodes: SimNode[] = displayNodes.map((n, index) => {
      const forced = snapshotLayout?.[n.id]
      const pos = initialNodePosition(n, index, forced)
      return { id: n.id, item: n, x: pos.x, y: pos.y, fx: pos.fx, fy: pos.fy }
    })
    const idSet = new Set(simNodes.map(n => n.id))

    const simLinks: SimLink[] = displayEdges
      .filter(e => idSet.has(e.src_node_id) && idSet.has(e.dst_node_id))
      .map(e => ({
        edgeId: e.id,
        source: e.src_node_id,
        target: e.dst_node_id,
        strength: e.strength,
        kind: e.kind,
      }))

    const sim = forceSimulation<SimNode>(simNodes)
      .force(
        'link',
        forceLink<SimNode, SimLink>(simLinks).id(n => n.id).distance(90).strength(0.5),
      )
      .force('charge', forceManyBody<SimNode>().strength(-45))
      .force('collide', forceCollide<SimNode>(18))
      .force('center', forceCenter(0, 0))
      .force('x', forceX<SimNode>(0).strength(0.02))
      .force('y', forceY<SimNode>(0).strength(0.02))
      .alphaDecay(0.03)
      .stop()

    // Pre-warm for stable initial layout without pushing isolated nodes out of camera bounds.
    for (let i = 0; i < 80; i++) sim.tick()

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
    (node: NodeItem, isMulti: boolean) => {
      if (isMulti) {
        setMultiSelectedIds(prev => {
          const next = new Set(prev)
          if (next.has(node.id)) next.delete(node.id)
          else next.add(node.id)
          onMultiSelectChange?.(next)
          return next
        })
      } else {
        const nextId = selectedIdRef.current === node.id ? null : node.id
        setSelectedId(nextId)
        setMultiSelectedIds(new Set())
        onMultiSelectChange?.(new Set())
        onNodeSelect?.(nextId ? node : null)
      }
    },
    [onNodeSelect, onMultiSelectChange],
  )

  // P0-04 fix: 拖动状态统一管理 — 由 GraphScene 的全局事件处理
  const draggingNodeIdRef = useRef<string | null>(null)
  const dragOffsetRef = useRef({ x: 0, y: 0 })
  const controlsRef = useRef<any>(null)

  const handleDragStart = useCallback((nodeId: string, nodeX: number, nodeY: number, e: any) => {
    // 记录拖动偏移
    dragOffsetRef.current = { x: e.point.x - nodeX, y: e.point.y - nodeY }
    draggingNodeIdRef.current = nodeId

    // 通知 D3 simulation 开始拖动
    const sn = simNodeMap.get(nodeId)
    if (sn) {
      sn.fx = sn.x
      sn.fy = sn.y
    }
    sim.alphaTarget(0.1)

    // 禁用 OrbitControls 防止拖动时误触画布
    if (controlsRef.current) {
      controlsRef.current.enabled = false
    }
  }, [sim, simNodeMap])

  // 全局 pointermove / pointerup 处理拖动（解决鼠标移出节点边界时事件丢失的问题）
  useEffect(() => {
    const canvas = document.querySelector('canvas')
    if (!canvas) return

    const onMove = (e: MouseEvent) => {
      if (!draggingNodeIdRef.current) return
      const sn = simNodeMap.get(draggingNodeIdRef.current)
      if (!sn) return
      const mesh = sn._mesh
      if (!mesh) return

      const rect = canvas.getBoundingClientRect()
      const ndcX = ((e.clientX - rect.left) / rect.width) * 2 - 1
      const ndcY = -((e.clientY - rect.top) / rect.height) * 2 + 1
      const nx = ndcX * 300 - dragOffsetRef.current.x
      const ny = ndcY * 300 - dragOffsetRef.current.y
      sn.x = nx
      sn.y = ny
      sn.fx = nx
      sn.fy = ny
      mesh.position.set(nx, ny, 0)
    }

    const onUp = () => {
      if (draggingNodeIdRef.current) {
        const sn = simNodeMap.get(draggingNodeIdRef.current)
        if (sn) {
          sn.fx = sn.x
          sn.fy = sn.y
        }
        draggingNodeIdRef.current = null
        if (controlsRef.current) {
          controlsRef.current.enabled = true
        }
      }
    }

    canvas.addEventListener('pointermove', onMove)
    canvas.addEventListener('pointerup', onUp)
    return () => {
      canvas.removeEventListener('pointermove', onMove)
      canvas.removeEventListener('pointerup', onUp)
    }
  }, [simNodeMap])

  const highlightedIds = useMemo(() => {
    if (!searchQuery) return new Set<string>()
    const q = searchQuery.toLowerCase()
    return new Set(displayNodes.filter(n => n.content.toLowerCase().includes(q)).map(n => n.id))
  }, [searchQuery, displayNodes])

  // 图谱价值增强：聚类高亮
  const clusterHighlightedIds = useMemo(() => {
    if (!clusters || clusters.length === 0 || !focusedClusterId) return new Set<string>()
    const cluster = clusters.find(c => c.id === focusedClusterId)
    if (!cluster) return new Set<string>()
    return new Set(cluster.node_ids)
  }, [clusters, focusedClusterId])

  const dimmedIds = useMemo(() => {
    if (!focusedClusterId) return new Set<string>()
    const allNodeIds = new Set(displayNodes.map(n => n.id))
    const dimmed = new Set<string>()
    for (const id of allNodeIds) if (!clusterHighlightedIds.has(id)) dimmed.add(id)
    return dimmed
  }, [displayNodes, clusterHighlightedIds, focusedClusterId])

  return (
    <>
      <ambientLight intensity={0.5} />
      <pointLight position={[0, 200, 200]} intensity={1.2} />
      <pointLight position={[0, -200, -100]} intensity={0.4} color="#4a8eff" />

      <SimTicker sim={sim} />
      <SpringEdges simLinks={simLinks} onEdgeHover={onEdgeHover} />
      <EdgeParticles simLinks={simLinks} />

      {/* 3-P1-02: Crystallize burst animation */}
      {crystallizeIds && crystallizeIds.size > 0 && (() => {
        const srcPositions = [...crystallizeIds]
          .map(id => simNodeMap.get(id))
          .filter(Boolean)
          .map(sn => new THREE.Vector3(sn!.x ?? 0, sn!.y ?? 0, 0))
        return srcPositions.length > 0 ? <CrystallizeEffect key={`cryst-${[...crystallizeIds].join(',')}`} sourcePositions={srcPositions} /> : null
      })()}

      {displayNodes.map(node => {
        const pos3 = positions.get(node.id) ?? new THREE.Vector3()
        const sn = simNodeMap.get(node.id)!
        return (
          <AnimatedNode
            key={node.id}
            node={node}
            position={[pos3.x, pos3.y, 0]}
            selected={selectedId === node.id}
            multiSelected={multiSelectedIds.has(node.id)}
            onClick={(isMulti) => handleNodeClick(node, isMulti)}
            isNew={newNodeIds?.has(node.id) ?? false}
            onDragStart={handleDragStart}
            simNode={sn}
            highlighted={highlightedIds.has(node.id)}
            isDragging={draggingNodeIdRef.current === node.id}
            recCount={recCountByNode?.get(node.id) ?? 0}
            dimmed={dimmedIds.has(node.id)}
          />
        )
      })}

      <OrbitControls
        ref={controlsRef}
        makeDefault
        enableRotate={false}
        enablePan={true}
        mouseButtons={{
          LEFT: THREE.MOUSE.PAN,
          MIDDLE: THREE.MOUSE.DOLLY,
          RIGHT: THREE.MOUSE.PAN,
        }}
        touches={{
          ONE: THREE.TOUCH.PAN,
          TWO: THREE.TOUCH.DOLLY_PAN,
        }}
        minDistance={50}
        maxDistance={2000}
        zoomSpeed={1.2}
        panSpeed={1.0}
        enableDamping={true}
        dampingFactor={0.05}
      />
    </>
  )
}

// ---------------------------------------------------------------------------
// CameraController -- 暴露缩放/适配命令给父组件
// ---------------------------------------------------------------------------
interface CameraControllerProps {
  onZoomIn: React.MutableRefObject<() => void>
  onZoomOut: React.MutableRefObject<() => void>
  onFit: React.MutableRefObject<() => void>
}

function CameraController({ onZoomIn, onZoomOut, onFit }: CameraControllerProps) {
  const { camera } = useThree()
  useEffect(() => {
    onZoomIn.current = () => { camera.position.z = Math.max(50, camera.position.z * 0.7) }
    onZoomOut.current = () => { camera.position.z = Math.min(2000, camera.position.z * 1.4) }
    onFit.current = () => { camera.position.set(0, 0, 600) }
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [camera])
  return null
}

// ---------------------------------------------------------------------------
// LakeGraph -- public export
// ---------------------------------------------------------------------------
export interface RemoteCursor {
  x: number // 0-1 归一化（相对容器宽）
  y: number // 0-1 归一化（相对容器高）
  name?: string  // P28: 用户显示名称
  lastSeen?: number // P28: 最后活跃时间 ms
}

export interface LakeGraphProps {
  nodes: NodeItem[]
  edges: EdgeItem[]
  onNodeSelect?: (node: NodeItem | null) => void
  /** P20-B: 多选节点 IDs 变化回调 */
  onMultiSelectChange?: (ids: Set<string>) => void
  searchQuery?: string
  snapshotLayout?: Record<string, { x: number; y: number }>
  /** P19-C：协作光标 */
  remoteCursors?: Map<string, RemoteCursor> // user_id → 归一化坐标
  onSendCursor?: (x: number, y: number) => void
  /** 3-P1-02: 凝结动画节点 IDs */
  crystallizeIds?: Set<string>
  /** 图谱价值增强：发现面板 */
  showDiscovery?: boolean
  recommendations?: Recommendation[]
  loadingRecommendations?: boolean
  activePath?: PathResult | null
  loadingPath?: boolean
  recCountByNode?: Map<string, number>
  onToggleDiscovery?: () => void  // 切换发现面板
  onAcceptRec?: (rec: Recommendation) => void
  onIgnoreRec?: (id: string) => void
  onTracePath?: (src: string, dst: string) => void
  onClosePath?: () => void
  /** 图谱价值增强：聚类 */
  showCluster?: boolean
  clusters?: Cluster[]
  focusedClusterId?: string | null
  loadingClusters?: boolean
  onFocusCluster?: (id: string | null) => void
  onRefreshClusters?: () => void
  onCloseCluster?: () => void
  /** 图谱价值增强：规划 */
  showPlanning?: boolean
  planningSuggestions?: PlanningSuggestion[]
  loadingPlanning?: boolean
  onAcceptPlanning?: (s: PlanningSuggestion) => void
  onRefreshPlanning?: () => void
  onClosePlanning?: () => void
}

// P19-C：协作光标颜色（按 user_id hash 分配）
const CURSOR_COLORS = ['#ff6b6b', '#4ecdc4', '#ffd93d', '#a8e6cf', '#ff8b94', '#b3cde0']
function cursorColor(userId: string): string {
  let h = 0
  for (let i = 0; i < userId.length; i++) h = (h * 31 + userId.charCodeAt(i)) >>> 0
  return CURSOR_COLORS[h % CURSOR_COLORS.length]
}

export default function LakeGraph({
  nodes, edges, onNodeSelect, onMultiSelectChange, searchQuery, snapshotLayout,
  remoteCursors, onSendCursor, crystallizeIds,
  showDiscovery, recommendations, loadingRecommendations, activePath, loadingPath,
  recCountByNode, onToggleDiscovery, onAcceptRec, onIgnoreRec, onTracePath, onClosePath,
  showCluster, clusters, focusedClusterId, loadingClusters, onFocusCluster, onRefreshClusters, onCloseCluster,
  showPlanning, planningSuggestions, loadingPlanning, onAcceptPlanning, onRefreshPlanning, onClosePlanning,
}: LakeGraphProps) {
  const displayNodes = useMemo(
    () => nodes.filter(n => n.state !== 'ERASED' && n.state !== 'GHOST').slice(0, MAX_NODES),
    [nodes],
  )
  const tooMany =
    nodes.filter(n => n.state !== 'ERASED' && n.state !== 'GHOST').length > MAX_NODES

  // 缩放控制 refs
  const zoomInRef = useRef<() => void>(() => undefined)
  const zoomOutRef = useRef<() => void>(() => undefined)
  const fitRef = useRef<() => void>(() => undefined)

  // P19-C: throttle ref for cursor send（50ms）
  const cursorThrottleRef = useRef<ReturnType<typeof setTimeout> | null>(null)
  // P19-C: 组件卸载时清理 throttle 计时器，防止卸载后回调更新失效 ref
  useEffect(() => {
    return () => {
      if (cursorThrottleRef.current) clearTimeout(cursorThrottleRef.current)
    }
  }, [])

  // P15-C: reset layout token — incrementing re-randomizes force simulation
  const [resetToken, setResetToken] = useState(0)
  // P2-03: edge hover tooltip
  const [edgeHoverInfo, setEdgeHoverInfo] = useState<{ x: number; y: number; strength: number; kind: string } | null>(null)

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
      onMouseMove={onSendCursor ? (e) => {
        if (cursorThrottleRef.current) return
        cursorThrottleRef.current = setTimeout(() => { cursorThrottleRef.current = null }, 50)
        const rect = (e.currentTarget as HTMLDivElement).getBoundingClientRect()
        onSendCursor(
          (e.clientX - rect.left) / rect.width,
          (e.clientY - rect.top) / rect.height,
        )
      } : undefined}
      onMouseLeave={onSendCursor ? () => {
        if (cursorThrottleRef.current) { clearTimeout(cursorThrottleRef.current); cursorThrottleRef.current = null }
      } : undefined}
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
      {/* P15-C: 重置布局按钮 */}
      <button
        onClick={() => setResetToken(t => t + 1)}
        title="重排布局：重新计算节点位置"
        style={{
          position: 'absolute', bottom: 12, right: 12, zIndex: 10,
          background: 'rgba(0,0,0,0.6)', border: '1px solid #2a4a7e',
          color: '#9ec5ee', borderRadius: 4, padding: '3px 10px',
          fontSize: 12, cursor: 'pointer',
        }}
      >
        重排布局 ↻
      </button>
      {/* P0-04: 缩放控制按钮组 */}
      <div style={{ position: 'absolute', bottom: 12, left: 12, zIndex: 10, display: 'flex', flexDirection: 'column', gap: 4 }}>
        <button onClick={() => zoomInRef.current()} title="放大" style={zoomBtnStyle}>+</button>
        <button onClick={() => zoomOutRef.current()} title="缩小" style={zoomBtnStyle}>−</button>
        <button onClick={() => fitRef.current()} title="适配画布：恢复默认视角" style={zoomBtnStyle}>⊡</button>
      </div>
      <Canvas camera={{ position: [0, 0, 600], fov: 50 }} gl={{ antialias: true }} frameloop="always">
        <React.Suspense fallback={null}>
          <GraphScene
            displayNodes={displayNodes}
            displayEdges={edges}
            onNodeSelect={onNodeSelect}
            onMultiSelectChange={onMultiSelectChange}
            newNodeIds={newNodeIds}
            resetToken={resetToken}
            searchQuery={searchQuery}
            snapshotLayout={snapshotLayout}
            onEdgeHover={setEdgeHoverInfo}
            crystallizeIds={crystallizeIds}
            recCountByNode={recCountByNode}
            clusters={clusters}
            focusedClusterId={focusedClusterId}
          />
        </React.Suspense>
        <CameraController onZoomIn={zoomInRef} onZoomOut={zoomOutRef} onFit={fitRef} />
      </Canvas>

      {/* 图谱价值增强：面板切换按钮 */}
      <div style={{ position: 'absolute', top: 12, left: 12, zIndex: 10, display: 'flex', gap: 4 }}>
        <button onClick={onToggleDiscovery ?? (() => {})} style={{
          background: showDiscovery ? 'rgba(46,139,144,0.3)' : 'rgba(0,0,0,0.6)',
          border: '1px solid #2e8b90', color: '#9ec5ee', borderRadius: 4,
          padding: '3px 8px', fontSize: 11, cursor: 'pointer',
        }}>
          💡 发现
        </button>
        <button onClick={onCloseCluster ?? (() => {})} style={{
          background: showCluster ? 'rgba(46,74,126,0.3)' : 'rgba(0,0,0,0.6)',
          border: '1px solid #2a4a7e', color: '#9ec5ee', borderRadius: 4,
          padding: '3px 8px', fontSize: 11, cursor: 'pointer',
        }}>
          🗂 领域
        </button>
        <button onClick={onClosePlanning ?? (() => {})} style={{
          background: showPlanning ? 'rgba(46,74,126,0.3)' : 'rgba(0,0,0,0.6)',
          border: '1px solid #2a4a7e', color: '#9ec5ee', borderRadius: 4,
          padding: '3px 8px', fontSize: 11, cursor: 'pointer',
        }}>
          📋 规划
        </button>
      </div>

      {/* === 发现面板 === */}
      {showDiscovery && (
        <DiscoveryPanel
          recommendations={recommendations ?? []}
          loading={loadingRecommendations ?? false}
          activePath={activePath ?? null}
          loadingPath={loadingPath ?? false}
          onAccept={onAcceptRec ?? (() => {})}
          onIgnore={onIgnoreRec ?? (() => {})}
          onTracePath={onTracePath ?? (() => {})}
          onClosePath={onClosePath ?? (() => {})}
          onClose={onToggleDiscovery ?? (() => {})}
        />
      )}

      {/* === 路径追溯面板 === */}
      {!showDiscovery && activePath && (
        <PathTracePanel
          path={activePath}
          loading={loadingPath ?? false}
          onClose={onClosePath ?? (() => {})}
        />
      )}

      {/* === 聚类视图 === */}
      {showCluster && (
        <ClusterView
          clusters={clusters ?? []}
          focusedClusterId={focusedClusterId ?? null}
          loading={loadingClusters ?? false}
          onFocus={onFocusCluster ?? (() => {})}
          onRefresh={onRefreshClusters ?? (() => {})}
          onClose={onCloseCluster ?? (() => {})}
        />
      )}

      {/* === 规划面板 === */}
      {showPlanning && (
        <PlanningPanel
          suggestions={planningSuggestions ?? []}
          loading={loadingPlanning ?? false}
          onAccept={onAcceptPlanning ?? (() => {})}
          onRefresh={onRefreshPlanning ?? (() => {})}
          onClose={onClosePlanning ?? (() => {})}
        />
      )}
      {/* P19-C / P28: 协作光标 DOM overlay（百分比定位，避免 SVG preserveAspectRatio 字体变形） */}
      {remoteCursors && remoteCursors.size > 0 && (
        <div style={{ position: 'absolute', inset: 0, pointerEvents: 'none', zIndex: 20, overflow: 'hidden' }}>
          {[...remoteCursors.entries()].map(([uid, pos]) => {
            // P28: 5s 不活跃则隐藏光标
            const inactive = pos.lastSeen != null && Date.now() - pos.lastSeen > 5000
            if (inactive) return null
            const color = cursorColor(uid)
            const label = pos.name ?? uid.slice(0, 8)
            return (
              <div
                key={uid}
                style={{
                  position: 'absolute',
                  left: `${pos.x * 100}%`,
                  top: `${pos.y * 100}%`,
                  transform: 'translate(0, 0)',
                  pointerEvents: 'none',
                  // P28: 平滑光标移动
                  transition: 'left 0.12s linear, top 0.12s linear',
                }}
              >
                {/* 光标三角形 */}
                <svg width="14" height="18" viewBox="0 0 14 18" style={{ display: 'block' }}>
                  <polygon points="0,0 0,14 5,10" fill={color} opacity="0.9" />
                </svg>
                {/* 用户名标签 */}
                <div style={{
                  background: color,
                  color: '#fff',
                  fontSize: 10,
                  padding: '1px 5px',
                  borderRadius: 3,
                  marginTop: -2,
                  maxWidth: 80,
                  overflow: 'hidden',
                  textOverflow: 'ellipsis',
                  whiteSpace: 'nowrap',
                  opacity: 0.9,
                }}>
                  {label}
                </div>
              </div>
            )
          })}
        </div>
      )}
      {/* P2-03: edge hover tooltip */}
      {edgeHoverInfo && (
        <div style={{
          position: 'absolute',
          left: edgeHoverInfo.x + 12,
          top: edgeHoverInfo.y - 8,
          zIndex: 30,
          pointerEvents: 'none',
          background: 'rgba(6,13,26,0.92)',
          border: '1px solid #2e8b90',
          borderRadius: 5,
          padding: '4px 8px',
          fontSize: 11,
          color: '#9ec5ee',
          whiteSpace: 'nowrap',
        }}>
          <span style={{ color: '#2e8b90', marginRight: 4 }}>{edgeHoverInfo.kind}</span>
          {edgeHoverInfo.strength > 0 && (
            <span>相似度 {Math.round(edgeHoverInfo.strength * 100)}%</span>
          )}
        </div>
      )}
    </div>
  )
}

const zoomBtnStyle: React.CSSProperties = {
  background: 'rgba(0,0,0,0.6)', border: '1px solid #2a4a7e',
  color: '#9ec5ee', borderRadius: 4, padding: '3px 8px',
  fontSize: 14, cursor: 'pointer', lineHeight: 1, fontWeight: 600,
}
