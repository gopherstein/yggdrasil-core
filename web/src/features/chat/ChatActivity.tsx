import { Ratatoskr } from '@/components/ui/Ratatoskr'
import type { MascotState } from '@/lib/ratatoskr/rig'

/** Shown while a reply has not produced visible text yet. Ratatoskr thinks, or delivers when the reply runs on a paired computer. */
export function ChatActivity({ label, mascot = 'think' }: { label?: string | null; mascot?: MascotState }) {
  return (
    <div
      className="flex max-w-[min(42rem,85%)] items-center gap-3 rounded-2xl bg-raised/80 px-4 py-3"
      role="status"
      aria-live="polite"
      aria-label={label || 'Working'}
    >
      <Ratatoskr state={mascot} size={32} />
      <span className="text-sm text-ink-muted">{label || 'Working…'}</span>
    </div>
  )
}
