import React from 'react'
import { useToast, type ToastItem } from './ToastContext'
import styles from './Toast.module.css'

const TOAST_ICONS: Record<string, string> = {
  success: '✓',
  error:   '✕',
  warning: '⚠',
  info:    'ℹ',
}

const ToastContainer: React.FC = () => {
  const { toasts, remove } = useToast()

  return (
    <div
      className={styles.container}
      role="region"
      aria-label="通知"
      aria-live="polite"
    >
      {toasts.map((t: ToastItem) => (
        <div
          key={t.id}
          className={`${styles.toast} ${styles[t.type]} slide-in-right`}
          role="alert"
        >
          <span className={styles.icon} aria-hidden="true">
            {TOAST_ICONS[t.type]}
          </span>
          <span className={styles.message}>{t.message}</span>
          {t.action && (
            <button
              className={styles.action}
              onClick={t.action.onClick}
            >
              {t.action.label}
            </button>
          )}
          <button
            className={styles.close}
            onClick={() => remove(t.id)}
            aria-label="关闭通知"
          >
            ✕
          </button>
        </div>
      ))}
    </div>
  )
}

export { ToastContainer }
export { useToast, toast, ToastProvider } from './ToastContext'
export type { ToastItem, ToastType } from './ToastContext'
export default ToastContainer