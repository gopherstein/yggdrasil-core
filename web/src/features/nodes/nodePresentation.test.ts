import { afterEach, describe, expect, it } from 'vitest'
import { applyLanguage } from '@/i18n'
import type { Model, Node } from '@/types/api'
import { availableForLabels, describeNodeHardware, onlineLabel, onlineState, teamBestAt } from './nodePresentation'

afterEach(async () => {
  await applyLanguage('en')
  localStorage.clear()
})

const coder = { id: 'coder', display_name: 'Coder', capabilities: { coding: true, tool_calling: true }, tags: ['large'] } as Model
const gb = 1024 ** 3

describe('node presentation', () => {
  it('names what a computer is good for, in order', () => {
    expect(availableForLabels([coder])).toEqual(['Coding', 'Tools', 'Large models'])
    expect(availableForLabels([], { memory: { total_bytes: 32 * gb } } as Node['hardware'])).toEqual([
      'General',
      'Coding',
      'Large models',
    ])
  })

  it('keeps the same abilities and order in another language', async () => {
    await applyLanguage('en-XA')
    const labels = availableForLabels([coder])
    expect(labels).toHaveLength(3)
    for (const label of labels) expect(label).toMatch(/^\[!! .* !!\]$/)
    const fleet = [{ id: 'a', hardware: undefined } as Node]
    const withModel = [{ ...coder, installed_on: [{ node_id: 'a', node_name: 'A' }] }] as Model[]
    expect(teamBestAt(fleet, withModel)).toHaveLength(3)
  })

  it('reads online state from the status, not the label', async () => {
    const node = { id: 'b', is_local: false, status: 'online' } as Node
    await applyLanguage('en-XA')
    expect(onlineState(node)).toBe('online')
    expect(onlineLabel(node)).toMatch(/^\[!! .* !!\]$/)
    expect(onlineState({ ...node, status: 'offline' })).toBe('offline')
    expect(onlineState({ ...node, status: 'unknown' })).toBe('checking')
  })

  it('describes hardware memory', () => {
    const view = describeNodeHardware({ cpu: { model: 'Apple M4' }, memory: { total_bytes: 24 * gb } } as Node['hardware'])
    expect(view.secondary).toBe('24 GB memory')
  })
})
