/**
 * P27: Node version diff panel — side-by-side line comparison of two revisions.
 * No external diff library; uses a simple LCS-based diff algorithm.
 */
import { useMemo, useState } from 'react'
import type { NodeRevision } from '../api/types'

interface Props {
  revisions: NodeRevision[]
  onClose: () => void
}

type DiffLine = { type: 'ctx' | 'add' | 'del' | 'empty'; text: string }

/** Compute LCS lengths table */
function lcsTable(a: string[], b: string[]): number[][] {
  const m = a.length, n = b.length
  const dp: number[][] = Array.from({ length: m + 1 }, () => new Array(n + 1).fill(0))
  for (let i = 1; i <= m; i++)
    for (let j = 1; j <= n; j++)
      dp[i][j] = a[i - 1] === b[j - 1] ? dp[i - 1][j - 1] + 1 : Math.max(dp[i - 1][j], dp[i][j - 1])
  return dp
}

/** Produce diff pairs: [left DiffLine, right DiffLine] */
function diffLines(oldText: string, newText: string): Array<[DiffLine, DiffLine]> {
  // Guard against very large texts to keep UI responsive
  const a = oldText.split('\n').slice(0, 2000)
  const b = newText.split('\n').slice(0, 2000)
  const dp = lcsTable(a, b)

  const left: DiffLine[] = []
  const right: DiffLine[] = []

  function backtrack(i: number, j: number) {
    if (i === 0 && j === 0) return
    if (i > 0 && j > 0 && a[i - 1] === b[j - 1]) {
      backtrack(i - 1, j - 1)
      left.push({ type: 'ctx', text: a[i - 1] })
      right.push({ type: 'ctx', text: b[j - 1] })
    } else if (j > 0 && (i === 0 || dp[i][j - 1] >= dp[i - 1][j])) {
      backtrack(i, j - 1)
      left.push({ type: 'empty', text: '' })
      right.push({ type: 'add', text: b[j - 1] })
    } else {
      backtrack(i - 1, j)
      left.push({ type: 'del', text: a[i - 1] })
      right.push({ type: 'empty', text: '' })
    }
  }

  backtrack(a.length, b.length)
  return left.map((l, i) => [l, right[i]])
}

const BG: Record<DiffLine['type'], string> = {
  ctx: 'transparent',
  add: 'rgba(82,196,26,0.15)',
  del: 'rgba(255,77,79,0.15)',
  empty: 'rgba(0,0,0,0.15)',
}
const FG: Record<DiffLine['type'], string> = {
  ctx: '#c0d8f0',
  add: '#95de64',
  del: '#ff7875',
  empty: '#2a4a6e',
}

