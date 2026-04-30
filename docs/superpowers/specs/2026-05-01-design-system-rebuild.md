# 设计系统重建（Design System Rebuid）

**日期：** 2026-05-01
**归属：** Phase 3-C（设计系统重建）

**目标：** 从零建立统一设计系统，以浅色青碧为视觉基调，以 token 驱动为技术架构，为 Ripple 提供完整的组件库基础设施。

---

## 一、设计原则

1. **水文隐喻**：视觉语言体现"涟漪、水波、清澈"意象，用青碧色系和微妙渐变表达
2. **Token 驱动**：所有设计值定义为可复用 Token，组件通过引用而非硬编码实现一致
3. **可访问性优先**：每个组件支持键盘导航 + ARIA 标签
4. **最小侵入**：不引入新运行时库，不破坏现有组件，双轨并行迁移
5. **渐进增强**：按优先级逐个替换现有组件，每步可验证

---

## 二、视觉方向

### 2.1 整体基调

- **关键词**：透明、清澈、呼吸感、水文流动
- **主基调**：浅色清澈 + 青碧主色 + 微妙水波纹理
- **氛围**：类似 Linear/Notion 的清澈通透感

### 2.2 与旧版对比

| 维度 | 旧版 | 新版 |
|------|------|------|
| 背景 | `#0d1526` 深蓝黑 | `#f8fafc` 浅灰白 |
| 主色 | `#4a8eff` 冷蓝 | `#2e8b90` 青碧 |
| 层次 | 阴影分隔 | 留白分隔 + 微妙边框 |
| 氛围 | 科技暗 | 清澈通透 |

### 2.3 青碧色系

```
极浅水色  #f0fdfd （背景点缀）
浅青      #a8e6e8 （hover 态）
青碧      #2e8b90 （主强调色）
深青      #1a6b72 （active / 按下）
暗青      #0f4a52 （文字中的青碧）
```

### 2.4 渐变

在卡片标题、Tab 选中态、按钮 hover 等处使用线性渐变表现水文意象：
```css
/* 按钮 hover */
background: linear-gradient(135deg, #3dd9d2, #1a6b72);

/* 热点/活跃元素背景 */
background: linear-gradient(135deg, #e0f7f8, #2e8b90);
```

### 2.5 留白

大量留白，呼吸感优先。阴影仅用于悬浮态（Floating）和 Modal，不用于静态卡片。

---

## 三、Design Token 架构

### 3.1 三层 Token 结构

```
Layer 1: Base Token — 原始设计值，不含语义（--color-cyan-500: #2e8b90）
    ↓ 映射
Layer 2: Semantic Token — 表达用途（--color-primary: var(--color-cyan-500)）
    ↓ 引用
Layer 3: Component Token — 组件级定制（--button-primary-bg: var(--color-primary)）
```

### 3.2 Base Token（基础色板）

```typescript
const baseColors = {
  // 青碧色系（Teal）
  cyan: {
    50:  '#f0fdfd',
    100: '#cffafa',
    200: '#a7f3ec',
    300: '#6ee7e0',
    400: '#3dd9d2',
    500: '#2e8b90',   // 主色
    600: '#1a6b72',
    700: '#0f4a52',
    800: '#0a3438',
    900: '#051f22',
  },
  // 中性灰（Neutral，浅色主题用）
  gray: {
    50:  '#f9fafb',
    100: '#f3f4f6',
    200: '#e5e7eb',
    300: '#d1d5db',
    400: '#9ca3af',
    500: '#6b7280',
    600: '#4b5563',
    700: '#374151',
    800: '#1f2937',
    900: '#111827',
  },
  // 语义色
  semantic: {
    success: '#10b981',
    warning: '#f59e0b',
    danger:  '#ef4444',
    info:    '#2e8b90',
  }
}
```

### 3.3 Semantic Token（语义映射）

