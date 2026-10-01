import { act, renderHook } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import type { YggdrasilEvent } from '@/types/api'
import { SUCCESS_MS, mascotForEvent, resolveMascotState, useMascotState } from './useMascotState'

let emit: (event: YggdrasilEvent) => void = () => {}
vi.mock('@/lib/events', () => ({
  subscribeEvents: ({ onEvent }: { onEvent: (event: YggdrasilEvent) => void }) => {
    emit = onEvent
    return () => {}
  },
}))

const ev = (type: string, payload: Record<string, unknown> = {}): YggdrasilEvent => ({ id: type, type, timestamp: '', payload })

describe('mascotForEvent', () => {
  it('maps events to states as the README table lists', () => {
    expect(mascotForEvent(ev('tool.started'))).toBe('think')
    expect(mascotForEvent(ev('plan.step', { status: 'running' }))).toBe('think')
    expect(mascotForEvent(ev('plan.step', { status: 'done' }))).toBeNull()
    expect(mascotForEvent(ev('scheduler.placement', { node_id: 'peer' }), 'me')).toBe('deliver')
    expect(mascotForEvent(ev('scheduler.placement', { node_id: 'me' }), 'me')).toBe('think')
    expect(mascotForEvent(ev('scheduler.placement', { node_id: 'peer' }))).toBeNull()
    expect(mascotForEvent(ev('chat.token'))).toBe('clear')
    for (const type of ['model.download.completed', 'training.deployed', 'node.paired']) {
      expect(mascotForEvent(ev(type))).toBe('success')
    }
    for (const type of ['chat.error', 'model.download.failed', 'task.failed']) {
      expect(mascotForEvent(ev(type))).toBe('error')
    }
    expect(mascotForEvent(ev('model.unloaded'))).toBe('sleep')
    expect(mascotForEvent(ev('memory.saved'))).toBeNull()
  })

  it('combines events with the spot', () => {
    expect(resolveMascotState({ fromEvent: null })).toBe('idle')
    expect(resolveMascotState({ fromEvent: null, busy: true })).toBe('think')
    expect(resolveMascotState({ fromEvent: 'deliver', busy: true })).toBe('deliver')
    expect(resolveMascotState({ fromEvent: 'error', busy: true })).toBe('error')
    expect(resolveMascotState({ fromEvent: null, asleep: true })).toBe('sleep')
    expect(resolveMascotState({ fromEvent: null, base: 'greet' })).toBe('greet')
  })
})

describe('useMascotState', () => {
  beforeEach(() => vi.useFakeTimers())
  afterEach(() => vi.useRealTimers())

  it('plays success once, then returns to idle', () => {
    const { result } = renderHook(() => useMascotState())
    act(() => emit(ev('model.download.completed')))
    expect(result.current).toBe('success')
    act(() => vi.advanceTimersByTime(SUCCESS_MS + 10))
    expect(result.current).toBe('idle')
  })

  it('delivers while busy on a peer, and the reply clears it', () => {
    const { result, rerender } = renderHook(({ busy }) => useMascotState({ busy, localNodeId: 'me' }), {
      initialProps: { busy: true },
    })
    expect(result.current).toBe('think')
    act(() => emit(ev('model.load.started', { node_id: 'peer' })))
    expect(result.current).toBe('deliver')
    act(() => emit(ev('chat.token')))
    expect(result.current).toBe('think')
    rerender({ busy: false })
    expect(result.current).toBe('idle')
  })

  it('holds an error until the next piece of work', () => {
    const { result, rerender } = renderHook(({ busy }) => useMascotState({ busy }), { initialProps: { busy: false } })
    act(() => emit(ev('chat.error')))
    act(() => vi.advanceTimersByTime(10_000))
    expect(result.current).toBe('error')
    rerender({ busy: true })
    expect(result.current).toBe('think')
  })

  it('reacts only to accepted events and allowed states', () => {
    const { result } = renderHook(() =>
      useMascotState({ react: ['success'], accept: (e) => e.payload?.conversation_id !== 'other' }),
    )
    act(() => emit(ev('chat.error')))
    expect(result.current).toBe('idle')
    act(() => emit(ev('node.paired', { conversation_id: 'other' })))
    expect(result.current).toBe('idle')
  })

  it('celebrates when asked', () => {
    const { result, rerender } = renderHook(({ done }) => useMascotState({ base: 'greet', react: [], celebrate: done }), {
      initialProps: { done: false },
    })
    expect(result.current).toBe('greet')
    rerender({ done: true })
    expect(result.current).toBe('success')
  })
})
