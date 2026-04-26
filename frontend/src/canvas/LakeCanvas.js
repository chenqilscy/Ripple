import { jsx as _jsx } from "react/jsx-runtime";
import { useEffect, useRef } from 'react';
import * as THREE from 'three';
/**
 * 湖面画布 · Three.js 最小实现
 * 参考 造浪池-交互视图设计.md + 流体动效引擎-WebGL.md
 * M1：平面 + 鼠标涟漪；M2 接入 Perlin Noise 流场 + MeshLine 粒子流
 */
export function LakeCanvas() {
    const mountRef = useRef(null);
    useEffect(() => {
        if (!mountRef.current)
            return;
        const mount = mountRef.current;
        const scene = new THREE.Scene();
        scene.background = new THREE.Color(0x001e2b);
        const camera = new THREE.PerspectiveCamera(60, mount.clientWidth / mount.clientHeight, 0.1, 1000);
        camera.position.set(0, 6, 10);
        camera.lookAt(0, 0, 0);
        const renderer = new THREE.WebGLRenderer({ antialias: true });
        renderer.setPixelRatio(window.devicePixelRatio);
        renderer.setSize(mount.clientWidth, mount.clientHeight);
        mount.appendChild(renderer.domElement);
        // 湖面
        const geometry = new THREE.PlaneGeometry(40, 40, 64, 64);
        geometry.rotateX(-Math.PI / 2);
        const material = new THREE.MeshBasicMaterial({
            color: 0x2e8b57,
            wireframe: true,
            transparent: true,
            opacity: 0.35,
        });
        const lake = new THREE.Mesh(geometry, material);
        scene.add(lake);
        // 涟漪节点占位（浮萍）
        const nodeGeom = new THREE.SphereGeometry(0.3, 16, 16);
        const nodeMat = new THREE.MeshBasicMaterial({ color: 0xff7f50 });
        const pebble = new THREE.Mesh(nodeGeom, nodeMat);
        scene.add(pebble);
        let t = 0;
        let raf = 0;
        const render = () => {
            t += 0.02;
            const pos = geometry.attributes.position;
            for (let i = 0; i < pos.count; i++) {
                const x = pos.getX(i);
                const z = pos.getZ(i);
                pos.setY(i, Math.sin(t + x * 0.3) * 0.15 + Math.cos(t + z * 0.3) * 0.15);
            }
            pos.needsUpdate = true;
            pebble.position.y = 0.4 + Math.sin(t * 2) * 0.1;
            renderer.render(scene, camera);
            raf = requestAnimationFrame(render);
        };
        render();
        const onResize = () => {
            camera.aspect = mount.clientWidth / mount.clientHeight;
            camera.updateProjectionMatrix();
            renderer.setSize(mount.clientWidth, mount.clientHeight);
        };
        window.addEventListener('resize', onResize);
        return () => {
            cancelAnimationFrame(raf);
            window.removeEventListener('resize', onResize);
            mount.removeChild(renderer.domElement);
            geometry.dispose();
            material.dispose();
            nodeGeom.dispose();
            nodeMat.dispose();
            renderer.dispose();
        };
    }, []);
    return _jsx("div", { ref: mountRef, style: { width: '100%', height: '100%' } });
}