```typescript
// 背景
'bg.primary':    '#f8fafc',   // 主背景
'bg.secondary':  '#f1f5f9',   // 侧边栏/面板
'bg.tertiary':   '#ffffff',   // 卡片表面
'bg.hover':      '#e0f7f8',   // hover 态（青碧浅底）
'bg.overlay':    'rgba(46,139,144,0.15)',

// 文字
'text.primary':   '#0f172a',
'text.secondary': '#475569',
'text.tertiary':  '#94a3b8',
'text.inverse':   '#ffffff',

// 边框
'border.default': '#e2e8f0',
'border.subtle':  '#f1f5f9',
'border.active':   '#2e8b90',

// 主色
'color.primary':       '#2e8b90',
'color.primary.subtle': '#e0f7f8',
'color.primary.hover': '#1a6b72',
'color.primary.active': '#0f4a52',

// 状态色
'status.success': '#10b981',
'status.warning': '#f59e0b',
'status.danger':  '#ef4444',
'status.info':    '#2e8b90',
```

### 3.4 间距 Token

```typescript
const space = {
  0: 0,
  1: '4px',
  2: '8px',
  3: '12px',
  4: '16px',
  6: '24px',
  8: '32px',
  12: '48px',
  16: '64px',
}
```

### 3.5 圆角 Token

```typescript
const radius = {
  none:  '0',
  sm:    '4px',   // 小元素（badge）
  md:    '8px',   // 按钮、输入框
  lg:    '12px',  // 卡片
  xl:    '16px',  // Modal
  full:  '9999px', // 胶囊按钮
}
```

### 3.6 阴影 Token

```typescript
const shadows = {
  sm:   '0 1px 2px rgba(0,0,0,0.05)',
  md:   '0 4px 6px -1px rgba(0,0,0,0.07), 0 2px 4px -1px rgba(0,0,0,0.04)',
  lg:   '0 10px 15px -3px rgba(0,0,0,0.08), 0 4px 6px -2px rgba(0,0,0,0.04)',
  xl:   '0 20px 25px -5px rgba(0,0,0,0.1), 0 10px 10px -5px rgba(0,0,0,0.04)',
  float: '0 8px 24px rgba(46,139,144,0.15)',   // 浮层（青碧光晕）
  modal: '0 24px 48px rgba(15,23,42,0.2)',      // Modal
}
```

### 3.7 与现有 tokens.ts 的关系

现有 `frontend/src/styles/tokens.ts` 扩展策略：
- 添加 `lightTokens` 导出块（新色板 + 语义映射）
- 不删除旧 Token，兼容现有组件
- 新组件只引用新的语义 Token
- 逐步弃用 `catppuccin` 部分

---

## 四、组件库规格

### 4.1 Button（按钮）

**变体（Variants）：**
- `primary` — 青碧实心，白字，用于主要操作
- `secondary` — 青碧描边，青碧字，用于次要操作
- `ghost` — 透明背景，hover 显示浅青底，用于第三级操作
- `danger` — 红色实心，白字，用于删除/危险操作

**尺寸：**
- `sm` — 高度 28px，字号 11px
- `md` — 高度 36px，字号 13px，**默认**
- `lg` — 高度 44px，字号 14px，用于主 CTA

**状态：** 默认 → hover（背景加深10%）→ active（背景加深15%）→ disabled（opacity 0.4）

**API：**
```typescript
interface ButtonProps {
  variant?: 'primary' | 'secondary' | 'ghost' | 'danger'
  size?: 'sm' | 'md' | 'lg'
  disabled?: boolean
  loading?: boolean     // 显示 spinner
  icon?: ReactNode
  iconPosition?: 'left' | 'right'
  children?: ReactNode
  onClick?: () => void
}
```

**渐变：** `primary` 按钮 hover 态使用 `linear-gradient(135deg, #3dd9d2, #1a6b72)`

---

### 4.2 Input（输入框）

**组成：** Label + Input 字段 + HelperText/ErrorText

**状态：**
- 默认：`#e2e8f0` 边框
- focus：`#2e8b90` 边框 + 青碧微光晕
- error：`#ef4444` 边框 + 红色文字
- disabled：opacity 0.5，灰色背景

