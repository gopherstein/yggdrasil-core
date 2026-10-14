import { describe, expect, it } from 'vitest'
import { reportMailLink, reportText } from './report'

const report = {
  version: '1.9.0',
  commit: 'abcdef1234567890',
  contract: '1.32',
  os: 'darwin',
  arch: 'arm64',
  cpu: 'Apple M3 Pro',
  memory_gb: 36,
  gpus: ['Apple M3 Pro'],
  log: Array.from({ length: 40 }, (_, i) => `t${i} ERROR something went wrong ${'x'.repeat(200)}`),
}

describe('report text (#588)', () => {
  it('says what, where, and on what, scrubbed', () => {
    const text = reportText(
      {
        error: 'failed for mike@example.com',
        code: 'MODEL_LOAD_FAILED',
        where: '/chat',
        app: 'web',
      },
      'asked about weather',
      report,
      1,
    )
    expect(text).toContain('Where: /chat')
    expect(text).toContain('Code: MODEL_LOAD_FAILED')
    expect(text).toContain('Error: failed for <email>')
    expect(text).toContain('What I was doing:\nasked about weather')
    expect(text).toContain('Toskar 1.9.0 (abcdef123456), contract 1.32')
    expect(text).toContain('Computer: darwin arm64, Apple M3 Pro, 36 GB')
    expect(text).toContain('t39 ERROR')
    expect(text).not.toContain('t38 ERROR')
  })

  it('leaves out the log when there is none, and before Core answers', () => {
    expect(reportText({ error: '', where: '/', app: 'web' }, '', undefined)).toBe(
      'Toskar problem report\nWhere: /\nApp: web\nError: none shown',
    )
  })

  it('drops the oldest log lines to fit a mail link', () => {
    const problem = { error: 'boom', where: '/', app: 'web' }
    const link = reportMailLink('Toskar problem', (n) => reportText(problem, '', report, n), 40)
    expect(link.startsWith('mailto:info@yeix.io?subject=Toskar%20problem&body=')).toBe(true)
    expect(link.length).toBeLessThanOrEqual(6000)
    const body = decodeURIComponent(link.split('&body=')[1])
    expect(body).toContain('t39 ERROR')
    expect(body).not.toContain('t0 ERROR')
  })
})
