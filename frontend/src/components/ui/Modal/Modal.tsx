import React, { useEffect, useCallback, type ReactNode } from 'react'
import { createPortal } from 'react-dom'
import styles from './Modal.module.css'

export interface ModalProps {
  open: boolean
  onClose: () => void
  title?: string
  description?: string
  size?: 'sm' | 'md' | 'lg' | 'xl' | 'full'
  closeOnOverlayClick?: boolean
  footer?: ReactNode
  footerLeft?: ReactNode
  children?: ReactNode
}

const Modal: React.FC<ModalProps> = ({
  open,
  onClose,
  title,
  description,
  size = 'md',
  closeOnOverlayClick = true,
  footer,
  footerLeft,
  children,
}) => {
  // ESC 键关闭
  const handleKeyDown = useCallback((e: KeyboardEvent) => {
    if (e.key === 'Escape') onClose()
  }, [onClose])

  // 滚动锁定
  useEffect(() => {
    if (!open) return
    const original = document.body.style.overflow
    document.body.style.overflow = 'hidden'
    document.addEventListener('keydown', handleKeyDown)
    return () => {
      document.body.style.overflow = original
      document.removeEventListener('keydown', handleKeyDown)
    }
  }, [open, handleKeyDown])

  if (!open) return null

  const content = (
    <div
      className={styles.overlay}
      onClick={closeOnOverlayClick ? onClose : undefined}
      aria-modal="true"
      role="dialog"
      aria-labelledby={title ? 'modal-title' : undefined}
    >
      <div
        className={`${styles.modal} ${styles[size]}`}
        onClick={e => e.stopPropagation()}
      >
        {(title || description) && (
          <div className={styles.header}>
            <div>
              {title && (
                <h2 id="modal-title" className={styles.title}>{title}</h2>
              )}
              {description && (
                <p className={styles.description}>{description}</p>
              )}
            </div>
            <button
              className={styles.closeBtn}
              onClick={onClose}
              aria-label="关闭"
            >
              ✕
            </button>
          </div>
        )}
        <div className={styles.body}>{children}</div>
        {(footer || footerLeft) && (
          <div className={styles.footer}>
            {footerLeft && <div className={styles.footerLeft}>{footerLeft}</div>}
            {footer}
          </div>
        )}
      </div>
    </div>
  )

  return createPortal(content, document.body)
}

export default Modal