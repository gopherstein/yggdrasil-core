import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { act, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { api } from '@/lib/api'
import type { JoinToken, JoinTokenCreated } from '@/types/api'
import { JoinByCommand } from './JoinByCommand'

vi.mock('@/lib/api', () => ({
  ApiError: class extends Error {},
  api: { createJoinToken: vi.fn(), listJoinTokens: vi.fn(), revokeJoinToken: vi.fn() },
}))

const now = new Date('2026-10-02T12:00:00Z')
const created: JoinTokenCreated = {
  id: 'abcd1234',
  created_at: now.toISOString(),
  expires_at: new Date(now.getTime() + 15 * 60_000).toISOString(),
  status: 'active',
  token: 'ygj_abcd1234_secret',
  server: '192.168.1.10:7332',
  fingerprint: 'sha256:b855',
  command: 'yggctl join --server 192.168.1.10:7332 --token ygj_abcd1234_secret --fingerprint sha256:b855',
  install_command: 'curl -fsSL https://example/install.sh | sh -s -- join --server 192.168.1.10:7332',
  windows_command: '& ([scriptblock]::Create((irm https://example/install.ps1))) join -Server 192.168.1.10:7332',
}

function renderIt() {
  return render(
    <QueryClientProvider client={new QueryClient({ defaultOptions: { queries: { retry: false } } })}>
      <JoinByCommand onClose={() => {}} />
    </QueryClientProvider>,
  )
}

describe('JoinByCommand', () => {
  let tokens: JoinToken[] = []
  beforeEach(() => {
    vi.useFakeTimers({ shouldAdvanceTime: true, now })
    tokens = [{ id: 'old00000', created_at: '2026-10-01T12:00:00Z', expires_at: '2026-10-01T12:15:00Z', status: 'used', used_by: 'laptop' }]
    vi.mocked(api.listJoinTokens).mockImplementation(async () => tokens)
    vi.mocked(api.createJoinToken).mockImplementation(async () => {
      tokens = [created, ...tokens]
      return created
    })
    vi.mocked(api.revokeJoinToken).mockImplementation(async () => {
      tokens = tokens.map((x) => (x.id === created.id ? { ...x, status: 'revoked' } : x))
      return tokens[0]
    })
    Object.assign(navigator, { clipboard: { writeText: vi.fn().mockResolvedValue(undefined) } })
  })
  afterEach(() => vi.useRealTimers())

  it('makes a command for each kind of computer and copies it', async () => {
    renderIt()
    expect(api.createJoinToken).not.toHaveBeenCalled()
    fireEvent.click(screen.getByRole('button', { name: 'Make a join command' }))
    expect(await screen.findByText(created.command)).toBeInTheDocument()
    expect(screen.getByText(/Expires in 15:00/)).toBeInTheDocument()

    fireEvent.click(screen.getByRole('tab', { name: 'Install and join' }))
    expect(screen.getByText(created.install_command)).toBeInTheDocument()
    fireEvent.click(screen.getByRole('tab', { name: 'Windows' }))
    expect(screen.getByText(created.windows_command)).toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: 'Copy' }))
    await waitFor(() => expect(navigator.clipboard.writeText).toHaveBeenCalledWith(created.windows_command))
    expect(await screen.findByRole('button', { name: 'Copied' })).toBeInTheDocument()

    // The earlier command is listed, without its token.
    fireEvent.click(screen.getByText('1 recent command'))
    expect(screen.getByText(/used by laptop/)).toBeInTheDocument()
  })

  it('counts down, and says when the computer joined', async () => {
    renderIt()
    fireEvent.click(screen.getByRole('button', { name: 'Make a join command' }))
    await screen.findByText(created.command)
    await act(async () => {
      vi.advanceTimersByTime(61_000)
    })
    expect(screen.getByText(/Expires in 13:59/)).toBeInTheDocument()
    tokens = tokens.map((x) => (x.id === created.id ? { ...x, status: 'used', used_by: 'gpu-box' } : x))
    await act(async () => {
      vi.advanceTimersByTime(3_500)
    })
    expect(await screen.findByText('✓ gpu-box joined the team.')).toBeInTheDocument()
  })

  it('revokes an unused command', async () => {
    renderIt()
    fireEvent.click(screen.getByRole('button', { name: 'Make a join command' }))
    await screen.findByText(created.command)
    fireEvent.click(screen.getByRole('button', { name: 'Revoke' }))
    await waitFor(() => expect(api.revokeJoinToken).toHaveBeenCalledWith('abcd1234'))
    expect(await screen.findByText('Revoked')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Copy' })).toBeDisabled()
  })
})