**变体：** `default` | `textarea` | `search` | `password`

**API：**
```typescript
interface InputProps {
  label?: string
  placeholder?: string
  value?: string
  error?: string
  helperText?: string
  disabled?: boolean
  prefix?: ReactNode
  suffix?: ReactNode
}
```

---

### 4.3 Modal（弹窗）

**结构：** Header（标题 + 关闭）| Body（滚动内容）| Footer（操作按钮）

**行为：**
- 点击遮罩关闭（`closeOnOverlayClick` 可配置）
- ESC 键关闭
- 打开时 body scroll 锁定
- 动画：scale 0.95→1 + opacity 0→1，200ms ease-out

**API：**
```typescript
interface ModalProps {
  open: boolean
  onClose: () => void
  title?: string
  description?: string
  size?: 'sm' | 'md' | 'lg' | 'xl' | 'full'
  closeOnOverlayClick?: boolean
  footer?: ReactNode
  children?: ReactNode
}
```

**尺寸：** sm(400px) | md(560px) | lg(720px) | xl(960px) | full(100vw×100vh)

---

### 4.4 Toast（轻提示）

**类型：** `success` | `error` | `warning` | `info`

**位置：** 右上角（`position: fixed; top: 24px; right: 24px`）

**行为：**
- 自动消失（默认 4 秒，可配置 `duration`）
- 手动关闭（X 按钮）
- 堆叠展示（垂直排列，间距 8px）
- 动画：从右滑入（translateX 100%→0，300ms ease-out）

**品牌化文案（P0-01 修复）：**
```
LLM error → "潮汐异常"
network timeout → "涟漪未至"
rate limit → "水位暂歇"
zhipu error → "潮汐暂歇"
```

**API：**
```typescript
interface ToastProps {
  type: 'success' | 'error' | 'warning' | 'info'
  message: string
  duration?: number  // ms，默认 4000
  action?: { label: string; onClick: () => void }
}

// 全局调用
toast.success('已保存')
toast.error('潮汐异常，请稍候重试')
toast.info('新节点已创建')
```

---

### 4.5 Tooltip（工具提示）

**触发：** hover / focus

**位置：** 自动翻转（优先上方，空间不足则换方向）

**样式：** 深灰背景（`#1f2937`），白字，圆角 6px，箭头指向触发元素，青碧光晕阴影

**延迟：** 500ms 后显示

**品牌化文案（P0-03 修复）：**
```
删除 → "蒸发（删除）"
复制 → "分蘖（复制）"
历史 → "涟漪（历史）"
保存 → "固形（保存）"
```

---

### 4.6 Sidebar（侧边栏）

**视觉：**
- 宽度 240px（可折叠为 60px）
- 背景 `#f1f5f9`，左侧 3px 青碧渐变条
- 导航项 hover：`#e0f7f8` 背景
- 导航项选中：青碧底 + 青碧文字 + 左侧 3px 边框

**结构：** Logo区 | 导航区 | 可折叠区 | 用户区

---

### 4.7 Panel（面板）

**视觉：** 背景 `#ffffff`，圆角 12px，`float` 阴影（青碧光晕）

**结构：** PanelHeader（标题 + Tab）| PanelBody（flex:1）| PanelFooter（翻页/操作）

**行为：** 可拖拽调整宽度（从左侧边缘）

---

### 4.8 Drawer（抽屉）

**位置：** 从右侧滑入

**动画：** translateX 100%→0，300ms ease-out

**API：**
```typescript
interface DrawerProps {
  open: boolean
  onClose: () => void
  title?: string
  width?: number | string  // 默认 480px
  placement?: 'left' | 'right'  // 默认 right
  children?: ReactNode
}
```

---

### 4.9 Card（卡片）

**视觉：** 背景 `#ffffff`，圆角 12px，无阴影静态，hover 时 `shadow.md` 浮起

