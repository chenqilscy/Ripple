/**
 * Ripple Design Tokens — 统一设计体系
 * 用法：在组件中 import { colors, space, radius, shadows } from './styles/tokens'
 * CSS 变量会自动注入到 :root，可在 globals.css 中覆盖或扩展。
 */
import './globals.css'

export const colors = {
  bg: {
    primary:   '#0d1526',   // 主背景：深蓝黑
    secondary: '#111827',   // 次级背景：侧边栏/面板
    tertiary:  '#181825',   // 第三级背景：卡片/设置区（Catppuccin Mocha Base）
    surface:   '#0d1b2a',   // 面板表面
    overlay:   'rgba(0,0,0,0.55)',  // 遮罩背景
    modal:     '#1f2330',   // Modal 专用背景
    input:     '#081020',   // 输入框背景
  },
  border: {
    primary:   '#1e3a5a',   // 主边框
    subtle:    'rgba(255,255,255,0.06)',  // 淡边框
    input:     '#1e3050',
    active:    '#4a8eff',
  },
  accent: {
    default:   '#4a8eff',   // 主强调色
    subtle:    'rgba(74,142,255,0.12)',  // 强调色淡底
    hover:     '#6ba3ff',   // hover 态
    disabled:  '#2d5278',
  },
  text: {
    primary:   '#c8d8e8',   // 主文字
    secondary: '#6a8aaa',   // 次级文字
    tertiary:  '#4a6a8e',   // 占位符/辅助
    inverse:   '#ffffff',
  },
  status: {
    success:   '#52c41a',
    warning:   '#f5a623',
    danger:    '#f5222d',
    info:      '#4a8eff',
  },
  // 保留原有 Catppuccin 配色（部分组件还在用）
  catppuccin: {
    text:      '#cdd6f4',
    subtext:   '#bac2de',
    surface0:   '#313244',
    surface1:   '#1e1e2e',
    surface2:   '#11111b',
    blue:      '#89b4fa',
    pink:      '#f5c2e7',
    green:     '#a6e3a1',
    teal:      '#94e2d5',
    red:       '#f38ba8',
    yellow:    '#f9e2af',
    overlay0:  '#6c7086',
  },
} as const

export const space = {
  xs: 4,
  sm: 8,
  md: 12,
  lg: 16,
  xl: 24,
  xxl: 32,
} as const

export const radius = {
  sm: 4,
  md: 6,
  lg: 10,
  xl: 12,
  full: 9999,
} as const

export const shadows = {
  card:  '0 4px 24px rgba(0,0,0,0.5)',
  overlay: '0 16px 48px rgba(0,0,0,0.6)',
  modal:  '0 18px 48px rgba(0,0,0,0.4)',
} as const

export const fontSize = {
  xs: 10,
  sm: 11,
  md: 12,
  base: 13,
  lg: 14,
  xl: 15,
  xxl: 16,
} as const

export const font = {
  body: "-apple-system, 'PingFang SC', 'Microsoft YaHei', sans-serif",
  mono: "Consolas, 'Fira Code', 'Source Han Mono', monospace",
} as const

// ─────────────────────────────────────────────
// Light Theme Tokens（浅色青碧主题）
// ─────────────────────────────────────────────
export const lightTheme = {
  colors: {
    // 青碧色系
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
    // 中性灰（浅色主题）
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
    },
  },

  // 间距
  space: {
    0: 0, 1: '4px', 2: '8px', 3: '12px',
    4: '16px', 6: '24px', 8: '32px',
    12: '48px', 16: '64px',
  },

  // 圆角
  radius: {
    none: '0', sm: '4px', md: '8px',
    lg: '12px', xl: '16px', full: '9999px',
  },

  // 阴影
  shadows: {
    sm:   '0 1px 2px rgba(0,0,0,0.05)',
    md:   '0 4px 6px -1px rgba(0,0,0,0.07), 0 2px 4px -1px rgba(0,0,0,0.04)',
    lg:   '0 10px 15px -3px rgba(0,0,0,0.08), 0 4px 6px -2px rgba(0,0,0,0.04)',
    xl:   '0 20px 25px -5px rgba(0,0,0,0.1), 0 10px 10px -5px rgba(0,0,0,0.04)',
    float: '0 8px 24px rgba(46,139,144,0.15)',
    modal: '0 24px 48px rgba(15,23,42,0.2)',
  },

  // 语义 Token（用途映射）
  semanticTokens: {
    'bg.primary':    '#f8fafc',
    'bg.secondary':  '#f1f5f9',
    'bg.tertiary':   '#ffffff',
    'bg.hover':     '#e0f7f8',
    'bg.overlay':   'rgba(46,139,144,0.15)',
    'text.primary':   '#0f172a',
    'text.secondary': '#475569',
    'text.tertiary':  '#94a3b8',
    'text.inverse':   '#ffffff',
    'border.default': '#e2e8f0',
    'border.subtle':  '#f1f5f9',
    'border.active':  '#2e8b90',
    'color.primary':       '#2e8b90',
    'color.primary.subtle': '#e0f7f8',
    'color.primary.hover':  '#1a6b72',
    'color.primary.active': '#0f4a52',
  },
} as const