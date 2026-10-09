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
    vi.mocked(api.getRemoteAccess).mockResolvedValue({ enabled: true, port: 7333, listening: '[::]:7333' })
    vi.mocked(api.updateSettings).mockResolvedValue({} as never)
    render(
      <QueryClientProvider client={new QueryClient()}>
        <RemoteAccessCard />
      </QueryClientProvider>,
    )
    expect(await screen.findByText('Listening on port 7333.')).toBeInTheDocument()
    fireEvent.change(screen.getByLabelText('Forwarded address (optional)'), { target: { value: ' home.example.com ' } })
    fireEvent.click(screen.getByRole('button', { name: 'Save' }))
    await waitFor(() => expect(api.updateSettings).toHaveBeenCalledWith({ remote_access_port: 7333, remote_access_address: 'home.example.com' }))
    fireEvent.click(screen.getByRole('switch', { name: 'Access from anywhere' }))
    await waitFor(() => expect(api.updateSettings).toHaveBeenCalledWith({ remote_access_enabled: false }))
  })
})
