import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { fireEvent, render, screen, waitFor, within } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { api, ApiError } from '@/lib/api'
import type { Person } from '@/types/api'
import { PeoplePage } from './PeoplePage'

vi.mock('@/lib/api', async () => {
  const actual = await vi.importActual<typeof import('@/lib/api')>('@/lib/api')
  return {
    ApiError: actual.ApiError,
    api: {
      getMe: vi.fn(),
      listPeople: vi.fn(),
      addPerson: vi.fn(),
      changePerson: vi.fn(),
      personLink: vi.fn(),
      getSettings: vi.fn(),
      getNodes: vi.fn(),
      getApiTLS: vi.fn(),
    },
  }
})

const owner: Person = { id: 'p-owner', name: 'Ada', role: 'owner', created_at: '2026-10-01T00:00:00Z', sign_in: false }
const admin: Person = { id: 'p-admin', name: 'Grace', username: 'grace', role: 'admin', created_at: '2026-10-01T00:00:00Z', sign_in: true }
const kid: Person = { id: 'p-kid', name: 'Sam', role: 'member', created_at: '2026-10-02T00:00:00Z', sign_in: false }

function renderIt() {
  return render(
    <QueryClientProvider client={new QueryClient({ defaultOptions: { queries: { retry: false } } })}>
      <MemoryRouter initialEntries={['/people']}>
        <PeoplePage />
      </MemoryRouter>
    </QueryClientProvider>,
  )
}

describe('PeoplePage', () => {
  beforeEach(() => {
    vi.mocked(api.getSettings).mockResolvedValue({ lan_api_enabled: true, api_port: 7331 } as never)
    vi.mocked(api.getNodes).mockResolvedValue([{ id: 'local', is_local: true, address: '192.168.1.20:7332' }] as never)
    vi.mocked(api.getApiTLS).mockResolvedValue({ enabled: true } as never)
    vi.mocked(api.listPeople).mockResolvedValue([owner, admin, kid])
  })

  it('adds a person and shows the link to give them', async () => {
    vi.mocked(api.getMe).mockResolvedValue({ person: owner, via: 'local' })
    vi.mocked(api.addPerson).mockResolvedValue({
      person: { ...kid, id: 'p-new', name: 'Robin' },
      link: { path: '/invite/abc', kind: 'invite', expires_at: '2026-10-15T00:00:00Z' },
    })
    renderIt()
    expect(await screen.findByText('Sam')).toBeInTheDocument()
    expect(screen.getByText('@grace')).toBeInTheDocument()
    expect(screen.getByText('(you)')).toBeInTheDocument()

    fireEvent.change(screen.getByLabelText('Name'), { target: { value: 'Robin' } })
    fireEvent.change(screen.getByLabelText('Role'), { target: { value: 'visitor' } })
    fireEvent.click(screen.getByRole('button', { name: 'Add' }))
    await waitFor(() => expect(api.addPerson).toHaveBeenCalledWith('Robin', 'visitor'))
    expect(await screen.findByText(/Send Robin this link/)).toBeInTheDocument()
    expect(screen.getByText('https://192.168.1.20:7331/invite/abc')).toBeInTheDocument()
    expect(screen.queryByText(/until local network access is on/)).not.toBeInTheDocument()
  })

  it('lets an admin manage members but not other admins or the owner', async () => {
    vi.mocked(api.getMe).mockResolvedValue({ person: admin, via: 'session' })
    vi.mocked(api.changePerson).mockResolvedValue({ ...kid, role: 'visitor' })
    vi.mocked(api.personLink).mockResolvedValue({ path: '/invite/xyz', kind: 'invite', expires_at: '2026-10-15T00:00:00Z' })
    renderIt()
    await screen.findByText('Sam')
    expect(screen.queryByLabelText('Role for Ada')).not.toBeInTheDocument()
    expect(Array.from((screen.getByLabelText('Role') as HTMLSelectElement).options).map((o) => o.value)).toEqual(['member', 'visitor'])

    fireEvent.change(screen.getByLabelText('Role for Sam'), { target: { value: 'visitor' } })
    await waitFor(() => expect(api.changePerson).toHaveBeenCalledWith('p-kid', { role: 'visitor' }))

    const row = screen.getByText('Sam').closest('li') as HTMLElement
    fireEvent.click(within(row).getByRole('button', { name: 'Sign-in link' }))
    expect(await screen.findByText(/Send Sam this link/)).toBeInTheDocument()
    fireEvent.click(within(row).getByRole('button', { name: 'Disable' }))
    await waitFor(() => expect(api.changePerson).toHaveBeenCalledWith('p-kid', { disabled: true }))
  })

  it('warns that a link only opens here while local network access is off', async () => {
    vi.mocked(api.getSettings).mockResolvedValue({ lan_api_enabled: false, api_port: 7331 } as never)
    vi.mocked(api.getMe).mockResolvedValue({ person: owner, via: 'local' })
    vi.mocked(api.personLink).mockResolvedValue({ path: '/invite/xyz', kind: 'reset', expires_at: '2026-10-15T00:00:00Z' })
    renderIt()
    const row = (await screen.findByText('Grace')).closest('li') as HTMLElement
    fireEvent.click(within(row).getByRole('button', { name: 'New password link' }))
    expect(await screen.findByText(/Send Grace this link to choose a new password/)).toBeInTheDocument()
    expect(screen.getByText(/until local network access is on/)).toBeInTheDocument()
  })

  it('tells people who are not admins that only admins manage people', async () => {
    vi.mocked(api.getMe).mockResolvedValue({ person: kid, via: 'session' })
    vi.mocked(api.listPeople).mockRejectedValue(new ApiError(403, 'not allowed', 'ROLE_REQUIRED'))
    renderIt()
    expect(await screen.findByText('Only Admins and the Owner manage people.')).toBeInTheDocument()
    expect(screen.queryByLabelText('Name')).not.toBeInTheDocument()
  })
})
