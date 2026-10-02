import { describe, expect, it } from 'vitest'
import { displayChatText } from './displayChatText'

describe('displayChatText', () => {
  it('keeps the written answer and drops a leading tool call', () => {
    const raw = `{"tool_call":{"id":"terminal","args":{"command":"curl -s https://example.com"}}} A typical training plan keeps the work easy.`
    expect(displayChatText(raw)).toBe('A typical training plan keeps the work easy.')
  })

  it('does not show a reply that is only a tool call', () => {
    const raw = '```json\n{"tool_call":{"id":"filesystem.read","args":{"path":"README.md"}}}\n```'
    expect(displayChatText(raw)).toBe('')
  })

  it('leaves an ordinary JSON example intact', () => {
    const raw = 'Example:\n{"name":"Ada","year":1815}'
    expect(displayChatText(raw)).toContain('"name"')
    expect(displayChatText(raw)).toContain('1815')
  })

  it('leaves a fenced code sample intact', () => {
    const raw = '```json\n{"query":"not a tool"}\n```'
    expect(displayChatText(raw)).toContain('"query"')
  })

  it('drops a tool result envelope', () => {
    const raw = 'Juneau is cloudy.\n{"tool_result":{"error":"timeout"}}'
    expect(displayChatText(raw)).toBe('Juneau is cloudy.')
  })

  it('keeps the indentation of a nested list', () => {
    const raw = '- one\n- two\n  1. nested\n  2. also nested'
    expect(displayChatText(raw)).toBe(raw)
  })

  it('keeps the indentation inside a fenced code block', () => {
    const raw = '```python\ndef f():\n    if True:\n        return 1\n```'
    expect(displayChatText(raw)).toBe(raw)
  })

  it('collapses double spaces in the middle of a line', () => {
    expect(displayChatText('Juneau  is \t cloudy.')).toBe('Juneau is cloudy.')
  })
})
