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