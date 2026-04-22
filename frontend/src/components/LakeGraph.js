import { jsx as _jsx, jsxs as _jsxs, Fragment as _Fragment } from "react/jsx-runtime";
/**
 * P9-C: Three.js 力导向节点图 MVP
 * 依赖：@react-three/fiber@8 + @react-three/drei@9 + three@0.160 + d3-force@3
 */
import React, { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { Canvas } from '@react-three/fiber';
import { OrbitControls, Html } from '@react-three/drei';
import * as THREE from 'three';
import { forceSimulation, forceLink, forceManyBody, forceCollide, forceCenter, } from 'd3-force';
const MAX_NODES = 200;
// 共享球体几何体，避免每节点各创建一份（200 个节点场景下节约 GPU 内存）
const SPHERE_GEO_NORMAL = new THREE.SphereGeometry(5, 12, 12);
const SPHERE_GEO_SELECTED = new THREE.SphereGeometry(7, 12, 12);
const STATE_COLOR = {
    MIST: '#9ec5ee',
    DROP: '#52c41a',
    FROZEN: '#4a8eff',
    VAPOR: '#555566',
    ERASED: '#333344',
    GHOST: '#333344',
};
function GraphScene({ displayNodes, displayEdges, onNodeSelect }) {
    const [selectedId, setSelectedId] = useState(null);
    // 用 ref 避免 onClick 闭包捕获陈旧的 selectedId
    const selectedIdRef = useRef(selectedId);
    useEffect(() => { selectedIdRef.current = selectedId; }, [selectedId]);
    // 力导向布局（在 CPU 预计算 300 tick，不做动态模拟）
    const { positions, lineObj } = useMemo(() => {
        const simNodes = displayNodes.map(n => ({ id: n.id, item: n }));
        const idSet = new Set(simNodes.map(n => n.id));
        const simLinks = displayEdges
            .filter(e => idSet.has(e.src_node_id) && idSet.has(e.dst_node_id))
            .map(e => ({
            edgeId: e.id,
            source: e.src_node_id,
            target: e.dst_node_id,
        }));
        const sim = forceSimulation(simNodes)
            .force('link', forceLink(simLinks)
            .id(n => n.id)
            .distance(80)
            .strength(0.5))
            .force('charge', forceManyBody().strength(-100))
            .force('collide', forceCollide(20))
            .force('center', forceCenter(0, 0))
            .stop();
        for (let i = 0; i < 300; i++)
            sim.tick();
        const positions = new Map();
        for (const n of simNodes)
            positions.set(n.id, [n.x ?? 0, n.y ?? 0]);
        // 所有边拼成一个 LineSegments，减少 draw call
        const pts = [];
        for (const lk of simLinks) {
            const src = lk.source;
            const dst = lk.target;
            pts.push(src.x ?? 0, src.y ?? 0, 0, dst.x ?? 0, dst.y ?? 0, 0);
        }
        const geo = new THREE.BufferGeometry();
        geo.setAttribute('position', new THREE.Float32BufferAttribute(pts, 3));
        const mat = new THREE.LineBasicMaterial({ color: '#2a3a5e', transparent: true, opacity: 0.5 });
        const lineObj = new THREE.LineSegments(geo, mat);
        return { positions, lineObj };
    }, [displayNodes, displayEdges]);
    // 显式释放 GPU 资源
    useEffect(() => () => {
        lineObj.geometry.dispose();
        lineObj.material.dispose();
    }, [lineObj]);
    const handleNodeClick = useCallback((node, stop) => {
        stop();
        const nextId = selectedIdRef.current === node.id ? null : node.id;
        setSelectedId(nextId);
        onNodeSelect?.(nextId ? node : null);
    }, [onNodeSelect]);
    return (_jsxs(_Fragment, { children: [_jsx("ambientLight", { intensity: 0.6 }), _jsx("pointLight", { position: [0, 200, 200], intensity: 1 }), _jsx("primitive", { object: lineObj }), displayNodes.map(node => {
                const [x, y] = positions.get(node.id) ?? [0, 0];
                const color = STATE_COLOR[node.state] ?? '#888888';
                const sel = selectedId === node.id;
                return (_jsxs("mesh", { position: [x, y, 0], onClick: e => handleNodeClick(node, () => e.stopPropagation()), children: [_jsx("primitive", { object: sel ? SPHERE_GEO_SELECTED : SPHERE_GEO_NORMAL, attach: "geometry" }), _jsx("meshStandardMaterial", { color: color, emissive: sel ? color : '#000000', emissiveIntensity: sel ? 0.5 : 0 }), sel && (_jsxs(Html, { position: [0, 12, 0], style: {
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
                            }, children: [node.content.slice(0, 60), node.content.length > 60 ? '…' : ''] }))] }, node.id));
            }), _jsx(OrbitControls, { makeDefault: true })] }));
}
export default function LakeGraph({ nodes, edges, onNodeSelect }) {
    const displayNodes = useMemo(() => nodes.filter(n => n.state !== 'ERASED' && n.state !== 'GHOST').slice(0, MAX_NODES), [nodes]);
    const tooMany = nodes.filter(n => n.state !== 'ERASED' && n.state !== 'GHOST').length > MAX_NODES;
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
                }, children: ["\u4EC5\u5C55\u793A\u524D ", MAX_NODES, " \u4E2A\u8282\u70B9"] })), _jsx(Canvas, { camera: { position: [0, 0, 600], fov: 50 }, gl: { antialias: true }, children: _jsx(React.Suspense, { fallback: null, children: _jsx(GraphScene, { displayNodes: displayNodes, displayEdges: edges, onNodeSelect: onNodeSelect }) }) })] }));
}
