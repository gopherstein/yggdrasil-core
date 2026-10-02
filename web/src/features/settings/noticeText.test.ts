import { afterEach, describe, expect, it } from 'vitest'
import { applyLanguage } from '@/i18n'
import type { AppNotification } from '@/types/api'
import { noticeText } from './notificationLabels'

afterEach(async () => {
  await applyLanguage('en')
  localStorage.clear()
})

const modelReady: Pick<AppNotification, 'title' | 'body' | 'message'> = {
  title: 'Model ready',
  body: 'Qwen 2.5 7B finished downloading and is ready to use.',
  message: {
    title: { key: 'notifications:notices.modelReady' },
    body: [{ key: 'notifications:notices.modelReadyBody', params: { model: 'Qwen 2.5 7B' } }],
  },
}

describe('notification text', () => {
  it('writes a notice from its message, in the App language', async () => {
    expect(noticeText(modelReady)).toEqual({ title: 'Model ready', body: 'Qwen 2.5 7B finished downloading and is ready to use.' })
    await applyLanguage('de')
    const german = noticeText(modelReady)
    expect(german.title).not.toBe('Model ready')
    expect(german.body).toContain('Qwen 2.5 7B')
  })

  it('joins sentences, and shows literal text as it is', async () => {
    const failed = {
      title: 'Price check',
      body: '',
      message: {
        title: { text: 'Price check' },
        body: [{ key: 'notifications:notices.automationFailedInARow', params: { count: 2 } }, { text: 'The page did not load.' }],
      },
    }
    expect(noticeText(failed)).toEqual({ title: 'Price check', body: 'Could not run (2 failures in a row): The page did not load.' })
  })

  it('shows a notice without a message as it came', () => {
    expect(noticeText({ title: 'Morning price', body: 'The laptop is $420.' })).toEqual({ title: 'Morning price', body: 'The laptop is $420.' })
  })
})
