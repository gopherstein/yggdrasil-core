import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { fireEvent, render, screen } from '@testing-library/react'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { applyLanguage } from '@/i18n'
import { api } from '@/lib/api'
import { useUIStore } from '@/stores/uiStore'
import type { Conversation } from '@/types/api'
import { ChatCleanup } from './ChatCleanup'
import { olderChats } from './olderChats'

vi.mock('@/lib/api', () => ({ api: { getConversations: vi.fn(), deleteConversations: vi.fn() } }))

const day = 24 * 60 * 60 * 1000
const chat = (id: string, daysAgo: number): Conversation => {
  const at = new Date(Date.now() - daysAgo * day).toISOString()
  return { id, title: id, created_at: at, updated_at: at }
}
const chats = [chat('new', 2), chat('old', 120), chat('older', 400), chat('pinned-old', 200)]

describe('ChatCleanup (#452)', () => {
  beforeEach(async () => {
    await applyLanguage('en')
    useUIStore.setState({ pinnedConversationIds: ['pinned-old'] })
    vi.mocked(api.getConversations).mockResolvedValue(chats)
    vi.mocked(api.deleteConversations).mockReset().mockResolvedValue({ deleted: [], skipped: [] })
  })

  it('finds chats last used before the cutoff, leaving pinned ones', () => {
    expect(olderChats(chats, 90, ['pinned-old']).map((c) => c.id)).toEqual(['old', 'older'])
    expect(olderChats(chats, 365, []).map((c) => c.id)).toEqual(['older'])
  })

  it('counts the chats before you confirm, and deletes them in one request', async () => {
    render(
      <QueryClientProvider client={new QueryClient({ defaultOptions: { queries: { retry: false } } })}>
        <ChatCleanup />
      </QueryClientProvider>,
    )
    fireEvent.click(await screen.findByRole('button', { name: 'Delete 2 chats' }))
    expect(screen.getByText('Delete 2 chats last used more than 90 days ago?')).toBeInTheDocument()
    expect(api.deleteConversations).not.toHaveBeenCalled()
    fireEvent.click(screen.getAllByRole('button', { name: 'Delete 2 chats' })[1])
    expect(await screen.findByText('Deleted 2 chats.')).toBeInTheDocument()
    expect(api.deleteConversations).toHaveBeenCalledWith(['old', 'older'])

    fireEvent.change(screen.getByRole('combobox'), { target: { value: '365' } })
    expect(screen.getByRole('button', { name: 'Delete 1 chat' })).toBeInTheDocument()
    expect(screen.getByText('Pinned chats are kept.')).toBeInTheDocument()
  })
})
