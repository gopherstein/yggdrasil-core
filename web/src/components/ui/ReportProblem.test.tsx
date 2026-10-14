import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { fireEvent, render, screen } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { describe, expect, it, vi } from 'vitest'
import { ReportProblem } from './ReportProblem'

vi.mock('@/lib/api', () => ({
  api: {
    getReport: vi.fn().mockResolvedValue({ version: '1.9.0', contract: '1.32', os: 'darwin', arch: 'arm64', log: ['t ERROR load failed'] }),
  },
}))

describe('ReportProblem (#588)', () => {
  it('shows everything a report sends, and sends it by email', async () => {
    render(
      <QueryClientProvider client={new QueryClient()}>
        <MemoryRouter initialEntries={['/chat']}>
          <ReportProblem error="model failed for mike@example.com" code="MODEL_LOAD_FAILED" />
        </MemoryRouter>
      </QueryClientProvider>,
    )
    fireEvent.click(screen.getByRole('button', { name: 'Report this' }))
    expect(screen.getByRole('dialog')).toHaveTextContent('info@yeix.io')
    fireEvent.change(screen.getByRole('textbox'), { target: { value: 'asked a question' } })
    expect(await screen.findByText(/Toskar 1\.9\.0, contract 1\.32/)).toHaveTextContent('Error: model failed for <email>')
    expect(screen.getByRole('dialog')).toHaveTextContent('asked a question')
    expect(screen.getByRole('dialog')).toHaveTextContent('t ERROR load failed')
    const send = screen.getByRole('link', { name: 'Send by email' })
    expect(send.getAttribute('href')).toMatch(/^mailto:info@yeix\.io\?subject=Toskar%20problem%3A%20MODEL_LOAD_FAILED&body=/)
  })
})
