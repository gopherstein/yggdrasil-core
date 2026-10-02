import { useEffect, useRef } from 'react'
import { useTranslation } from 'react-i18next'
import { LoreButton } from '@/components/ui/LoreButton'
import { mascotLore } from '@/lib/lore'
import { animate } from '@/lib/ratatoskr/loop'
import { Rig, type MascotState } from '@/lib/ratatoskr/rig'
import { readScreenshotLaunch } from '@/lib/screenshotMode'

/** Below this size he is too small for blinks and particles to read. */
const DETAIL_PX = 48

function wantsStill(): boolean {
  if (readScreenshotLaunch()?.enabled) return true
  return typeof window !== 'undefined' && typeof window.matchMedia === 'function' && window.matchMedia('(prefers-reduced-motion: reduce)').matches
}

/**
 * Ratatoskr, the Yggdrasil mascot (docs/brand/mascot/README.md). He marks a
 * moment next to its text, never in place of it, so the SVG is hidden from
 * assistive technology. Show at most one per view.
 *
 * With reduced motion, or in screenshot mode, he is a still frame so
 * captures stay deterministic.
 */
export function Ratatoskr({
  state = 'idle',
  size = 96,
  className = '',
  lore = true,
}: {
  state?: MascotState
  size?: number
  className?: string
  /** Clicking him opens a short lore entry. Off where he must not take focus. */
  lore?: boolean
}) {
  const { t } = useTranslation('lore')
  const ref = useRef<SVGSVGElement>(null)
  const rig = useRef<Rig | null>(null)
  const detail = size >= DETAIL_PX

  useEffect(() => {
    const svg = ref.current
    if (!svg) return
    const r = new Rig(svg, state, { fx: detail, blinks: detail, still: wantsStill() })
    rig.current = r
    const stop = animate(r)
    return () => {
      stop()
      r.destroy()
      rig.current = null
    }
    // The rig is created once per size; state changes go through setState below.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [detail])

  useEffect(() => {
    if (rig.current && rig.current.state !== state) rig.current.setState(state)
  }, [state])

  const svg = (
    <svg
      ref={ref}
      width={size}
      height={size}
      aria-hidden="true"
      focusable="false"
      data-mascot-state={state}
      className={['ratatoskr shrink-0', lore ? '' : className].filter(Boolean).join(' ')}
    />
  )
  if (!lore) return svg
  return (
    <LoreButton lore={mascotLore()} label={t('mascot.label')} className={['inline-flex shrink-0', className].filter(Boolean).join(' ')}>
      {svg}
    </LoreButton>
  )
}
