import { useCallback, useEffect, useId, useLayoutEffect, useRef, useState, type ReactNode } from 'react'
import { createPortal } from 'react-dom'
import { Rune } from '@/components/ui/Rune'
import type { Lore } from '@/lib/lore'

const WIDTH = 288
const GAP = 10
const EDGE = 8

type Place = { left: number; top: number; tail: number; above: boolean }

/**
 * Wraps something with a lore entry: clicking it opens a small speech bubble
 * that says who or what it is in the myths and what it is in Yggdrasil.
 * The bubble is portalled so containers that clip, like the sidebar, cannot
 * cut it off. Escape, a click elsewhere, or the close button dismisses it.
 */
export function LoreButton({
  lore,
  children,
  label,
  className = '',
}: {
  lore: Lore
  children: ReactNode
  /** Accessible name when the children are not text, such as the mascot. */
  label?: string
  className?: string
}) {
  const [open, setOpen] = useState(false)
  const [place, setPlace] = useState<Place | null>(null)
  const button = useRef<HTMLButtonElement>(null)
  const bubble = useRef<HTMLDivElement>(null)
  const id = useId()

  const measure = useCallback(() => {
    const el = button.current
    if (!el) return
    const r = el.getBoundingClientRect()
    const vw = window.innerWidth
    const vh = window.innerHeight
    const width = Math.min(WIDTH, vw - EDGE * 2)
    const center = r.left + r.width / 2
    const left = Math.max(EDGE, Math.min(center - width / 2, vw - width - EDGE))
    const height = bubble.current?.offsetHeight ?? 220
    const above = r.bottom + GAP + height > vh && r.top - GAP - height > 0
    const top = above ? r.top - GAP - height : r.bottom + GAP
    setPlace({ left, top, tail: Math.max(16, Math.min(center - left, width - 16)), above })
  }, [])

  useLayoutEffect(() => {
    if (open) measure()
  }, [open, measure])

  useEffect(() => {
    if (!open) return
    bubble.current?.focus()
    const close = (restore: boolean) => {
      setOpen(false)
      if (restore) button.current?.focus()
    }
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape') close(true)
    }
    const onPointer = (e: PointerEvent) => {
      const t = e.target as Node
      if (!bubble.current?.contains(t) && !button.current?.contains(t)) close(false)
    }
    document.addEventListener('keydown', onKey)
    document.addEventListener('pointerdown', onPointer)
    window.addEventListener('resize', measure)
    window.addEventListener('scroll', measure, true)
    return () => {
      document.removeEventListener('keydown', onKey)
      document.removeEventListener('pointerdown', onPointer)
      window.removeEventListener('resize', measure)
      window.removeEventListener('scroll', measure, true)
    }
  }, [open, measure])

  return (
    <>
      <button
        ref={button}
        type="button"
        className={['lore-button', className].filter(Boolean).join(' ')}
        aria-label={label}
        aria-expanded={open}
        aria-haspopup="dialog"
        aria-controls={open ? id : undefined}
        onClick={() => setOpen((v) => !v)}
      >
        {children}
      </button>
      {open
        ? createPortal(
            <div
              ref={bubble}
              id={id}
              role="dialog"
              aria-label={`About ${lore.name}`}
              tabIndex={-1}
              className="lore-bubble"
              data-side={place?.above ? 'top' : 'bottom'}
              style={{
                width: Math.min(WIDTH, window.innerWidth - EDGE * 2),
                left: place?.left ?? -9999,
                top: place?.top ?? -9999,
                ['--lore-tail' as string]: `${place?.tail ?? WIDTH / 2}px`,
              }}
            >
              <div className="flex items-start gap-2.5">
                {lore.rune ? <Rune id={lore.rune} className="mt-1 h-5 w-3 text-primary" /> : null}
                <div className="min-w-0 flex-1">
                  <p className="font-display text-base font-semibold leading-tight text-ink">{lore.name}</p>
                  <p className="mt-0.5 text-[11px] text-ink-faint">{lore.kind}</p>
                </div>
                <button
                  type="button"
                  className="-mr-1 -mt-1 rounded px-1.5 text-lg leading-none text-ink-faint hover:text-ink"
                  aria-label="Close"
                  onClick={() => {
                    setOpen(false)
                    button.current?.focus()
                  }}
                >
                  ×
                </button>
              </div>
              <p className="mt-2.5 text-[13px] leading-relaxed text-ink">{lore.story}</p>
              <p className="label-caps mt-3 text-[10px]">In Yggdrasil</p>
              <p className="mt-1 text-[13px] leading-relaxed text-ink-muted">{lore.here}</p>
            </div>,
            document.body,
          )
        : null}
    </>
  )
}
