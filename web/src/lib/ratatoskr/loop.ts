import type { Rig } from './rig'

/**
 * One requestAnimationFrame loop for every mounted rig. A rig that is
 * offscreen is skipped, and the loop stops entirely while the page is hidden,
 * so the mascot costs nothing when nobody can see him.
 */

const rigs = new Set<Rig>()
const visible = new Set<Rig>()
let observer: IntersectionObserver | null = null
let frame = 0
let last = 0

function hidden(): boolean {
  return typeof document !== 'undefined' && document.hidden
}

function tick(now: number) {
  frame = 0
  // Cap a frame at 50 ms, as the prototype does, so a slow frame or a tab
  // switch does not make him jump.
  const dt = last ? Math.min(0.05, (now - last) / 1000) : 1 / 60
  last = now
  visible.forEach((rig) => rig.step(dt))
  schedule()
}

function schedule() {
  if (frame || rigs.size === 0 || hidden() || typeof requestAnimationFrame === 'undefined') return
  frame = requestAnimationFrame(tick)
}

function stop() {
  if (frame && typeof cancelAnimationFrame !== 'undefined') cancelAnimationFrame(frame)
  frame = 0
  last = 0
}

function onVisibility() {
  if (hidden()) stop()
  else schedule()
}

function ensureObserver(): IntersectionObserver | null {
  if (observer || typeof IntersectionObserver === 'undefined') return observer
  observer = new IntersectionObserver((entries) => {
    for (const entry of entries) {
      const rig = [...rigs].find((r) => r.svg === entry.target)
      if (!rig) continue
      if (entry.isIntersecting) visible.add(rig)
      else visible.delete(rig)
    }
  })
  return observer
}

/** Animate a rig until the returned function is called. Still rigs are not animated. */
export function animate(rig: Rig): () => void {
  if (rig.still) return () => {}
  if (rigs.size === 0 && typeof document !== 'undefined') {
    document.addEventListener('visibilitychange', onVisibility)
  }
  rigs.add(rig)
  const io = ensureObserver()
  if (io) io.observe(rig.svg)
  else visible.add(rig)
  schedule()
  return () => {
    rigs.delete(rig)
    visible.delete(rig)
    observer?.unobserve(rig.svg)
    if (rigs.size === 0) {
      stop()
      if (typeof document !== 'undefined') document.removeEventListener('visibilitychange', onVisibility)
    }
  }
}

/** For tests: how many rigs are animating, and how many are on screen. */
export function loopStats(): { mounted: number; visible: number; running: boolean } {
  return { mounted: rigs.size, visible: visible.size, running: frame !== 0 }
}
