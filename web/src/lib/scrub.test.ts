import { describe, expect, it } from 'vitest'
import { scrub } from './scrub'

describe('scrub (#588)', () => {
  it('takes out who and where', () => {
    expect(scrub('from mike@example.com with ygg_abc123')).toBe('from <email> with <key>')
    expect(scrub('Authorization: Bearer abc.def and password=hunter2')).toBe('Authorization: Bearer <hidden> and password=<hidden>')
    expect(scrub('open /Users/mike/Documents/x.pdf')).toBe('open /Users/<you>/Documents/x.pdf')
    expect(scrub('dial 8.8.8.8 from 192.168.1.4')).toBe('dial <public IPv4> from 192.168.1.4')
    expect(scrub('token aGVsbG8gd29ybGQgdGhpcyBpcyBhIHNlY3JldCB0b2tlbg9')).toBe('token <hidden>')
  })

  it('keeps what helps', () => {
    expect(scrub('API key invalid, model qwen2.5-7b-q4 at 18:16:04')).toBe('API key invalid, model qwen2.5-7b-q4 at 18:16:04')
    expect(scrub('run 211c1b31-3d9a-427a-bbe9-26a2f4a61135')).toBe('run 211c1b31-3d9a-427a-bbe9-26a2f4a61135')
  })
})
