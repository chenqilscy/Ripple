import { jsx as _jsx, jsxs as _jsxs, Fragment as _Fragment } from "react/jsx-runtime";
/**
 * M5-A: Three.js full animation
 * - Spring edges: D3 live simulation driven each frame, edges follow nodes
 * - Particle flow: one particle per edge flows src -> dst
 * - Weave animation: new nodes spring-scale in on first appearance
 *
 * deps: @react-three/fiber@8 + @react-three/drei@9 + three@0.160 + d3-force@3
 */
import React, { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { Canvas, useFrame } from '@react-three/fiber';
import { OrbitControls, Html } from '@react-three/drei';
import * as THREE from 'three';
import { forceSimulation, forceLink, forceManyBody, forceCollide, forceCenter, } from 'd3-force';
const MAX_NODES = 200;
const STATE_COLOR = {
    MIST: '#9ec5ee',
    DROP: '#52c41a',
    FROZEN: '#4a8eff',
    VAPOR: '#555566',
    ERASED: '#333344',
    GHOST: '#333344',
};
function EdgeParticles({ simLinks, speed = 0.4 }) {
    const progressRef = useRef(Float32Array.from({ length: simLinks.length }, () => Math.random()));
    const meshRef = useRef(null);
    // Resize progress buffer when edge count changes
    useEffect(() => {
        const cur = progressRef.current;
        const next = new Float32Array(simLinks.length);
        for (let i = 0; i < next.length; i++) {
            next[i] = i < cur.length ? cur[i] : Math.random();
        }
        progressRef.current = next;
    }, [simLinks.length]);
    const tmpMat = useMemo(() => new THREE.Matrix4(), []);
    const tmpVec = useMemo(() => new THREE.Vector3(), []);
    useFrame((_state, delta) => {
        if (!meshRef.current)
            return;
        const prog = progressRef.current;
        for (let i = 0; i < simLinks.length; i++) {
            prog[i] = (prog[i] + delta * speed) % 1.0;
            const lk = simLinks[i];
            const src = lk.source;
            const dst = lk.target;
            const t = prog[i];
            tmpVec.set((src.x ?? 0) * (1 - t) + (dst.x ?? 0) * t, (src.y ?? 0) * (1 - t) + (dst.y ?? 0) * t, 2);
            tmpMat.setPosition(tmpVec);
            meshRef.current.setMatrixAt(i, tmpMat);
        }
        meshRef.current.instanceMatrix.needsUpdate = true;
    });
    if (simLinks.length === 0)
        return null;
    return (_jsxs("instancedMesh", { ref: meshRef, args: [undefined, undefined, simLinks.length], children: [_jsx("sphereGeometry", { args: [2.5, 6, 6] }), _jsx("meshBasicMaterial", { color: "#89dceb", transparent: true, opacity: 0.9 })] }));
}
const STATE_LABEL = {
    MIST: '雾态',
    DROP: '水滴',
    FROZEN: '冻结',
    VAPOR: '蒸发',
    ERASED: '已消除',
    GHOST: '幽灵',
};
/** easeOutBack: slight overshoot then settle at 1.0 (spring weave effect) */
function easeOutBack(x) {
    const c1 = 1.70158;
    const c3 = c1 + 1;
    return 1 + c3 * Math.pow(x - 1, 3) + c1 * Math.pow(x - 1, 2);
}
function AnimatedNode({ node, position, selected, onClick, isNew, onDragStart, onDragEnd, simNode, highlighted }) {
    const meshRef = useRef(null);
    const scaleRef = useRef(isNew ? 0 : 1);
    const color = STATE_COLOR[node.state] ?? '#888888';
    const isDraggingRef = useRef(false);
    const dragOffsetRef = useRef({ x: 0, y: 0 });
    const [hovered, setHovered] = useState(false);
    useFrame((_state, delta) => {
        if (!meshRef.current)
            return;
        if (scaleRef.current < 1.0) {
            scaleRef.current = Math.min(1.0, scaleRef.current + delta * 3.5);
            const s = easeOutBack(scaleRef.current);
            meshRef.current.scale.setScalar(Math.max(0, s));
        }
        // Follow live simulation position during non-drag
        if (!isDraggingRef.current) {
            meshRef.current.position.set(simNode.x ?? 0, simNode.y ?? 0, 0);
        }
    });
    return (_jsxs("mesh", { ref: meshRef, position: position, scale: isNew ? 0 : 1, onClick: e => { if (!isDraggingRef.current) {
            e.stopPropagation();
            onClick();
        } }, onPointerDown: e => {
            e.stopPropagation();
            isDraggingRef.current = true;
            dragOffsetRef.current = { x: e.point.x - (simNode.x ?? 0), y: e.point.y - (simNode.y ?? 0) };
            onDragStart(node.id);
        }, onPointerMove: e => {
            if (!isDraggingRef.current || !meshRef.current)
                return;
            const nx = e.point.x - dragOffsetRef.current.x;
            const ny = e.point.y - dragOffsetRef.current.y;
            simNode.x = nx;
            simNode.y = ny;
            simNode.fx = nx;
            simNode.fy = ny;
            meshRef.current.position.set(nx, ny, 0);
        }, onPointerUp: e => {
            if (!isDraggingRef.current)
                return;
            isDraggingRef.current = false;
            const nx = e.point.x - dragOffsetRef.current.x;
            const ny = e.point.y - dragOffsetRef.current.y;
            onDragEnd(node.id, nx, ny);
        }, onPointerEnter: e => { e.stopPropagation(); if (!isDraggingRef.current)
            setHovered(true); }, onPointerLeave: () => setHovered(false), children: [_jsx("sphereGeometry", { args: selected ? [7, 14, 14] : [5, 12, 12] }), _jsx("meshStandardMaterial", { color: color, emissive: selected ? color : (highlighted ? '#ffd700' : (hovered ? color : '#000000')), emissiveIntensity: selected ? 0.6 : (highlighted ? 0.8 : (hovered ? 0.25 : 0)), roughness: 0.4, metalness: selected ? 0.3 : 0.1 }), highlighted && (_jsxs("mesh", { scale: 1.6, children: [_jsx("torusGeometry", { args: [5, 0.8, 8, 24] }), _jsx("meshBasicMaterial", { color: "#ffd700", transparent: true, opacity: 0.55 })] })), hovered && !selected && (_jsxs(Html, { position: [0, 14, 0], style: {
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
                }, children: [_jsx("div", { style: { fontWeight: 600, marginBottom: 3, color: '#9ec5ee' }, children: STATE_LABEL[node.state] ?? node.state }), _jsxs("div", { style: { wordBreak: 'break-all', opacity: 0.9 }, children: [node.content.slice(0, 80), node.content.length > 80 ? '…' : ''] }), _jsx("div", { style: { marginTop: 4, opacity: 0.55, fontSize: 10 }, children: new Date(node.created_at).toLocaleDateString('zh-CN') })] })), selected && (_jsxs(Html, { position: [0, 12, 0], style: {
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
                }, children: [node.content.slice(0, 60), node.content.length > 60 ? '\u2026' : ''] }))] }));
}
function SpringEdges({ simLinks }) {
    const geoRef = useRef(null);
    useFrame(() => {
        if (!geoRef.current || simLinks.length === 0)
            return;
        const attr = geoRef.current.attributes['position'];
        if (!attr)
            return;
        const arr = attr.array;
        for (let i = 0; i < simLinks.length; i++) {
            const lk = simLinks[i];
            const src = lk.source;
            const dst = lk.target;
            const base = i * 6;
            arr[base] = src.x ?? 0;
            arr[base + 1] = src.y ?? 0;
            arr[base + 2] = 0;
            arr[base + 3] = dst.x ?? 0;
            arr[base + 4] = dst.y ?? 0;
            arr[base + 5] = 0;
        }
        attr.needsUpdate = true;
    });
    const initPositions = useMemo(() => {
        const pts = new Float32Array(simLinks.length * 6);
        for (let i = 0; i < simLinks.length; i++) {
            const lk = simLinks[i];
            const src = lk.source;
            const dst = lk.target;
            const base = i * 6;
            pts[base] = src.x ?? 0;
            pts[base + 1] = src.y ?? 0;
            pts[base + 2] = 0;
            pts[base + 3] = dst.x ?? 0;
            pts[base + 4] = dst.y ?? 0;
            pts[base + 5] = 0;
        }
        return pts;
    }, [simLinks]);
    if (simLinks.length === 0)
        return null;
    return (_jsxs("lineSegments", { children: [_jsx("bufferGeometry", { ref: geoRef, children: _jsx("bufferAttribute", { attach: "attributes-position", array: initPositions, count: simLinks.length * 2, itemSize: 3 }) }), _jsx("lineBasicMaterial", { color: "#2a4a7e", transparent: true, opacity: 0.6 })] }));
}
function SimTicker({ sim }) {
    useFrame(() => {
        if (sim.alpha() > 0.001)
            sim.tick();
    });
    return null;
}
function GraphScene({ displayNodes, displayEdges, onNodeSelect, newNodeIds, resetToken, searchQuery, snapshotLayout }) {
    const [selectedId, setSelectedId] = useState(null);
    const selectedIdRef = useRef(selectedId);
    useEffect(() => { selectedIdRef.current = selectedId; }, [selectedId]);
    const { sim, simLinks, positions, simNodes } = useMemo(() => {
        const simNodes = displayNodes.map(n => {
            const forced = snapshotLayout?.[n.id];
            return { id: n.id, item: n, x: forced?.x ?? 0, y: forced?.y ?? 0, fx: forced?.x, fy: forced?.y };
        });
        const idSet = new Set(simNodes.map(n => n.id));
        const simLinks = displayEdges
            .filter(e => idSet.has(e.src_node_id) && idSet.has(e.dst_node_id))
            .map(e => ({
            edgeId: e.id,
            source: e.src_node_id,
            target: e.dst_node_id,
        }));
        const sim = forceSimulation(simNodes)
            .force('link', forceLink(simLinks).id(n => n.id).distance(80).strength(0.6))
            .force('charge', forceManyBody().strength(-120))
            .force('collide', forceCollide(22))
            .force('center', forceCenter(0, 0))
            .alphaDecay(0.01)
            .stop();
        // Pre-warm 100 ticks for stable initial layout
        for (let i = 0; i < 100; i++)
            sim.tick();
        const positions = new Map();
        for (const n of simNodes) {
            positions.set(n.id, new THREE.Vector3(n.x ?? 0, n.y ?? 0, 0));
        }
        // Reset alpha so SimTicker continues spring animation
        sim.alpha(0.3).restart().stop();
        return { sim, simLinks, positions, simNodes };
        // eslint-disable-next-line react-hooks/exhaustive-deps
    }, [displayNodes, displayEdges, resetToken, snapshotLayout]);
    const simNodeMap = useMemo(() => {
        const m = new Map();
        for (const n of simNodes)
            m.set(n.id, n);
        return m;
    }, [simNodes]);
    const handleNodeClick = useCallback((node) => {
        const nextId = selectedIdRef.current === node.id ? null : node.id;
        setSelectedId(nextId);
        onNodeSelect?.(nextId ? node : null);
    }, [onNodeSelect]);
    const handleDragStart = useCallback((nodeId) => {
        const sn = simNodeMap.get(nodeId);
        if (sn) {
            sn.fx = sn.x;
            sn.fy = sn.y;
        }
        sim.alphaTarget(0.1);
    }, [sim, simNodeMap]);
    const handleDragEnd = useCallback((nodeId, x, y) => {
        const sn = simNodeMap.get(nodeId);
        if (sn) {
            sn.x = x;
            sn.y = y;
            sn.fx = x;
            sn.fy = y;
        }
    }, [simNodeMap]);
    const highlightedIds = useMemo(() => {
        if (!searchQuery)
            return new Set();
        const q = searchQuery.toLowerCase();
        return new Set(displayNodes.filter(n => n.content.toLowerCase().includes(q)).map(n => n.id));
    }, [searchQuery, displayNodes]);
    return (_jsxs(_Fragment, { children: [_jsx("ambientLight", { intensity: 0.5 }), _jsx("pointLight", { position: [0, 200, 200], intensity: 1.2 }), _jsx("pointLight", { position: [0, -200, -100], intensity: 0.4, color: "#4a8eff" }), _jsx(SimTicker, { sim: sim }), _jsx(SpringEdges, { simLinks: simLinks }), _jsx(EdgeParticles, { simLinks: simLinks }), displayNodes.map(node => {
                const pos3 = positions.get(node.id) ?? new THREE.Vector3();
                const sn = simNodeMap.get(node.id);
                return (_jsx(AnimatedNode, { node: node, position: [pos3.x, pos3.y, 0], selected: selectedId === node.id, onClick: () => handleNodeClick(node), isNew: newNodeIds?.has(node.id) ?? false, onDragStart: handleDragStart, onDragEnd: handleDragEnd, simNode: sn, highlighted: highlightedIds.has(node.id) }, node.id));
            }), _jsx(OrbitControls, { makeDefault: true })] }));
}
export default function LakeGraph({ nodes, edges, onNodeSelect, searchQuery, snapshotLayout }) {
    const displayNodes = useMemo(() => nodes.filter(n => n.state !== 'ERASED' && n.state !== 'GHOST').slice(0, MAX_NODES), [nodes]);
    const tooMany = nodes.filter(n => n.state !== 'ERASED' && n.state !== 'GHOST').length > MAX_NODES;
    // P15-C: reset layout token — incrementing re-randomizes force simulation
    const [resetToken, setResetToken] = useState(0);
    // Track newly added node IDs for weave animation
    const prevNodeIdsRef = useRef(new Set());
    const [newNodeIds, setNewNodeIds] = useState(new Set());
    useEffect(() => {
        const prev = prevNodeIdsRef.current;
        const currentIds = new Set(displayNodes.map(n => n.id));
        const incoming = new Set([...currentIds].filter(id => !prev.has(id)));
        prevNodeIdsRef.current = currentIds;
        if (incoming.size > 0) {
            setNewNodeIds(incoming);
            const t = setTimeout(() => setNewNodeIds(new Set()), 800);
            return () => clearTimeout(t);
        }
    }, [displayNodes]);
    if (displayNodes.length === 0) {
        return (_jsx("div", { style: {
                width: '100%',
                height: 480,
                borderRadius: 8,
                background: '#060d1a',
                display: 'flex',
                alignItems: 'center',
                justifyContent: 'center',
                color: '#4a6a8e',
                fontSize: 13,
            }, children: "\u6B64\u6E56\u6682\u65E0\u8282\u70B9" }));
    }
    return (_jsxs("div", { style: {
            position: 'relative',
            width: '100%',
            height: 480,
            borderRadius: 8,
            overflow: 'hidden',
            background: '#060d1a',
        }, children: [tooMany && (_jsxs("div", { style: {
                    position: 'absolute',
                    top: 8,
                    right: 12,
                    zIndex: 10,
                    fontSize: 11,
                    color: '#7a9ab0',
                    background: 'rgba(0,0,0,0.6)',
                    padding: '3px 8px',
                    borderRadius: 4,
                }, children: ["\u4EC5\u5C55\u793A\u524D ", MAX_NODES, " \u4E2A\u8282\u70B9"] })), _jsx("button", { onClick: () => setResetToken(t => t + 1), title: "\u91CD\u7F6E\u5E03\u5C40", style: {
                    position: 'absolute', bottom: 12, right: 12, zIndex: 10,
                    background: 'rgba(0,0,0,0.6)', border: '1px solid #2a4a7e',
                    color: '#9ec5ee', borderRadius: 4, padding: '3px 10px',
                    fontSize: 12, cursor: 'pointer',
                }, children: "\u91CD\u6392\u5E03\u5C40 \u21BB" }), _jsx(Canvas, { camera: { position: [0, 0, 600], fov: 50 }, gl: { antialias: true }, children: _jsx(React.Suspense, { fallback: null, children: _jsx(GraphScene, { displayNodes: displayNodes, displayEdges: edges, onNodeSelect: onNodeSelect, newNodeIds: newNodeIds, resetToken: resetToken, searchQuery: searchQuery, snapshotLayout: snapshotLayout }) }) })] }));
}
