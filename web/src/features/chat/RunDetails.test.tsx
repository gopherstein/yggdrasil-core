import { fireEvent, render, screen } from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'
import { api } from '@/lib/api'
import { useUIStore } from '@/stores/uiStore'
import { AnswerDetails } from './AnswerDetails'
import { roleLabel } from './runRoles'

vi.mock('@/lib/api', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@/lib/api')>()),
  api: { getRun: vi.fn() },
}))

describe('Run details', () => {
  it('appear in advanced mode and load the trace when opened', async () => {
    vi.mocked(api.getRun).mockResolvedValue({
      id: 'abcdef123456',
      strategy: ['Looked up the web first'],
      effort: 'Balanced',
      status: 'completed',
      started_at: '2026-10-01T20:00:00Z',
      latency_ms: 4200,
      models: [{ model_id: 'llama-3.2-1b-q4', role: 'assistant', node: 'This Mac', calls: 2, load_ms: 1800, first_token_ms: 2100, prompt_tokens: 900, completion_tokens: 80, cached_tokens: 600, tok_per_sec: 61.2 }],
      tools: [{ tool_id: 'internet.search', calls: 1, total_ms: 900 }],
      nodes: ['This Mac'],
      verification_passes: 1,
      retries: 0,
    })
    const meta = { run_id: 'abcdef123456' }
    const { rerender } = render(<AnswerDetails meta={meta} />)
    expect(screen.queryByText(/Run details/)).not.toBeInTheDocument()

    useUIStore.setState({ advancedMode: true })
    rerender(<AnswerDetails meta={meta} />)
    fireEvent.click(screen.getByRole('button', { name: /Run details/ }))
    expect(await screen.findByText(/llama-3.2-1b-q4 on This Mac · 2 calls · load 1.8 s/)).toBeInTheDocument()
    expect(screen.getByText(/600 cached/)).toBeInTheDocument()
    expect(screen.getByText('internet.search ×1 (900 ms)')).toBeInTheDocument()
    expect(screen.getByText('Looked up the web first')).toBeInTheDocument()
    expect(api.getRun).toHaveBeenCalledWith('abcdef123456')
    useUIStore.setState({ advancedMode: false })
  })

  it('labels each role and counts model calls for a team', async () => {
    vi.mocked(api.getRun).mockResolvedValue({
      id: 'team123456',
      strategy: ['Team'],
      status: 'completed',
      started_at: '2026-10-01T20:00:00Z',
      models: [
        { model_id: 'qwen-7b', role: 'assistant', node: 'This Mac', calls: 1, prompt_tokens: 1, completion_tokens: 1, cached_tokens: 0 },
        { model_id: 'qwen-3b', role: 'worker:2', node: 'Studio', calls: 1, prompt_tokens: 1, completion_tokens: 1, cached_tokens: 0 },
        { model_id: 'qwen-3b', role: 'worker:1', node: 'Laptop', calls: 1, prompt_tokens: 1, completion_tokens: 1, cached_tokens: 0 },
        { model_id: 'qwen-14b', role: 'planner', node: 'This Mac', calls: 1, prompt_tokens: 1, completion_tokens: 1, cached_tokens: 0 },
      ],
      tools: [],
      nodes: ['This Mac', 'Studio', 'Laptop'],
      verification_passes: 0,
      retries: 0,
    })
    useUIStore.setState({ advancedMode: true })
    render(<AnswerDetails meta={{ run_id: 'team123456' }} />)
    fireEvent.click(screen.getByRole('button', { name: /Run details/ }))
    expect(await screen.findByText('Worker 1')).toBeInTheDocument()
    const labels = screen.getAllByText(/^(Planner|Worker \d|Answer)$/).map((el) => el.textContent)
    expect(labels).toEqual(['Planner', 'Worker 1', 'Worker 2', 'Answer'])
    expect(screen.getByText('Model calls').nextSibling?.textContent).toBe('4')
    useUIStore.setState({ advancedMode: false })
  })

  it('names roles', () => {
    expect(roleLabel('assistant')).toBe('Answer')
    expect(roleLabel('worker:3')).toBe('Worker 3')
    expect(roleLabel('reviewer')).toBe('Reviewer')
  })

  it('shows a failed run in the App language, keeping the English text', async () => {
    vi.mocked(api.getRun).mockResolvedValue({
      id: 'failed123456',
      strategy: [],
      status: 'failed',
      error: 'llama-server: failed to allocate buffer',
      error_code: 'OUT_OF_MEMORY',
      started_at: '2026-10-01T20:00:00Z',
      models: [],
      tools: [],
      nodes: [],
      verification_passes: 0,
      retries: 0,
    })
    useUIStore.setState({ advancedMode: true })
    render(<AnswerDetails meta={{ run_id: 'failed123456' }} />)
    fireEvent.click(screen.getByRole('button', { name: /Run details/ }))
    const error = await screen.findByText('The model ran out of memory.')
    expect(error).toHaveAttribute('title', 'llama-server: failed to allocate buffer')
    expect(screen.getByText(/· Failed ·/)).toBeInTheDocument()
    useUIStore.setState({ advancedMode: false })
  })
})
