import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { MemoryRouter } from 'react-router-dom'
import { api } from '@/lib/api'
import type { Automation, AutomationDetail } from '@/types/api'
import { AutomationsPage } from './AutomationsPage'

vi.mock('@/lib/api', async () => {
  const actual = await vi.importActual<typeof import('@/lib/api')>('@/lib/api')
  return {
    ...actual,
    api: {
      ...actual.api,
      listAutomations: vi.fn(),
      getAutomation: vi.fn(),
      getProfiles: vi.fn(),
      listTools: vi.fn(),
      createAutomation: vi.fn(),
      updateAutomation: vi.fn(),
      pauseAutomation: vi.fn(),
      resumeAutomation: vi.fn(),
      deleteAutomation: vi.fn(),
      runAutomation: vi.fn(),
    },
  }
})

const saved: Automation = {
  id: 'auto-1',
  name: 'Price below $500',
  enabled: true,
  schedule: { kind: 'daily', time_zone: 'UTC', hour: 8, minute: 0 },
  prompt: 'Check the price',
  profile_id: 'general-assistant',
  tools: [],
  notification: { mode: 'condition', condition: { kind: 'threshold', op: 'below', value: 500 } },
  created_at: '2026-09-28T15:00:00Z',
  updated_at: '2026-09-28T15:00:00Z',
  next_run_at: '2026-09-29T08:00:00Z',
  last_run_at: '2026-09-28T08:05:00Z',
  consecutive_failures: 0,
  last_status: 'succeeded',
  last_result: 'price is $420',
}

const detail: AutomationDetail = {
  ...saved,
  history: [
    {
      id: 'run-1',
      automation_id: 'auto-1',
      occurrence_at: '2026-09-28T08:00:00Z',
      status: 'succeeded',
      started_at: '2026-09-28T08:00:05Z',
      finished_at: '2026-09-28T08:05:00Z',
      result: 'price is $420',
      notification_sent: true,
      model_id: 'model-a',
      node_id: 'this-computer',
      attempt: 1,
    },
  ],
}

function renderPage() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  return render(
    <QueryClientProvider client={client}>
      <MemoryRouter>
        <AutomationsPage />
      </MemoryRouter>
    </QueryClientProvider>,
  )
}

describe('AutomationsPage', () => {
  beforeEach(() => {
    vi.mocked(api.listAutomations).mockResolvedValue([saved])
    vi.mocked(api.getAutomation).mockResolvedValue(detail)
    vi.mocked(api.getProfiles).mockResolvedValue([
      { id: 'general-assistant', name: 'General', purpose: 'general', orchestrator_id: 'simple', roles: [], node_policy: { mode: 'automatic' } },
    ])
    vi.mocked(api.listTools).mockResolvedValue([
      {
        id: 'internet.search',
        name: 'Web Search',
        description: 'Search the public internet.',
        capability: 'internet',
        source: 'builtin',
        schema: '{}',
        default_policy: 'allow',
        risk: 'read',
        enabled: true,
        profiles: [],
      },
    ])
    vi.mocked(api.pauseAutomation).mockResolvedValue({ ...saved, enabled: false })
    vi.mocked(api.createAutomation).mockResolvedValue({ ...saved, id: 'auto-2' })
  })

  it('shows the latest result and runs the history actions', async () => {
    renderPage()
    expect(await screen.findByRole('button', { name: /Price below \$500/ })).toBeInTheDocument()
    expect(screen.getByText('price is $420')).toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: /Price below \$500/ }))
    expect(await screen.findByText('History')).toBeInTheDocument()
    expect(screen.getByText('Notified')).toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: 'Pause' }))
    await waitFor(() => expect(api.pauseAutomation).toHaveBeenCalledWith('auto-1'))
  })

  it('fills a structured task from a description and saves it', async () => {
    renderPage()
    fireEvent.click(await screen.findByRole('button', { name: 'New automation' }))
    fireEvent.change(screen.getByPlaceholderText(/Every morning at 8:00 AM/), {
      target: { value: 'Every morning at 8:00 AM, check this product and tell me if the price is below $500.' },
    })
    fireEvent.click(screen.getByRole('button', { name: 'Apply description' }))
    expect(screen.getByDisplayValue('Price below $500')).toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: 'Save' }))
    await waitFor(() => expect(api.createAutomation).toHaveBeenCalled())
    const body = vi.mocked(api.createAutomation).mock.calls[0][0]
    expect(body.schedule).toMatchObject({ kind: 'daily', hour: 8, minute: 0 })
    expect(body.notification).toEqual({ mode: 'condition', condition: { kind: 'threshold', op: 'below', value: 500 } })
    expect(body.prompt).toContain('{"price": 420}')
    expect(body.profile_id).toBe('general-assistant')
  })
})
