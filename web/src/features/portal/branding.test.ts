import { describe, expect, it } from 'vitest'
import { channels, prompts, readableOn, safeLogo, text, theme, themeVars } from './branding'

describe('portal branding (#205)', () => {
  it('takes only hex colours', () => {
    expect(channels('#0a7d6f')).toBe('10 125 111')
    expect(channels('#fff')).toBe('255 255 255')
    expect(channels('red')).toBeNull()
    expect(channels('#12345g')).toBeNull()
    expect(channels('#000; background: url(x)')).toBeNull()
  })

  it('picks text that reads on the accent', () => {
    expect(readableOn('255 255 255')).toBe('11 15 20')
    expect(readableOn('10 30 60')).toBe('255 255 255')
  })

  it('shows only web and image addresses as the logo', () => {
    expect(safeLogo('https://example.com/logo.png')).toBe('https://example.com/logo.png')
    expect(safeLogo('data:image/png;base64,iVBORw0KGgo=')).toBe('data:image/png;base64,iVBORw0KGgo=')
    expect(safeLogo('javascript:alert(1)')).toBeUndefined()
    expect(safeLogo('data:text/html;base64,PHNjcmlwdD4=')).toBeUndefined()
    expect(safeLogo('https://example.com/a" onerror="x')).toBeUndefined()
  })

  it('keeps up to six short suggested prompts', () => {
    const many = ['a', 'b', 'c', 'd', 'e', 'f', 'g']
    expect(prompts({ prompts: many })).toHaveLength(6)
    expect(prompts({ prompts: ['  hi  ', '', 3 as unknown as string] })).toEqual(['hi'])
    expect(prompts({})).toEqual([])
  })

  it('follows the visitor for a system theme, and sets the colours', () => {
    expect(theme({ theme: 'light' }, true)).toBe('light')
    expect(theme({}, false)).toBe('light')
    expect(theme({ theme: 'system' }, true)).toBe('dark')
    expect(themeVars({ accent: '#0a7d6f', background: 'nope' })).toEqual({
      '--rgb-primary': '10 125 111',
      '--rgb-primary-hover': '10 125 111',
      '--rgb-primary-fg': '255 255 255',
    })
    expect(text('  Welcome  ')).toBe('Welcome')
    expect(text(42, 'fallback')).toBe('fallback')
  })
})