**变体：**
- `default` — 纯白卡片
- `bordered` — 更强边框
- `highlighted` — 左侧青碧条（用于推荐/热点头条）

**内容布局：** CardHeader | CardBody | CardFooter

---

## 五、动效系统

### 5.1 过渡时长变量

```css
--transition-fast:   150ms;   /* hover / 颜色切换 */
--transition-base:  200ms;   /* 展开/收起 */
--transition-slow:   300ms;   /* 面板滑入 / Modal 淡入 */
--transition-spring: 400ms;   /* 弹跳效果 cubic-bezier(0.34, 1.56, 0.64, 1) */
```

### 5.2 涟漪动效（Ripple）

点击按钮时从点击位置扩散水波纹：
```css
@keyframes ripple {
  0%   { transform: scale(0); opacity: 0.5; }
  100% { transform: scale(4); opacity: 0; }
}
/* 600ms，ease-out */
```
应用于：Button、Card 头部。

### 5.3 滑入动效（Slide）

Drawer / Toast 从右滑入，Modal 从中心缩放淡入：
```css
@keyframes slideInRight {
  from { transform: translateX(100%); opacity: 0; }
  to   { transform: translateX(0);    opacity: 1; }
}
```

### 5.4 呼吸动效（Breathe）

热节点/活跃元素的持续发光：
```css
@keyframes breathe {
  0%, 100% { box-shadow: 0 0 0 0 rgba(46,139,144,0.4); }
  50%       { box-shadow: 0 0 12px 4px rgba(46,139,144,0.2); }
}
/* 2s infinite，ease-in-out */
```
应用于：热点 Tab 排名节点。

### 5.5 Reduce Motion

```css
@media (prefers-reduced-motion: reduce) {
  *, *::before, *::after {
    animation-duration: 0.01ms !important;
    transition-duration: 0.01ms !important;
  }
}
```

---

## 六、目录结构

```
frontend/src/
├── styles/
│   ├── tokens.ts          # 扩展：light + semantic tokens
│   ├── globals.css         # 更新 CSS 变量，引入动效
│   └── animations.css      # 新增：涟漪/呼吸/滑入动画
├── components/ui/          # 新设计系统组件库
│   ├── Button/
│   │   ├── Button.tsx
│   │   └── Button.module.css
│   ├── Input/
│   ├── Modal/
│   ├── Toast/
│   ├── Tooltip/
│   ├── Sidebar/
│   ├── Panel/
│   ├── Drawer/
│   ├── Card/
│   └── index.ts            # 统一导出
└── components/graph/       # 现有组件（待逐步替换）
```

---

## 七、迁移策略

### 双轨并行

- **新轨道：** 在 `components/ui/` 目录下并行建设新组件库
- **旧轨道：** 现有组件继续工作，不改动
- **替换时机：** 新组件完成一个 → 替换一个 → 验证功能正常

### 替换优先级

```
第一阶段（P0）：
  Button  → 解决 P0-02（按钮过多问题）
  Toast   → 解决 P0-01（错误提示暴露技术细节）
  Tooltip → 解决 P0-03（图标缺少 Tooltip）
  Modal   → 基础组件

第二阶段（P1）：
  Input  → 表单体验优化
  Card   → 节点卡片重设计
  Panel  → 图谱分析面板统一化
  Drawer → 详情面板统一化
  Sidebar → 导航重设计
```

---

## 八、与 tokens.ts 的关系

现有 `frontend/src/styles/tokens.ts` 中 `catppuccin` 部分逐步弃用，新组件只引用新的语义 Token。

---

## 九、验收标准

- [ ] Token 系统完整覆盖颜色/间距/圆角/阴影/动效
- [ ] 9 个组件全部实现并通过 TypeScript 编译
- [ ] P0-01（Toast 品牌化文案）修复
- [ ] P0-03（Tooltip 含水文术语文案）修复
- [ ] 所有组件支持 keyboard 导航
- [ ] `prefers-reduced-motion` 媒体查询生效
- [ ] 现有组件不受影响（双轨并行）
