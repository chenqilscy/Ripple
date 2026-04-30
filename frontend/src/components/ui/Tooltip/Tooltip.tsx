import React, { useState, useRef, useCallback, type ReactNode } from 'react'
import styles from './Tooltip.module.css'

type Placement = 'top' | 'bottom' | 'left' | 'right'

export interface TooltipProps {
  /** Tooltip 显示文案 */
  label: string
  /** 子元素（触发器） */
  children: ReactNode
  /** 位置，默认上方 */
  placement?: Placement
  /** 延迟显示（毫秒），默认 500 */
  delay?: number
}

const Tooltip: React.FC<TooltipProps> = ({
  label,
  children,
  placement = 'top',
  delay = 500,
}) => {
  const [visible, setVisible] = useState(false)
  const timerRef = useRef<ReturnType<typeof setTimeout> | null>(null)

  const show = useCallback(() => {
    timerRef.current = setTimeout(() => setVisible(true), delay)
  }, [delay])

  const hide = useCallback(() => {
    if (timerRef.current) {
      clearTimeout(timerRef.current)
      timerRef.current = null
    }
    setVisible(false)
  }, [])

  return (
    <div
      className={styles.wrapper}
      onMouseEnter={show}
      onMouseLeave={hide}
      onFocus={show}
      onBlur={hide}
    >
      {children}
      <div
        className={`${styles.tooltip} ${styles[placement]} ${visible ? styles.visible : ''}`}
        role="tooltip"
      >
        {label}
      </div>
    </div>
  )
}

export default Tooltip

// 预置水文术语映射（P0-03）
export const BRAND_TOOLTIPS: Record<string, string> = {
  '删除':   '蒸发（删除）',
  '复制':   '分蘖（复制）',
  '历史':   '涟漪（历史）',
  '保存':   '固形（保存）',
  '分享':   '传递（分享）',
  '更多':   '更多操作',
  '详情':   '查看详情',
  '编辑':   '编辑内容',
  '刷新':   '刷新',
  '下载':   '导出',
  '蒸发':   '蒸发（删除）',
  '凝结':   '凝结（归档）',
  '凝露':   '凝露（恢复）',
}