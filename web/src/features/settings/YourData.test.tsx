import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render, screen } from '@testing-library/react'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { applyLanguage } from '@/i18n'
import { api } from '@/lib/api'
import type { PrivacyOverview } from '@/types/api'
import { YourData } from './YourData'

vi.mock('@/lib/api', () => ({ api: { getPrivacy: vi.fn() } }))

function renderIt(overview: Partial<PrivacyOverview>) {
  vi.mocked(api.getPrivacy).mockResolvedValue({ retention_days: 30, last_30_days: {}, ...overview })
  return render(
    <QueryClientProvider client={new QueryClient({ defaultOptions: { queries: { retry: false } } })}>
      <YourData />
    </QueryClientProvider>,
  )
}

describe('YourData', () => {
  beforeEach(async () => {
    await applyLanguage('en')
  })

  it('says where data lives and what is encrypted', async () => {
    renderIt({})
    expect(screen.getByRole('heading', { name: 'Your data stays private' })).toBeInTheDocument()
    expect(screen.getByText(/stored on this computer/)).toBeInTheDocument()
    expect(screen.getByText(/between your own computers is encrypted/)).toBeInTheDocument()
    // Not known: the general advice.
    expect(await screen.findByText(/FileVault on macOS, BitLocker on Windows, or LUKS on Linux/)).toBeInTheDocument()
  })

  it('says the disk is encrypted when it is', async () => {
    renderIt({ disk_encryption: { state: 'on', method: 'FileVault' } })
    expect(await screen.findByText(/Your disk is encrypted with FileVault/)).toBeInTheDocument()
  })

  it('says how to turn encryption on when it is off', async () => {
    renderIt({ disk_encryption: { state: 'off', method: 'BitLocker' } })
    const point = await screen.findByText(/Your disk isn't encrypted/)
    expect(point).toHaveTextContent('Device encryption in Settings')
    expect(point).toHaveClass('text-warning')
  })
})
