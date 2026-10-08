import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { fireEvent, render, screen, waitFor, within } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { api } from '@/lib/api'
import type { Portal } from '@/types/api'
import { PortalsPage } from './PortalsPage'
import { slugFrom } from './slug'

vi.mock('@/lib/api', async () => {
  const actual = await vi.importActual<typeof import('@/lib/api')>('@/lib/api')
  return {
    ApiError: actual.ApiError,
    api: {
      listPortals: vi.fn(),
      createPortal: vi.fn(),
      updatePortal: vi.fn(),
      deletePortal: vi.fn(),
      getProfiles: vi.fn(),
      listPortalVisitors: vi.fn(),
      invitePortalVisitor: vi.fn(),
      portalVisitorLink: vi.fn(),
      removePortalVisitor: vi.fn(),
      getSettings: vi.fn(),
      getNodes: vi.fn(),
      getApiTLS: vi.fn(),
    },
  }
})

const portal: Portal = {
  id: 'p1', slug: 'support', name: 'Help desk', profile_id: '', tools: 'none', memory: false, language: '', access: 'passcode',
  has_passcode: true, branding: { welcome: 'Hi!' }, enabled: true, hourly_limit: 30, max_message: 2000, concurrency: 2,
  created_at: '', updated_at: '',
}

function renderIt() {
  return render(
    <QueryClientProvider client={new QueryClient({ defaultOptions: { queries: { retry: false } } })}>
      <MemoryRouter initialEntries={['/portals']}>
        <PortalsPage />
      </MemoryRouter>
    </QueryClientProvider>,
  )
}

describe('PortalsPage (#205)', () => {
  beforeEach(() => {
    vi.mocked(api.listPortals).mockResolvedValue([portal])
    vi.mocked(api.getProfiles).mockResolvedValue([{ id: 'general-assistant', name: 'General Assistant' }] as never)
    vi.mocked(api.getSettings).mockResolvedValue({ lan_api_enabled: true, api_port: 7331 } as never)
    vi.mocked(api.getNodes).mockResolvedValue([{ id: 'local', is_local: true, address: '192.168.1.20:7332' }] as never)
    vi.mocked(api.getApiTLS).mockResolvedValue({ enabled: false } as never)
  })

  it('makes an address from a name', () => {
    expect(slugFrom('Juneau Tire Help!')).toBe('juneau-tire-help')
    expect(slugFrom('Café Ñandú')).toBe('cafe-nandu')
    expect(slugFrom('  --  ')).toBe('')
  })

  it('adds a portal with an address from its name, then opens it for editing', async () => {
    vi.mocked(api.createPortal).mockResolvedValue({ ...portal, id: 'p2', slug: 'juneau-tire-help', name: 'Juneau Tire Help' })
    vi.mocked(api.listPortals).mockResolvedValueOnce([portal]).mockResolvedValue([portal, { ...portal, id: 'p2', slug: 'juneau-tire-help', name: 'Juneau Tire Help' }])
    renderIt()
    const add = (await screen.findByRole('heading', { name: 'Add a portal' })).closest('section') as HTMLElement
    fireEvent.change(within(add).getByLabelText('Name'), { target: { value: 'Juneau Tire Help' } })
    expect(within(add).getByLabelText('Address')).toHaveValue('juneau-tire-help')
    fireEvent.change(within(add).getByLabelText('Passcode'), { target: { value: 'tide-pool' } })
    fireEvent.click(within(add).getByRole('button', { name: 'Add portal' }))
    await waitFor(() =>
      expect(api.createPortal).toHaveBeenCalledWith({ name: 'Juneau Tire Help', slug: 'juneau-tire-help', access: 'passcode', passcode: 'tide-pool' }),
    )
    expect(await screen.findByRole('heading', { name: 'Edit Juneau Tire Help' })).toBeInTheDocument()
  })

  it('shows the link, turns a portal off, and saves its looks with a preview', async () => {
    vi.mocked(api.updatePortal).mockResolvedValue(portal)
    renderIt()
    expect(await screen.findByText('http://192.168.1.20:7331/p/support')).toBeInTheDocument()
    fireEvent.click(screen.getByRole('switch', { name: 'Help desk on' }))
    await waitFor(() => expect(api.updatePortal).toHaveBeenCalledWith('p1', { enabled: false }))

    fireEvent.click(screen.getByRole('button', { name: 'Edit' }))
    const editor = (await screen.findByRole('heading', { name: 'Edit Help desk' })).closest('section') as HTMLElement
    fireEvent.change(within(editor).getByLabelText('Heading'), { target: { value: 'Tire questions' } })
    fireEvent.change(within(editor).getByLabelText('Suggested prompts, one per line'), { target: { value: 'Winter tires?\n\nBook a rotation' } })
    fireEvent.change(within(editor).getByLabelText('Tools'), { target: { value: 'read_only' } })
    fireEvent.change(within(editor).getByLabelText('Messages per visitor an hour'), { target: { value: '10' } })
    expect(within(editor).getByText('Tire questions')).toBeInTheDocument() // the preview
    fireEvent.click(within(editor).getByRole('button', { name: 'Save' }))
    await waitFor(() =>
      expect(api.updatePortal).toHaveBeenLastCalledWith(
        'p1',
        expect.objectContaining({ tools: 'read_only', hourly_limit: 10, max_message: 2000, concurrency: 2, branding: expect.objectContaining({ title: 'Tire questions', prompts: ['Winter tires?', 'Book a rotation'], welcome: 'Hi!' }) }),
      ),
    )
    expect(api.updatePortal).not.toHaveBeenLastCalledWith('p1', expect.objectContaining({ passcode: expect.anything() }))
    expect(await within(editor).findByText('Saved')).toBeInTheDocument()
  })

  it('invites visitors to an invited portal, each with a link', async () => {
    const club: Portal = { ...portal, access: 'invited', has_passcode: false }
    vi.mocked(api.listPortals).mockResolvedValue([club])
    vi.mocked(api.listPortalVisitors).mockResolvedValue([])
    vi.mocked(api.invitePortalVisitor).mockResolvedValue({
      person: { id: 'g1', name: 'Robin', role: 'visitor', created_at: '', sign_in: false },
      link: { path: '/p/support?invite=tok-9', expires_at: '2026-10-15T00:00:00Z' },
    })
    renderIt()
    fireEvent.click(await screen.findByRole('button', { name: 'Edit' }))
    expect(await screen.findByText('No one invited yet.')).toBeInTheDocument()
    const section = screen.getByText('Invited visitors').closest('div') as HTMLElement
    fireEvent.change(within(section).getByLabelText('Name'), { target: { value: 'Robin' } })
    fireEvent.click(within(section).getByRole('button', { name: 'Invite' }))
    await waitFor(() => expect(api.invitePortalVisitor).toHaveBeenCalledWith('p1', 'Robin'))
    expect(await within(section).findByText('http://192.168.1.20:7331/p/support?invite=tok-9')).toBeInTheDocument()
  })
})
