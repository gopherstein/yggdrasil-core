import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render, waitFor } from '@testing-library/react'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { api } from '@/lib/api'
import { notifyDesktopBackgroundMode } from '@/lib/desktopBridge'
import { ScheduleBackgroundSync } from './ScheduleBackgroundSync'

vi.mock('@/lib/api', () => ({
  api: {
    listAutomations: vi.fn(),
    getSettings: vi.fn(),
    updateSettings: vi.fn(),
  },
}))

vi.mock('@/lib/desktopBridge', () => ({
  notifyDesktopBackgroundMode: vi.fn(),
}))

function renderSync() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  return render(
    <QueryClientProvider client={client}>
      <ScheduleBackgroundSync />
    </QueryClientProvider>,
  )
}

describe('ScheduleBackgroundSync', () => {
  beforeEach(() => {
    vi.mocked(api.listAutomations).mockReset()
    vi.mocked(api.getSettings).mockReset()
    vi.mocked(api.updateSettings).mockReset()
    vi.mocked(notifyDesktopBackgroundMode).mockReset()
    vi.mocked(api.updateSettings).mockResolvedValue({} as never)
  })

  it('enables background mode when a schedule exists', async () => {
    vi.mocked(api.listAutomations).mockResolvedValue([
      { id: 'auto-1', name: 'Price', enabled: true } as never,
    ])
    vi.mocked(api.getSettings).mockResolvedValue({ keep_running_in_background: false } as never)

    renderSync()

    await waitFor(() => expect(api.updateSettings).toHaveBeenCalledWith({ keep_running_in_background: true }))
    await waitFor(() => expect(notifyDesktopBackgroundMode).toHaveBeenCalledWith(true))
  })

  it('leaves background mode alone when there are no schedules', async () => {
    vi.mocked(api.listAutomations).mockResolvedValue([])
    vi.mocked(api.getSettings).mockResolvedValue({ keep_running_in_background: false } as never)

    renderSync()

    await waitFor(() => expect(api.listAutomations).toHaveBeenCalled())
    expect(api.updateSettings).not.toHaveBeenCalled()
    expect(notifyDesktopBackgroundMode).not.toHaveBeenCalled()
  })
})
