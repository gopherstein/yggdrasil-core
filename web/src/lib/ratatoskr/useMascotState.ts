import { useEffect, useRef, useState } from 'react'
import { subscribeEvents } from '@/lib/events'
import type { YggdrasilEvent } from '@/types/api'
import type { MascotState } from './rig'

/** How long success plays before he returns to idle, in milliseconds. */
export const SUCCESS_MS = 1800

/** "clear" ends a busy moment: a reply's text arrived, or it finished. */
export type MascotSignal = MascotState | 'clear'

/**
 * Maps one event to a mascot state, following the table in
 * docs/brand/mascot/README.md. It returns null for events that do not move
 * him. localNodeId tells placement on this computer from placement on a
 * paired one.
 */
export function mascotForEvent(event: Pick<YggdrasilEvent, 'type' | 'payload' | 'node_id'>, localNodeId?: string): MascotSignal | null {
  const payload = event.payload ?? {}
  switch (event.type) {
    case 'tool.started':
      return 'think'
    case 'plan.step':
      return payload.status === 'running' ? 'think' : null
    case 'scheduler.placement':
    case 'model.load.started': {
      const node = (payload.node_id as string | undefined) ?? event.node_id
      if (!node || !localNodeId) return null
      return node === localNodeId ? 'think' : 'deliver'
    }
    case 'chat.token':
    case 'chat.complete':
      return 'clear'
    case 'model.download.completed':
    case 'training.deployed':
    case 'node.paired':
      return 'success'
    case 'chat.error':
    case 'model.download.failed':
    case 'task.failed':
      return 'error'
    case 'model.unloaded':
      return 'sleep'
    default:
      return null
  }
}

export interface MascotStateOptions {
  /** What he shows when nothing is happening. Default idle. */
  base?: MascotState
  /** Work for this spot is under way, such as a reply being written: think, or deliver when it runs on a paired computer. */
  busy?: boolean
  /** No model is loaded. */
  asleep?: boolean
  /** Only these events count, for example one conversation's. Default all. */
  accept?: (event: YggdrasilEvent) => boolean
  /** Only these states may come from events. Default all. */
  react?: MascotState[]
  /** This computer's node id, so work placed elsewhere reads as deliver. */
  localNodeId?: string
  /** Play success once whenever this value changes to something truthy, such as a finished step. */
  celebrate?: unknown
}

/**
 * The mascot state for one spot, from the event stream and the spot's own
 * state. success plays once, then he returns to idle; error holds until the
 * spot is busy again.
 */
export function useMascotState({ base = 'idle', busy = false, asleep = false, accept, react, localNodeId, celebrate }: MascotStateOptions = {}): MascotState {
  const [fromEvent, setFromEvent] = useState<MascotState | null>(null)
  const timer = useRef<ReturnType<typeof setTimeout> | null>(null)
  const opts = useRef({ accept, react, localNodeId })
  opts.current = { accept, react, localNodeId }

  const show = (state: MascotState | null) => {
    if (timer.current) clearTimeout(timer.current)
    timer.current = null
    setFromEvent(state)
    if (state === 'success') {
      timer.current = setTimeout(() => {
        timer.current = null
        setFromEvent(null)
      }, SUCCESS_MS)
    }
  }

  useEffect(() => {
    const unsubscribe = subscribeEvents({
      onEvent: (event) => {
        const { accept: ok, react: allowed, localNodeId: local } = opts.current
        if (ok && !ok(event)) return
        const next = mascotForEvent(event, local)
        if (next === null) return
        if (next === 'clear') {
          setFromEvent((cur) => (cur === 'think' || cur === 'deliver' ? null : cur))
          return
        }
        if (allowed && !allowed.includes(next)) return
        show(next)
      },
    })
    return () => {
      unsubscribe()
      if (timer.current) clearTimeout(timer.current)
    }
  }, [])

  // A new piece of work starts fresh: an earlier error or delivery no longer applies.
  const wasBusy = useRef(busy)
  useEffect(() => {
    if (busy && !wasBusy.current) show(null)
    wasBusy.current = busy
  }, [busy])

  useEffect(() => {
    if (celebrate) show('success')
  }, [celebrate])

  return resolveMascotState({ fromEvent, base, busy, asleep })
}

/** Combines what events said with the spot's own state. */
export function resolveMascotState({ fromEvent, base = 'idle', busy = false, asleep = false }: { fromEvent: MascotState | null; base?: MascotState; busy?: boolean; asleep?: boolean }): MascotState {
  if (fromEvent === 'success' || fromEvent === 'error') return fromEvent
  if (busy) return fromEvent === 'deliver' ? 'deliver' : 'think'
  if (fromEvent === 'sleep' || asleep) return 'sleep'
  return base
}
