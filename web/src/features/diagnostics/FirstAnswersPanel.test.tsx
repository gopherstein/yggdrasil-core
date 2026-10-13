import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render, screen, waitFor, within } from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'
import { api } from '@/lib/api'
import type { FirstAnswersGroup } from '@/types/api'
import { FirstAnswersPanel, rowsByClient } from './FirstAnswersPanel'

const group = (client: string, cold: boolean, first: number, runs = 3, load?: number): FirstAnswersGroup => ({
  client,
  cold,
  runs,
  first_word_ms: first,
  first_word_p90_ms: first * 2,
  load_ms: load,
})

vi.mock('@/lib/api', () => ({ api: { getFirstAnswers: vi.fn() } }))

function renderPanel() {
  return render(
    <QueryClientProvider client={new QueryClient({ defaultOptions: { queries: { retry: false } } })}>
      <FirstAnswersPanel />
    </QueryClientProvider>,
  )
}

describe('rowsByClient', () => {
  it('pairs warm and cold per app, apps in a fixed order', () => {
    const rows = rowsByClient([group('web', false, 300), group('', false, 200), group('phone', true, 9000), group('phone', false, 500)])
    expect(rows.map((r) => r.client)).toEqual(['phone', 'web', ''])
    expect(rows[0].warm?.first_word_ms).toBe(500)
    expect(rows[0].cold?.first_word_ms).toBe(9000)
  })
})

describe('FirstAnswersPanel', () => {
  it('shows each app with ready and cold times', async () => {
    vi.mocked(api.getFirstAnswers).mockResolvedValue({
      since: '2026-10-05T00:00:00Z',
      groups: [group('phone', false, 480, 12), group('phone', true, 9100, 2, 8000)],
    })
    renderPanel()
    const row = (await screen.findByText('Phone')).closest('tr')!
    expect(within(row).getByText('480 ms')).toBeInTheDocument()
    expect(within(row).getByText('12 answers')).toBeInTheDocument()
    expect(within(row).getByText('2 answers · loading 8.0 s')).toBeInTheDocument()
  })

  // An older daemon, or a week without answers, must not break the page.
  it('renders nothing without groups', async () => {
    vi.mocked(api.getFirstAnswers).mockResolvedValue({} as never)
    const { container } = renderPanel()
    await waitFor(() => expect(api.getFirstAnswers).toHaveBeenCalled())
    expect(container).toBeEmptyDOMElement()
  })
})
