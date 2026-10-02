import { describe, expect, it } from 'vitest'
import { speakableText } from './speakableText'

describe('speakableText', () => {
  it('drops code, link addresses, and formatting marks', () => {
    const md = [
      '## Plan',
      '',
      'Use **pandas** and see [the docs](https://example.com).',
      '- First `step`',
      '1. Second step',
      '```python',
      'print("hi")',
      '```',
      '| a | b |',
      '|---|---|',
      '| 1 | 2 |',
    ].join('\n')
    expect(speakableText(md)).toBe('Plan\nUse pandas and see the docs.\nFirst step\nSecond step\na b\n1 2')
  })

  it('stops at a sentence when the answer is too long to read at once', () => {
    const text = speakableText('One sentence here. '.repeat(400))
    expect(text.length).toBeLessThanOrEqual(5000)
    expect(text.endsWith('.')).toBe(true)
  })
})
