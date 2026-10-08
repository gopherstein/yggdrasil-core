import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { api, ApiError, setPortal, streamChat } from '@/lib/api'
import type { PortalPageView } from '@/types/api'
import { PortalPage } from './PortalPage'

vi.mock('@/lib/api', async () => {
  const actual = await vi.importActual<typeof import('@/lib/api')>('@/lib/api')
  return {
    ApiError: actual.ApiError,
    errorText: actual.errorText,
    setPortal: vi.fn(),
    streamChat: vi.fn(),
    api: { getPortalPage: vi.fn(), enterPortal: vi.fn(), getMessages: vi.fn(), createConversation: vi.fn(), stopChat: vi.fn() },
  }
})

const guest = { person: { id: 'g1', name: 'Guest', role: 'visitor' as const, created_at: '', sign_in: false }, via: 'session' }

function page(over: Partial<PortalPageView> = {}): PortalPageView {
  return {
    slug: 'support',
    name: 'Help desk',
    access: 'passcode',
    language: 'en',
    entered: false,
    max_message: 2000,
    branding: { welcome: 'Hi! Ask about your order.', prompts: ['Where is my order?'], footer: 'Run by the Juneau shop', accent: '#0a7d6f' },
    ...over,
  }
}

function renderIt() {
  return render(
    <QueryClientProvider client={new QueryClient({ defaultOptions: { queries: { retry: false } } })}>
      <PortalPage slug="support" />
    </QueryClientProvider>,
  )
}

describe('PortalPage (#205)', () => {
  beforeEach(() => {
    vi.mocked(setPortal).mockClear()
    vi.mocked(api.enterPortal).mockReset()
    localStorage.clear()
  })

  it('asks for the passcode, then chats as the portal’s guest', async () => {
    vi.mocked(api.getPortalPage).mockResolvedValue(page())
    vi.mocked(api.enterPortal).mockRejectedValueOnce(new ApiError(401, 'wrong', 'PORTAL_PASSCODE')).mockResolvedValueOnce(guest)
    vi.mocked(api.createConversation).mockResolvedValue({ id: 'c1' } as never)
    vi.mocked(api.getMessages).mockResolvedValue([])
    // The reply streams after a moment, as the service's does, while the
    // new chat's id is known.
    vi.mocked(streamChat).mockImplementation(async ({ onToken }) => {
      await new Promise((resolve) => setTimeout(resolve, 20))
      onToken('It ships ')
      onToken('tomorrow.')
    })
    renderIt()
    expect(setPortal).toHaveBeenCalledWith('support')
    expect(await screen.findByRole('heading', { name: 'Help desk' })).toBeInTheDocument()
    expect(screen.getByText('Run by the Juneau shop')).toBeInTheDocument()

    fireEvent.change(screen.getByLabelText('Passcode'), { target: { value: 'nope' } })
    fireEvent.click(screen.getByRole('button', { name: 'Start chatting' }))
    expect(await screen.findByText("That passcode isn't right.")).toBeInTheDocument()
    fireEvent.change(screen.getByLabelText('Passcode'), { target: { value: 'tide-pool' } })
    fireEvent.click(screen.getByRole('button', { name: 'Start chatting' }))
    await waitFor(() => expect(api.enterPortal).toHaveBeenLastCalledWith('support', 'tide-pool', ''))

    expect(await screen.findByText('Hi! Ask about your order.')).toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: 'Where is my order?' }))
    expect(await screen.findByText('It ships tomorrow.')).toBeInTheDocument()
    expect(streamChat).toHaveBeenCalledWith(expect.objectContaining({ body: { conversation_id: 'c1', message: 'Where is my order?' } }))
    expect(localStorage.getItem('toskar.portal.support.conversation')).toBe('c1')
  })

  it('lets anyone into an open portal at once', async () => {
    vi.mocked(api.getPortalPage).mockResolvedValue(page({ access: 'open' }))
    vi.mocked(api.enterPortal).mockResolvedValue(guest)
    renderIt()
    expect(await screen.findByText('Hi! Ask about your order.')).toBeInTheDocument()
    expect(api.enterPortal).toHaveBeenCalledWith('support', '', '')
    expect(screen.queryByLabelText('Passcode')).not.toBeInTheDocument()
  })

  it('says when there’s no portal, or it’s off', async () => {
    vi.mocked(api.getPortalPage).mockResolvedValue(null)
    renderIt()
    expect(await screen.findByRole('heading', { name: "This chat isn't available" })).toBeInTheDocument()
  })

  it('says which limit stopped a message', async () => {
    vi.mocked(api.getPortalPage).mockResolvedValue(page({ access: 'open', entered: true }))
    vi.mocked(api.createConversation).mockResolvedValue({ id: 'c2' } as never)
    vi.mocked(streamChat).mockImplementation(async ({ onError }) => {
      onError?.("you've sent a lot of messages", 'PORTAL_RATE')
    })
    renderIt()
    fireEvent.click(await screen.findByRole('button', { name: 'Where is my order?' }))
    expect(await screen.findByRole('alert')).toHaveTextContent("You've sent a lot of messages. Try again later.")
    expect(screen.queryByText('Thinking…')).not.toBeInTheDocument()
    expect(screen.getByRole('textbox', { name: 'Message Help desk' })).toHaveAttribute('maxlength', '2000')
  })

  it('enters with an invitation link, and takes the token out of the address', async () => {
    window.history.pushState({}, '', '/p/support?invite=tok-1')
    vi.mocked(api.getPortalPage).mockResolvedValue(page({ access: 'invited' }))
    vi.mocked(api.enterPortal).mockResolvedValue(guest)
    renderIt()
    expect(await screen.findByText('Hi! Ask about your order.')).toBeInTheDocument()
    expect(api.enterPortal).toHaveBeenCalledWith('support', '', 'tok-1')
    expect(window.location.search).toBe('')
    window.history.pushState({}, '', '/')
  })

  it('says an invited portal needs its link, and a Members one a sign-in', async () => {
    vi.mocked(api.getPortalPage).mockResolvedValue(page({ access: 'invited' }))
    const { unmount } = renderIt()
    expect(await screen.findByText('Help desk is by invitation. Open the link you were sent.')).toBeInTheDocument()
    unmount()
    vi.mocked(api.getPortalPage).mockResolvedValue(page({ access: 'members' }))
    renderIt()
    expect(await screen.findByRole('link', { name: 'Sign in to chat' })).toHaveAttribute('href', '/?next=%2Fp%2Fsupport')
  })
})
