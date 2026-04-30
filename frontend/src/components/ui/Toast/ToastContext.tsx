import React, {
  createContext,
  useContext,
  useCallback,
  useState,
  type ReactNode,
} from 'react'

export type ToastType = 'success' | 'error' | 'warning' | 'info'

export interface ToastItem {
  id: string
  type: ToastType
  message: string
  duration?: number
  action?: { label: string; onClick: () => void }
}

interface ToastContextValue {
  toasts: ToastItem[]
  success: (message: string, duration?: number) => void
  error: (message: string, duration?: number) => void
  warning: (message: string, duration?: number) => void
  info: (message: string, duration?: number) => void
  remove: (id: string) => void
}

// 品牌化错误文案映射（P0-01）
const ERROR_MAPPING: Record<string, string> = {
  'LLM error': '潮汐异常',
  'zhipu': '潮汐暂歇',
  'network timeout': '涟漪未至',
  'timeout': '涟漪未至',
  'rate limit': '水位暂歇',
  '429': '水位暂歇',
}

export function brandError(raw: string): string {
  for (const [key, value] of Object.entries(ERROR_MAPPING)) {
    if (raw.includes(key)) return value
  }
  return raw
}

const ToastContext = createContext<ToastContextValue | null>(null)

export const ToastProvider: React.FC<{ children: ReactNode }> = ({ children }) => {
  const [toasts, setToasts] = useState<ToastItem[]>([])

  const remove = useCallback((id: string) => {
    setToasts(prev => prev.filter(t => t.id !== id))
  }, [])

  const addToast = useCallback((type: ToastType, message: string, duration?: number) => {
    const id = Math.random().toString(36).slice(2)
    const item: ToastItem = { id, type, message, duration: duration ?? 4000 }
    setToasts(prev => [...prev, item])
    if (item.duration && item.duration > 0) {
      setTimeout(() => remove(id), item.duration)
    }
  }, [remove])

  const success = useCallback((message: string, duration?: number) =>
    addToast('success', message, duration), [addToast])

  const error = useCallback((message: string, duration?: number) =>
    addToast('error', brandError(message), duration), [addToast])

  const warning = useCallback((message: string, duration?: number) =>
    addToast('warning', message, duration), [addToast])

  const info = useCallback((message: string, duration?: number) =>
    addToast('info', message, duration), [addToast])

  return (
    <ToastContext.Provider value={{ toasts, success, error, warning, info, remove }}>
      {children}
    </ToastContext.Provider>
  )
}

export function useToast(): ToastContextValue {
  const ctx = useContext(ToastContext)
  if (!ctx) throw new Error('useToast must be used within ToastProvider')
  return ctx
}

// 全局 toast 实例（供非 React 上下文调用）
export const toast = {
  success: (_msg: string, _d?: number) =>
    console.warn('[toast] put <ToastProvider> at root to enable toast'),
  error: (_msg: string, _d?: number) =>
    console.warn('[toast] put <ToastProvider> at root to enable toast'),
  warning: (_msg: string, _d?: number) =>
    console.warn('[toast] put <ToastProvider> at root to enable toast'),
  info: (_msg: string, _d?: number) =>
    console.warn('[toast] put <ToastProvider> at root to enable toast'),
}