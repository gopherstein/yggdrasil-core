import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render, screen } from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'
import { api } from '@/lib/api'
import { RequireRole } from './RequireRole'

vi.mock('@/lib/api', () => ({ api: { getMe: vi.fn() } }))

const person = { id: 'p', name: 'P', created_at: '', sign_in: true }

function renderAs(role: 'visitor' | 'member' | 'admin' | 'owner', min: 'member' | 'admin') {
  vi.mocked(api.getMe).mockResolvedValue({ person: { ...person, role }, via: 'session' })
  return render(
    <QueryClientProvider client={new QueryClient()}>
      <RequireRole min={min}>
        <p>the page</p>
      </RequireRole>
    </QueryClientProvider>,
  )
}

describe('RequireRole (#203)', () => {
  it('shows the page to people with the role', async () => {
    renderAs('admin', 'admin')
    expect(await screen.findByText('the page')).toBeInTheDocument()
  })

  it('tells others whose page it is', async () => {
    renderAs('member', 'admin')
    expect(await screen.findByText('This page is for Admins and the Owner.')).toBeInTheDocument()
    expect(screen.queryByText('the page')).not.toBeInTheDocument()
  })

  it('keeps Members’ pages from Visitors', async () => {
    renderAs('visitor', 'member')
    expect(await screen.findByText('This page is for Members, Admins, and the Owner.')).toBeInTheDocument()
  })
})
