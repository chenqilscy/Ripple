# Ripple 前端 UI/UX 修复计划

> 基于 2026-04-30 分析，覆盖除图谱（LakeGraph.tsx）之外的全部非图谱组件。
> 状态：`TODO` = 待修复，`DONE` = 已完成，`IN_PROGRESS` = 进行中

---

## 问题总览

| 类别 | 问题 | 影响 |
|---|---|---|
| 设计系统 | 3 套冲突色彩主题，无 CSS 变量 | 全局改色需改 15+ 文件 |
| 样式复用 | 全部内联 `React.CSSProperties`，零复用 | 代码膨胀，维护困难 |
| 布局 | 滚动边界失控、固定宽度无响应式 | 平板/手机体验差 |
| 语言 | OrgPanel 英中混用 | 用户体验割裂 |
| Accessibility | 大量按钮无 aria-label，焦点管理为零 | 不符合 WCAG |
| 动画 | 几乎无 CSS 过渡 | 交互反馈差 |

---

## P0 · 基础层：设计 Token

| 状态 | 任务 | 文件 |
|---|---|---|
| DONE | 建立 `src/styles/tokens.ts`：颜色、间距、字号 token | `src/styles/tokens.ts` |
| DONE | 建立 `src/styles/globals.css`：CSS 变量注入 | `src/styles/globals.css` |
| DONE | `main.tsx` 引入 globals.css | `src/main.tsx` |
| DONE | `index.html` / `App.tsx` 确保 CSS 变量全局生效 | `index.html` / `App.tsx` |

---

## P1 · 高影响 / 低成本

| 状态 | 任务 | 文件 |
|---|---|---|
| DONE | 模态框打开时锁定 body 滚动（Modal, SearchModal, NodeVersionHistory, ImportModal, NodeExplorer, OrgPanel, SubscriptionPanel, PromptTemplateManager, SpaceSwitcher） | 各组件 |
| DONE | SearchModal：键盘上下键导航 + 结果项高度一致 | `SearchModal.tsx` |
| DONE | NodeDetailPanel：窄屏 bottom sheet 模式 | `NodeDetailPanel.tsx` |
| DONE | ImportModal：预览区高度固定分配 + scroll lock | `ImportModal.tsx` |
| DONE | OrgPanel：Tab 内容 min-height 兜底 + 间距统一 | `OrgPanel.tsx` |
| DONE | SubscriptionPanel：套餐卡片响应式优化 | `SubscriptionPanel.tsx` |
| DONE | PromptTemplateManager：3 列 Grid 响应式 | `PromptTemplateManager.tsx` |
| DONE | SpaceSwitcher：Logo sticky + 列表独立滚动 | `SpaceSwitcher.tsx` |
| DONE | NodeVersionHistory：scroll lock + CSS 变量 | `NodeVersionHistory.tsx` |
| DONE | NodeExplorer：scroll lock + Deep Ocean Dark 主题替换 Catppuccin | `NodeExplorer.tsx` |
| DONE | AuditLogViewer：scroll lock + CSS 变量 + 表格 overflow | `AuditLogViewer.tsx` |
| DONE | AiTriggerButton：CSS 变量替换硬编码颜色 | `AiTriggerButton.tsx` |
| DONE | Modal：scroll lock + CSS 变量 | `Modal.tsx` |

---

## P2 · 中等成本

| 状态 | 任务 | 文件 |
|---|---|---|
| TODO | Home.tsx：侧栏响应式（窄屏收起）+ LakeHealth 可视化 | `Home.tsx` |
| DONE | 统一 OrgPanel 文案：Members → 成员 等 | `OrgPanel.tsx` |
| DONE | AiTriggerButton：按钮 hover 过渡动画 | `AiTriggerButton.tsx` |
| DONE | NodeExplorer：AI 摘要区与卡片间距 + scroll lock | `NodeExplorer.tsx` |
| DONE | AuditLogViewer：表格横屏滚动 + min-height | `AuditLogViewer.tsx` |

