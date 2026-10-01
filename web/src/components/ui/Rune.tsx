import { runePaths, type RuneId } from '@/lib/realms'

/** An Elder Futhark rune, drawn as strokes in the current text color. */
export function Rune({ id, className = 'h-4 w-2.5' }: { id: RuneId; className?: string }) {
  return (
    <svg
      viewBox="0 0 10 16"
      className={['shrink-0 overflow-visible', className].join(' ')}
      fill="none"
      stroke="currentColor"
      strokeWidth={1.6}
      strokeLinecap="round"
      strokeLinejoin="round"
      aria-hidden
    >
      <path d={runePaths[id]} />
    </svg>
  )
}
