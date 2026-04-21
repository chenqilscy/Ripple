# G9 · 设计系统与 Tokens

**版本：** v1.0
**日期：** 2026‑04‑21
**适用对象：** 前端 / 设计 / 品牌
**核心隐喻：** 水的物理常数

> 设计的一致性不靠纪律，靠规范化的 Tokens。

---

## 一、Token 哲学

- 所有视觉值不允许在组件层硬编码
- 三层结构：**Primitive → Semantic → Component**
  - Primitive：原始色值、间距、字号
  - Semantic：使用语义（如 `color.surface.primary`、`color.action.publish`）
  - Component：组件专属（如 `node.bg.default`）

---

## 二、颜色 Tokens

### Primitive
```yaml
qing-50:  '#E8F5E9'
qing-500: '#2E8B57'   # 主色：青碧
qing-700: '#1B5E20'

orange-300: '#FFAB91'
orange-500: '#FF7F50'  # 行动点：日出橙
orange-700: '#D84315'

ink-50:  '#F5F5F5'
ink-500: '#666666'
ink-900: '#1A1A1A'

deep-blue-700: '#003366'  # 冰山深海蓝
```

### Semantic
```yaml
color.bg.canvas:        deep-blue-700  (深色) / qing-50 (浅色)
color.bg.node.default:  rgba(qing-500, 0.6)
color.bg.node.frozen:   rgba(qing-700, 0.9)
color.bg.node.vapor:    rgba(ink-500, 0.3)

color.action.publish:   orange-500
color.action.save:      qing-500
color.action.delete:    ink-500

color.text.primary:     白 (深色画布) / ink-900 (浅色)
color.text.muted:       ink-500
color.text.danger:      orange-700
```

### Component
```yaml
node.bg.default:        color.bg.node.default
node.bg.hover:          rgba(qing-500, 0.75)
node.bg.active:         rgba(qing-500, 0.9)

edge.color.active:      rgba(106,90,205,1)
edge.color.potential:   rgba(106,90,205,0.4)

button.cta.bg:          color.action.publish
button.cta.text:        白
```

---

## 三、字体 Tokens

```yaml
font.family.zh:         "PingFang SC", "思源宋体", serif
font.family.en:         "Inter", system-ui, sans-serif
font.family.mono:       "JetBrains Mono", monospace

font.size.xs:   12px
font.size.sm:   14px
font.size.base: 16px   # 节点正文
font.size.lg:   20px
font.size.xl:   28px
font.size.hero: 48px   # 登录页 slogan

font.weight.regular: 400
font.weight.medium:  500
font.weight.bold:    700

font.lineheight.tight:  1.2
font.lineheight.normal: 1.5
font.lineheight.loose:  1.8
```

---

## 四、间距 Tokens

8px 基础栅格：

```yaml
spacing.0:  0
spacing.1:  4px
spacing.2:  8px
spacing.3:  12px
spacing.4:  16px
spacing.6:  24px
spacing.8:  32px
spacing.12: 48px
spacing.16: 64px
```

---

## 五、圆角与阴影

```yaml
radius.sm:  4px
radius.md:  8px
radius.lg:  16px
radius.full: 9999px   # 节点：圆形

shadow.sm: 0 2px 4px rgba(0,0,0,0.06)
shadow.md: 0 4px 15px rgba(0,0,0,0.1)
shadow.lg: 0 8px 25px rgba(46,139,87,0.3)
shadow.glow.publish: 0 0 30px rgba(255,127,80,0.5)
```

---

## 六、动效 Tokens

```yaml
duration.fast:    150ms
duration.normal:  300ms
duration.slow:    500ms
duration.epic:    1200ms   # 决堤口

easing.standard: cubic-bezier(0.4, 0.0, 0.2, 1)
easing.fluid:    cubic-bezier(0.25, 0.46, 0.45, 0.94)  # 水波
easing.bounce:   cubic-bezier(0.68, -0.55, 0.27, 1.55)
```

详细 Shader 与动效曲线：见 [D3](D3-流体动效引擎-WebGL.md)。

---

## 七、文案 Tokens（关键 CTA / 空状态）

详见 [01-品牌与产品概念 §第三篇](../pinpai/01-品牌与产品概念.md)，集中维护避免散落。

---

## 八、暗色 / 浅色模式

- 默认暗色（深海背景，呼应"水"主题）
- 浅色模式：`color.bg.canvas` 切换为 `qing-50`，节点透明度 +0.1
- 用户偏好持久化在 PG `users.preferences` JSONB

---

## 九、技术实现

- 来源：JSON / YAML 单一真源（`design-tokens/`）
- 编译：Style Dictionary → CSS Variables / Tailwind Theme / Three.js Uniform
- 自动同步设计稿：Figma Tokens 插件双向同步

---

## 十、相关文档

- 视觉细节：[D1](D1-造浪池-Delta-View.md)
- 状态机视觉：[D2](D2-节点与连线状态机规范.md)
- 品牌交互词典：[01-品牌与产品概念](../pinpai/01-品牌与产品概念.md)

---

**文档状态：** 定稿
