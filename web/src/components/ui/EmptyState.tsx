import type { ReactNode } from 'react'
import { Ratatoskr } from '@/components/ui/Ratatoskr'
import type { MascotState } from '@/lib/ratatoskr/rig'

interface EmptyStateProps {
  title: string
  description: string
  action?: ReactNode
  /** Show Ratatoskr above the title. Use it only where no other mascot is in view. */
  mascot?: MascotState
}

export function EmptyState({ title, description, action, mascot }: EmptyStateProps) {
  return (
    <div className="empty-hero max-w-lg">
      {mascot ? <Ratatoskr state={mascot} size={96} className="-mb-1" /> : null}
      <h3 className="font-display text-xl font-semibold text-ink">{title}</h3>
      <p className="text-[15px] leading-relaxed text-ink-muted">{description}</p>
      {action ? <div className="pt-2">{action}</div> : null}
    </div>
  )
}
