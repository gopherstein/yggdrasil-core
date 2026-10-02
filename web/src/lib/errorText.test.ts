import { afterEach, describe, expect, it } from 'vitest'
import { applyLanguage } from '@/i18n'
import { errorText } from './api'

afterEach(async () => {
  await applyLanguage('en')
  localStorage.clear()
})

describe('error text', () => {
  it('names a language by its name in the App language', async () => {
    expect(errorText('SPEECH_NO_VOICE', 'there is no voice', { language: 'ja' })).toBe(
      'There is no voice to read Japanese aloud yet.',
    )
    await applyLanguage('de')
    expect(errorText('SPEECH_NO_VOICE', 'there is no voice', { language: 'ja' })).toContain('Japanisch')
  })
})
