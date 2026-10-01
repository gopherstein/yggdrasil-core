import { render, screen } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { ChatActivity } from '@/features/chat/ChatActivity'
import { loopStats } from '@/lib/ratatoskr/loop'
import { MASCOT_STATES } from '@/lib/ratatoskr/rig'
import { Ratatoskr } from './Ratatoskr'

function reducedMotion(on: boolean) {
  Object.defineProperty(window, 'matchMedia', {
    configurable: true,
    value: (query: string) => ({ matches: on && query.includes('reduce'), media: query, addEventListener() {}, removeEventListener() {} }),
  })
}

afterEach(() => {
  // @ts-expect-error jsdom has no matchMedia; tests add one.
  delete window.matchMedia
  window.history.replaceState(null, '', '/')
  vi.restoreAllMocks()
})

describe('Ratatoskr', () => {
  it('is hidden from assistive technology and sized as asked', () => {
    const { container } = render(<Ratatoskr state="idle" size={96} />)
    const svg = container.querySelector('svg')!
    expect(svg).toHaveAttribute('aria-hidden', 'true')
    expect(svg).toHaveAttribute('width', '96')
    expect(svg.querySelector('.char')).not.toBeNull()
  })

  it('is a still frame under reduced motion, the same every time', () => {
    reducedMotion(true)
    for (const state of MASCOT_STATES) {
      const a = render(<Ratatoskr state={state} size={96} />)
      const b = render(<Ratatoskr state={state} size={96} />)
      const strip = (html: string) => html.replace(/ratatoskr\d+g/g, 'g')
      expect(strip(a.container.innerHTML)).toBe(strip(b.container.innerHTML))
      a.unmount()
      b.unmount()
    }
    render(<Ratatoskr state="think" size={96} />)
    expect(loopStats().mounted).toBe(0)
  })

  it('is a still frame in screenshot mode', () => {
    window.history.replaceState(null, '', '/?screenshot=1&screen=chat')
    const { container } = render(<Ratatoskr state="error" size={48} />)
    expect(loopStats().mounted).toBe(0)
    // The still error frame has the acorn on the ground.
    expect(container.querySelector('.acorn')?.getAttribute('transform')).toMatch(/^translate\(142\.00 171\.00\)/)
  })

  it('animates otherwise, and stops when unmounted', () => {
    const { unmount } = render(<Ratatoskr state="idle" size={96} />)
    expect(loopStats().mounted).toBe(1)
    unmount()
    expect(loopStats().mounted).toBe(0)
  })
})

describe('ChatActivity', () => {
  it('still exposes its label next to the mascot', () => {
    render(<ChatActivity label="Searching the web…" mascot="deliver" />)
    const status = screen.getByRole('status', { name: 'Searching the web…' })
    expect(status).toHaveTextContent('Searching the web…')
    expect(status.querySelector('svg')).toHaveAttribute('aria-hidden', 'true')
    expect(status.querySelector('svg')).toHaveAttribute('data-mascot-state', 'deliver')
  })
})