export default function NodeVersionDiff({ revisions, onClose }: Props) {
  const sorted = [...revisions].sort((a, b) => b.rev_number - a.rev_number)
  const [leftNum, setLeftNum] = useState(sorted.length >= 2 ? sorted[1].rev_number : sorted[0]?.rev_number ?? 0)
  const [rightNum, setRightNum] = useState(sorted[0]?.rev_number ?? 0)

  const leftRev = revisions.find(r => r.rev_number === leftNum)
  const rightRev = revisions.find(r => r.rev_number === rightNum)

  const lines = useMemo<Array<[DiffLine, DiffLine]>>(() => {
    if (!leftRev || !rightRev) return []
    return diffLines(leftRev.content, rightRev.content)
  }, [leftRev, rightRev])

  return (
    <div style={{
      position: 'fixed', inset: 0, zIndex: 2100,
      background: 'rgba(0,0,0,0.75)',
      display: 'flex', alignItems: 'center', justifyContent: 'center',
      padding: 16,
    }} onClick={e => { if (e.target === e.currentTarget) onClose() }}>
      <div style={{
        background: '#0d1526', border: '1px solid #1e3050', borderRadius: 10,
        width: '100%', maxWidth: 900, maxHeight: '88vh',
        display: 'flex', flexDirection: 'column',
        overflow: 'hidden',
      }}>
        {/* header */}
        <div style={{
          display: 'flex', alignItems: 'center', gap: 12,
          padding: '12px 16px', borderBottom: '1px solid #1e3050',
          flexShrink: 0,
        }}>
          <span style={{ color: '#9ec5ee', fontWeight: 600, fontSize: 14 }}>版本对比</span>
          <div style={{ display: 'flex', alignItems: 'center', gap: 6 }}>
            <span style={{ color: '#4a6a8e', fontSize: 12 }}>左：</span>
            <select
              value={leftNum}
              onChange={e => setLeftNum(Number(e.target.value))}
              style={{ background: '#061020', border: '1px solid #2a4a6e', color: '#9ec5ee', borderRadius: 4, padding: '2px 6px', fontSize: 12 }}
            >
              {sorted.map(r => (
                <option key={r.rev_number} value={r.rev_number}>
                  rev {r.rev_number} ({new Date(r.created_at).toLocaleDateString('zh-CN')})
                </option>
              ))}
            </select>
          </div>
          <div style={{ display: 'flex', alignItems: 'center', gap: 6 }}>
            <span style={{ color: '#4a6a8e', fontSize: 12 }}>右：</span>
            <select
              value={rightNum}
              onChange={e => setRightNum(Number(e.target.value))}
              style={{ background: '#061020', border: '1px solid #2a4a6e', color: '#9ec5ee', borderRadius: 4, padding: '2px 6px', fontSize: 12 }}
            >
              {sorted.map(r => (
                <option key={r.rev_number} value={r.rev_number}>
                  rev {r.rev_number} ({new Date(r.created_at).toLocaleDateString('zh-CN')})
                </option>
              ))}
            </select>
          </div>
          <div style={{ marginLeft: 'auto', display: 'flex', gap: 10, fontSize: 11 }}>
            <span style={{ color: '#95de64' }}>■ 新增</span>
            <span style={{ color: '#ff7875' }}>■ 删除</span>
          </div>
          <button onClick={onClose} style={{ background: 'none', border: 'none', color: '#9ec5ee', fontSize: 18, cursor: 'pointer' }}>✕</button>
        </div>

        {/* diff body */}
        <div style={{ display: 'flex', flex: 1, overflow: 'hidden', fontFamily: 'monospace', fontSize: 12 }}>
          {/* left pane */}
          <div style={{ flex: 1, overflowY: 'auto', borderRight: '1px solid #1e3050' }}>
            <div style={{ padding: '6px 0', color: '#4a8eff', fontSize: 11, textAlign: 'center', borderBottom: '1px solid #1e3050' }}>
              rev {leftNum} {leftRev?.edit_reason ? `· ${leftRev.edit_reason}` : ''}
            </div>
            {lines.map(([l], i) => (
              <div key={i} style={{
                background: BG[l.type],
                color: FG[l.type],
                padding: '1px 10px',
                whiteSpace: 'pre-wrap', wordBreak: 'break-all',
                lineHeight: 1.6, minHeight: 22,
                borderLeft: l.type === 'del' ? '3px solid #ff7875' : l.type === 'ctx' ? '3px solid transparent' : '3px solid transparent',
              }}>
                {l.type === 'del' ? '−' : l.type === 'empty' ? '' : ' '} {l.text}
              </div>
            ))}
          </div>

          {/* right pane */}
          <div style={{ flex: 1, overflowY: 'auto' }}>
            <div style={{ padding: '6px 0', color: '#4a8eff', fontSize: 11, textAlign: 'center', borderBottom: '1px solid #1e3050' }}>
              rev {rightNum} {rightRev?.edit_reason ? `· ${rightRev.edit_reason}` : ''}
            </div>
            {lines.map(([, r], i) => (
              <div key={i} style={{
                background: BG[r.type],
                color: FG[r.type],
                padding: '1px 10px',
                whiteSpace: 'pre-wrap', wordBreak: 'break-all',
                lineHeight: 1.6, minHeight: 22,
                borderLeft: r.type === 'add' ? '3px solid #95de64' : r.type === 'ctx' ? '3px solid transparent' : '3px solid transparent',
              }}>
                {r.type === 'add' ? '+' : r.type === 'empty' ? '' : ' '} {r.text}
              </div>
            ))}
          </div>
        </div>
      </div>
    </div>
  )
}
