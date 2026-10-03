import { afterEach, describe, expect, it, vi } from 'vitest'
import { api, localTimeZone } from './api'

afterEach(() => {
  vi.unstubAllGlobals()
})

// Answers use the person's date and time, so chat sends the browser's zone.
describe('chat requests', () => {
  it("send the browser's time zone", async () => {
    const fetchMock = vi.fn().mockResolvedValue(
      new Response(JSON.stringify({ conversation_id: 'c1' }), { status: 200, headers: { 'Content-Type': 'application/json' } }),
    )
    vi.stubGlobal('fetch', fetchMock)
    await api.sendChat({ conversation_id: 'c1', message: 'What day is it?' })
    const body = JSON.parse(fetchMock.mock.calls[0][1].body as string)
    expect(body.time_zone).toBe(Intl.DateTimeFormat().resolvedOptions().timeZone)
    expect(localTimeZone()).toBe(body.time_zone)
  })
})