---

## P3 · 润色

| 状态 | 任务 | 文件 |
|---|---|---|
| PARTIAL | 关键按钮 hover 加 CSS 过渡 | 全局（已部分覆盖） |
| PARTIAL | 关键按钮/列表加 `aria-label` / `role` | 全局（已部分覆盖） |
| TODO | 加载状态骨架屏替代纯文字"加载中…" | 全局 |

---

## 设计 Token 规范

### 色彩（Deep Ocean Dark 主题）
```css
:root {
  --bg-primary: #0d1526;
  --bg-secondary: #111827;
  --bg-tertiary: #181825;
  --bg-surface: #0f1e35;
  --bg-card: #111d30;
  --bg-overlay: rgba(0,0,0,0.55);
  --bg-input: #0a1525;
  --border: #1e3a5a;
  --border-subtle: rgba(255,255,255,0.06);
  --border-input: #2a4a7a;
  --accent: #4a8eff;
  --accent-subtle: rgba(74,142,255,0.12);
  --accent-hover: #6ba3ff;
  --text-primary: #c8d8e8;
  --text-secondary: #6a8aaa;
  --text-tertiary: #4a6a8e;
  --text-inverse: #ffffff;
  --status-danger: #f5222d;
  --status-danger-subtle: rgba(245,34,45,0.1);
  --status-success: #52c41a;
  --status-success-subtle: rgba(82,196,26,0.1);
  --status-warning: #f5a623;
  --status-warning-subtle: rgba(245,166,35,0.1);
  --shadow-card: 0 4px 24px rgba(0,0,0,0.5);
  --shadow-overlay: 0 16px 48px rgba(0,0,0,0.6);
}
```

### 间距
```css
:root {
  --space-xs: 4px;
  --space-sm: 8px;
  --space-md: 12px;
  --space-lg: 16px;
  --space-xl: 24px;
  --space-2xl: 32px;
}
```

### 圆角 / 阴影
```css
:root {
  --radius-sm: 4px;
  --radius-md: 6px;
  --radius-lg: 10px;
  --radius-xl: 12px;
  --radius-full: 9999px;
}
```

### 字号
```css
:root {
  --font-xs: 11px;
  --font-sm: 12px;
  --font-md: 14px;
  --font-base: 14px;
  --font-lg: 16px;
  --font-xl: 18px;
}
```

### 字体
```css
:root {
  --font-body: system-ui, -apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif;
  --font-mono: 'Fira Code', 'JetBrains Mono', 'Cascadia Code', Consolas, monospace;
}
```

---

## 已修复组件汇总（共 11 个）

| 组件 | 修复项 |
|---|---|
| SearchModal | scroll lock, 键盘导航, CSS 变量 |
| NodeDetailPanel | scroll lock, 窄屏 bottom sheet, CSS 变量 |
| ImportModal | scroll lock, 预览区固定高度, CSS 变量 |
| OrgPanel | scroll lock, 中文标签, tab min-height, CSS 变量 |
| PromptTemplateManager | scroll lock, 响应式 3 列 Grid, CSS 变量 |
| SpaceSwitcher | scroll lock, sticky header, 独立滚动, CSS 变量 |
| NodeVersionHistory | scroll lock, CSS 变量 |
| NodeExplorer | scroll lock, Escape 关闭, Deep Ocean Dark 主题替换 Catppuccin |
| AuditLogViewer | scroll lock, CSS 变量, 表格 overflow |
| SubscriptionPanel | scroll lock, CSS 变量 |
| AiTriggerButton | CSS 变量, statusColor 函数 |
| Modal | scroll lock, CSS 变量 |

---

## 执行顺序

1. **P0**：建立设计 token → 所有组件逐步替换硬编码颜色
2. **P1**：scroll lock → 响应式 → 文案统一
3. **P2**：Home.tsx 响应式 → accessibility
4. **P3**：动画 + aria

---

*更新于 2026-04-30 | 全部非图谱组件已修复完成（Home.tsx 除外）*