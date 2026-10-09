import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'
import { api } from '@/lib/api'
import { RemoteAccessCard } from './RemoteAccessCard'

vi.mock('@/lib/api', () => ({
  api: { getSettings: vi.fn(), getRemoteAccess: vi.fn(), updateSettings: vi.fn() },
}))

describe('RemoteAccessCard (#456)', () => {
  it('turns access from anywhere on, shows the listener, and saves a forwarded address', async () => {
    vi.mocked(api.getSettings).mockResolvedValue({ remote_access_enabled: true, remote_access_port: 7333 } as never)
    vi.mocked(api.getRemoteAccess).mockResolvedValue({ enabled: true, port: 7333, listening: '[::]:7333', reachable: 'direct', mapped: '203.0.113.9:7333', mapped_by: 'upnp' })
    vi.mocked(api.updateSettings).mockResolvedValue({} as never)
    render(
      <QueryClientProvider client={new QueryClient()}>
        <RemoteAccessCard />
      </QueryClientProvider>,
    )
    expect(await screen.findByText('Reachable from anywhere at 203.0.113.9:7333. Your router opened the port (UPnP).')).toBeInTheDocument()
    fireEvent.click(screen.getByLabelText('Open a port on the router automatically'))
    await waitFor(() => expect(api.updateSettings).toHaveBeenCalledWith({ remote_access_port_mapping: false }))
    fireEvent.change(screen.getByLabelText('Forwarded address (optional)'), { target: { value: ' home.example.com ' } })
    fireEvent.click(screen.getByRole('button', { name: 'Save' }))
    await waitFor(() => expect(api.updateSettings).toHaveBeenCalledWith({ remote_access_port: 7333, remote_access_address: 'home.example.com' }))
    fireEvent.click(screen.getByRole('switch', { name: 'Access from anywhere' }))
    await waitFor(() => expect(api.updateSettings).toHaveBeenCalledWith({ remote_access_enabled: false }))
  })

  it("connects to an organization's relay, keeps the secret out of sight, and says how it's going", async () => {
    vi.mocked(api.getSettings).mockResolvedValue({
      remote_access_enabled: true,
      remote_access_port: 7333,
      remote_access_relay: 'relay.example.com',
      remote_access_relay_enrolled: true,
    } as never)
    vi.mocked(api.getRemoteAccess).mockResolvedValue({
      enabled: true,
      port: 7333,
      listening: '[::]:7333',
      reachable: 'relay',
      relay: { name: 'relay.example.com', state: 'connected' },
    })
    vi.mocked(api.updateSettings).mockClear().mockResolvedValue({} as never)
    render(
      <QueryClientProvider client={new QueryClient()}>
        <RemoteAccessCard />
      </QueryClientProvider>,
    )
    expect(await screen.findByText('Reachable from anywhere through relay.example.com.')).toBeInTheDocument()
    expect(screen.getByText('Connected to relay.example.com.')).toBeInTheDocument()
    const secret = screen.getByLabelText('Enrollment secret')
    expect(secret).toHaveValue('')
    expect(secret).toHaveAttribute('placeholder', 'Saved. Paste a new one to replace it.')
    fireEvent.change(secret, { target: { value: ' new-secret-0123456789abcdef ' } })
    fireEvent.click(screen.getByRole('button', { name: 'Connect' }))
    await waitFor(() => expect(api.updateSettings).toHaveBeenCalledWith({ remote_access_relay_secret: 'new-secret-0123456789abcdef' }))
    expect(secret).toHaveValue('')
    fireEvent.click(screen.getByRole('button', { name: "Use Toskar's relay instead" }))
    await waitFor(() => expect(api.updateSettings).toHaveBeenCalledWith({ remote_access_relay: '', remote_access_relay_secret: '' }))
  })
})
