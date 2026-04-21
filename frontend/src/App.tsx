import { LakeCanvas } from './canvas/LakeCanvas'

export function App() {
  return (
    <div style={{ width: '100vw', height: '100vh', position: 'relative' }}>
      <div style={{ position: 'absolute', top: 16, left: 16, zIndex: 10, opacity: 0.8 }}>
        <strong>青萍 · Ripple</strong> · M1 骨架
      </div>
      <LakeCanvas />
      <div style={{ position: 'absolute', bottom: 16, left: 16, zIndex: 10, fontSize: 12, opacity: 0.5 }}>
        此处风平浪静，等待第一缕青萍。
      </div>
    </div>
  )
}
