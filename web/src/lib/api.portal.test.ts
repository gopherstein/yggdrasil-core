import { afterEach, describe, expect, it, vi } from 'vitest'
import { api, forgetApiKey, rememberApiKey, setPortal, setPortalSession } from './api'

afterEach(() => {
  setPortal('')
  forgetApiKey()
  vi.unstubAllGlobals()
})

// A chat portal's requests name the portal and carry no stored key, which
// would speak for whoever made it instead of the visitor (#205).
describe('portal requests', () => {
  it('name the portal and send no key', async () => {
    const fetchMock = vi.fn().mockImplementation(async () => new Response('[]', { status: 200 }))
    vi.stubGlobal('fetch', fetchMock)
    rememberApiKey('ygg_owner_key')

    await api.getConversations()
    expect(fetchMock.mock.calls[0][1].headers.Authorization).toBe('Bearer ygg_owner_key')

    setPortal('support')
    await api.getConversations()
    const headers = fetchMock.mock.calls[1][1].headers
    expect(headers['X-Toskar-Portal']).toBe('support')
    expect(headers.Authorization).toBeUndefined()
  })

  it('send a framed guest\'s session in a header (#205)', async () => {
    const fetchMock = vi.fn().mockImplementation(async () => new Response('[]', { status: 200 }))
    vi.stubGlobal('fetch', fetchMock)
    setPortal('shop')
    setPortalSession('sess-1')
    await api.getConversations()
    expect(fetchMock.mock.calls[0][1].headers['X-Toskar-Portal-Session']).toBe('sess-1')
    setPortal('shop')
    await api.getConversations()
    expect(fetchMock.mock.calls[1][1].headers['X-Toskar-Portal-Session']).toBeUndefined()
  })
})
