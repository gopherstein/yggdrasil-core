import { describe, expect, it } from 'vitest'
import { capabilityGap } from './capabilityGap'
import type { Model } from '@/types/api'

function model(partial: Partial<Model> & Pick<Model, 'id' | 'display_name'>): Model {
  return {
    capabilities: { tool_calling: false, vision: false, coding: false },
    installed: false,
    ...partial,
  }
}

const llama = model({
  id: 'llama-3.2-3b-q4',
  display_name: 'Llama 3.2 3B',
  capabilities: { tool_calling: true, vision: false, coding: false },
  memory_needed_bytes: 300,
})
const gemma = model({
  id: 'gemma-2-2b-q4',
  display_name: 'Gemma 2 2B',
  memory_needed_bytes: 100,
})
const qwen = model({
  id: 'qwen2.5-3b-q4',
  display_name: 'Qwen 2.5 3B',
  capabilities: { tool_calling: true, vision: false, coding: false },
  installed: true,
  memory_needed_bytes: 200,
})
const coder = model({
  id: 'qwen2.5-coder-7b-q4',
  display_name: 'Qwen 2.5 Coder 7B',
  capabilities: { tool_calling: true, vision: false, coding: true },
  memory_needed_bytes: 400,
})
const llava = model({
  id: 'gemma-3-4b-q4',
  display_name: 'Gemma 3 4B',
  capabilities: { tool_calling: false, vision: true, coding: false },
})

const catalog = [llama, gemma, qwen, coder, llava]

describe('capabilityGap', () => {
  it('lets a tool-calling model look up the web', () => {
    expect(capabilityGap('What is the current weather in Juneau AK?', llama, catalog)).toBeNull()
  })

  it('suggests tool-calling models when this one cannot fetch the web', () => {
    const gap = capabilityGap('What is the current weather in Juneau AK?', gemma, catalog)
    expect(gap?.notes[0]?.text).toContain('tool-capable')
    expect(gap?.notes[0]?.suggestions[0]?.id).toBe('qwen2.5-3b-q4')
  })

  it('explains when Internet is turned off instead of swapping models', () => {
    const gap = capabilityGap('What is the current weather in Juneau AK?', llama, catalog, {
      internetAllowed: false,
    })
    expect(gap?.notes[0]?.text).toContain("isn't available")
    expect(gap?.notes[0]?.action).toBe('enable-internet')
    expect(gap?.notes[0]?.suggestions).toEqual([])
  })

  it('suggests tool-calling models when this one cannot use files', () => {
    const gap = capabilityGap('Read README.md in my repo', gemma, catalog)
    expect(gap?.notes[0]?.text).toContain('Gemma 2 2B')
    expect(gap?.notes[0]?.suggestions.map((item) => item.id)).toEqual([
      'qwen2.5-3b-q4',
      'llama-3.2-3b-q4',
      'qwen2.5-coder-7b-q4',
    ])
    expect(gap?.notes[0]?.suggestions[0]?.installed).toBe(true)
  })

  it('leaves ordinary chat alone when the model can already use tools', () => {
    expect(capabilityGap('Explain what a mutex is.', llama, catalog)).toBeNull()
    expect(capabilityGap('Read README.md', llama, catalog)).toBeNull()
  })

  it('suggests a vision model for an image request', () => {
    const gap = capabilityGap('Describe this screenshot', llama, catalog)
    expect(gap?.notes[0]?.suggestions.map((item) => item.name)).toEqual(['Gemma 3 4B'])
  })

  it('leaves an image request alone when a vision model is installed', () => {
    const installed = [llama, { ...llava, installed: true }]
    expect(capabilityGap('Describe this screenshot', llama, installed)).toBeNull()
  })
})
