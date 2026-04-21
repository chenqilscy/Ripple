# D3 · 流体动效引擎实现 (WebGL)

**版本：** v3.3（消化整合版）
**日期：** 2026‑04‑21
**适用对象：** 前端图形 / Shader / 高级 UI

> 本文档由原 `系统开发文档-流体动效引擎实现 (WebGL).md` (v3.2) 消化整合而成。

---

## 一、技术选型

| 方案 | 推荐 | 理由 |
| :--- | :--- | :--- |
| 渲染引擎 | **Three.js / PixiJS (WebGL)** | Three.js 生态完善，PixiJS 更轻 |
| 图计算 | **D3-force**（仅算位置） | 渲染交给 WebGL |
| 状态管理 | Zustand / MobX | 响应式驱动 Shader uniform |

---

## 二、核心动效一：有机节点（Organic Blob）

### 2.1 视觉目标

节点不应是静态圆，而像水滴或荷叶边缘有轻微蠕动（Morphing）和张力。

### 2.2 Vertex Shader（Perlin Noise 扰动）

```glsl
uniform float uTime;
uniform float uRadius;
uniform vec2  uPosition;

void main() {
  float angle = atan(position.y, position.x);
  float noiseFreq = 3.0;
  float noiseAmp  = 0.15;
  float offset = noise(vec2(angle * noiseFreq, uTime)) * noiseAmp;
  vec3 newPosition = position * (uRadius + offset);
  gl_Position = projectionMatrix * modelViewMatrix
              * vec4(newPosition + vec3(uPosition, 0.0), 1.0);
  vUv = uv;
}
```

### 2.3 Fragment Shader（材质与光影）

- **内发光：** `length(uv - 0.5)` 计算到中心距离，越近越亮 → 模拟水体透光
- **毛玻璃：** 在节点背后渲染低分辨率屏幕截图（RenderTarget）+ 高斯模糊 → 节点背景纹理

---

## 三、核心动效二：水体连线（Fluid Edge）

### 3.1 曲线渲染

- **MeshLine** 库（Three.js）或自定义 TubeGeometry
- **路径：** D3-force 计算的贝塞尔曲线控制点
- **宽度变化：** 与速度变化（快则细，慢则粗）—— 伯努利原理

### 3.2 粒子流（UV Animation）

```glsl
uniform sampler2D uTexture;
uniform float uTime;
varying vec2 vUv;

void main() {
  vec2 flowDirection = vec2(vUv.x - uTime * 0.2, vUv.y);
  vec4 particleColor = texture2D(uTexture, flowDirection);
  vec4 lineColor = vec4(0.2, 0.4, 0.8, 1.0);
  gl_FragColor = mix(lineColor, particleColor, particleColor.a);
}
```

> 不需要真实创建上千个粒子 Mesh，UV 偏移即可。

---

## 四、核心动效三：涟漪交互（Ripple）

### 4.1 JS 触发

```javascript
node.on('hover', (event) => {
  node.material.uniforms.uRippleCenter.value = event.localPosition;
  node.material.uniforms.uRippleTime.value = 0.0;
});
```

### 4.2 Fragment Shader

```glsl
uniform vec2  uRippleCenter;
uniform float uRippleTime;

void main() {
  float dist  = distance(vUv, uRippleCenter);
  float ripple = sin(dist * 20.0 - uRippleTime * 5.0);
  vec3  normal = vec3(ripple * 0.1, 1.0, 0.0);
  // ... 后续光照计算 ...
}
```

---

## 五、性能优化清单

1. **InstancedMesh：** 节点 > 1000 时合并为单个 Draw Call
2. **视口剔除 (Culling)：** 视口外的节点与连线不进行渲染计算
3. **LOD：**
   - 近距离：高模节点（高频噪声）+ 显示粒子流
   - 远距离：降级为简单圆形 + 纯色线条，关闭噪声
4. **Shader 精度：** 移动端使用 `precision mediump float;` 而非 `highp`

---

## 六、相关文档

- 视觉描述：[D1 Delta View](D1-造浪池-Delta-View.md)
- 状态机：[D2](D2-节点与连线状态机规范.md)

---

**文档状态：** 定稿
**版本来源：** 整合自原 `系统开发文档-流体动效引擎实现 (WebGL).md` (v3.2)，原文件已删除。
