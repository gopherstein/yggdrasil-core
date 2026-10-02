import { afterEach, describe, expect, it } from 'vitest'
import i18n, { applyLanguage } from '@/i18n'
import { formatNumber, formatSequence } from './format'
import { pseudoRtlLocalize } from './pseudo'

afterEach(async () => {
  await applyLanguage('en')
  localStorage.clear()
})

const sources = import.meta.glob<string>(['../**/*.{ts,tsx}', '!../**/*.test.{ts,tsx}'], {
  eager: true,
  query: '?raw',
  import: 'default',
})

// Tailwind utilities for a physical side. Their logical forms (ms-, pe-,
// start-, text-end, border-s, rounded-e) mirror in a right-to-left language.
const physical =
  /(?<![\w/-])-?(?:(?:m[lr]|p[lr]|scroll-[mp][lr]|border-[lr]|rounded-(?:[lr]|[tb][lr]))(?:-[\w./[\]%-]+)?|text-(?:left|right)|float-(?:left|right)|(?:left|right)-[\w./[\]%-]+)(?![\w-])/g

// Centering on the page is the same in both directions.
const allowed = new Set(['left-1/2'])

describe('right-to-left layout', () => {
  it('uses logical sides, not left and right, in class names', () => {
    expect('flex -ml-2 pr-4 text-left right-0 border-l-primary sm:rounded-tl-lg'.match(physical)).toEqual([
      '-ml-2',
      'pr-4',
      'text-left',
      'right-0',
      'border-l-primary',
      'rounded-tl-lg',
    ])
    expect('ms-2 pe-4 text-start end-0 border-s-primary left this computer'.match(physical)).toBeNull()
    const found: string[] = []
    for (const [file, text] of Object.entries(sources)) {
      for (const literal of text.match(/"[^"\n]*"|'[^'\n]*'|`[^`]*`/g) ?? []) {
        // Class lists only: prose such as "left this computer" is not a class.
        if (!/(?<![\w-])(?:flex|grid|text|bg|border|rounded|absolute|relative|inline|block|px|py|gap|w|h)-?[\w[]/.test(literal)) continue
        for (const match of literal.match(physical) ?? []) {
          if (!allowed.has(match.replace(/^-/, ''))) found.push(`${file.replace('../', 'src/')}: ${match}`)
        }
      }
    }
    expect(found).toEqual([])
  })

  it('lays the page out right to left in ar-XB, with English words reversed', async () => {
    await applyLanguage('ar-XB')
    expect(document.documentElement.dir).toBe('rtl')
    expect(document.documentElement.lang).toBe('ar-XB')
    expect(i18n.t('nav.settings')).toBe('‮Settings‬')
    expect(formatSequence(['Planner', 'Worker'])).toBe('Planner ← Worker')
    expect(formatNumber(1234.5)).toBe('1,234.5')
    await applyLanguage('en')
    expect(document.documentElement.dir).toBe('ltr')
    expect(formatSequence(['Planner', 'Worker'])).toBe('Planner → Worker')
  })

  it('keeps placeholders and markup as they are', () => {
    expect(pseudoRtlLocalize('Open <link>Settings</link> for {{name}}')).toBe(
      '‮Open‬ <link>‮Settings‬</link> ‮for‬ {{name}}',
    )
  })
})
