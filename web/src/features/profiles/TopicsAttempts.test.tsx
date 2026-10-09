import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'
import { api } from '@/lib/api'
import { TopicsAttempts } from './TopicsAttempts'

vi.mock('@/lib/api', () => ({ api: { getTopicAttempts: vi.fn(), markOnTopic: vi.fn() } }))

describe('TopicsAttempts (#345)', () => {
  it('shows counts, where from, and the messages, and marks one as on topic', async () => {
    const today = new Date()
    const day = `${today.getFullYear()}-${String(today.getMonth() + 1).padStart(2, '0')}-${String(today.getDate()).padStart(2, '0')}`
    vi.mocked(api.getTopicAttempts).mockResolvedValue({
      days: 30,
      total: 3,
      by_day: [{ day, count: 3 }],
      by_where: [
        { kind: 'portal', id: 'po1', name: 'Shop chat', count: 2 },
        { kind: 'key', id: 'k1', name: 'Website', count: 1 },
      ],
      attempts: [
        { id: 'a1', at: today.toISOString(), label: 'off_topic', message: 'Do you sell rims?', where: { kind: 'portal', id: 'po1', name: 'Shop chat' } },
      ],
    })
    vi.mocked(api.markOnTopic).mockResolvedValue(null as never)
    render(
      <QueryClientProvider client={new QueryClient()}>
        <TopicsAttempts profileId="p1" />
      </QueryClientProvider>,
    )
    expect(await screen.findByText('3 in the last 30 days')).toBeInTheDocument()
    expect(screen.getByRole('img', { name: '3 off-topic attempts in the last 30 days, by day' })).toBeInTheDocument()
    expect(screen.getByText('Portal Shop chat:', { exact: false })).toBeInTheDocument()
    expect(screen.getByText('API key Website:', { exact: false })).toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: 'Mark as on topic' }))
    await waitFor(() => expect(api.markOnTopic).toHaveBeenCalledWith('p1', 'a1'))
  })
})
