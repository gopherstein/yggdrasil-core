import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { describe, expect, it, vi } from 'vitest'
import { api } from '@/lib/api'
import { YourDevices } from './YourDevices'

vi.mock('@/lib/api', async () => {
  const actual = await vi.importActual<typeof import('@/lib/api')>('@/lib/api')
  return {
    ...actual,
    api: {
      listMyDevices: vi.fn(),
      disconnectMyDevice: vi.fn(),
      getMe: vi.fn(),
      getSettings: vi.fn(),
      startDevicePairing: vi.fn(),
      getDevicePairing: vi.fn(),
      cancelDevicePairing: vi.fn(),
    },
  }
})

function renderIt() {
  return render(
    <QueryClientProvider client={new QueryClient({ defaultOptions: { queries: { retry: false } } })}>
      <MemoryRouter>
        <YourDevices />
      </MemoryRouter>
    </QueryClientProvider>,
  )
}

describe('YourDevices (#206)', () => {
  it('lists the person’s devices and disconnects one', async () => {
    vi.mocked(api.listMyDevices).mockResolvedValue([
      { id: 'd1', name: 'Sam’s phone', prefix: 'ygg_', created_at: '', revoked: false, kind: 'device' },
    ])
    vi.mocked(api.disconnectMyDevice).mockResolvedValue(null)
    vi.spyOn(window, 'confirm').mockReturnValue(true)
    renderIt()
    expect(await screen.findByText('Sam’s phone')).toBeInTheDocument()
    expect(screen.getByText('Not used yet')).toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: 'Disconnect' }))
    await waitFor(() => expect(api.disconnectMyDevice).toHaveBeenCalledWith('d1'))
  })

  it('asks a Member to have network access turned on instead of turning it on', async () => {
    vi.mocked(api.listMyDevices).mockResolvedValue([])
    vi.mocked(api.getMe).mockResolvedValue({
      person: { id: 'sam', name: 'Sam', role: 'member', created_at: '', sign_in: true },
      via: 'session',
    })
    vi.mocked(api.getSettings).mockResolvedValue({ lan_api_enabled: false } as never)
    renderIt()
    expect(await screen.findByText('No devices are connected as you yet.')).toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: 'Connect a device' }))
    expect(await screen.findByText(/Ask an Admin or the Owner to turn it on/)).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: /Allow/ })).not.toBeInTheDocument()
  })
})
