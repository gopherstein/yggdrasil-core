import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'
import { api } from '@/lib/api'
import { TopicsTry } from './TopicsTry'

vi.mock('@/lib/api', () => ({ api: { tryTopics: vi.fn() } }))

describe('TopicsTry (#345)', () => {
  it('tries a sample with the draft controls and shows the label and reply', async () => {
    vi.mocked(api.tryTopics).mockResolvedValue({ label: 'off_topic', held: false, replaced: false, reply: 'A poem…' })
    const topics = { stays_on: 'Tires', examples: ['Do you have winter tires?'] }
    render(
      <QueryClientProvider client={new QueryClient()}>
        <TopicsTry profileId="p1" topics={topics} examples={topics.examples} />
      </QueryClientProvider>,
    )
    expect(screen.getByRole('button', { name: 'Do you have winter tires?' })).toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: 'Write me a poem about the sea.' }))
    await waitFor(() => expect(api.tryTopics).toHaveBeenCalledWith('p1', 'Write me a poem about the sea.', topics))
    expect(await screen.findByText('Off topic')).toBeInTheDocument()
    expect(screen.getByText('With Enforce, this would get the set reply')).toBeInTheDocument()
    expect(screen.getByText('A poem…')).toBeInTheDocument()
  })

  it('shows nothing without topic controls', () => {
    const { container } = render(
      <QueryClientProvider client={new QueryClient()}>
        <TopicsTry profileId="p1" examples={[]} />
      </QueryClientProvider>,
    )
    expect(container).toBeEmptyDOMElement()
  })
})
