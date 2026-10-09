import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render, screen } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { describe, expect, it, vi } from 'vitest'
import { api } from '@/lib/api'
import { TopicsModelNote } from './TopicsModelNote'

vi.mock('@/lib/api', () => ({ api: { getCapabilities: vi.fn() } }))

function renderIt() {
  return render(
    <QueryClientProvider client={new QueryClient()}>
      <MemoryRouter>
        <TopicsModelNote />
      </MemoryRouter>
    </QueryClientProvider>,
  )
}

describe('TopicsModelNote (#457)', () => {
  it('says when no installed model is verified to check topics', async () => {
    vi.mocked(api.getCapabilities).mockResolvedValue({ abilities: [{ id: 'topic_checks', label: 'x', available: false }] } as never)
    renderIt()
    expect(await screen.findByRole('link', { name: 'Install it' })).toHaveAttribute('href', '/models')
  })

  it('says nothing when one is', async () => {
    vi.mocked(api.getCapabilities).mockResolvedValue({ abilities: [{ id: 'topic_checks', label: 'x', available: true }] } as never)
    const { container } = renderIt()
    await new Promise((r) => setTimeout(r, 20))
    expect(container).toBeEmptyDOMElement()
  })
})
